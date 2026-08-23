package certs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteStoreAndEncryptedCSRKey(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "authority.sqlite")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := NewIssuerDefinition("Server", WithAllowedProfiles(TLSServer))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("SQLite CA"), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	csr, err := NewCSR(WithSubject(Identity{CommonName: "server"}), WithKeyPassphrase([]byte("test passphrase")))
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "key.pem")
	if err = csr.SaveKey(keyPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBundle(WithKey(keyPath), WithKeyPassphrase([]byte("test passphrase")))
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasKey() || loaded.Certificate() != nil {
		t.Fatal("expected a key-only bundle")
	}
	var mode string
	if err = store.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "delete" {
		t.Fatalf("journal mode = %q", mode)
	}
}

func TestSQLiteLedgerUsesSeparateDatabase(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := NewSQLiteStore(filepath.Join(directory, "authority.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	definition, err := NewIssuerDefinition("Ledger Issuer", WithAllowedProfiles(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Ledger Authority"), WithIssuanceMode(Ledger), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(store.ledgerPath()); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuerManager(WithStore(store), WithIssuerName("Ledger Issuer"))
	if err != nil {
		t.Fatal(err)
	}
	csr, err := NewCSR(WithSubject(Identity{CommonName: "device"}))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := issuer.SignCSR(ctx, csr.CSRPEM(), WithProfile(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = issuer.LookupCertificate(ctx, 1, bundle.Certificate().SerialNumber.Text(16)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = store.db.QueryRow(`SELECT count(*) FROM root_generations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("root table count = %d", count)
	}
}
