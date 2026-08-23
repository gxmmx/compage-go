package certs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newPolicyIssuer(t *testing.T, definition IssuerDefinition) *IssuerManager {
	t.Helper()
	store := newTestFileStore(t)
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Policy Authority"), WithIssuer(definition))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authority.Close() })
	if _, err = authority.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuerManager(WithStore(store), WithIssuerName(definition.Name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = issuer.Close() })
	return issuer
}

func TestIssuerValidatesCSRPolicyBeforeOverrides(t *testing.T) {
	definition, err := NewIssuerDefinition("Policy", WithAllowedProfiles(TLSClient), WithSubjectOwner("Acme"), WithRequiredSubject(Identity{OrganizationalUnit: "Agents"}), WithDNSSuffixes("example.com"))
	if err != nil {
		t.Fatal(err)
	}
	issuer := newPolicyIssuer(t, definition)
	csr, err := NewCSR(WithSubject(Identity{Organization: "Wrong", OrganizationalUnit: "Agents", CommonName: "device"}), WithSANs(SANs{DNSNames: []string{"device.example.com"}}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = issuer.SignCSR(context.Background(), csr.CSRPEM(), WithProfile(TLSClient), WithSubject(Identity{Organization: "Acme", OrganizationalUnit: "Agents", CommonName: "device"}))
	var forbidden *ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("expected forbidden policy error, got %v", err)
	}
}

func TestIssuerRejectsProfileAndLifetime(t *testing.T) {
	definition, err := NewIssuerDefinition("Policy", WithAllowedProfiles(TLSClient))
	if err != nil {
		t.Fatal(err)
	}
	issuer := newPolicyIssuer(t, definition)
	csr, err := NewCSR(WithSubject(Identity{CommonName: "device"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = issuer.SignCSR(context.Background(), csr.CSRPEM(), WithProfile(TLSServer)); err == nil {
		t.Fatal("disallowed profile succeeded")
	}
	if _, err = issuer.SignCSR(context.Background(), csr.CSRPEM(), WithProfile(TLSClient), WithLifetime(366*24*time.Hour)); err == nil {
		t.Fatal("excessive lifetime succeeded")
	}
}
