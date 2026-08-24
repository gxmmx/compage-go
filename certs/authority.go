package certs

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const clockSkew = 5 * time.Minute
const defaultRotateBefore = 20 * 24 * time.Hour
const maximumLeafValidity = 365 * 24 * time.Hour

type AuthorityManager struct {
	mu        sync.Mutex
	store     backend
	logger    *slog.Logger
	config    optionValues
	generated map[int][]byte
	closed    bool
	now       func() time.Time
}

func NewAuthorityManager(opts ...Option) (*AuthorityManager, error) {
	o, err := parseOptions(scopeAuthority, opts)
	if err != nil {
		return nil, err
	}
	if o.store == nil {
		return nil, invalid("WithStore is required", nil)
	}
	if secure, ok := o.store.(securedBackend); ok {
		if err := secure.authorizeOwner(); err != nil {
			return nil, err
		}
	}
	owner, group := o.store.principal()
	if !o.ownerSet {
		o.owner = owner
	} else if o.owner != owner {
		return nil, invalid("manager owner differs from store owner", nil)
	}
	if !o.groupSet {
		o.group = group
	} else if o.group != group {
		return nil, invalid("manager group differs from store group", nil)
	}
	if o.rootKeySpecSet && !o.rootKeySpec.valid() {
		return nil, invalid("unsupported root key specification", nil)
	}
	if o.issuanceModeSet && !o.issuanceMode.valid() {
		return nil, invalid("unsupported issuance mode", nil)
	}
	if o.identitySet {
		o.identity, err = normalizeIdentity(o.identity)
		if err != nil {
			return nil, err
		}
	}
	if o.authorityNameSet {
		o.authorityName, err = normalizeDisplayName(o.authorityName)
		if err != nil {
			return nil, err
		}
	}
	if !o.authorityNameSet && o.identitySet && o.identity.Organization != "" {
		o.authorityName, err = normalizeDisplayName(o.identity.Organization)
		if err != nil {
			return nil, invalid("identity organization requires an explicit valid authority name", err)
		}
		o.authorityNameSet = true
	}
	seen := map[string]string{}
	for i, d := range o.issuers {
		n, e := normalizeDisplayName(d.Name)
		if e != nil {
			return nil, e
		}
		fixed, e := validateDefinition(d, n)
		if e != nil {
			return nil, e
		}
		o.issuers[i] = fixed
		if prior, ok := seen[fixed.Slug]; ok {
			return nil, &ConflictError{Message: fmt.Sprintf("issuer names %q and %q have the same storage identifier", prior, fixed.Name)}
		}
		seen[fixed.Slug] = fixed.Name
	}
	if o.rootValiditySet && o.rootValidity <= 0 {
		return nil, invalid("root validity must be positive", nil)
	}
	// Distinguish a new authority so missing identity fails at construction.
	if _, loadErr := o.store.load(context.Background(), false); loadErr != nil {
		var nf *NotFoundError
		if errors.As(loadErr, &nf) && !o.authorityNameSet {
			return nil, invalid("WithAuthorityName or WithIdentity is required for a new authority", nil)
		}
		if !errors.As(loadErr, &nf) {
			return nil, loadErr
		}
	}
	return &AuthorityManager{store: o.store, logger: o.logger, config: o, generated: map[int][]byte{}, now: time.Now}, nil
}

