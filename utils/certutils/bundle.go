package certutils

import (
	"crypto"
	"crypto/x509"
	"fmt"
)

type Bundle interface {
	// Getters
	Crt() *x509.Certificate
	Key() crypto.PrivateKey
	CaChain() []*x509.Certificate
	CaRoot() *x509.Certificate
	CaBundle() []*x509.Certificate
	AllCrts() []*x509.Certificate
	AllCrtsToBytes() [][]byte

	// Manual Setters
	SetCrt(crt *x509.Certificate)
	SetKey(key crypto.PrivateKey)
	SetCaChain(caChain []*x509.Certificate)
	SetCaRoot(caRoot *x509.Certificate)

	// Fetchers
	FetchCrt(src string) error
	FetchCa(src string) error
	FetchKey(src string) error

	// Validate
	Validate(expiry bool) error

	// Checks
	ExpiresWithin(days int) error
	ExpiresIn() int
	HasCrt() bool
	HasKey() bool
	HasCa() bool
	IsSelfsigned() bool
	Sources() (int, int, int, int)
}

type CertBundle struct {
	crt          *x509.Certificate
	key          crypto.PrivateKey
	caChain      []*x509.Certificate
	caRoot       *x509.Certificate
	srcofCrt     int
	srcofKey     int
	srcofCaChain int
	srcofCaRoot  int
}

func NewBundle() *CertBundle {
	return &CertBundle{}
}

// -----------------------------------------------------------------------------
// Getters
// -----------------------------------------------------------------------------

// Gets the certificate
func (cb *CertBundle) Crt() *x509.Certificate {
	return cb.crt
}

// Gets the private key
func (cb *CertBundle) Key() crypto.PrivateKey {
	return cb.key
}

// Gets the certificate chain
func (cb *CertBundle) CaChain() []*x509.Certificate {
	return cb.caChain
}

// Gets the root certificate
func (cb *CertBundle) CaRoot() *x509.Certificate {
	return cb.caRoot
}

// Gets the CA bundle
func (cb *CertBundle) CaBundle() []*x509.Certificate {
	crts := make([]*x509.Certificate, 0)
	if len(cb.caChain) > 0 {
		crts = append(crts, cb.caChain...)
	}
	if cb.caRoot != nil {
		crts = append(crts, cb.caRoot)
	}
	return crts
}

// Gets all certificates
func (cb *CertBundle) AllCrts() []*x509.Certificate {
	crts := make([]*x509.Certificate, 0)
	if cb.crt != nil {
		crts = append(crts, cb.crt)
	}
	if len(cb.caChain) > 0 {
		crts = append(crts, cb.caChain...)
	}
	if cb.caRoot != nil {
		crts = append(crts, cb.caRoot)
	}
	return crts
}

// Gets all certificates as byte slices
func (cb *CertBundle) AllCrtsToBytes() [][]byte {
	crts := cb.AllCrts()
	return CrtsToByteSlice(crts...)
}

// -----------------------------------------------------------------------------
// Setters
// -----------------------------------------------------------------------------

// Sets the certificate from a passed certificate directly
// This is useful for setting a certificate that has been generated in memory
// but in most cases you will want to use the FetchCrt method to load a certificate
func (cb *CertBundle) SetCrt(crt *x509.Certificate) {
	cb.srcofCrt = 5
	cb.crt = crt
}

// Sets the private key from a passed private key directly
// This is useful for setting a private key that has been generated in memory
// but in most cases you will want to use the FetchKey method to load a private key
func (cb *CertBundle) SetKey(key crypto.PrivateKey) {
	cb.srcofKey = 5
	cb.key = key
}

// Sets the certificate chain from a passed certificate chain directly
// This is useful for setting a certificate chain that has been generated in memory
// but in most cases you will want to use the FetchCa method to load a certificate chain
func (cb *CertBundle) SetCaChain(caChain []*x509.Certificate) {
	cb.srcofCaChain = 5
	cb.caChain = caChain
}

// Sets the root certificate from a passed root certificate directly
// This is useful for setting a root certificate that has been generated in memory
// but in most cases you will want to use the FetchCa method to load a root certificate
func (cb *CertBundle) SetCaRoot(caRoot *x509.Certificate) {
	cb.srcofCaRoot = 5
	cb.caRoot = caRoot
}

// -----------------------------------------------------------------------------
// Fetchers
// -----------------------------------------------------------------------------

