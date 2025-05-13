package certutils

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// -----------------------------------------------------------------------------
// Loaders
// -----------------------------------------------------------------------------

// Loads content from a source
// Load order:
// 0. If src is empty, return nil
// 1. If src is valid PEM content it will load the content.
// 2. If src is b64 encoded it will decode and load the content.
// 3. If src is a file path it will load the content from the file.
// 4. If src is a file path and the file is b64 encoded it will decode and load the content.
// 5. If src is invalid, return an error
func LoadPem(src string) (int, []*x509.Certificate, []crypto.PrivateKey, []*x509.CertificateRequest, error) {
	// If src is empty, return nil
	if src == "" {
		return 0, nil, nil, nil, nil
	}

	// 1. Try parsing direct PEM content
	if strings.Contains(src, "-----BEGIN") {
		crts, keys, csrs, err := parsePemContent(src)
		return 1, crts, keys, csrs, err
	}

	// 2. Try decoding base64 → then parse as PEM
	if decoded, err := base64.StdEncoding.DecodeString(src); err == nil {
		if strings.Contains(string(decoded), "-----BEGIN") {
			crts, keys, csrs, err := parsePemContent(string(decoded))
			return 2, crts, keys, csrs, err
		}
	}

	// 3. Try reading from file path
	data, err := os.ReadFile(src)
	if err == nil {
		content := string(data)

		// 3a. Try as raw PEM
		if strings.Contains(content, "-----BEGIN") {
			crts, keys, csrs, err := parsePemContent(content)
			return 3, crts, keys, csrs, err
		}

		// 3b. Try base64-decoded file contents
		if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
			if strings.Contains(string(decoded), "-----BEGIN") {
				crts, keys, csrs, err := parsePemContent(string(decoded))
				return 4, crts, keys, csrs, err
			}
		}
	}

	// Failed all attempts
	return 5, nil, nil, nil, fmt.Errorf("unable to determine PEM content from source")
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

// parse Pem content into certificates, private keys, and CSRs
func parsePemContent(src string) ([]*x509.Certificate, []crypto.PrivateKey, []*x509.CertificateRequest, error) {
	data := []byte(strings.ReplaceAll(src, `\n`, "\n"))
	data = bytes.TrimSpace(data)
	// data := []byte(src)

	var crts []*x509.Certificate = make([]*x509.Certificate, 0)
	var keys []crypto.PrivateKey = make([]crypto.PrivateKey, 0)
	var csrs []*x509.CertificateRequest = make([]*x509.CertificateRequest, 0)

	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}

		switch block.Type {
		case "CERTIFICATE":
			crt, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
			}
			crts = append(crts, crt)

		case "PRIVATE KEY", "RSA PRIVATE KEY":
			key, err := parseKeyBlock(block)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse private key: %w", err)
			}
			keys = append(keys, key)

		case "CERTIFICATE REQUEST":
			csr, err := x509.ParseCertificateRequest(block.Bytes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse CSR: %w", err)
			}
			csrs = append(csrs, csr)
		}
	}

	if len(crts)+len(keys)+len(csrs) == 0 {
		return nil, nil, nil, fmt.Errorf("no valid PEM blocks found")
	}
	return crts, keys, csrs, nil
}

// Parse a pem block into a private key
func parseKeyBlock(block *pem.Block) (crypto.PrivateKey, error) {
	switch block.Type {
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}
