package certs

import (
	"context"
	"encoding/pem"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

func currentPrincipal(t *testing.T) (string, string) {
	t.Helper()
	if !splitSecuritySupported() {
		t.Skip("split-principal FileStore is unsupported on this platform")
	}
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	g, err := user.LookupGroupId(strconv.Itoa(effectiveGID()))
	if err != nil {
		t.Fatal(err)
	}
	return u.Username, g.Name
}

func newTestFileStore(t *testing.T) *FileStore {
	t.Helper()
	owner, group := currentPrincipal(t)
	directory := filepath.Join(t.TempDir(), "authority")
	if err := os.Mkdir(directory, 0750); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(directory, WithOwner(owner), WithGroup(group))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func countPEMCerts(data []byte) int {
	count := 0
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			return count
		}
		if block.Type == "CERTIFICATE" {
			count++
		}
		data = rest
	}
	return count
}

func newIssuedBundle(t *testing.T) (*CertificateBundle, *CSRBundle) {
	t.Helper()
	store := newTestFileStore(t)
	definition, err := NewIssuerDefinition("Test Issuer")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Test Authority"), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authority.Close() })
	if _, err = authority.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuerManager(WithStore(store), WithIssuerName("Test Issuer"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = issuer.Close() })
	csr, err := NewCSR(WithSubject(Identity{CommonName: "test-leaf"}), WithSANs(SANs{DNSNames: []string{"test.example"}}))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := issuer.SignCSR(context.Background(), csr.CSRPEM(), WithProfile(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	return bundle, csr
}
