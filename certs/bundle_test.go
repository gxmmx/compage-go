package certs

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBundleAcceptsArbitraryCertificateOrder(t *testing.T) {
	issued, csr := newIssuedBundle(t)
	directory := t.TempDir()
	certs := issued.Certificates()
	paths := []string{filepath.Join(directory, "root.pem"), filepath.Join(directory, "leaf.pem"), filepath.Join(directory, "issuer.pem")}
	order := []int{2, 0, 1}
	for i, index := range order {
		if err := os.WriteFile(paths[i], certsPEM([]*x509.Certificate{certs[index]}), 0644); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(directory, "key.pem")
	keyPEM, err := csr.KeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBundle(WithCert(paths[0]), WithCert(paths[1]), WithCert(paths[2]), WithKey(keyPath))
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Complete() || !loaded.KeyMatchesLeaf() {
		t.Fatal("loaded bundle is incomplete or key does not match")
	}
}

func TestBundleKeyAdditionIsTransactional(t *testing.T) {
	issued, _ := newIssuedBundle(t)
	other, err := NewCSR(WithSubject(Identity{CommonName: "other"}))
	if err != nil {
		t.Fatal(err)
	}
	key, err := other.KeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	next, err := issued.AddKeyPEM(key, nil)
	if err == nil {
		t.Fatal("mismatched key was accepted")
	}
	if next != issued || issued.HasKey() {
		t.Fatal("failed key addition changed the original bundle")
	}
}

func TestRootOnlyAndIntermediateOnlyBundles(t *testing.T) {
	issued, _ := newIssuedBundle(t)
	root, err := normalizeBundle([]*x509.Certificate{issued.Root()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if root.Root() == nil || root.Certificate() != nil || root.Complete() {
		t.Fatal("unexpected root-only representation")
	}
	intermediate, err := normalizeBundle([]*x509.Certificate{issued.Intermediates()[0]}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if intermediate.Certificate() != nil || len(intermediate.Intermediates()) != 1 || intermediate.Root() != nil {
		t.Fatal("unexpected intermediate-only representation")
	}
}
