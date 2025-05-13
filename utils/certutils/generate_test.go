package certutils

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"net"
	"net/url"
	"testing"
)

// mockReader always returns an error
type mockReader struct{}

func (m mockReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock read failure")
}

func TestRSAKeyGeneration(t *testing.T) {
	t.Run("GenerateRsaPrivateKey", func(t *testing.T) {
		key, err := GenerateRsaPrivateKey(2048)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key == nil {
			t.Fatal("expected non-nil key")
		}
		if key.PublicKey.N == nil {
			t.Fatal("expected non-nil public key")
		}
	})
	t.Run("GenerateRsaPrivateKey with small bits", func(t *testing.T) {
		_, err := GenerateRsaPrivateKey(1024)
		if err == nil {
			t.Fatal("expected error for small key size")
		}
	})
	t.Run("GenerateRSAPrivateKey with rand read error", func(t *testing.T) {
		original := rand.Reader
		rand.Reader = mockReader{} // override

		defer func() {
			rand.Reader = original // restore
		}()

		_, err := GenerateRsaPrivateKey(2048)
		if err == nil {
			t.Fatal("expected error from mock rand.Reader, got nil")
		}
	})
}

func TestED25519KeyGeneration(t *testing.T) {
	t.Run("GenerateEd25519PrivateKey", func(t *testing.T) {
		key, err := GenerateEd25519PrivateKey()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key == nil {
			t.Fatal("expected non-nil key")
		}
	})
	t.Run("GenerateEd25519PrivateKey with rand read error", func(t *testing.T) {
		// Mock the rand.Reader to simulate an error
		original := rand.Reader
		rand.Reader = mockReader{} // override

		defer func() {
			rand.Reader = original // restore
		}()

		_, err := GenerateEd25519PrivateKey()
		if err == nil {
			t.Fatal("expected error from mock rand.Reader, got nil")
		}
	})
}

