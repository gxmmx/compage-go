package certs

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestConcurrentLedgerSigning(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	definition, err := NewIssuerDefinition("Concurrent", WithAllowedProfiles(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Concurrent Authority"), WithIssuanceMode(Ledger), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if err = authority.StoreLocalUnlock(); err != nil {
		t.Fatal(err)
	}
	before, err := authority.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := authority.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision {
		t.Fatalf("idempotent Ensure changed revision from %d to %d", before.Revision, after.Revision)
	}
	csr, err := NewCSR(WithSubject(Identity{CommonName: "device"}))
	if err != nil {
		t.Fatal(err)
	}
	const count = 6
	managers := make([]*IssuerManager, count)
	for i := range managers {
		managers[i], err = NewIssuerManager(WithStore(store), WithIssuerName("Concurrent"))
		if err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	failures := make(chan error, count)
	for _, manager := range managers {
		wg.Add(1)
		go func(m *IssuerManager) {
			defer wg.Done()
			_, signErr := m.SignCSR(ctx, csr.CSRPEM(), WithProfile(TLSClient))
			failures <- signErr
		}(manager)
	}
	wg.Wait()
	close(failures)
	for signErr := range failures {
		if signErr != nil {
			t.Fatal(signErr)
		}
	}
	records, next, err := managers[0].ListCertificates(ctx, "", count+1)
	if err != nil {
		t.Fatal(err)
	}
	if next != "" || len(records) != count {
		t.Fatalf("ledger records = %d, next = %q", len(records), next)
	}
}

func TestFileStoreLockHonorsContextCancellation(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Lock Authority"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if err = authority.StoreLocalUnlock(); err != nil {
		t.Fatal(err)
	}
	lock, err := openNoFollow(store.lockPath(), os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = acquireFileReadLock(ctx, lock); err != nil {
		t.Fatal(err)
	}
	defer releaseFileLock(lock)
	waitCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer cancel()
	_, err = authority.Ensure(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
