package certs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRejectsSymlinkAuthorityPath(t *testing.T) {
	owner, group := currentPrincipal(t)
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(link, WithOwner(owner), WithGroup(group)); err == nil {
		t.Fatal("symlink FileStore path was accepted")
	}
}

func TestFileStoreNoneModeDoesNotCreateLedger(t *testing.T) {
	store := newTestFileStore(t)
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("No Ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(store.directory, "ledger")); !os.IsNotExist(err) {
		t.Fatalf("ledger exists in None mode: %v", err)
	}
}