func TestPublicKeyGeneration(t *testing.T) {
	t.Run("GeneratePublicKey with RSA", func(t *testing.T) {
		privKey, err := GenerateRsaPrivateKey(2048)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pubKey, err := GeneratePublicKey(privKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pubKey == nil {
			t.Fatal("expected non-nil public key")
		}
	})
	t.Run("GeneratePublicKey with ED25519", func(t *testing.T) {
		privKey, err := GenerateEd25519PrivateKey()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pubKey, err := GeneratePublicKey(privKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pubKey == nil {
			t.Fatal("expected non-nil public key")
		}
	})
	t.Run("GeneratePublicKey with unsupported type", func(t *testing.T) {
		_, err := GeneratePublicKey(nil)
		if err == nil {
			t.Fatal("expected error for unsupported private key type, got nil")
		}
	})
}

func TestCSRGeneration(t *testing.T) {
	// Gen key for tests
	privKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Create a subject
	subject := GenerateCertificateSubject(
		"",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)

	t.Run("GenerateCSR", func(t *testing.T) {
		csr, err := GenerateCsr(subject, privKey, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if csr == nil {
			t.Fatal("expected non-nil CSR")
		}
		if csr.Subject.CommonName != "undefined" {
			t.Fatalf("expected common name 'undefined', got '%s'", csr.Subject.CommonName)
		}
	})
	t.Run("GenerateCSR with create error", func(t *testing.T) {

		_, err := GenerateCsr(subject, nil, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for nil private key, got nil")
		}
	})
	t.Run("GenerateCSR with parse error", func(t *testing.T) {
		// Mock the x509.ParseCertificateRequest function to simulate an error
		defer func() { parseCertificateRequestCov = x509.ParseCertificateRequest }() // reset after test
		parseCertificateRequestCov = func(_ []byte) (*x509.CertificateRequest, error) {
			return nil, errors.New("injected failure")
		}

		_, err := GenerateCsr(subject, privKey, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
	})
}

func TestSelfSignedGeneration(t *testing.T) {
	// Gen key for tests
	privKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Create a subject
	subject := GenerateCertificateSubject(
		"some.domain.local",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	// Create a CSR
	csr, err := GenerateCsr(subject, privKey, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("GenerateSelfSignedCertificate", func(t *testing.T) {
		cert, err := GenerateSelfsignedCertificate(csr, privKey, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
	})
	t.Run("GenerateSelfSignedCertificate with create error", func(t *testing.T) {
		cert, err := GenerateSelfsignedCertificate(csr, nil, 1)
		if err == nil {
			t.Fatal("expected error for nil private key, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
	t.Run("GenerateSelfSignedCertificate with parse error", func(t *testing.T) {
		// Mock the x509.ParseCertificate function to simulate an error
		defer func() { parseCertificateCov = x509.ParseCertificate }() // reset after test
		parseCertificateCov = func(_ []byte) (*x509.Certificate, error) {
			return nil, errors.New("injected failure")
		}

		cert, err := GenerateSelfsignedCertificate(csr, privKey, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
}

func TestRootCAGeneration(t *testing.T) {
	// Create a subject
	subject := GenerateCertificateSubject(
		"Org G1 Root CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)

	t.Run("GenerateRootCA", func(t *testing.T) {
		cert, key, err := GenerateRootCA(subject, 2048, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
		if key == nil {
			t.Fatal("expected non-nil private key")
		}
		if cert.Subject.CommonName != "Org G1 Root CA" {
			t.Fatalf("expected common name 'Org G1 Root CA', got '%s'", cert.Subject.CommonName)
		}
		if cert.IsCA != true {
			t.Fatalf("expected IsCA true, got %v", cert.IsCA)
		}
	})
	t.Run("GenerateRootCA with key error", func(t *testing.T) {
		// invalid key size
		_, _, err := GenerateRootCA(subject, 1, 1)
		if err == nil {
			t.Fatal("expected error for invalid key size, got nil")
		}
	})
	t.Run("GenerateRootCA with CSR error", func(t *testing.T) {
		// Mock the GenerateCsr function to simulate an error
		generateCsrOrg := generateCsrCov
		defer func() { generateCsrCov = generateCsrOrg }() // reset after test
		generateCsrCov = func(_ pkix.Name, _ crypto.Signer, _ []string, _ []net.IP, _ []string, _ []*url.URL) (*x509.CertificateRequest, error) {
			return nil, errors.New("injected failure")
		}
		_, _, err := GenerateRootCA(subject, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
	})
	t.Run("GenerateRootCA with sign error", func(t *testing.T) {
		// Mock the x509.CreateCertificate function to simulate an error
		createCertOrg := generateCertificateCov
		defer func() { generateCertificateCov = createCertOrg }() // reset after test
		generateCertificateCov = func(_ io.Reader, _ *x509.Certificate, _ *x509.Certificate, _ any, _ any) ([]byte, error) {
			return nil, errors.New("injected failure")
		}
		_, _, err := GenerateRootCA(subject, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
	})
	t.Run("GenerateRootCA with parse error", func(t *testing.T) {
		defer func() { parseCertificateCov = x509.ParseCertificate }() // reset after test
		parseCertificateCov = func(_ []byte) (*x509.Certificate, error) {
			return nil, errors.New("injected failure")
		}

		cert, _, err := GenerateRootCA(subject, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
}

func TestIntermediateCAGeneration(t *testing.T) {
	// Create a root subject
	subject := GenerateCertificateSubject(
		"Org G1 Root CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	// Create a root CA
	rootCert, rootKey, err := GenerateRootCA(subject, 2048, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Create an intermediate subject
	intermediateSubject := GenerateCertificateSubject(
		"Org G1 Intermediate CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	t.Run("GenerateIntermediateCA", func(t *testing.T) {
		cert, key, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 2048, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
		if key == nil {
			t.Fatal("expected non-nil private key")
		}
		if cert.Subject.CommonName != "Org G1 Intermediate CA" {
			t.Fatalf("expected common name 'Org G1 Intermediate CA', got '%s'", cert.Subject.CommonName)
		}
		if cert.IsCA != true {
			t.Fatalf("expected IsCA true, got %v", cert.IsCA)
		}
		if cert.Issuer.CommonName != "Org G1 Root CA" {
			t.Fatalf("expected issuer common name 'Org G1 Root CA', got '%s'", cert.Issuer.CommonName)
		}
	})
	t.Run("GenerateIntermediateCA with key error", func(t *testing.T) {
		// invalid key size
		_, _, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 1, 1)
		if err == nil {
			t.Fatal("expected error for invalid key size, got nil")
		}
	})
	t.Run("GenerateIntermediateCA with CSR error", func(t *testing.T) {
		// Mock the GenerateCsr function to simulate an error
		generateCsrOrg := generateCsrCov
		defer func() { generateCsrCov = generateCsrOrg }() // reset after test
		generateCsrCov = func(_ pkix.Name, _ crypto.Signer, _ []string, _ []net.IP, _ []string, _ []*url.URL) (*x509.CertificateRequest, error) {
			return nil, errors.New("injected failure")
		}
		_, _, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
	})
	t.Run("GenerateIntermediateCA with sign error", func(t *testing.T) {
		// Mock the x509.CreateCertificate function to simulate an error
		createCertOrg := generateCertificateCov
		defer func() { generateCertificateCov = createCertOrg }() // reset after test
		generateCertificateCov = func(_ io.Reader, _ *x509.Certificate, _ *x509.Certificate, _ any, _ any) ([]byte, error) {
			return nil, errors.New("injected failure")
		}
		_, _, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
	})
	t.Run("GenerateIntermediateCA with parse error", func(t *testing.T) {
		defer func() { parseCertificateCov = x509.ParseCertificate }() // reset after test
		parseCertificateCov = func(_ []byte) (*x509.Certificate, error) {
			return nil, errors.New("injected failure")
		}

		cert, _, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 2048, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
}

func TestCertificateGeneration(t *testing.T) {
	// Create a root subject
	subject := GenerateCertificateSubject(
		"Org G1 Root CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	// Create a root CA
	rootCert, rootKey, err := GenerateRootCA(subject, 2048, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Create an intermediate subject
	intermediateSubject := GenerateCertificateSubject(
		"Org G1 Intermediate CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	intermediateCert, intermediateKey, err := GenerateIntermediateCA(intermediateSubject, rootCert, rootKey, 2048, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Run("GenerateCertificate", func(t *testing.T) {
		// create a key
		crtPrivKey, err := GenerateRsaPrivateKey(2048)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Create a subject
		subject := GenerateCertificateSubject(
			"some.domain.local",
			"TestOrg",
			"IT",
			"US",
			"California",
			"San Francisco",
		)
		// Create a CSR
		csr, err := GenerateCsr(subject, crtPrivKey, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cert, err := GenerateCertificate(csr, intermediateCert, intermediateKey, nil, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
		if cert.Subject.CommonName != "some.domain.local" {
			t.Fatalf("expected common name 'some.domain.local', got '%s'", cert.Subject.CommonName)
		}
		if cert.Issuer.CommonName != "Org G1 Intermediate CA" {
			t.Fatalf("expected issuer common name 'Org G1 Intermediate CA', got '%s'", cert.Issuer.CommonName)
		}
	})
	t.Run("GenerateCertificate with signing error", func(t *testing.T) {
		// Mock the x509.CreateCertificate function to simulate an error
		createCertOrg := generateCertificateCov
		defer func() { generateCertificateCov = createCertOrg }() // reset after test
		generateCertificateCov = func(_ io.Reader, _ *x509.Certificate, _ *x509.Certificate, _ any, _ any) ([]byte, error) {
			return nil, errors.New("injected failure")
		}
		// create a key
		crtPrivKey, err := GenerateRsaPrivateKey(2048)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Create a subject
		subject := GenerateCertificateSubject(
			"some.domain.local",
			"TestOrg",
			"IT",
			"US",
			"California",
			"San Francisco",
		)
		// Create a CSR
		csr, err := GenerateCsr(subject, crtPrivKey, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cert, err := GenerateCertificate(csr, intermediateCert, intermediateKey, nil, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
	t.Run("GenerateCertificate with parse error", func(t *testing.T) {
		defer func() { parseCertificateCov = x509.ParseCertificate }() // reset after test
		parseCertificateCov = func(_ []byte) (*x509.Certificate, error) {
			return nil, errors.New("injected failure")
		}
		// create a key
		crtPrivKey, err := GenerateRsaPrivateKey(2048)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Create a subject
		subject := GenerateCertificateSubject(
			"some.domain.local",
			"TestOrg",
			"IT",
			"US",
			"California",
			"San Francisco",
		)
		// Create a CSR
		csr, err := GenerateCsr(subject, crtPrivKey, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cert, err := GenerateCertificate(csr, intermediateCert, intermediateKey, nil, 1)
		if err == nil {
			t.Fatal("expected error from injected failure, got nil")
		}
		if cert != nil {
			t.Fatalf("expected nil certificate, got %v", cert)
		}
	})
}
