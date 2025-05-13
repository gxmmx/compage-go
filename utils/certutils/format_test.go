package certutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {

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

	t.Run("Empty", func(t *testing.T) {
		// Convert to PEM format
		pemCrt := EncodeCrtToPem(nil, false, false)
		if pemCrt != "" {
			t.Errorf("Expected empty PEM string, got: %s", pemCrt)
		}
		pemKey := EncodeKeyToPem(nil, false, false)
		if pemKey != "" {
			t.Errorf("Expected empty PEM string, got: %s", pemKey)
		}
		pemKeyPKCS1 := EncodeKeyToPemPKCS1(nil, false, false)
		if pemKeyPKCS1 != "" {
			t.Errorf("Expected empty PEM string, got: %s", pemKeyPKCS1)
		}
		csrPem := EncodeCsrToPem(nil, false, false)
		if csrPem != "" {
			t.Errorf("Expected empty PEM string, got: %s", csrPem)
		}
	})
	t.Run("Crt", func(t *testing.T) {
		// Multiline PEM
		pemCrt := EncodeCrtToPem(crt, false, false)
		if strings.Contains(pemCrt, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemCrt)
		}
		// Single line PEM
		pemCrtTrim := EncodeCrtToPem(crt, true, false)
		if !strings.Contains(pemCrtTrim, "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemCrtTrim)
		}
		// Base64 PEM
		pemCrtB64 := EncodeCrtToPem(crt, false, true)
		if strings.Contains(pemCrtB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemCrtB64)
		}
		decoded, err := base64.StdEncoding.DecodeString(pemCrtB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decoded), "-----BEGIN CERTIFICATE-----") {
			t.Errorf("Expected decoded PEM string to contain BEGIN CERTIFICATE, got: %s", string(decoded))
		}
		if strings.Contains(pemCrt, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemCrt)
		}
		// Single line base64 PEM
		pemCrtTrimB64 := EncodeCrtToPem(crt, true, true)
		if strings.Contains(pemCrtTrimB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemCrtB64)
		}
		decodedTrim, err := base64.StdEncoding.DecodeString(pemCrtTrimB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decodedTrim), "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemCrtTrim)
		}
	})
	t.Run("Key", func(t *testing.T) {
		// Unsupported key type
		unsupportedKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pemKey := EncodeKeyToPem(unsupportedKey, false, false)
		if pemKey != "" {
			t.Errorf("Expected empty PEM string, got: %s", pemKey)
		}
		// Multiline PEM
		pemKey = EncodeKeyToPem(key, false, false)
		if strings.Contains(pemKey, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemKey)
		}
		// Single line PEM
		pemKeyTrim := EncodeKeyToPem(key, true, false)
		if !strings.Contains(pemKeyTrim, "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemKeyTrim)
		}
		// Base64 PEM
		pemKeyB64 := EncodeKeyToPem(key, false, true)
		if strings.Contains(pemKeyB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemKeyB64)
		}
		decoded, err := base64.StdEncoding.DecodeString(pemKeyB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decoded), "-----BEGIN PRIVATE KEY-----") {
			t.Errorf("Expected decoded PEM string to contain BEGIN PRIVATE KEY, got: %s", string(decoded))
		}
		if strings.Contains(pemKey, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemKey)
		}
		// Single line base64 PEM
		pemKeyTrimB64 := EncodeKeyToPem(key, true, true)
		if strings.Contains(pemKeyTrimB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemKeyB64)
		}
		decodedTrim, err := base64.StdEncoding.DecodeString(pemKeyTrimB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decodedTrim), "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemKeyTrim)
		}
	})
	t.Run("Key PKCS1", func(t *testing.T) {
		// Multiline PEM
		pemKey := EncodeKeyToPemPKCS1(key, false, false)
		if strings.Contains(pemKey, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemKey)
		}
		// Single line PEM
		pemKeyTrim := EncodeKeyToPemPKCS1(key, true, false)
		if !strings.Contains(pemKeyTrim, "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemKeyTrim)
		}
		// Base64 PEM
		pemKeyB64 := EncodeKeyToPemPKCS1(key, false, true)
		if strings.Contains(pemKeyB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemKeyB64)
		}
		decoded, err := base64.StdEncoding.DecodeString(pemKeyB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decoded), "-----BEGIN RSA PRIVATE KEY-----") {
			t.Errorf("Expected decoded PEM string to contain BEGIN RSA PRIVATE KEY, got: %s", string(decoded))
		}
		if strings.Contains(pemKey, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemKey)
		}
		// Single line base64 PEM
		pemKeyTrimB64 := EncodeKeyToPemPKCS1(key, true, true)
		if strings.Contains(pemKeyTrimB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemKeyB64)
		}
		decodedTrim, err := base64.StdEncoding.DecodeString(pemKeyTrimB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decodedTrim), "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemKeyTrim)
		}
	})
	t.Run("CSR", func(t *testing.T) {
		// Multiline PEM
		pemCsr := EncodeCsrToPem(csr, false, false)
		if strings.Contains(pemCsr, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemCsr)
		}
		// Single line PEM
		pemCsrTrim := EncodeCsrToPem(csr, true, false)
		if !strings.Contains(pemCsrTrim, "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemCsrTrim)
		}
		// Base64 PEM
		pemCsrB64 := EncodeCsrToPem(csr, false, true)
		if strings.Contains(pemCsrB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemCsrB64)
		}
		decoded, err := base64.StdEncoding.DecodeString(pemCsrB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decoded), "-----BEGIN CERTIFICATE REQUEST-----") {
			t.Errorf("Expected decoded PEM string to contain BEGIN CERTIFICATE REQUEST, got: %s", string(decoded))
		}
		if strings.Contains(pemCsr, "\\n") {
			t.Errorf("Expected newlines in PEM string, got: %s", pemCsr)
		}
		// Single line base64 PEM
		pemCsrTrimB64 := EncodeCsrToPem(csr, true, true)
		if strings.Contains(pemCsrTrimB64, "-----BEGIN") {
			t.Errorf("Expected base64 encoded PEM string, got: %s", pemCsrB64)
		}
		decodedTrim, err := base64.StdEncoding.DecodeString(pemCsrTrimB64)
		if err != nil {
			t.Errorf("Failed to decode base64 PEM string: %v", err)
		}
		if !strings.Contains(string(decodedTrim), "\\n") {
			t.Errorf("Expected literal newlines in PEM string, got: %s", pemCsrTrim)
		}
	})
}
