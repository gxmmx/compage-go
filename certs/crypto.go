package certs

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // SKI convention; not used for signatures.
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"reflect"

	"github.com/youmark/pkcs8"
)

func generateSigner(spec KeySpec) (crypto.Signer, error) {
	if spec == "" {
		spec = ECDSAP256
	}
	switch spec {
	case ECDSAP256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case ECDSAP384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case Ed25519:
		_, key, err := ed25519.GenerateKey(rand.Reader)
		return key, err
	case RSA3072:
		return rsa.GenerateKey(rand.Reader, 3072)
	case RSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	default:
		return nil, invalid("unsupported key specification", nil)
	}
}

func supportedPublicKey(pub any) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() && k.Curve != elliptic.P384() {
			return invalid("unsupported ECDSA curve", nil)
		}
	case ed25519.PublicKey:
		if len(k) != ed25519.PublicKeySize {
			return invalid("invalid Ed25519 public key", nil)
		}
	case *rsa.PublicKey:
		if k.N.BitLen() != 3072 && k.N.BitLen() != 4096 {
			return invalid("RSA keys must be 3072 or 4096 bits", nil)
		}
	default:
		return invalid("unsupported public key", nil)
	}
	return nil
}

func keySpecForPublic(pub any) (KeySpec, error) {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if k.Curve == elliptic.P256() {
			return ECDSAP256, nil
		}
		if k.Curve == elliptic.P384() {
			return ECDSAP384, nil
		}
	case ed25519.PublicKey:
		if len(k) == ed25519.PublicKeySize {
			return Ed25519, nil
		}
	case *rsa.PublicKey:
		if k.N.BitLen() == 3072 {
			return RSA3072, nil
		}
		if k.N.BitLen() == 4096 {
			return RSA4096, nil
		}
	}
	return "", invalid("unsupported public key", nil)
}

func marshalKeyDER(key crypto.Signer) ([]byte, error) { return x509.MarshalPKCS8PrivateKey(key) }
func marshalKeyPEM(key crypto.Signer, passphrase []byte) ([]byte, error) {
	var der []byte
	var err error
	label := "PRIVATE KEY"
	if len(passphrase) == 0 {
		der, err = x509.MarshalPKCS8PrivateKey(key)
	} else {
		der, err = pkcs8.MarshalPrivateKey(key, passphrase, &pkcs8.Opts{Cipher: pkcs8.AES256CBC, KDFOpts: pkcs8.ScryptOpts{SaltSize: 16, CostParameter: 32768, BlockSize: 8, ParallelizationParameter: 1}})
		label = "ENCRYPTED PRIVATE KEY"
	}
	if err != nil {
		return nil, fmt.Errorf("encode private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: label, Bytes: der}), nil
}

func parseKeyDER(der, passphrase []byte) (crypto.Signer, error) {
	var v any
	var err error
	if len(passphrase) > 0 {
		v, err = pkcs8.ParsePKCS8PrivateKey(der, passphrase)
	} else {
		v, err = x509.ParsePKCS8PrivateKey(der)
	}
	if err != nil {
		return nil, corrupt("invalid PKCS#8 private key", err)
	}
	s, ok := v.(crypto.Signer)
	if !ok {
		return nil, corrupt("private key cannot sign", nil)
	}
	if err := supportedPublicKey(s.Public()); err != nil {
		return nil, err
	}
	return s, nil
}

func parseKeyPEM(data, passphrase []byte) (crypto.Signer, error) {
	b, rest := pem.Decode(data)
	if b == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, corrupt("private key PEM must contain exactly one block", nil)
	}
	switch b.Type {
	case "PRIVATE KEY":
		if len(passphrase) > 0 {
			return nil, invalid("passphrase supplied for an unencrypted key", nil)
		}
		return parseKeyDER(b.Bytes, nil)
	case "ENCRYPTED PRIVATE KEY":
		if len(passphrase) == 0 {
			return nil, &NotFoundError{Resource: "private key passphrase"}
		}
		return parseKeyDER(b.Bytes, passphrase)
	default:
		return nil, corrupt("unsupported private key PEM block", nil)
	}
}

func publicKeysEqual(a, b any) bool {
	aa, ea := x509.MarshalPKIXPublicKey(a)
	bb, eb := x509.MarshalPKIXPublicKey(b)
	return ea == nil && eb == nil && bytes.Equal(aa, bb)
}
func subjectKeyID(pub any) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum(der)
	return sum[:], nil
}
func fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}
func randomSerial() (*big.Int, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	b[0] &= 0x7f
	b[0] |= 0x40
	return new(big.Int).SetBytes(b), nil
}
func signerClone(s crypto.Signer) crypto.Signer { return s }
func signerIsNil(s crypto.Signer) bool {
	if s == nil {
		return true
	}
	v := reflect.ValueOf(s)
	return v.Kind() == reflect.Ptr && v.IsNil()
}