func validateDefinition(d IssuerDefinition, name string) (IssuerDefinition, error) {
	if d.Name == "" {
		return IssuerDefinition{}, invalid("issuer name is required", nil)
	}
	if d.Slug != "" && d.Slug != slug(name) {
		return IssuerDefinition{}, invalid("issuer slug does not match its name", nil)
	}
	o := optionValues{}
	o.issuerValiditySet = true
	o.issuerValidity = d.Validity
	if d.Validity == 0 {
		o.issuerValidity = 2 * 365 * 24 * time.Hour
	}
	o.profilesSet = true
	o.profiles = d.AllowedProfiles
	if len(o.profiles) == 0 {
		o.profiles = []Profile{TLSClient, TLSServer, TLSClientServer}
	}
	o.subjectOwner = d.SubjectOwner
	o.requiredSubject = d.RequiredSubject
	o.requiredSubjectSet = d.RequiredSubject != (Identity{})
	o.sanPolicy = d.SANPolicy
	return finishIssuerDefinition(name, o)
}
func slug(v string) string                     { return strings.ToLower(strings.ReplaceAll(v, " ", "-")) }
func rootCommonName(name string, g int) string { return fmt.Sprintf("%s G%d Root CA", name, g) }
func issuerCommonName(authority string, g int, name string, v int) string {
	return fmt.Sprintf("%s G%d %s Issuer V%d", authority, g, name, v)
}
func identityFromCAName(n pkix.Name) Identity {
	return Identity{Organization: first(n.Organization), OrganizationalUnit: first(n.OrganizationalUnit), Country: first(n.Country), Province: first(n.Province), Locality: first(n.Locality)}
}
func caName(identity Identity, cn string) pkix.Name {
	n := identityName(identity)
	n.CommonName = cn
	return n
}

