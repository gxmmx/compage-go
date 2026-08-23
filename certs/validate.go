package certs

import (
	"bytes"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

func validateState(s *authorityState) error {
	if s == nil || s.Spec.ID == "" {
		return corrupt("missing authority specification", nil)
	}
	if s.Spec.FormatVersion != stateFormatVersion {
		return &UnavailableError{Message: "unsupported authority format version"}
	}
	if !s.Spec.KeySpec.valid() || !s.Spec.IssuanceMode.valid() || !s.Spec.EncryptedRootKeys {
		return corrupt("invalid authority specification", nil)
	}
	name, err := normalizeDisplayName(s.Spec.Name)
	if err != nil || name != s.Spec.Name {
		return corrupt("invalid stored authority name", err)
	}
	identity, err := normalizeIdentity(s.Spec.Identity)
	if err != nil || identity != s.Spec.Identity {
		return corrupt("invalid stored authority identity", err)
	}
	if s.ActiveRoot < 1 || s.Roots[s.ActiveRoot] == nil {
		return corrupt("missing active root", nil)
	}
	for generation, r := range s.Roots {
		if r == nil || r.Generation != generation {
			return corrupt("invalid root generation record", nil)
		}
		c, err := parseCertDER(r.CertificateDER)
		if err != nil {
			return err
		}
		if !c.IsCA || !c.BasicConstraintsValid || c.MaxPathLen != 1 || c.KeyUsage&x509.KeyUsageCertSign == 0 || c.KeyUsage&x509.KeyUsageCRLSign == 0 || c.CheckSignatureFrom(c) != nil {
			return corrupt("invalid root certificate", nil)
		}
		if fingerprint(c) != r.Fingerprint {
			return &DriftError{Field: "root fingerprint", Expected: r.Fingerprint, Actual: fingerprint(c)}
		}
		keySpec, keyErr := keySpecForPublic(c.PublicKey)
		if keyErr != nil || keySpec != s.Spec.KeySpec {
			return corrupt("root key specification mismatch", keyErr)
		}
		requiresKey := generation == s.ActiveRoot || (s.Pending != nil && s.Pending.Kind == "root" && generation == s.Pending.RootGeneration)
		if requiresKey && len(r.EncryptedKeyDER) == 0 {
			return corrupt("active or pending root key is missing", nil)
		}
		if !requiresKey && len(r.EncryptedKeyDER) > 0 {
			return corrupt("retired root private key was retained", nil)
		}
		expected := rootCommonName(s.Spec.Name, generation)
		if c.Subject.CommonName != expected || identityFromCAName(c.Subject) != s.Spec.Identity {
			return &DriftError{Field: "root subject", Expected: expected, Actual: c.Subject.String()}
		}
	}
	for slug, issuer := range s.Issuers {
		if issuer == nil || issuer.Definition.Slug != slug || issuer.ActiveVersion < 1 || issuer.Versions[issuer.ActiveVersion] == nil {
			return corrupt("invalid issuer topology", nil)
		}
		normalized, definitionErr := validateDefinition(issuer.Definition, issuer.Definition.Name)
		if definitionErr != nil || !definitionsEqual(normalized, issuer.Definition) {
			return corrupt("invalid stored issuer definition", definitionErr)
		}
		for version, v := range issuer.Versions {
			if v == nil || v.Version != version {
				return corrupt("invalid issuer version record", nil)
			}
			c, err := parseCertDER(v.CertificateDER)
			if err != nil {
				return err
			}
			root := s.Roots[v.RootGeneration]
			if root == nil {
				return corrupt("issuer refers to missing root", nil)
			}
			rc, _ := parseCertDER(root.CertificateDER)
			if c.CheckSignatureFrom(rc) != nil || !c.IsCA || !c.BasicConstraintsValid || c.MaxPathLen != 0 || !c.MaxPathLenZero || c.KeyUsage&x509.KeyUsageCertSign == 0 {
				return corrupt("invalid issuer certificate", nil)
			}
			if c.Subject.CommonName != issuerCommonName(s.Spec.Name, v.RootGeneration, issuer.Definition.Name, version) {
				return &DriftError{Field: "issuer subject", Expected: issuer.Definition.Name, Actual: c.Subject.CommonName}
			}
			if fingerprint(c) != v.Fingerprint {
				return &DriftError{Field: "issuer fingerprint", Expected: v.Fingerprint, Actual: fingerprint(c)}
			}
			keySpec, keyErr := keySpecForPublic(c.PublicKey)
			if keyErr != nil || keySpec != s.Spec.KeySpec {
				return corrupt("issuer key specification mismatch", keyErr)
			}
			if c.NotAfter.After(rc.NotAfter.Add(-clockSkew)) {
				return corrupt("issuer validity exceeds its root", nil)
			}
			requiresKey := version == issuer.ActiveVersion || (s.Pending != nil && s.Pending.Issuers[slug] == version)
			if requiresKey && len(v.KeyDER) == 0 {
				return corrupt("active or pending issuer key is missing", nil)
			}
			if !requiresKey && len(v.KeyDER) > 0 {
				return corrupt("retired issuer private key was retained", nil)
			}
		}
		if issuer.Versions[issuer.ActiveVersion].RootGeneration != s.ActiveRoot {
			return corrupt("active issuer is not under the active root", nil)
		}
		active := issuer.Versions[issuer.ActiveVersion]
		if len(active.KeyDER) == 0 {
			return corrupt("active issuer key is missing", nil)
		}
		key, err := parseKeyDER(active.KeyDER, nil)
		if err != nil {
			return err
		}
		cert, _ := parseCertDER(active.CertificateDER)
		if !publicKeysEqual(key.Public(), cert.PublicKey) {
			return corrupt("issuer key does not match certificate", nil)
		}
	}
	if s.Pending != nil {
		if s.Pending.ExpectedRevision > s.Revision {
			return corrupt("pending revision is impossible", nil)
		}
		if s.Pending.Kind != "root" && s.Pending.Kind != "issuer" {
			return corrupt("unknown pending operation", nil)
		}
		if s.Pending.Kind == "root" && s.Roots[s.Pending.RootGeneration] == nil {
			return corrupt("pending root is missing", nil)
		}
		if s.Pending.Kind == "root" && len(s.Pending.Issuers) != len(s.Issuers) {
			return corrupt("pending root does not cascade every issuer", nil)
		}
		for slug, version := range s.Pending.Issuers {
			if s.Issuers[slug] == nil || s.Issuers[slug].Versions[version] == nil {
				return corrupt("pending issuer is missing", nil)
			}
		}
	}
	return nil
}
func validateLedgerRecord(r *LedgerRecord) error {
	if r == nil || r.AuthorityRevision == 0 || r.Fingerprint == "" || r.Issuer == "" || r.IssuerVersion < 1 || r.Serial == "" || len(r.CertificateDER) == 0 {
		return corrupt("incomplete ledger record", nil)
	}
	c, err := parseCertDER(r.CertificateDER)
	if err != nil {
		return err
	}
	if fingerprint(c) != r.Fingerprint {
		return corrupt("ledger fingerprint mismatch", nil)
	}
	if c.SerialNumber.Text(16) != strings.ToLower(r.Serial) {
		return corrupt("ledger serial mismatch", nil)
	}
	if _, err = hex.DecodeString(r.Fingerprint); err != nil {
		return corrupt("invalid ledger fingerprint", err)
	}
	if !r.Profile.valid() || r.IssuedAt.IsZero() || r.ExpiresAt.IsZero() || !c.NotAfter.Equal(r.ExpiresAt) || !c.NotBefore.Add(clockSkew).Equal(r.IssuedAt) {
		return corrupt("ledger certificate metadata mismatch", nil)
	}
	if identityFromName(c.Subject) != r.Subject {
		return corrupt("ledger subject mismatch", nil)
	}
	certSANs, sanErr := normalizeSANsAllowDuplicates(SANs{DNSNames: c.DNSNames, IPAddresses: c.IPAddresses, URIs: c.URIs, EmailAddresses: c.EmailAddresses})
	if sanErr != nil {
		return corrupt("invalid ledger certificate SAN", sanErr)
	}
	recordSANs, sanErr := normalizeSANsAllowDuplicates(r.SANs)
	if sanErr != nil {
		return corrupt("invalid ledger SAN record", sanErr)
	}
	left, _ := json.Marshal(certSANs)
	right, _ := json.Marshal(recordSANs)
	if !bytes.Equal(left, right) {
		return corrupt("ledger SAN mismatch", nil)
	}
	usage, extended := r.Profile.usages()
	if c.KeyUsage != usage || !slices.Equal(c.ExtKeyUsage, extended) {
		return corrupt("ledger profile usage mismatch", nil)
	}
	return nil
}

func validateLedgerAuthority(state *authorityState, r *LedgerRecord) error {
	if state == nil || r == nil {
		return corrupt("missing ledger authority state", nil)
	}
	if state.Revision != r.AuthorityRevision {
		return &ConflictError{Message: "authority revision changed during signing"}
	}
	issuer := state.Issuers[r.Issuer]
	if issuer == nil || issuer.ActiveVersion != r.IssuerVersion {
		return &ConflictError{Message: "active issuer changed during signing"}
	}
	return validateLedgerRecord(r)
}

func validateLedgerRecords(state *authorityState, records []LedgerRecord) error {
	for i := range records {
		r := &records[i]
		if r.AuthorityRevision > state.Revision {
			return corrupt("ledger record has a future authority revision", nil)
		}
		issuer := state.Issuers[r.Issuer]
		if issuer == nil {
			return corrupt("ledger refers to unknown issuer", nil)
		}
		version := issuer.Versions[r.IssuerVersion]
		if version == nil {
			return corrupt("ledger refers to unknown issuer version", nil)
		}
		cert, err := parseCertDER(r.CertificateDER)
		if err != nil {
			return err
		}
		parent, err := parseCertDER(version.CertificateDER)
		if err != nil {
			return err
		}
		if cert.CheckSignatureFrom(parent) != nil {
			return corrupt("ledger certificate signature mismatch", nil)
		}
	}
	return nil
}
