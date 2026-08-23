package certs

import (
	"context"
	"crypto/x509"
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
	defer authority.Close()
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
	defer issuer.Close()
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
	if !prepared.Pending {
		t.Fatal("issuer rotation was not staged")
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err = issuer.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	prepared, err = authority.Ensure(ctx, WithRootRotateBefore(0))
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Pending || prepared.RootGeneration != 2 {
		t.Fatalf("root rotation not staged: %+v", prepared)
	}
	trust, err := authority.TrustBundlePEM()
	if err != nil {
		t.Fatal(err)
	}
	if countPEMCerts(trust) != 2 {
		t.Fatalf("trust bundle has %d roots", countPEMCerts(trust))
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err = issuer.Reload(ctx); err != nil {
		t.Fatal(err)
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
