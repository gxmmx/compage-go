package certs

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

var oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

type IssuerManager struct {
	mu                   sync.Mutex
	store                backend
	logger               *slog.Logger
	name, slug           string
	revision             uint64
	definition           IssuerDefinition
	version              *issuerVersion
	issuerCert, rootCert *x509.Certificate
	signer               crypto.Signer
	mode                 IssuanceMode
	now                  func() time.Time
	closed               bool
}

func NewIssuerManager(opts ...Option) (*IssuerManager, error) {
	o, err := parseOptions(scopeIssuerManager, opts)
	if err != nil {
		return nil, err
	}
	if o.store == nil {
		return nil, invalid("WithStore is required", nil)
	}
	if secure, ok := o.store.(securedBackend); ok {
		if err := secure.authorizeIssuer(); err != nil {
			return nil, err
		}
	}
	if !o.issuerNameSet {
		return nil, invalid("WithIssuerName is required", nil)
	}
	name, err := normalizeDisplayName(o.issuerName)
	if err != nil {
		return nil, err
	}
	m := &IssuerManager{store: o.store, logger: o.logger, name: name, slug: slug(name), now: time.Now}
	if err = m.reload(context.Background()); err != nil {
		return nil, err
	}
	return m, nil
}
func (m *IssuerManager) reload(ctx context.Context) error {
	s, err := m.store.load(ctx, true)
	if err != nil {
		return err
	}
	issuer := s.Issuers[m.slug]
	if issuer == nil || issuer.Definition.Name != m.name {
		return &NotFoundError{Resource: "issuer " + m.name}
	}
	v := issuer.Versions[issuer.ActiveVersion]
	key, err := parseKeyDER(v.KeyDER, nil)
	if err != nil {
		return err
	}
	cert, err := parseCertDER(v.CertificateDER)
	if err != nil {
		return err
	}
	root, err := parseCertDER(s.Roots[v.RootGeneration].CertificateDER)
	if err != nil {
		return err
	}
	if !publicKeysEqual(key.Public(), cert.PublicKey) {
		return corrupt("issuer private key mismatch", nil)
	}
	m.revision = s.Revision
	m.definition = issuer.Definition
	m.version = v
	m.issuerCert = cert
	m.rootCert = root
	m.signer = key
	m.mode = s.Spec.IssuanceMode
	return nil
}
func (m *IssuerManager) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return &ConflictError{Message: "issuer manager is closed"}
	}
	return m.reload(ctx)
}

func parseCSRPEM(data []byte) (*x509.CertificateRequest, error) {
	b, rest := pem.Decode(data)
	if b == nil || b.Type != "CERTIFICATE REQUEST" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, invalid("CSR PEM must contain exactly one CERTIFICATE REQUEST block", nil)
	}
	csr, err := x509.ParseCertificateRequest(b.Bytes)
	if err != nil {
		return nil, invalid("invalid CSR", err)
	}
	if err = csr.CheckSignature(); err != nil {
		return nil, invalid("CSR signature verification failed", err)
	}
	if err = supportedPublicKey(csr.PublicKey); err != nil {
		return nil, err
	}
	for _, extension := range csr.Extensions {
		if !extension.Id.Equal(oidSubjectAltName) {
			return nil, invalid("CSR requests an unsupported extension", nil)
		}
	}
	return csr, nil
}

