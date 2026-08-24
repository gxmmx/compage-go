package certs

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLifecycleLogging(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store := newTestFileStore(t)
	definition, err := NewIssuerDefinition("Enrollment", WithAllowedProfiles(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthorityManager(
		WithStore(store),
		WithLogger(logger),
		WithAuthorityName("Logged Authority"),
		WithRootKeySpec(Ed25519),
		WithIssuer(definition),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authority.Close() })

	ctx := context.Background()
	if _, err = authority.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuerManager(WithStore(store), WithLogger(logger), WithIssuerName("Enrollment"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = issuer.Close() })
	csr, err := NewCSR(WithLogger(logger), WithSubject(Identity{CommonName: "device"}), WithKeySpec(Ed25519))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = issuer.SignCSR(ctx, csr.CSRPEM(), WithProfile(TLSClient)); err != nil {
		t.Fatal(err)
	}

	if _, err = authority.Ensure(ctx, WithIssuerRotateBefore("Enrollment", 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx, WithRootRotateBefore(0)); err != nil {
		t.Fatal(err)
	}
	if err = authority.DiscardPending(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(ctx, WithRootRotateBefore(0)); err != nil {
		t.Fatal(err)
	}
	if _, err = authority.PromotePending(ctx); err != nil {
		t.Fatal(err)
	}

	for _, event := range []string{
		`msg="key created" key_type=ed25519 purpose=root generation=1`,
		`msg="root certificate created" generation=1 pending=false`,
		`msg="issuer certificate created" issuer=Enrollment generation=1 version=1 pending=false`,
		`msg="leaf certificate issued" issuer=Enrollment version=1 subject_common_name=device profile=tls-client`,
		`msg="issuer certificate promoted" issuer=Enrollment version=2 generation=1`,
		`msg="pending root certificate discarded" generation=2`,
		`msg="root certificate promoted" generation=2`,
	} {
		if !strings.Contains(output.String(), event) {
			t.Errorf("log output missing %q:\n%s", event, output.String())
		}
	}
}

func TestNilLoggerDoesNotLog(t *testing.T) {
	if _, err := NewCSR(WithLogger(nil), WithSubject(Identity{CommonName: "device"})); err != nil {
		t.Fatal(err)
	}
}