func (m *AuthorityManager) Ensure(ctx context.Context, opts ...Option) (EnsureResult, error) {
	o, err := parseOptions(scopeEnsure, opts)
	if err != nil {
		return EnsureResult{}, err
	}
	if o.rootRotateBeforeSet && o.rootRotateBefore < 0 {
		return EnsureResult{}, invalid("root rotate-before cannot be negative", nil)
	}
	for name, duration := range o.issuerRotateBefore {
		if duration < 0 {
			return EnsureResult{}, invalid("issuer rotate-before cannot be negative", nil)
		}
		normalized, normalizeErr := normalizeDisplayName(name)
		if normalizeErr != nil || normalized != name {
			return EnsureResult{}, invalid("issuer rotation name must be normalized", normalizeErr)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return EnsureResult{}, &ConflictError{Message: "authority manager is closed"}
	}
	var result EnsureResult
	err = m.store.update(ctx, func(state *authorityState) error {
		if state.Spec.ID == "" {
			created, createErr := m.provision(state)
			if createErr != nil {
				return createErr
			}
			result = created
			return nil
		}
		if assertErr := m.assertSpec(state); assertErr != nil {
			return assertErr
		}
		if state.Pending != nil {
			return &ConflictError{Message: "a pending operation must be promoted or discarded"}
		}
		if _, signerErr := m.activeRootSigner(ctx, state); signerErr != nil {
			return signerErr
		}
		changed := false
		for _, definition := range m.config.issuers {
			if existing := state.Issuers[definition.Slug]; existing != nil {
				if !definitionsEqual(existing.Definition, definition) {
					return &DriftError{Field: "issuer " + definition.Name, Expected: existing.Definition, Actual: definition}
				}
				continue
			}
			for _, existing := range state.Issuers {
				if existing.Definition.Slug == definition.Slug && existing.Definition.Name != definition.Name {
					return &ConflictError{Message: "issuer slug collision"}
				}
			}
			signer, signerErr := m.activeRootSigner(ctx, state)
			if signerErr != nil {
				return signerErr
			}
			version, createErr := makeIssuer(state.Spec, state.ActiveRoot, 1, definition, state.Roots[state.ActiveRoot], signer, m.now())
			if createErr != nil {
				return createErr
			}
			state.Issuers[definition.Slug] = &issuerRecord{Definition: definition, ActiveVersion: 1, Versions: map[int]*issuerVersion{1: version}}
			result.Issuers = append(result.Issuers, definition.Name)
			changed = true
		}
		now := m.now()
		rootBefore := defaultRotateBefore
		if o.rootRotateBeforeSet {
			rootBefore = o.rootRotateBefore
		}
		activeRoot, _ := parseCertDER(state.Roots[state.ActiveRoot].CertificateDER)
		if (o.rootRotateBeforeSet && rootBefore == 0) || !activeRoot.NotAfter.After(now.Add(rootBefore)) {
			if prepareErr := m.prepareRoot(state, now); prepareErr != nil {
				return prepareErr
			}
			result.Pending = true
			result.RootGeneration = state.Pending.RootGeneration
			return nil
		}
		due := map[string]bool{}
		for issuerSlug, issuer := range state.Issuers {
			before, explicit := defaultRotateBefore, false
			if value, ok := o.issuerRotateBefore[issuer.Definition.Name]; ok {
				before, explicit = value, true
			}
			cert, _ := parseCertDER(issuer.Versions[issuer.ActiveVersion].CertificateDER)
			if (explicit && before == 0) || !cert.NotAfter.After(now.Add(before)) {
				due[issuerSlug] = true
			}
		}
		if len(due) > 0 {
			if prepareErr := m.prepareIssuers(ctx, state, due, now); prepareErr != nil {
				return prepareErr
			}
			result.Pending = true
			changed = true
		}
		result.RootGeneration = state.ActiveRoot
		if !changed {
			return errNoMutation
		}
		return nil
	})
	if err != nil {
		return EnsureResult{}, err
	}
	state, err := m.store.load(ctx, false)
	if err == nil {
		result.Revision = state.Revision
		if result.RootGeneration == 0 {
			result.RootGeneration = state.ActiveRoot
		}
		m.logEnsureChanges(result, state)
	}
	return result, err
}

func (m *AuthorityManager) provision(state *authorityState) (EnsureResult, error) {
	name := m.config.authorityName
	if name == "" {
		return EnsureResult{}, invalid("authority name is required", nil)
	}
	identity := m.config.identity
	spec := m.config.rootKeySpec
	if !m.config.rootKeySpecSet {
		spec = ECDSAP256
	}
	mode := m.config.issuanceMode
	if !m.config.issuanceModeSet {
		mode = None
	}
	validity := m.config.rootValidity
	if !m.config.rootValiditySet {
		validity = 10 * 365 * 24 * time.Hour
	}
	id, err := randomID()
	if err != nil {
		return EnsureResult{}, err
	}
	state.Spec = authoritySpec{ID: id, FormatVersion: stateFormatVersion, Name: name, Identity: identity, RootValidity: validity, KeySpec: spec, EncryptedRootKeys: true, IssuanceMode: mode, Owner: m.config.owner, Group: m.config.group, Backend: m.store.kind()}
	state.Roots = map[int]*rootRecord{}
	state.Issuers = map[string]*issuerRecord{}
	state.ActiveRoot = 1
	root, key, secret, err := makeRoot(state.Spec, 1, m.now())
	if err != nil {
		return EnsureResult{}, err
	}
	state.Roots[1] = root
	m.generated[1] = secret
	for _, d := range m.config.issuers {
		v, e := makeIssuer(state.Spec, 1, 1, d, root, key, m.now())
		if e != nil {
			return EnsureResult{}, e
		}
		state.Issuers[d.Slug] = &issuerRecord{Definition: d, ActiveVersion: 1, Versions: map[int]*issuerVersion{1: v}}
	}
	return EnsureResult{Created: true, RootGeneration: 1}, nil
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b), nil
}
func (m *AuthorityManager) assertSpec(s *authorityState) error {
	owner, group := m.store.principal()
	if s.Spec.Backend != m.store.kind() {
		return &DriftError{Field: "backend", Expected: s.Spec.Backend, Actual: m.store.kind()}
	}
	if s.Spec.Owner != owner || s.Spec.Group != group {
		return &DriftError{Field: "store principal", Expected: s.Spec.Owner + ":" + s.Spec.Group, Actual: owner + ":" + group}
	}
	checks := []struct {
		set       bool
		field     string
		want, got any
	}{{m.config.authorityNameSet, "authority name", m.config.authorityName, s.Spec.Name}, {m.config.identitySet, "identity", m.config.identity, s.Spec.Identity}, {m.config.rootKeySpecSet, "root key specification", m.config.rootKeySpec, s.Spec.KeySpec}, {m.config.issuanceModeSet, "issuance mode", m.config.issuanceMode, s.Spec.IssuanceMode}, {m.config.rootValiditySet, "root validity", m.config.rootValidity, s.Spec.RootValidity}}
	for _, c := range checks {
		if c.set && fmt.Sprint(c.want) != fmt.Sprint(c.got) {
			return &DriftError{Field: c.field, Expected: c.got, Actual: c.want}
		}
	}
	return nil
}
func definitionsEqual(a, b IssuerDefinition) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(aa, bb)
}

