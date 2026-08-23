package certs

import (
	"encoding/pem"
	"testing"
)

func TestCSRSupportedKeySpecifications(t *testing.T) {
	for _, spec := range []KeySpec{ECDSAP256, ECDSAP384, Ed25519, RSA3072, RSA4096} {
		t.Run(string(spec), func(t *testing.T) {
			bundle, err := NewCSR(WithSubject(Identity{CommonName: "device"}), WithKeySpec(spec))
			if err != nil {
				t.Fatal(err)
			}
			if err = bundle.CSR().CheckSignature(); err != nil {
				t.Fatal(err)
			}
			actual, err := keySpecForPublic(bundle.CSR().PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			if actual != spec {
				t.Fatalf("key spec = %s", actual)
			}
		})
	}
}

func TestCSREncryptedPKCS8RoundTrip(t *testing.T) {
	passphrase := []byte("correct horse battery staple")
	bundle, err := NewCSR(WithSubject(Identity{CommonName: "device"}), WithKeyPassphrase(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := bundle.KeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil || block.Type != "ENCRYPTED PRIVATE KEY" {
		t.Fatalf("unexpected PEM type")
	}
	if _, err = parseKeyPEM(keyPEM, passphrase); err != nil {
		t.Fatal(err)
	}
	if _, err = parseKeyPEM(keyPEM, []byte("wrong")); err == nil {
		t.Fatal("wrong passphrase succeeded")
	}
}
