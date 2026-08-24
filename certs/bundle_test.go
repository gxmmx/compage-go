package certs

import (
	"bytes"
	"context"
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
	loaded, err := LoadBundle(WithCertPath(paths[0]), WithCertPath(paths[1]), WithCertPath(paths[2]), WithKey(keyPath))
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

func TestCertificateBundleAcceptsObjectAndPEMSources(t *testing.T) {
	issued, _ := newIssuedBundle(t)
	loaded, err := NewCertificateBundle(
		WithCert(issued.Certificate()),
		WithCertPEM(certsPEM(issued.Intermediates())),
		WithCertPEM(certsPEM([]*x509.Certificate{issued.Root()})),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Complete() || len(loaded.Certificates()) != 3 {
		t.Fatal("object and PEM sources did not produce a complete certificate bundle")
	}
}

func TestTrustBundleAcceptsUnrelatedSelfSignedRoots(t *testing.T) {
	ctx := context.Background()
	issued, _ := newIssuedBundle(t)
	otherStore := newTestFileStore(t)
	otherAuthority, err := NewAuthorityManager(WithStore(otherStore), WithAuthorityName("Other Authority"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = otherAuthority.Close() })
	if _, err = otherAuthority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	otherRoots, err := otherAuthority.TrustBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rootA := issued.Root()
	rootB := otherRoots.Certificates()[0]

	trust, err := NewTrustBundle(WithCert(rootA), WithCert(rootB))
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := NewTrustBundle(WithCert(rootB), WithCert(rootA))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(trust.PEM(), reversed.PEM()) || trust.SHA256() != reversed.SHA256() {
		t.Fatal("trust bundle canonicalization depends on input order")
	}
	if len(trust.Certificates()) != 2 || trust.CertPool() == nil {
		t.Fatal("trust bundle did not expose both roots")
	}
	pemTrust, err := NewTrustBundle(WithCertPEM(trust.PEM()))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "roots.pem")
	if err = os.WriteFile(path, trust.PEM(), 0644); err != nil {
		t.Fatal(err)
	}
	pathTrust, err := NewTrustBundle(WithCertPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if pemTrust.SHA256() != trust.SHA256() || pathTrust.SHA256() != trust.SHA256() {
		t.Fatal("trust bundle PEM and path sources differ")
	}
}

func TestTrustBundleRejectsNonRootCertificates(t *testing.T) {
	issued, _ := newIssuedBundle(t)
	for name, cert := range map[string]*x509.Certificate{
		"leaf":         issued.Certificate(),
		"intermediate": issued.Intermediates()[0],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewTrustBundle(WithCert(cert)); err == nil {
				t.Fatalf("accepted %s as a trust root", name)
			}
		})
	}
}