// Fetches the certificate from a source
// The source can be raw PEM content, base64 encoded PEM content, or a file path.
// It is best to call this fetcher first to load the certificate,
// fallowed by the FetchCa method to load the CA chain and root certificate, which
// will overwrite the CA chain and root certificate if they are already set.
// Lastly, call the FetchKey method to load the private key and overwrite existing.
func (cb *CertBundle) FetchCrt(src string) error {
	if src == "" {
		return nil
	}
	from, crts, keys, _, err := LoadPem(src)
	if err != nil {
		return err
	}

	// First key found
	if len(keys) > 0 {
		cb.srcofKey = from
		cb.key = keys[0]
	}

	// Certificate chain
	n := len(crts)
	switch n {
	case 0:
		// No certificates found
	case 1:
		// Only one certificate found
		cb.srcofCrt = from
		cb.crt = crts[0]
	case 2:
		// Two certificates found
		// The first is the certificate, second is chain or root if self-signed
		cb.srcofCrt = from
		cb.crt = crts[0]
		if IsSelfsigned(crts[1]) {
			cb.srcofCaRoot = from
			cb.caRoot = crts[1]
		} else {
			cb.srcofCaChain = from
			cb.caChain = crts[1:2]
		}
	default:
		// More than two certificates found
		// The first is the certificate, rest is chain, last is root if self-signed
		cb.srcofCrt = from
		cb.crt = crts[0]
		last := crts[n-1]
		if IsSelfsigned(last) {
			cb.srcofCaRoot = from
			cb.caRoot = last
			cb.srcofCaChain = from
			cb.caChain = crts[1 : n-1]
		} else {
			cb.srcofCaChain = from
			cb.caChain = crts[1:]
		}
	}
	return nil
}

// Fetches the CA from a source
// The source can be raw PEM content, base64 encoded PEM content, or a file path.
// It is best to call FetchCrt first to load the certificate, then this to overwrite.
func (cb *CertBundle) FetchCa(src string) error {
	if src == "" {
		return nil
	}
	from, crts, _, _, err := LoadPem(src)
	if err != nil {
		return err
	}
	// Certificate chain
	n := len(crts)
	switch n {
	case 0:
		// No certificates found
	case 1:
		// Only one certificate found, chain or root if self-signed
		if IsSelfsigned(crts[0]) {
			cb.srcofCaRoot = from
			cb.caRoot = crts[0]
		} else {
			cb.srcofCaChain = from
			cb.caChain = crts
		}
	default:
		// More than one certificate found
		// The last is root if self-signed
		// The rest is chain
		last := crts[n-1]
		if IsSelfsigned(last) {
			cb.srcofCaRoot = from
			cb.caRoot = last
			cb.srcofCaChain = from
			cb.caChain = crts[:n-1]
		} else {
			cb.srcofCaChain = from
			cb.caChain = crts
		}
	}
	return nil
}

// Fetches the key from a source
// The source can be raw PEM content, base64 encoded PEM content, or a file path.
// It is best to call FetchCrt first to load the certificate, and possible key,
// then this to overwrite.
func (cb *CertBundle) FetchKey(src string) error {
	if src == "" {
		return nil
	}
	from, _, keys, _, err := LoadPem(src)
	if err != nil {
		return err
	}
	// First key found
	if len(keys) > 0 {
		cb.srcofKey = from
		cb.key = keys[0]
	}
	return nil
}

// -----------------------------------------------------------------------------
// Validate
// -----------------------------------------------------------------------------

// Validates the certificate chain
// If expiry is true, it will check if the certificate is expired.
// If the certificate is nil, it will return nil.
func (cb *CertBundle) Validate(expiry bool) error {
	crts := cb.AllCrts()

	// Check expiry
	if expiry {
		for _, crt := range crts {
			err := ValidateCrtExpiry(crt)
			if err != nil {
				return err
			}
		}
	}

	// Check if certificate is signed by next in chain
	for i := 0; i < len(crts)-1; i++ {
		current := crts[i]
		issuer := crts[i+1]

		// Check if issuer actually signed current cert
		if err := current.CheckSignatureFrom(issuer); err != nil {
			return fmt.Errorf("certificate '%s' is not signed by '%s'", current.Subject.CommonName, issuer.Subject.CommonName)
		}
	}

	// Check if certificate matches private key
	if cb.key != nil && cb.crt != nil {
		err := ValidateCrtKeyPair(cb.crt, cb.key)
		if err != nil {
			return err
		}
	}
	return nil
}

// Returns an error if a certificate in the bundle expires within the specified number of days.
func (cb *CertBundle) ExpiresWithin(days int) error {
	crts := cb.AllCrts()
	if len(crts) == 0 {
		return nil
	}
	for _, crt := range crts {
		err := ValidateCrtExpiry(crt)
		if err != nil {
			return err
		}
		err = CrtExpiresSoon(crt, days)
		if err != nil {
			return err
		}
	}
	return nil
}

// Returns the number of days until the certificate expires.
func (cb *CertBundle) ExpiresIn() int {
	crts := cb.AllCrts()
	if len(crts) == 0 {
		return 0
	}
	return CrtExpiresIn(crts[0])
}

// Returns true if the certificate bundle has a certificate.
func (cb *CertBundle) HasCrt() bool {
	return cb.crt != nil
}

// Returns true if the certificate bundle has a private key.
func (cb *CertBundle) HasKey() bool {
	return cb.key != nil
}

// Returns true if the certificate bundle has a CA chain or root certificate.
func (cb *CertBundle) HasCa() bool {
	return cb.caRoot != nil || len(cb.caChain) > 0
}

// Returns true if the certificate is self-signed.
func (cb *CertBundle) IsSelfsigned() bool {
	return IsSelfsigned(cb.crt)
}

// Returns the sources of the certificate bundle.
func (cb *CertBundle) Sources() (int, int, int, int) {
	return cb.srcofCrt, cb.srcofKey, cb.srcofCaChain, cb.srcofCaRoot
}
