package certs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"sort"
	"time"
)

func buildTrustBundlePEM(s *authorityState, now time.Time) []byte {
	generations := []int{}
	for generation, root := range s.Roots {
		if generation == s.ActiveRoot || (s.Pending != nil && generation == s.Pending.RootGeneration) || root.RetiredTrustUntil.After(now) {
			generations = append(generations, generation)
		}
	}
	sort.Ints(generations)

	var out []byte
	for _, generation := range generations {
		out = append(out, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: s.Roots[generation].CertificateDER,
		})...)
	}
	return out
}

func trustBundleSHA256(bundlePEM []byte) string {
	sum := sha256.Sum256(bundlePEM)
	return hex.EncodeToString(sum[:])
}

func pendingKind(kind string) PendingKind {
	switch kind {
	case string(PendingRoot):
		return PendingRoot
	case string(PendingIssuer):
		return PendingIssuer
	default:
		return PendingNone
	}
}

func managerStatus(s *authorityState, now time.Time, issuerSlug string) ManagerStatus {
	status := ManagerStatus{
		Revision:          s.Revision,
		TrustBundleSHA256: trustBundleSHA256(buildTrustBundlePEM(s, now)),
	}
	if s.Pending == nil || (issuerSlug != "" && s.Pending.Issuers[issuerSlug] == 0) {
		return status
	}
	status.Pending = true
	status.PendingKind = pendingKind(s.Pending.Kind)
	return status
}

// Status returns the authority's current persisted state.
func (m *AuthorityManager) Status(ctx context.Context) (ManagerStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ManagerStatus{}, &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(ctx, false)
	if err != nil {
		return ManagerStatus{}, err
	}
	return managerStatus(s, m.now(), ""), nil
}

// IsPending reports whether the authority has a pending operation.
func (m *AuthorityManager) IsPending() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false, &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(context.Background(), false)
	if err != nil {
		return false, err
	}
	return s.Pending != nil, nil
}

// TrustBundleSHA256 returns the SHA-256 digest of TrustBundlePEM in lowercase hexadecimal.
func (m *AuthorityManager) TrustBundleSHA256() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(context.Background(), false)
	if err != nil {
		return "", err
	}
	return trustBundleSHA256(buildTrustBundlePEM(s, m.now())), nil
}

// Status returns this issuer's current persisted state.
func (m *IssuerManager) Status(ctx context.Context) (ManagerStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ManagerStatus{}, &ConflictError{Message: "issuer manager is closed"}
	}
	s, err := m.store.load(ctx, true)
	if err != nil {
		return ManagerStatus{}, err
	}
	issuer := s.Issuers[m.slug]
	if issuer == nil || issuer.Definition.Name != m.name {
		return ManagerStatus{}, &NotFoundError{Resource: "issuer " + m.name}
	}
	return managerStatus(s, m.now(), m.slug), nil
}

// IsPending reports whether this issuer has a staged certificate version.
func (m *IssuerManager) IsPending() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false, &ConflictError{Message: "issuer manager is closed"}
	}
	s, err := m.store.load(context.Background(), true)
	if err != nil {
		return false, err
	}
	issuer := s.Issuers[m.slug]
	if issuer == nil || issuer.Definition.Name != m.name {
		return false, &NotFoundError{Resource: "issuer " + m.name}
	}
	if s.Pending == nil {
		return false, nil
	}
	_, pending := s.Pending.Issuers[m.slug]
	return pending, nil
}

// TrustBundlePEM returns the currently trusted root certificates in PEM format.
func (m *IssuerManager) TrustBundlePEM() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "issuer manager is closed"}
	}
	s, err := m.store.load(context.Background(), true)
	if err != nil {
		return nil, err
	}
	return buildTrustBundlePEM(s, m.now()), nil
}

// TrustBundleSHA256 returns the SHA-256 digest of TrustBundlePEM in lowercase hexadecimal.
func (m *IssuerManager) TrustBundleSHA256() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", &ConflictError{Message: "issuer manager is closed"}
	}
	s, err := m.store.load(context.Background(), true)
	if err != nil {
		return "", err
	}
	return trustBundleSHA256(buildTrustBundlePEM(s, m.now())), nil
}