func strictCSRIdentity(csr *x509.CertificateRequest) (Identity, error) {
	n := csr.Subject
	if len(n.Organization) > 1 || len(n.OrganizationalUnit) > 1 || len(n.Country) > 1 || len(n.Province) > 1 || len(n.Locality) > 1 || len(n.StreetAddress) > 0 || len(n.PostalCode) > 0 || n.SerialNumber != "" || len(n.Names) > len(n.Organization)+len(n.OrganizationalUnit)+len(n.Country)+len(n.Province)+len(n.Locality)+boolInt(n.CommonName != "") {
		return Identity{}, invalid("CSR subject contains unsupported or repeated attributes", nil)
	}
	return normalizeIdentity(identityFromName(n))
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func csrSANs(csr *x509.CertificateRequest) SANs {
	return SANs{DNSNames: append([]string(nil), csr.DNSNames...), IPAddresses: append([]net.IP(nil), csr.IPAddresses...), URIs: append([]*url.URL(nil), csr.URIs...), EmailAddresses: append([]string(nil), csr.EmailAddresses...)}
}
func mergeSANs(a, b SANs) (SANs, error) {
	out := a.clone()
	out.DNSNames = append(out.DNSNames, b.DNSNames...)
	out.IPAddresses = append(out.IPAddresses, b.IPAddresses...)
	out.URIs = append(out.URIs, b.URIs...)
	out.EmailAddresses = append(out.EmailAddresses, b.EmailAddresses...)
	return dedupeSANs(out)
}
func dedupeSANs(s SANs) (SANs, error) { n, err := normalizeSANsAllowDuplicates(s); return n, err }
func normalizeSANsAllowDuplicates(s SANs) (SANs, error) {
	out := SANs{}
	seen := map[string]bool{}
	add := func(k string) bool {
		if seen[k] {
			return false
		}
		seen[k] = true
		return true
	}
	for _, d := range s.DNSNames {
		one, err := normalizeSANs(SANs{DNSNames: []string{d}})
		if err != nil {
			return SANs{}, err
		}
		if add("d:" + one.DNSNames[0]) {
			out.DNSNames = append(out.DNSNames, one.DNSNames[0])
		}
	}
	for _, ip := range s.IPAddresses {
		one, err := normalizeSANs(SANs{IPAddresses: []net.IP{ip}})
		if err != nil {
			return SANs{}, err
		}
		if add("i:" + one.IPAddresses[0].String()) {
			out.IPAddresses = append(out.IPAddresses, one.IPAddresses[0])
		}
	}
	for _, u := range s.URIs {
		one, err := normalizeSANs(SANs{URIs: []*url.URL{u}})
		if err != nil {
			return SANs{}, err
		}
		if add("u:" + one.URIs[0].String()) {
			out.URIs = append(out.URIs, one.URIs[0])
		}
	}
	for _, e := range s.EmailAddresses {
		one, err := normalizeSANs(SANs{EmailAddresses: []string{e}})
		if err != nil {
			return SANs{}, err
		}
		if add("e:" + strings.ToLower(one.EmailAddresses[0])) {
			out.EmailAddresses = append(out.EmailAddresses, one.EmailAddresses[0])
		}
	}
	return out, nil
}

func (m *IssuerManager) SignCSR(ctx context.Context, csrPEM []byte, opts ...Option) (*CertificateBundle, error) {
	o, err := parseOptions(scopeSign, opts)
	if err != nil {
		return nil, err
	}
	if !o.profileSet || !o.profile.valid() {
		return nil, invalid("an explicit supported profile is required", nil)
	}
	csr, err := parseCSRPEM(csrPEM)
	if err != nil {
		return nil, err
	}
	subject, err := strictCSRIdentity(csr)
	if err != nil {
		return nil, err
	}
	sans, err := normalizeSANsAllowDuplicates(csrSANs(csr))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "issuer manager is closed"}
	}
	if m.mode == Ledger {
		s, e := m.store.load(ctx, true)
		if e != nil {
			return nil, e
		}
		if s.Revision != m.revision {
			if e = m.reload(ctx); e != nil {
				return nil, e
			}
		}
		if e = m.validateLedger(ctx, s); e != nil {
			return nil, e
		}
	}
	if err = m.validateRequest(subject, sans); err != nil {
		return nil, err
	}
	if o.subjectSet {
		subject, err = normalizeIdentity(o.subject)
		if err != nil {
			return nil, err
		}
	}
	if o.sansSet {
		sans, err = normalizeSANs(o.sans)
		if err != nil {
			return nil, err
		}
	}
	if o.additionalSANsSet {
		sans, err = mergeSANs(sans, o.additionalSANs)
		if err != nil {
			return nil, err
		}
	}
	if err = m.validateRequest(subject, sans); err != nil {
		return nil, err
	}
	if !containsProfile(m.definition.AllowedProfiles, o.profile) {
		return nil, &ForbiddenError{Message: "issuer does not permit requested profile"}
	}
	lifetime := 90 * 24 * time.Hour
	if o.lifetimeSet {
		lifetime = o.lifetime
	}
	if lifetime <= 0 || lifetime > maximumLeafValidity {
		return nil, invalid("leaf validity must be positive and no more than one year", nil)
	}
	now := m.now().UTC().Truncate(time.Second)
	notAfter := now.Add(lifetime)
	if notAfter.After(m.issuerCert.NotAfter.Add(-clockSkew)) {
		return nil, invalid("leaf validity exceeds issuer remaining lifetime", nil)
	}
	usage, extUsage := o.profile.usages()
	ski, err := subjectKeyID(csr.PublicKey)
	if err != nil {
		return nil, err
	}
	for attempts := 0; attempts < 8; attempts++ {
		serial, e := randomSerial()
		if e != nil {
			return nil, e
		}
		if m.mode == Ledger {
			_, e = m.store.lookupLedger(ctx, m.slug, m.version.Version, serial.Text(16))
			if e == nil {
				continue
			}
			var nf *NotFoundError
			if !errors.As(e, &nf) {
				return nil, e
			}
		}
		t := &x509.Certificate{SerialNumber: serial, Subject: identityName(subject), Issuer: m.issuerCert.Subject, NotBefore: now.Add(-clockSkew), NotAfter: notAfter, KeyUsage: usage, ExtKeyUsage: extUsage, BasicConstraintsValid: true, IsCA: false, SubjectKeyId: ski, AuthorityKeyId: m.issuerCert.SubjectKeyId, DNSNames: sans.DNSNames, IPAddresses: sans.IPAddresses, URIs: sans.URIs, EmailAddresses: sans.EmailAddresses}
		der, e := x509.CreateCertificate(rand.Reader, t, m.issuerCert, csr.PublicKey, m.signer)
		if e != nil {
			return nil, fmt.Errorf("certs: sign CSR: %w", e)
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			return nil, corrupt("generated leaf certificate", e)
		}
		if m.mode == Ledger {
			record := LedgerRecord{AuthorityRevision: m.revision, Fingerprint: fingerprint(cert), Issuer: m.slug, IssuerVersion: m.version.Version, Serial: serial.Text(16), CertificateDER: der, Subject: subject, SANs: sans, Profile: o.profile, IssuedAt: now, ExpiresAt: notAfter}
			if e = m.store.appendLedger(ctx, record); e != nil {
				var conflict *ConflictError
				if errors.As(e, &conflict) {
					state, loadErr := m.store.load(ctx, true)
					if loadErr != nil {
						return nil, loadErr
					}
					if state.Revision != m.revision {
						if loadErr = m.reload(ctx); loadErr != nil {
							return nil, loadErr
						}
						if notAfter.After(m.issuerCert.NotAfter.Add(-clockSkew)) {
							return nil, invalid("leaf validity exceeds reloaded issuer remaining lifetime", nil)
						}
					}
					continue
				}
				return nil, e
			}
		}
		bundle, bundleErr := normalizeBundle([]*x509.Certificate{cert, m.issuerCert, m.rootCert}, nil)
		if bundleErr != nil {
			return nil, bundleErr
		}
		debugLog(m.logger, "leaf certificate issued", "issuer", m.name, "version", m.version.Version, "subject_common_name", subject.CommonName, "profile", o.profile)
		return bundle, nil
	}
	return nil, &ConflictError{Message: "could not allocate a unique certificate serial"}
}

