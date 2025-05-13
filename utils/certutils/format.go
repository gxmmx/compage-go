package certutils

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
)

func CrtsToByteSlice(crts ...*x509.Certificate) [][]byte {
	crtBytes := make([][]byte, len(crts))
	for i, crt := range crts {
		crtBytes[i] = crt.Raw
	}
	return crtBytes
}

func EncodeCrtToPem(crt *x509.Certificate, trim, b64 bool) string {
	if crt == nil {
		return ""
	}

	// Generate raw PEM bytes
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crt.Raw})
	// Optionally replace newlines with literal newlines. Create single line PEM
	if trim {
		pemBytes = bytes.ReplaceAll(pemBytes, []byte("\n"), []byte("\\n"))
	}
	// Remove trailing newlines
	pemBytes = bytes.TrimRight(pemBytes, "\\n\n")
	// Optionally base64 encode
	if b64 {
		return base64.StdEncoding.EncodeToString(pemBytes)
	}

	return strings.TrimRight(string(pemBytes), "\\n\n")
}

func EncodeKeyToPem(key crypto.PrivateKey, trim, b64 bool) string {
	if key == nil {
		return ""
	}

	var der []byte
	var blockType string = "PRIVATE KEY"

	// Cases for supported key types
	switch k := key.(type) {
	case *rsa.PrivateKey, ed25519.PrivateKey:
		der, _ = x509.MarshalPKCS8PrivateKey(k)
	default:
		return ""
	}

	// Generate raw PEM bytes
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	// Optionally trim newlines
	if trim {
		pemBytes = bytes.ReplaceAll(pemBytes, []byte("\n"), []byte("\\n"))
	}
	// Remove trailing newlines
	pemBytes = bytes.TrimRight(pemBytes, "\\n\n")
	// Optionally base64 encode
	if b64 {
		return base64.StdEncoding.EncodeToString(pemBytes)
	}
	return string(pemBytes)
}

func EncodeKeyToPemPKCS1(key *rsa.PrivateKey, trim, b64 bool) string {
	if key == nil {
		return ""
	}

	var der []byte
	var blockType string = "RSA PRIVATE KEY"

	der = x509.MarshalPKCS1PrivateKey(key)

	// Generate raw PEM bytes
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	// Optionally trim newlines
	if trim {
		pemBytes = bytes.ReplaceAll(pemBytes, []byte("\n"), []byte("\\n"))
	}
	// Remove trailing newlines
	pemBytes = bytes.TrimRight(pemBytes, "\\n\n")
	// Optionally base64 encode
	if b64 {
		return base64.StdEncoding.EncodeToString(pemBytes)
	}
	return string(pemBytes)
}

func EncodeCsrToPem(csr *x509.CertificateRequest, trim, b64 bool) string {
	if csr == nil {
		return ""
	}

	// Generate raw PEM bytes
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw})
	// Optionally trim newlines
	if trim {
		pemBytes = bytes.ReplaceAll(pemBytes, []byte("\n"), []byte("\\n"))
	}
	// Remove trailing newlines
	pemBytes = bytes.TrimRight(pemBytes, "\\n\n")
	// Optionally base64 encode
	if b64 {
		return base64.StdEncoding.EncodeToString(pemBytes)
	}
	return string(pemBytes)
}
