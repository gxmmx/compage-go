package certutils

import (
	"crypto/x509"
	"testing"
)

func TestCertBundle(t *testing.T) {

	// -------------------------------------
	// Variables
	// -------------------------------------

	// Subjects
	rootSubject := GenerateCertificateSubject(
		"Org G1 Root CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	intSubject := GenerateCertificateSubject(
		"Org G1 Intermediate CA",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)
	leafSubject := GenerateCertificateSubject(
		"some.domain.com",
		"TestOrg",
		"IT",
		"US",
		"California",
		"San Francisco",
	)

	// Generate Root CA
	rootCrt, rootKey, err := GenerateRootCA(rootSubject, 2048, 61)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate root CA: %v", err)
	}
	// Generate Intermediate CA
	intCrt, intKey, err := GenerateIntermediateCA(intSubject, rootCrt, rootKey, 2048, 60)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate intermediate CA: %v", err)
	}
	// Generate valid certificate
	key, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate RSA private key: %v", err)
	}
	csr, err := GenerateCsr(leafSubject, key, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate CSR: %v", err)
	}
	crt, err := GenerateCertificate(csr, intCrt, intKey, nil, 30)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate certificate: %v", err)
	}
	// Generate expired certificate
	expCrt, err := GenerateCertificate(csr, intCrt, intKey, nil, -1)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate expired certificate: %v", err)
	}
	// Generate unrelated key
	unrelatedKey, err := GenerateRsaPrivateKey(2048)
	if err != nil {
		t.Fatalf("Unexpected error: Failed to generate unrelated RSA private key: %v", err)
	}

	// Generate Pem content
	rootCrtPem := EncodeCrtToPem(rootCrt, true, false)
	intCrtPem := EncodeCrtToPem(intCrt, true, false)
	crtPem := EncodeCrtToPem(crt, true, false)
	keyPem := EncodeKeyToPem(key, true, false)
	invalidCrtPem := "-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----"
	invalidKeyPem := "-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----"

	// -------------------------------------
	// Tests
	// -------------------------------------

	t.Run("Empty bundle", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		// Validate the bundle
		if err := cb.Validate(false); err != nil {
			t.Fatalf("Unexpected error: Failed to validate empty bundle: %v", err)
		}
		// Fetch empty
		_ = cb.FetchCrt("")
		_ = cb.FetchKey("")
		_ = cb.FetchCa("")
		// Run checks
		_ = cb.ExpiresIn()
		_ = cb.ExpiresWithin(30)
		_ = cb.HasCrt()
		_ = cb.HasKey()
		_ = cb.HasCa()
		_ = cb.IsSelfsigned()
		_, _, _, _ = cb.Sources()
	})

	t.Run("Manual bundle", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		cb.SetCrt(crt)
		cb.SetKey(key)
		cb.SetCaChain([]*x509.Certificate{intCrt})
		cb.SetCaRoot(rootCrt)
		// Validate the bundle
		if err := cb.Validate(false); err != nil {
			t.Fatalf("Unexpected error: Failed to validate bundle: %v", err)
		}
		_ = cb.Crt()
		_ = cb.Key()
		_ = cb.CaChain()
		_ = cb.CaBundle()
		_ = cb.CaRoot()
		_ = cb.AllCrtsToBytes()
		if err := cb.ExpiresWithin(20); err != nil {
			t.Fatalf("Unexpected error: Certificate shouldbe valid for more than 20 days: %v", err)
		}
		expDays := cb.ExpiresIn()
		if expDays > 30 {
			t.Fatalf("Expected 30 days or less, got %d", expDays)
		}
		if err := cb.ExpiresWithin(29); err == nil {
			t.Fatalf("Expected error: Certificate should expire within 29 days")
		}
	})
	t.Run("Expired certificate", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		cb.SetCrt(expCrt)
		cb.SetKey(key)
		cb.SetCaChain([]*x509.Certificate{intCrt})
		cb.SetCaRoot(rootCrt)
		// Validate the bundle
		if err := cb.ExpiresWithin(30); err == nil {
			t.Fatalf("Expected error: Certificate should be expired")
		}
		if err := cb.Validate(true); err == nil {
			t.Fatalf("Expected error: Bundle should be invalid due to expired certificate")
		}
	})
	t.Run("Fetch certificate", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		if err := cb.FetchCrt(crtPem + "\n" + intCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err = cb.FetchCa(intCrtPem + "\n" + rootCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch CA: %v", err)
		}

		if err = cb.FetchKey(keyPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch key: %v", err)
		}
		// Validate
		if err := cb.Validate(false); err != nil {
			t.Fatalf("Unexpected error: Failed to validate bundle: %v", err)
		}
	})
	t.Run("Fetch invalid certificate", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		if err := cb.FetchCrt(invalidCrtPem); err == nil {
			t.Fatalf("Expected error: Invalid certificate should fail to fetch")
		}
		if err := cb.FetchCa(invalidCrtPem); err == nil {
			t.Fatalf("Expected error: Invalid certificate should fail to fetch")
		}
		if err := cb.FetchKey(invalidKeyPem); err == nil {
			t.Fatalf("Expected error: Invalid certificate should fail to fetch")
		}
	})
	t.Run("Fetch certificate with broken chain", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		if err := cb.FetchCrt(keyPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.FetchCa(keyPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.FetchCrt(crtPem + "\n" + rootCrtPem + "\n" + rootCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.FetchCrt(crtPem + "\n" + rootCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.FetchCrt(crtPem + "\n" + intCrtPem + "\n" + intCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.FetchCrt(crtPem + "\n" + intCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err = cb.FetchCa(intCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch CA: %v", err)
		}
		if err = cb.FetchCa(intCrtPem + "\n" + intCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch CA: %v", err)
		}
		if err = cb.FetchCa(rootCrtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch CA: %v", err)
		}
		cb.SetKey(unrelatedKey)
		if err = cb.Validate(false); err == nil {
			t.Fatalf("Expected error: Bundle should be invalid due to broken chain")
		}
	})
	t.Run("Fetch certificate with unrelated key", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		if err := cb.FetchCrt(crtPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}

		cb.SetKey(unrelatedKey)
		if err = cb.Validate(false); err == nil {
			t.Fatalf("Expected error: Bundle should be invalid due to unrelated key")
		}
	})
	t.Run("Fetch full chain", func(t *testing.T) {
		// Create a new CertBundle
		cb := NewBundle()
		fullPem := crtPem + "\n" + intCrtPem + "\n" + rootCrtPem + "\n" + keyPem
		if err := cb.FetchCrt(fullPem); err != nil {
			t.Fatalf("Unexpected error: Failed to fetch certificate: %v", err)
		}
		if err := cb.Validate(false); err != nil {
			t.Fatalf("Unexpected error: Failed to validate bundle: %v", err)
		}
	})
}
