package certutils

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"time"
)

// -----------------------------------------------------------------------------
// Key Generation
// -----------------------------------------------------------------------------

// Generate a new RSA private key
func GenerateRsaPrivateKey(bits int) (*rsa.PrivateKey, error) {
	if bits < 2048 {
		return nil, fmt.Errorf("RSA key size too small: %d", bits)
	}
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}
	return key, nil
}

// Generate a new ED25519 private key
func GenerateEd25519PrivateKey() (ed25519.PrivateKey, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ED25519 private key: %w", err)
	}
	return key, nil
}

// Get the public key from a crypto.PrivateKey
func GeneratePublicKey(privKey crypto.PrivateKey) (crypto.PublicKey, error) {
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		return k.Public(), nil
	// case *ecdsa.PrivateKey:
	// 	return k.Public(), nil
	case ed25519.PrivateKey:
		return k.Public(), nil
	default:
		return nil, fmt.Errorf("unsupported private key type")
	}
}

// -----------------------------------------------------------------------------
// Subject Generation
// -----------------------------------------------------------------------------

// NewSubject builds a pkix.Name from basic identity components
func GenerateCertificateSubject(commonName, org, orgUnit, country, province, locality string) pkix.Name {
	if commonName == "" {
		commonName = "undefined"
	}
	subject := pkix.Name{}
	// Country > Province > Locality > Organization > OrgUnit > CommonName
	if country != "" {
		subject.Country = []string{country}
	}
	if province != "" {
		subject.Province = []string{province}
	}
	if locality != "" {
		subject.Locality = []string{locality}
	}
	if org != "" {
		subject.Organization = []string{org}
	}
	if orgUnit != "" {
		subject.OrganizationalUnit = []string{orgUnit}
	}
	subject.CommonName = commonName

	return subject
}

// -----------------------------------------------------------------------------
// Certificate Signing Request Generation
// -----------------------------------------------------------------------------

// #Coverage replacement
var parseCertificateRequestCov = x509.ParseCertificateRequest

// Generate a CSR (Certificate Signing Request)
func GenerateCsr(subject pkix.Name, privKey crypto.Signer, dnsNames []string, ips []net.IP, emails []string, uris []*url.URL) (*x509.CertificateRequest, error) {
	template := x509.CertificateRequest{
		Subject:        subject,
		DNSNames:       dnsNames,
		IPAddresses:    ips,
		EmailAddresses: emails,
		URIs:           uris,
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSR: %w", err)
	}

	// #Coverage replacement
	csr, err := parseCertificateRequestCov(csrDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated CSR: %w", err)
	}

	return csr, nil
}

// -----------------------------------------------------------------------------
// Self-Signed Certificate Generation
// -----------------------------------------------------------------------------

// #Coverage replacement
var parseCertificateCov = x509.ParseCertificate

// Generate a self-signed certificate from a CSR
func GenerateSelfsignedCertificate(csr *x509.CertificateRequest, privKey crypto.PrivateKey, days int) (*x509.Certificate, error) {
	template := &x509.Certificate{
		Subject:               csr.Subject,
		DNSNames:              csr.DNSNames,
		IPAddresses:           csr.IPAddresses,
		URIs:                  csr.URIs,
		EmailAddresses:        csr.EmailAddresses,
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
		PublicKey:             csr.PublicKey,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, csr.PublicKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create self-signed certificate: %w", err)
	}

	// #Coverage replacement
	cert, err := parseCertificateCov(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated certificate: %w", err)
	}

	return cert, nil
}

// -----------------------------------------------------------------------------
// Certificate Authority Generation
// -----------------------------------------------------------------------------

// #Coverage replacement
var generateCsrCov = GenerateCsr
var generateCertificateCov = x509.CreateCertificate

// Generate a root CA certificate
func GenerateRootCA(subject pkix.Name, bits int, days int) (*x509.Certificate, crypto.PrivateKey, error) {
	privKey, err := GenerateRsaPrivateKey(bits)
	if err != nil {
		return nil, nil, err
	}

	csr, err := generateCsrCov(subject, privKey, nil, nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		PublicKey:             csr.PublicKey,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
	}

	certDER, err := generateCertificateCov(rand.Reader, template, template, csr.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create root CA: %w", err)
	}

	cert, err := parseCertificateCov(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse generated root CA: %w", err)
	}

	return cert, privKey, nil
}

// Generate an intermediate CA signed by a root CA
func GenerateIntermediateCA(subject pkix.Name, issuer *x509.Certificate, issuerKey crypto.PrivateKey, bits int, days int) (*x509.Certificate, crypto.PrivateKey, error) {
	privKey, err := GenerateRsaPrivateKey(bits)
	if err != nil {
		return nil, nil, err
	}

	csr, err := generateCsrCov(subject, privKey, nil, nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		PublicKey:             csr.PublicKey,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
	}

	certDER, err := generateCertificateCov(rand.Reader, template, issuer, csr.PublicKey, issuerKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create intermediate CA: %w", err)
	}

	cert, err := parseCertificateCov(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse generated intermediate CA: %w", err)
	}

	return cert, privKey, nil
}

// -----------------------------------------------------------------------------
// Signing Certificate Generation
// -----------------------------------------------------------------------------

// Generate a certificate from a CSR signed by a CA
func GenerateCertificate(csr *x509.CertificateRequest, issuer *x509.Certificate, issuerKey crypto.PrivateKey, usage []x509.ExtKeyUsage, days int) (*x509.Certificate, error) {
	if len(usage) == 0 || usage == nil {
		usage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}

	template := &x509.Certificate{
		Subject:               csr.Subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           usage,
		BasicConstraintsValid: true,
		PublicKey:             csr.PublicKey,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		DNSNames:              csr.DNSNames,
		IPAddresses:           csr.IPAddresses,
		EmailAddresses:        csr.EmailAddresses,
		URIs:                  csr.URIs,
	}

	certDER, err := generateCertificateCov(rand.Reader, template, issuer, csr.PublicKey, issuerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := parseCertificateCov(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated certificate: %w", err)
	}

	return cert, nil
}
