package certs

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"time"
)

var errNoMutation = errors.New("certs: no state mutation")

const stateFormatVersion = 1

type authoritySpec struct {
	ID                string        `json:"id"`
	FormatVersion     int           `json:"format_version"`
	Name              string        `json:"name"`
	Identity          Identity      `json:"identity"`
	RootValidity      time.Duration `json:"root_validity"`
	KeySpec           KeySpec       `json:"key_spec"`
	EncryptedRootKeys bool          `json:"encrypted_root_keys"`
	IssuanceMode      IssuanceMode  `json:"issuance_mode"`
	Owner             string        `json:"owner,omitempty"`
	Group             string        `json:"group,omitempty"`
	Backend           string        `json:"backend"`
}
type rootRecord struct {
	Generation        int       `json:"generation"`
	CertificateDER    []byte    `json:"certificate_der"`
	EncryptedKeyDER   []byte    `json:"encrypted_key_der,omitempty"`
	Fingerprint       string    `json:"fingerprint"`
	RetiredTrustUntil time.Time `json:"retired_trust_until,omitempty"`
}
type issuerVersion struct {
	Version        int    `json:"version"`
	RootGeneration int    `json:"root_generation"`
	CertificateDER []byte `json:"certificate_der"`
	KeyDER         []byte `json:"key_der,omitempty"`
	Fingerprint    string `json:"fingerprint"`
}
type issuerRecord struct {
	Definition    IssuerDefinition       `json:"definition"`
	ActiveVersion int                    `json:"active_version"`
	Versions      map[int]*issuerVersion `json:"versions"`
}
type pendingManifest struct {
	Kind             string         `json:"kind"`
	ExpectedRevision uint64         `json:"expected_revision"`
	RootGeneration   int            `json:"root_generation,omitempty"`
	Issuers          map[string]int `json:"issuers"`
	CreatedAt        time.Time      `json:"created_at"`
}
type authorityState struct {
	Spec       authoritySpec            `json:"spec"`
	Revision   uint64                   `json:"revision"`
	ActiveRoot int                      `json:"active_root"`
	Roots      map[int]*rootRecord      `json:"roots"`
	Issuers    map[string]*issuerRecord `json:"issuers"`
	Pending    *pendingManifest         `json:"pending,omitempty"`
}

func (s *authorityState) clone() (*authorityState, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var out authorityState
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type backend interface {
	load(context.Context, bool) (*authorityState, error)
	update(context.Context, func(*authorityState) error) error
	purge(context.Context) error
	loadUnlocks(context.Context) (map[int][]byte, error)
	storeUnlocks(context.Context, map[int][]byte, bool) error
	appendLedger(context.Context, LedgerRecord) error
	lookupLedger(context.Context, string, int, string) (*LedgerRecord, error)
	listLedger(context.Context, string, int) ([]LedgerRecord, string, error)
	kind() string
	principal() (string, string)
}

type securedBackend interface {
	authorizeOwner() error
	authorizeIssuer() error
}

func parseCertDER(der []byte) (*x509.Certificate, error) {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, corrupt("invalid certificate DER", err)
	}
	return c, nil
}
