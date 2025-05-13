package certutils

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// -----------------------------------------------------------------------------
// Validators
// -----------------------------------------------------------------------------

// Validates if the certificate is within validity period
// If the certificate is nil, it returns nil
func ValidateCrtExpiry(crt *x509.Certificate) error {
	if crt == nil {
		return nil
	}
	now := time.Now()
	if crt.NotBefore.After(now) {
		return fmt.Errorf("certificate '%s' is not yet valid", crt.Subject.CommonName)
	}
	if now.After(crt.NotAfter) {
		return fmt.Errorf("certificate '%s' has expired", crt.Subject.CommonName)
	}
	return nil
}

func ValidateCrtKeyPair(crt *x509.Certificate, key crypto.PrivateKey) error {
	if crt == nil || key == nil {
		return nil
	}
	switch k := key.(type) {
	case *rsa.PrivateKey:
		certPubKey, ok := crt.PublicKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("certificate does not contain RSA public key")
		}
		if certPubKey.N.Cmp(k.PublicKey.N) != 0 || certPubKey.E != k.PublicKey.E {
			return errors.New("private key does not match certificate")
		}
	case ed25519.PrivateKey:
		certPubKey, ok := crt.PublicKey.(ed25519.PublicKey)
		if !ok {
			return errors.New("certificate does not contain Ed25519 public key")
		}
		if !certPubKey.Equal(k.Public().(ed25519.PublicKey)) {
			return errors.New("private key does not match certificate")
		}
	default:
		return errors.New("unsupported private key type")
	}
	return nil
}

// Returns an error if the certificate expires within the specified number of days
// If the certificate is nil, it returns nil
func CrtExpiresSoon(crt *x509.Certificate, days int) error {
	if crt == nil {
		return nil
	}
	daysLeft := int(time.Until(crt.NotAfter).Hours() / 24)
	if daysLeft <= days {
		return fmt.Errorf("certificate '%s' expires in %d days", crt.Subject.CommonName, daysLeft)
	}
	return nil
}

// Returns the number of days until the certificate expires
func CrtExpiresIn(crt *x509.Certificate) int {
	if crt == nil {
		return 0
	}
	return int(time.Until(crt.NotAfter).Hours() / 24)
}

// Returns true if the certificate is self-signed
// If the certificate is nil, it returns false
func IsSelfsigned(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	// Compare raw subject and issuer
	if !bytes.Equal(cert.RawSubject, cert.RawIssuer) {
		return false
	}
	// Verify the cert was signed by its own public key
	return cert.CheckSignatureFrom(cert) == nil
}