func makeRoot(spec authoritySpec, generation int, now time.Time) (*rootRecord, crypto.Signer, []byte, error) {
	key, err := generateSigner(spec.KeySpec)
	if err != nil {
		return nil, nil, nil, err
	}
	ski, err := subjectKeyID(key.Public())
	if err != nil {
		return nil, nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	t := &x509.Certificate{SerialNumber: serial, Subject: caName(spec.Identity, rootCommonName(spec.Name, generation)), NotBefore: now.Add(-clockSkew), NotAfter: now.Add(spec.RootValidity), IsCA: true, BasicConstraintsValid: true, MaxPathLen: 1, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SubjectKeyId: ski, AuthorityKeyId: ski}
	der, err := x509.CreateCertificate(rand.Reader, t, t, key.Public(), key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return nil, nil, nil, err
	}
	pemKey, err := marshalKeyPEM(key, secret)
	if err != nil {
		return nil, nil, nil, err
	}
	block, _ := pem.Decode(pemKey)
	return &rootRecord{Generation: generation, CertificateDER: der, EncryptedKeyDER: block.Bytes, Fingerprint: fingerprint(cert)}, key, secret, nil
}
func makeIssuer(spec authoritySpec, generation, version int, d IssuerDefinition, root *rootRecord, rootKey crypto.Signer, now time.Time) (*issuerVersion, error) {
	rc, err := parseCertDER(root.CertificateDER)
	if err != nil {
		return nil, err
	}
	notAfter := now.Add(d.Validity)
	if notAfter.After(rc.NotAfter.Add(-clockSkew)) {
		return nil, invalid("issuer validity does not fit within root validity", nil)
	}
	key, err := generateSigner(spec.KeySpec)
	if err != nil {
		return nil, err
	}
	ski, err := subjectKeyID(key.Public())
	if err != nil {
		return nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	t := &x509.Certificate{SerialNumber: serial, Subject: caName(spec.Identity, issuerCommonName(spec.Name, generation, d.Name, version)), Issuer: rc.Subject, NotBefore: now.Add(-clockSkew), NotAfter: notAfter, IsCA: true, BasicConstraintsValid: true, MaxPathLen: 0, MaxPathLenZero: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SubjectKeyId: ski, AuthorityKeyId: rc.SubjectKeyId}
	der, err := x509.CreateCertificate(rand.Reader, t, rc, key.Public(), rootKey)
	if err != nil {
		return nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	keyDER, err := marshalKeyDER(key)
	if err != nil {
		return nil, err
	}
	return &issuerVersion{Version: version, RootGeneration: generation, CertificateDER: der, KeyDER: keyDER, Fingerprint: fingerprint(cert)}, nil
}

func (m *AuthorityManager) activeRootSigner(ctx context.Context, s *authorityState) (crypto.Signer, error) {
	r := s.Roots[s.ActiveRoot]
	secret := m.config.rootUnlock
	if v := m.generated[s.ActiveRoot]; len(v) > 0 {
		secret = v
	}
	if len(secret) == 0 {
		unlocks, err := m.store.loadUnlocks(ctx)
		if err == nil {
			secret = unlocks[s.ActiveRoot]
		} else {
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
		}
	}
	if len(secret) == 0 {
		return nil, &NotFoundError{Resource: fmt.Sprintf("unlock secret for root G%d", s.ActiveRoot)}
	}
	signer, err := parseKeyDER(r.EncryptedKeyDER, secret)
	if err != nil {
		return nil, invalid("root unlock secret is incorrect", nil)
	}
	cert, err := parseCertDER(r.CertificateDER)
	if err != nil {
		return nil, err
	}
	if !publicKeysEqual(signer.Public(), cert.PublicKey) {
		return nil, corrupt("root key does not match certificate", nil)
	}
	return signer, nil
}
func (m *AuthorityManager) prepareRoot(s *authorityState, now time.Time) error {
	g := s.ActiveRoot + 1
	for s.Roots[g] != nil {
		g++
	}
	r, key, secret, err := makeRoot(s.Spec, g, now)
	if err != nil {
		return err
	}
	s.Roots[g] = r
	m.generated[g] = secret
	pending := &pendingManifest{Kind: "root", ExpectedRevision: s.Revision, RootGeneration: g, Issuers: map[string]int{}, CreatedAt: now}
	for slug, issuer := range s.Issuers {
		v := issuer.ActiveVersion + 1
		record, err := makeIssuer(s.Spec, g, v, issuer.Definition, r, key, now)
		if err != nil {
			return err
		}
		issuer.Versions[v] = record
		pending.Issuers[slug] = v
	}
	s.Pending = pending
	return nil
}
func (m *AuthorityManager) prepareIssuers(ctx context.Context, s *authorityState, due map[string]bool, now time.Time) error {
	key, err := m.activeRootSigner(ctx, s)
	if err != nil {
		return err
	}
	pending := &pendingManifest{Kind: "issuer", ExpectedRevision: s.Revision, Issuers: map[string]int{}, CreatedAt: now}
	for slug := range due {
		issuer := s.Issuers[slug]
		v := issuer.ActiveVersion + 1
		record, err := makeIssuer(s.Spec, s.ActiveRoot, v, issuer.Definition, s.Roots[s.ActiveRoot], key, now)
		if err != nil {
			return err
		}
		issuer.Versions[v] = record
		pending.Issuers[slug] = v
	}
	s.Pending = pending
	return nil
}

func (m *AuthorityManager) logEnsureChanges(result EnsureResult, state *authorityState) {
	if result.Created {
		debugLog(m.logger, "key created", "key_type", state.Spec.KeySpec, "purpose", "root", "generation", state.ActiveRoot)
		debugLog(m.logger, "root certificate created", "generation", state.ActiveRoot, "pending", false)
		for _, issuer := range state.Issuers {
			debugLog(m.logger, "key created", "key_type", state.Spec.KeySpec, "purpose", "issuer", "issuer", issuer.Definition.Name, "generation", state.ActiveRoot, "version", issuer.ActiveVersion)
			debugLog(m.logger, "issuer certificate created", "issuer", issuer.Definition.Name, "generation", state.ActiveRoot, "version", issuer.ActiveVersion, "pending", false)
		}
		return
	}
	if result.Pending && state.Pending != nil {
		pending := state.Pending
		if pending.Kind == "root" {
			debugLog(m.logger, "key created", "key_type", state.Spec.KeySpec, "purpose", "root", "generation", pending.RootGeneration)
			debugLog(m.logger, "root certificate created", "generation", pending.RootGeneration, "pending", true)
		}
		for slug, version := range pending.Issuers {
			issuer := state.Issuers[slug]
			if issuer == nil || issuer.Versions[version] == nil {
				continue
			}
			record := issuer.Versions[version]
			debugLog(m.logger, "key created", "key_type", state.Spec.KeySpec, "purpose", "issuer", "issuer", issuer.Definition.Name, "generation", record.RootGeneration, "version", version)
			debugLog(m.logger, "issuer certificate created", "issuer", issuer.Definition.Name, "generation", record.RootGeneration, "version", version, "pending", true)
		}
		return
	}
	for _, name := range result.Issuers {
		for _, issuer := range state.Issuers {
			if issuer.Definition.Name != name {
				continue
			}
			record := issuer.Versions[issuer.ActiveVersion]
			debugLog(m.logger, "key created", "key_type", state.Spec.KeySpec, "purpose", "issuer", "issuer", name, "generation", record.RootGeneration, "version", record.Version)
			debugLog(m.logger, "issuer certificate created", "issuer", name, "generation", record.RootGeneration, "version", record.Version, "pending", false)
		}
	}
}
