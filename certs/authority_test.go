package certs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"testing"
)

func TestFileStoreLifecycleAndSigning(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	definition, err := NewIssuerDefinition("Enrollment", WithAllowedProfiles(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Acme Product"), WithIdentity(Identity{Organization: "Acme", Country: "IS"}), WithIssuanceMode(Ledger), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = authority.Close() }()
	result, err := authority.Ensure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.RootGeneration != 1 {
		t.Fatalf("unexpected ensure result: %+v", result)
	}
	if err = authority.StoreLocalUnlock(); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuerManager(WithStore(store), WithIssuerName("Enrollment"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = issuer.Close() }()
	if pending, err := authority.IsPending(); err != nil || pending {
		t.Fatalf("authority unexpectedly pending: %v, %v", pending, err)
	}
	if pending, err := issuer.IsPending(); err != nil || pending {
		t.Fatalf("issuer unexpectedly pending: %v, %v", pending, err)
	}
	initialTrust, err := authority.TrustBundlePEM()
	if err != nil {
		t.Fatal(err)
	}
	issuerTrust, err := issuer.TrustBundlePEM()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(initialTrust, issuerTrust) {
		t.Fatal("authority and issuer trust bundles differ")
	}
	sum := sha256.Sum256(initialTrust)
	wantTrustHash := hex.EncodeToString(sum[:])
	if got, err := authority.TrustBundleSHA256(); err != nil || got != wantTrustHash {
		t.Fatalf("unexpected authority trust bundle hash: %q, %v", got, err)
	}
	if got, err := issuer.TrustBundleSHA256(); err != nil || got != wantTrustHash {
		t.Fatalf("unexpected issuer trust bundle hash: %q, %v", got, err)
	}
	status, err := authority.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Pending || status.PendingKind != PendingNone || status.TrustBundleSHA256 != wantTrustHash {
		t.Fatalf("unexpected authority status: %+v", status)
	}
	issuerStatus, err := issuer.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if issuerStatus.Pending || issuerStatus.PendingKind != PendingNone || issuerStatus.TrustBundleSHA256 != wantTrustHash {
		t.Fatalf("unexpected issuer status: %+v", issuerStatus)
	}
	csr, err := NewCSR(WithSubject(Identity{Organization: "Acme", CommonName: "device-1"}), WithSANs(SANs{DNSNames: []string{"device-1.example.test"}}), WithKeySpec(Ed25519))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := issuer.SignCSR(ctx, csr.CSRPEM(), WithProfile(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	if !bundle.Complete() || bundle.HasKey() {
		t.Fatal("unexpected issued bundle")
	}
	roots := x509.NewCertPool()
	roots.AddCert(bundle.Root())
	if err = bundle.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatal(err)
	}
	record, err := issuer.LookupCertificate(ctx, 1, bundle.Certificate().SerialNumber.Text(16))
	if err != nil {
		t.Fatal(err)
	}
	if record.Fingerprint != fingerprint(bundle.Certificate()) {
		t.Fatal("ledger fingerprint mismatch")
	}
	prepared, err := authority.Ensure(ctx, WithIssuerRotateBefore("Enrollment", 0))
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Pending || prepared.PendingKind != PendingIssuer {
		t.Fatal("issuer rotation was not staged")
	}
	if pending, err := authority.IsPending(); err != nil || !pending {
		t.Fatalf("authority pending state = %v, %v", pending, err)
	}
	if pending, err := issuer.IsPending(); err != nil || !pending {
		t.Fatalf("issuer pending state = %v, %v", pending, err)
	}
	if got, err := issuer.TrustBundleSHA256(); err != nil || got != wantTrustHash {
		t.Fatalf("issuer-only rotation changed trust bundle hash: %q, %v", got, err)
	}
	status, err = authority.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Pending || status.PendingKind != PendingIssuer || status.TrustBundleSHA256 != wantTrustHash {
		t.Fatalf("unexpected issuer-pending authority status: %+v", status)
	}
	issuerStatus, err = issuer.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !issuerStatus.Pending || issuerStatus.PendingKind != PendingIssuer || issuerStatus.TrustBundleSHA256 != wantTrustHash {
		t.Fatalf("unexpected issuer-pending issuer status: %+v", issuerStatus)
	}
	if _, err = authority.Ensure(ctx); err == nil {
		t.Fatal("Ensure succeeded while an issuer operation was pending")
	} else {
		var pendingErr *PendingConflictError
		if !errors.As(err, &pendingErr) || pendingErr.Operation != PendingIssuer {
			t.Fatalf("expected typed issuer pending conflict, got %v", err)
		}
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err = issuer.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if pending, err := authority.IsPending(); err != nil || pending {
		t.Fatalf("authority remained pending after promotion: %v, %v", pending, err)
	}
	if pending, err := issuer.IsPending(); err != nil || pending {
		t.Fatalf("issuer remained pending after promotion: %v, %v", pending, err)
	}
	prepared, err = authority.Ensure(ctx, WithRootRotateBefore(0))
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Pending || prepared.PendingKind != PendingRoot || prepared.RootGeneration != 2 {
		t.Fatalf("root rotation not staged: %+v", prepared)
	}
	if pending, err := authority.IsPending(); err != nil || !pending {
		t.Fatalf("authority root pending state = %v, %v", pending, err)
	}
	if pending, err := issuer.IsPending(); err != nil || !pending {
		t.Fatalf("issuer root pending state = %v, %v", pending, err)
	}
	trust, err := authority.TrustBundlePEM()
	if err != nil {
		t.Fatal(err)
	}
	if countPEMCerts(trust) != 2 {
		t.Fatalf("trust bundle has %d roots", countPEMCerts(trust))
	}
	issuerTrust, err = issuer.TrustBundlePEM()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(trust, issuerTrust) {
		t.Fatal("authority and issuer pending trust bundles differ")
	}
	rootSum := sha256.Sum256(trust)
	wantRootTrustHash := hex.EncodeToString(rootSum[:])
	if wantRootTrustHash == wantTrustHash {
		t.Fatal("root rotation did not change trust bundle hash")
	}
	if got, err := authority.TrustBundleSHA256(); err != nil || got != wantRootTrustHash {
		t.Fatalf("unexpected pending authority trust bundle hash: %q, %v", got, err)
	}
	if got, err := issuer.TrustBundleSHA256(); err != nil || got != wantRootTrustHash {
		t.Fatalf("unexpected pending issuer trust bundle hash: %q, %v", got, err)
	}
	status, err = authority.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Pending || status.PendingKind != PendingRoot || status.TrustBundleSHA256 != wantRootTrustHash {
		t.Fatalf("unexpected root-pending authority status: %+v", status)
	}
	issuerStatus, err = issuer.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !issuerStatus.Pending || issuerStatus.PendingKind != PendingRoot || issuerStatus.TrustBundleSHA256 != wantRootTrustHash {
		t.Fatalf("unexpected root-pending issuer status: %+v", issuerStatus)
	}
	if _, err = authority.Ensure(ctx); err == nil {
		t.Fatal("Ensure succeeded while a root operation was pending")
	} else {
		var pendingErr *PendingConflictError
		if !errors.As(err, &pendingErr) || pendingErr.Operation != PendingRoot {
			t.Fatalf("expected typed root pending conflict, got %v", err)
		}
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err = issuer.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if pending, err := authority.IsPending(); err != nil || pending {
		t.Fatalf("authority remained pending after root promotion: %v, %v", pending, err)
	}
	if pending, err := issuer.IsPending(); err != nil || pending {
		t.Fatalf("issuer remained pending after root promotion: %v, %v", pending, err)
	}
}

func TestReopenedAuthorityRequiresValidRootUnlock(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Unlock Authority"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	secret, err := authority.ExportGeneratedUnlock()
	if err != nil {
		t.Fatal(err)
	}
	_ = authority.Close()
	reopened, err := NewAuthorityManager(WithStore(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.Ensure(ctx); err == nil {
		t.Fatal("Ensure succeeded without root unlock")
	} else {
		var missing *NotFoundError
		if !errors.As(err, &missing) {
			t.Fatalf("expected not found, got %v", err)
		}
	}
	wrong, err := NewAuthorityManager(WithStore(store), WithRootUnlock([]byte("wrong unlock")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = wrong.Ensure(ctx); err == nil {
		t.Fatal("Ensure succeeded with wrong unlock")
	}
	correct, err := NewAuthorityManager(WithStore(store), WithRootUnlock(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = correct.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
}
