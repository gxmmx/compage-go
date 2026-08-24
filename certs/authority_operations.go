package certs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (m *AuthorityManager) PromotePending(ctx context.Context, opts ...Option) (PromotionResult, error) {
	o, err := parseOptions(scopePromote, opts)
	if err != nil {
		return PromotionResult{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return PromotionResult{}, &ConflictError{Message: "authority manager is closed"}
	}
	var result PromotionResult
	var promotedSecret []byte
	var promotedRootGeneration int
	var promotedIssuers []struct {
		name    string
		version int
	}
	defer func() { zero(promotedSecret) }()
	err = m.store.update(ctx, func(s *authorityState) error {
		if err := m.assertSpec(s); err != nil {
			return err
		}
		if s.Pending == nil {
			return &NotFoundError{Resource: "pending operation"}
		}
		p := s.Pending
		if p.Kind == "root" {
			promotedRootGeneration = p.RootGeneration
		}
		for slug, v := range p.Issuers {
			if issuer := s.Issuers[slug]; issuer != nil {
				promotedIssuers = append(promotedIssuers, struct {
					name    string
					version int
				}{name: issuer.Definition.Name, version: v})
			}
		}
		if p.Kind == "root" {
			r := s.Roots[p.RootGeneration]
			secret := o.rootUnlock
			if len(secret) == 0 {
				secret = m.generated[p.RootGeneration]
			}
			promotedSecret = append([]byte(nil), secret...)
			if len(secret) == 0 {
				unlocks, e := m.store.loadUnlocks(ctx)
				if e == nil {
					secret = unlocks[p.RootGeneration]
				} else {
					var nf *NotFoundError
					if !errors.As(e, &nf) {
						return e
					}
				}
			}
			if len(secret) == 0 {
				return &NotFoundError{Resource: fmt.Sprintf("unlock secret for pending root G%d", p.RootGeneration)}
			}
			key, e := parseKeyDER(r.EncryptedKeyDER, secret)
			if e != nil {
				return invalid("pending root unlock secret is incorrect", nil)
			}
			cert, _ := parseCertDER(r.CertificateDER)
			if !publicKeysEqual(key.Public(), cert.PublicKey) {
				return corrupt("pending root key mismatch", nil)
			}
		}
		for slug, v := range p.Issuers {
			record := s.Issuers[slug].Versions[v]
			key, e := parseKeyDER(record.KeyDER, nil)
			if e != nil {
				return e
			}
			cert, _ := parseCertDER(record.CertificateDER)
			if !publicKeysEqual(key.Public(), cert.PublicKey) {
				return corrupt("pending issuer key mismatch", nil)
			}
		}
		now := m.now()
		if p.Kind == "root" {
			old := s.ActiveRoot
			s.ActiveRoot = p.RootGeneration
			s.Roots[old].EncryptedKeyDER = nil
			deadline := now.Add(maximumLeafValidity + clockSkew)
			if s.Spec.IssuanceMode == Ledger {
				ledgerDeadline, e := m.ledgerTrustDeadline(ctx, s, old)
				if e != nil {
					return e
				}
				deadline = ledgerDeadline
				if deadline.IsZero() {
					deadline = now
				}
			}
			s.Roots[old].RetiredTrustUntil = deadline
		}
		for slug, v := range p.Issuers {
			issuer := s.Issuers[slug]
			old := issuer.ActiveVersion
			issuer.ActiveVersion = v
			if oldRecord := issuer.Versions[old]; oldRecord != nil {
				oldRecord.KeyDER = nil
			}
		}
		s.Pending = nil
		result.RootGeneration = s.ActiveRoot
		return nil
	})
	if err != nil {
		return PromotionResult{}, err
	}
	if promotedRootGeneration > 0 {
		debugLog(m.logger, "root certificate promoted", "generation", promotedRootGeneration)
	}
	for _, issuer := range promotedIssuers {
		debugLog(m.logger, "issuer certificate promoted", "issuer", issuer.name, "version", issuer.version, "generation", result.RootGeneration)
	}
	s, err := m.store.load(ctx, false)
	if err != nil {
		return PromotionResult{}, err
	}
	result.Revision = s.Revision
	if unlocks, e := m.store.loadUnlocks(ctx); e == nil {
		keep := map[int][]byte{}
		if v := unlocks[s.ActiveRoot]; len(v) > 0 {
			keep[s.ActiveRoot] = v
		}
		if v := m.generated[s.ActiveRoot]; len(v) > 0 {
			keep[s.ActiveRoot] = append([]byte(nil), v...)
		}
		if len(promotedSecret) > 0 {
			keep[s.ActiveRoot] = append([]byte(nil), promotedSecret...)
		}
		if len(keep) > 0 {
			if e = m.store.storeUnlocks(ctx, keep, true); e != nil {
				return result, &PartialOperationError{Operation: "promotion unlock cleanup", Cause: e}
			}
		}
	}
	for generation := range m.generated {
		if generation != s.ActiveRoot {
			zero(m.generated[generation])
			delete(m.generated, generation)
		}
	}
	return result, nil
}

func (m *AuthorityManager) ledgerTrustDeadline(ctx context.Context, s *authorityState, generation int) (time.Time, error) {
	var deadline time.Time
	cursor := ""
	for {
		records, next, err := m.store.listLedger(ctx, cursor, 500)
		if err != nil {
			return time.Time{}, err
		}
		for _, r := range records {
			issuer := s.Issuers[r.Issuer]
			if issuer != nil {
				v := issuer.Versions[r.IssuerVersion]
				if v != nil && v.RootGeneration == generation && r.ExpiresAt.Add(clockSkew).After(deadline) {
					deadline = r.ExpiresAt.Add(clockSkew)
				}
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return deadline, nil
}

func (m *AuthorityManager) DiscardPending(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return &ConflictError{Message: "authority manager is closed"}
	}
	var generations []int
	var discardedIssuers []string
	var discardedRootGeneration int
	err := m.store.update(ctx, func(s *authorityState) error {
		if err := m.assertSpec(s); err != nil {
			return err
		}
		if s.Pending == nil {
			return &NotFoundError{Resource: "pending operation"}
		}
		p := s.Pending
		if p.RootGeneration > 0 {
			discardedRootGeneration = p.RootGeneration
			delete(s.Roots, p.RootGeneration)
			generations = append(generations, p.RootGeneration)
		}
		for slug, v := range p.Issuers {
			if issuer := s.Issuers[slug]; issuer != nil {
				discardedIssuers = append(discardedIssuers, issuer.Definition.Name)
			}
			delete(s.Issuers[slug].Versions, v)
		}
		s.Pending = nil
		return nil
	})
	if err == nil {
		if discardedRootGeneration > 0 {
			debugLog(m.logger, "pending root certificate discarded", "generation", discardedRootGeneration)
		}
		for _, name := range discardedIssuers {
			debugLog(m.logger, "pending issuer certificate discarded", "issuer", name)
		}
		for _, g := range generations {
			zero(m.generated[g])
			delete(m.generated, g)
		}
		if unlocks, loadErr := m.store.loadUnlocks(ctx); loadErr == nil {
			for _, g := range generations {
				delete(unlocks, g)
			}
			if storeErr := m.store.storeUnlocks(ctx, unlocks, true); storeErr != nil {
				return &PartialOperationError{Operation: "discard unlock cleanup", Cause: storeErr}
			}
		}
	}
	return err
}

func (m *AuthorityManager) TrustBundlePEM() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(context.Background(), false)
	if err != nil {
		return nil, err
	}
	return buildTrustBundlePEM(s, m.now()), nil
}

func (m *AuthorityManager) ExportGeneratedUnlock() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(context.Background(), false)
	if err != nil {
		return nil, err
	}
	g := s.ActiveRoot
	if s.Pending != nil && s.Pending.Kind == "root" {
		g = s.Pending.RootGeneration
	}
	secret := m.generated[g]
	if len(secret) == 0 {
		return nil, &NotFoundError{Resource: "newly generated unlock secret"}
	}
	return append([]byte(nil), secret...), nil
}

func (m *AuthorityManager) StoreLocalUnlock(opts ...Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return &ConflictError{Message: "authority manager is closed"}
	}
	o, err := parseOptions(scopeSave, opts)
	if err != nil {
		return err
	}
	s, err := m.store.load(context.Background(), false)
	if err != nil {
		return err
	}
	values := map[int][]byte{}
	if current, loadErr := m.store.loadUnlocks(context.Background()); loadErr == nil {
		for g, v := range current {
			values[g] = v
		}
	} else {
		var nf *NotFoundError
		if !errors.As(loadErr, &nf) {
			return loadErr
		}
	}
	required := []int{s.ActiveRoot}
	if s.Pending != nil && s.Pending.Kind == "root" {
		required = append(required, s.Pending.RootGeneration)
	}
	for _, g := range required {
		if v := m.generated[g]; len(v) > 0 {
			values[g] = append([]byte(nil), v...)
		} else if len(values[g]) == 0 && g == s.ActiveRoot && m.config.rootUnlockSet {
			values[g] = append([]byte(nil), m.config.rootUnlock...)
		}
	}
	for _, g := range required {
		if len(values[g]) == 0 {
			return &NotFoundError{Resource: fmt.Sprintf("unlock secret for root G%d", g)}
		}
	}
	return m.store.storeUnlocks(context.Background(), values, o.replace)
}

