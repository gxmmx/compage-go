package certutils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"
	"testing"
	"time"
)

func genCertCustomTestDates(csr *x509.CertificateRequest, privKey crypto.PrivateKey, before time.Time, after time.Time) (*x509.Certificate, error) {
	template := &x509.Certificate{
		Subject:               csr.Subject,
		DNSNames:              csr.DNSNames,
		IPAddresses:           csr.IPAddresses,
		URIs:                  csr.URIs,
		EmailAddresses:        csr.EmailAddresses,
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
		PublicKey:             csr.PublicKey,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		NotBefore:             before,
		NotAfter:              after,
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
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated certificate: %w", err)
	}

	return cert, nil
}

func TestValidate(t *testing.T) {

	// -------------------------------------
	// Variables
	// -------------------------------------

	// Subjects
	crtSubject := GenerateCertificateSubject(
		"some.domain.com",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	// Generate Keys
	edKey, err := GenerateEd25519PrivateKey()
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate Ed25519 private key: %v", err)
	}
	rsaKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate RSA private key: %v", err)
	}
	// Generate Request
	csr, err := GenerateCsr(crtSubject, rsaKey, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate CSR: %v", err)
	}
	// Generate Certificate reqquest from Ed25519 key
	edCsr, err := GenerateCsr(crtSubject, edKey, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate CSR: %v", err)
	}
	// Generate unused key
	unusedRsaKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate RSA private key: %v", err)
	}
	// Generate unused Ed25519 key
	unusedEdKey, err := GenerateEd25519PrivateKey()
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate Ed25519 private key: %v", err)
	}
	// Generate edcsa key
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate ECDSA private key: %v", err)
	}
	// Generate Certificate from ed25519 key
	edCrt, err := GenerateSelfsignedCertificate(edCsr, edKey, 30)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate certificate: %v", err)
	}
	// Generate Certificate from rsa key
	rsaCrt, err := GenerateSelfsignedCertificate(csr, rsaKey, 30)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate certificate: %v", err)
	}
	// Generate certificate that is not yet valid
	notYetValidCrt, err := genCertCustomTestDates(csr, rsaKey, time.Now().Add(24*time.Hour), time.Now().Add(30*time.Hour))
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate certificate: %v", err)
	}
	// Generate expired certificate
	expiredCrt, err := genCertCustomTestDates(csr, rsaKey, time.Now().Add(-30*time.Hour), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate certificate: %v", err)
	}

	t.Run("Test Expiry", func(t *testing.T) {
		// Empty certificate
		err := ValidateCrtExpiry(nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Not yet valid certificate
		err = ValidateCrtExpiry(notYetValidCrt)
		if err == nil {
			t.Fatalf("Expected error: certificate '%s' is not yet valid", notYetValidCrt.Subject.CommonName)
		}
		// Expired certificate
		err = ValidateCrtExpiry(expiredCrt)
		if err == nil {
			t.Fatalf("Expected error: certificate '%s' has expired", expiredCrt.Subject.CommonName)
		}
		// Nil expires soon
		err = CrtExpiresSoon(nil, 30)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Nil expires in
		days := CrtExpiresIn(nil)
		if days != 0 {
			t.Fatalf("Expected 0 days, got %d", days)
		}
	})
	t.Run("Test Key Pair", func(t *testing.T) {
		// Empty certificate
		err := ValidateCrtKeyPair(nil, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Ed25519 certificate with RSA key
		err = ValidateCrtKeyPair(edCrt, rsaKey)
		if err == nil {
			t.Fatalf("Expected error: private key does not match certificate")
		}
		// RSA certificate with Ed25519 key
		err = ValidateCrtKeyPair(rsaCrt, edKey)
		if err == nil {
			t.Fatalf("Expected error: private key does not match certificate")
		}
		// Valid RSA certificate with unused RSA key
		err = ValidateCrtKeyPair(rsaCrt, unusedRsaKey)
		if err == nil {
			t.Fatalf("Expected error: private key does not match certificate")
		}
		// Valid Ed25519 certificate with unused Ed25519 key
		err = ValidateCrtKeyPair(edCrt, unusedEdKey)
		if err == nil {
			t.Fatalf("Expected error: private key does not match certificate")
		}
		// Valid Ed25519 certificate with ECDSA key
		err = ValidateCrtKeyPair(edCrt, ecdsaKey)
		if err == nil {
			t.Fatalf("Expected error: private key does not match certificate")
		}
	})
}