func containsProfile(v []Profile, p Profile) bool {
	for _, x := range v {
		if x == p {
			return true
		}
	}
	return false
}
func (m *IssuerManager) validateRequest(subject Identity, sans SANs) error {
	d := m.definition
	if d.SubjectOwner != "" && subject.Organization != d.SubjectOwner {
		return &ForbiddenError{Message: "subject organization is not permitted"}
	}
	required := d.RequiredSubject
	pairs := [][2]string{{required.Organization, subject.Organization}, {required.OrganizationalUnit, subject.OrganizationalUnit}, {required.Country, subject.Country}, {required.Province, subject.Province}, {required.Locality, subject.Locality}, {required.CommonName, subject.CommonName}}
	for _, p := range pairs {
		if p[0] != "" && p[0] != p[1] {
			return &ForbiddenError{Message: "subject does not satisfy required fields"}
		}
	}
	for _, name := range sans.DNSNames {
		if len(d.SANPolicy.DNSSuffixes) > 0 && !matchesDNSSuffix(name, d.SANPolicy.DNSSuffixes) {
			return &ForbiddenError{Message: "DNS SAN is not permitted"}
		}
	}
	for _, ip := range sans.IPAddresses {
		if len(d.SANPolicy.IPRanges) > 0 {
			ok := false
			for _, network := range d.SANPolicy.IPRanges {
				if network != nil && network.Contains(ip) {
					ok = true
					break
				}
			}
			if !ok {
				return &ForbiddenError{Message: "IP SAN is not permitted"}
			}
		}
	}
	for _, u := range sans.URIs {
		if len(d.SANPolicy.URIPrefixes) > 0 {
			ok := false
			for _, prefix := range d.SANPolicy.URIPrefixes {
				if strings.HasPrefix(u.String(), prefix) {
					ok = true
					break
				}
			}
			if !ok {
				return &ForbiddenError{Message: "URI SAN is not permitted"}
			}
		}
	}
	for _, email := range sans.EmailAddresses {
		if len(d.SANPolicy.EmailDomains) > 0 {
			domain := email[strings.LastIndexByte(email, '@')+1:]
			ok := false
			for _, allowed := range d.SANPolicy.EmailDomains {
				if strings.EqualFold(domain, allowed) {
					ok = true
					break
				}
			}
			if !ok {
				return &ForbiddenError{Message: "email SAN is not permitted"}
			}
		}
	}
	return nil
}
func matchesDNSSuffix(name string, suffixes []string) bool {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	for _, suffix := range suffixes {
		suffix = strings.ToLower(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(suffix), "."), "."))
		if name == suffix || strings.HasSuffix(name, "."+suffix) {
			return true
		}
	}
	return false
}
func (m *IssuerManager) validateLedger(ctx context.Context, s *authorityState) error {
	cursor := ""
	var all []LedgerRecord
	for {
		records, next, err := m.store.listLedger(ctx, cursor, 500)
		if err != nil {
			return err
		}
		all = append(all, records...)
		if next == "" {
			return validateLedgerRecords(s, all)
		}
		cursor = next
	}
}
func (m *IssuerManager) LookupCertificate(ctx context.Context, version int, serial string) (*LedgerRecord, error) {
	if m.mode != Ledger {
		return nil, &UnavailableError{Message: "issuance ledger is disabled"}
	}
	return m.store.lookupLedger(ctx, m.slug, version, strings.ToLower(serial))
}
func (m *IssuerManager) ListCertificates(ctx context.Context, cursor string, limit int) ([]LedgerRecord, string, error) {
	if m.mode != Ledger {
		return nil, "", &UnavailableError{Message: "issuance ledger is disabled"}
	}
	return m.store.listLedger(ctx, cursor, limit)
}
func (m *IssuerManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signer = nil
	m.closed = true
	return nil
}