func (m *AuthorityManager) Inspect(ctx context.Context) (Inspection, error) {
	s, err := m.store.load(ctx, false)
	if err != nil {
		return Inspection{}, err
	}
	out := Inspection{AuthorityID: s.Spec.ID, Revision: s.Revision, AuthorityName: s.Spec.Name, ActiveRoot: s.ActiveRoot, Issuers: map[string]int{}}
	out.ActiveRootFingerprint = s.Roots[s.ActiveRoot].Fingerprint
	if s.Pending != nil {
		out.PendingKind = s.Pending.Kind
	}
	for _, v := range s.Issuers {
		out.Issuers[v.Definition.Name] = v.ActiveVersion
	}
	return out, nil
}
func (m *AuthorityManager) Repair(ctx context.Context) error {
	s, err := m.store.load(ctx, false)
	if err != nil {
		return err
	}
	return validateState(s)
}
func (m *AuthorityManager) PruneRoot(ctx context.Context, generation int, expectedFingerprint string) error {
	if generation < 1 || expectedFingerprint == "" {
		return invalid("generation and fingerprint are required", nil)
	}
	return m.store.update(ctx, func(s *authorityState) error {
		if generation == s.ActiveRoot || (s.Pending != nil && generation == s.Pending.RootGeneration) {
			return &ConflictError{Message: "cannot prune an active or pending root"}
		}
		r := s.Roots[generation]
		if r == nil {
			return &NotFoundError{Resource: "root generation"}
		}
		if r.Fingerprint != expectedFingerprint {
			return &DriftError{Field: "root fingerprint", Expected: r.Fingerprint, Actual: expectedFingerprint}
		}
		if r.RetiredTrustUntil.After(m.now()) {
			return &ConflictError{Message: "root is still inside its required trust window"}
		}
		r.RetiredTrustUntil = time.Time{}
		return nil
	})
}
func (m *AuthorityManager) Purge(ctx context.Context, expectedAuthorityID, expectedRootFingerprint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.store.load(ctx, false)
	if err != nil {
		return err
	}
	if expectedAuthorityID == "" || expectedRootFingerprint == "" {
		return invalid("expected authority ID and root fingerprint are required", nil)
	}
	if s.Spec.ID != expectedAuthorityID {
		return &DriftError{Field: "authority ID", Expected: s.Spec.ID, Actual: expectedAuthorityID}
	}
	if s.Roots[s.ActiveRoot].Fingerprint != expectedRootFingerprint {
		return &DriftError{Field: "root fingerprint", Expected: s.Roots[s.ActiveRoot].Fingerprint, Actual: expectedRootFingerprint}
	}
	if err = m.store.purge(ctx); err != nil {
		return err
	}
	for g := range m.generated {
		zero(m.generated[g])
	}
	m.generated = map[int][]byte{}
	m.closed = true
	return nil
}
func (m *AuthorityManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	for g := range m.generated {
		zero(m.generated[g])
	}
	zero(m.config.rootUnlock)
	m.generated = nil
	m.closed = true
	return nil
}
func zero(v []byte) {
	for i := range v {
		v[i] = 0
	}
}
