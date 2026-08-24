package certs

import (
	"bytes"
	"crypto/x509"
	"sort"
)

// TrustBundle contains an unordered set of self-signed CA certificates.
type TrustBundle struct {
	certs []*x509.Certificate
}

// NewTrustBundle creates a trust bundle from certificate objects, PEM data, or
// certificate files supplied through WithCert, WithCertPEM, and WithCertPath.
func NewTrustBundle(opts ...Option) (*TrustBundle, error) {
	o, err := parseOptions(scopeTrustBundle, opts)
	if err != nil {
		return nil, err
	}
	certificates, err := loadCertificateInputs(o)
	if err != nil {
		return nil, err
	}
	return newTrustBundle(certificates)
}

func newTrustBundle(certificates []*x509.Certificate) (*TrustBundle, error) {
	normalized, err := normalizeTrustCertificates(certificates)
	if err != nil {
		return nil, err
	}
	return &TrustBundle{certs: normalized}, nil
}

func normalizeTrustCertificates(certificates []*x509.Certificate) ([]*x509.Certificate, error) {
	if len(certificates) == 0 {
		return nil, invalid("at least one trust certificate is required", nil)
	}
	byRaw := map[string]bool{}
	cloned := make([]*x509.Certificate, 0, len(certificates))
	for _, cert := range certificates {
		if cert == nil {
			return nil, corrupt("nil trust certificate", nil)
		}
		if !cert.IsCA || !cert.BasicConstraintsValid || cert.KeyUsage&x509.KeyUsageCertSign == 0 {
			return nil, invalid("trust certificate must be a CA with certificate-signing usage", nil)
		}
		if !bytes.Equal(cert.RawSubject, cert.RawIssuer) || cert.CheckSignatureFrom(cert) != nil {
			return nil, invalid("trust certificate must be self-signed", nil)
		}
		if byRaw[string(cert.Raw)] {
			return nil, &ConflictError{Message: "duplicate trust certificate"}
		}
		byRaw[string(cert.Raw)] = true
		cloned = append(cloned, cloneCertificate(cert))
	}
	sort.Slice(cloned, func(i, j int) bool {
		return bytes.Compare(cloned[i].Raw, cloned[j].Raw) < 0
	})
	return cloned, nil
}

// AddCert returns a new trust bundle with cert added.
func (b *TrustBundle) AddCert(cert *x509.Certificate) (*TrustBundle, error) {
	if b == nil {
		return nil, invalid("nil trust bundle", nil)
	}
	return newTrustBundle(append(append([]*x509.Certificate(nil), b.certs...), cert))
}

// AddCertPEM returns a new trust bundle with certificates from data added.
func (b *TrustBundle) AddCertPEM(data []byte) (*TrustBundle, error) {
	if b == nil {
		return nil, invalid("nil trust bundle", nil)
	}
	add, err := parseCertificatesPEM(data)
	if err != nil {
		return b, err
	}
	all := append(append([]*x509.Certificate(nil), b.certs...), add...)
	return newTrustBundle(all)
}

// AddCertPath returns a new trust bundle with certificates from path added.
func (b *TrustBundle) AddCertPath(path string) (*TrustBundle, error) {
	if b == nil {
		return nil, invalid("nil trust bundle", nil)
	}
	data, err := readRegular(path)
	if err != nil {
		return b, err
	}
	return b.AddCertPEM(data)
}

// Certificates returns the trust certificates in canonical order.
func (b *TrustBundle) Certificates() []*x509.Certificate {
	if b == nil {
		return nil
	}
	return append([]*x509.Certificate(nil), b.certs...)
}

// PEM returns the trust certificates in canonical PEM form.
func (b *TrustBundle) PEM() []byte {
	if b == nil {
		return nil
	}
	return certsPEM(b.certs)
}

// SHA256 returns the SHA-256 digest of PEM in lowercase hexadecimal.
func (b *TrustBundle) SHA256() string {
	if b == nil {
		return ""
	}
	return trustBundleSHA256(b.PEM())
}

// CertPool returns a new x509 certificate pool containing the trust roots.
func (b *TrustBundle) CertPool() *x509.CertPool {
	if b == nil {
		return nil
	}
	pool := x509.NewCertPool()
	for _, cert := range b.certs {
		pool.AddCert(cert)
	}
	return pool
}

// Validate verifies the trust bundle's root-only invariants.
func (b *TrustBundle) Validate() error {
	if b == nil {
		return invalid("nil trust bundle", nil)
	}
	_, err := normalizeTrustCertificates(b.certs)
	return err
}

// Save writes the canonical trust bundle PEM to path.
func (b *TrustBundle) Save(path string, opts ...Option) error {
	if b == nil || len(b.certs) == 0 {
		return &NotFoundError{Resource: "trust certificates"}
	}
	return atomicSave(path, b.PEM(), 0644, opts...)
}
