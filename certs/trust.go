package certs

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"time"
)

func buildTrustBundle(s *authorityState, now time.Time) (*TrustBundle, error) {
	certificates := make([]*x509.Certificate, 0, len(s.Roots))
	for generation, root := range s.Roots {
		if generation != s.ActiveRoot && (s.Pending == nil || generation != s.Pending.RootGeneration) && !root.RetiredTrustUntil.After(now) {
			continue
		}
		cert, err := parseCertDER(root.CertificateDER)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, cert)
	}
	return newTrustBundle(certificates)
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

func managerStatus(s *authorityState, now time.Time, issuerSlug string) (ManagerStatus, error) {
	bundle, err := buildTrustBundle(s, now)
	if err != nil {
		return ManagerStatus{}, err
	}
	status := ManagerStatus{Revision: s.Revision, TrustBundleSHA256: bundle.SHA256()}
	if s.Pending == nil || (issuerSlug != "" && s.Pending.Issuers[issuerSlug] == 0) {
		return status, nil
	}
	status.Pending = true
	status.PendingKind = pendingKind(s.Pending.Kind)
	return status, nil
}

// TrustBundle returns the authority's currently trusted root certificates.
func (m *AuthorityManager) TrustBundle(ctx context.Context) (*TrustBundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "authority manager is closed"}
	}
	s, err := m.store.load(ctx, false)
	if err != nil {
		return nil, err
	}
	return buildTrustBundle(s, m.now())
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
	return managerStatus(s, m.now(), "")
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
	return managerStatus(s, m.now(), m.slug)
}

// TrustBundle returns this issuer's currently trusted root certificates.
func (m *IssuerManager) TrustBundle(ctx context.Context) (*TrustBundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, &ConflictError{Message: "issuer manager is closed"}
	}
	s, err := m.store.load(ctx, true)
	if err != nil {
		return nil, err
	}
	issuer := s.Issuers[m.slug]
	if issuer == nil || issuer.Definition.Name != m.name {
		return nil, &NotFoundError{Resource: "issuer " + m.name}
	}
	return buildTrustBundle(s, m.now())
}
