package certs

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"

	ctext "github.com/gxmmx/compage-go/text"
)

type Identity struct {
	Organization       string `json:"organization,omitempty"`
	OrganizationalUnit string `json:"organizational_unit,omitempty"`
	Country            string `json:"country,omitempty"`
	Province           string `json:"province,omitempty"`
	Locality           string `json:"locality,omitempty"`
	CommonName         string `json:"common_name,omitempty"`
}

// NewIdentity normalizes and validates an identity.
func NewIdentity(identity Identity) (Identity, error) { return normalizeIdentity(identity) }

func normalizeIdentity(v Identity) (Identity, error) {
	v.Organization = strings.TrimSpace(v.Organization)
	v.OrganizationalUnit = strings.TrimSpace(v.OrganizationalUnit)
	v.Country = strings.TrimSpace(v.Country)
	v.Province = strings.TrimSpace(v.Province)
	v.Locality = strings.TrimSpace(v.Locality)
	v.CommonName = strings.TrimSpace(v.CommonName)
	fields := []struct {
		name, value string
		max         int
	}{
		{"organization", v.Organization, 128}, {"organizational unit", v.OrganizationalUnit, 128},
		{"country", v.Country, 2}, {"province", v.Province, 128},
		{"locality", v.Locality, 128}, {"common name", v.CommonName, 253},
	}
	for _, f := range fields {
		if strings.IndexFunc(f.value, unicode.IsControl) >= 0 {
			return Identity{}, invalid(f.name+" contains a control character", nil)
		}
		if len(f.value) > f.max {
			return Identity{}, invalid(f.name+" is too long", nil)
		}
	}
	if v.Country != "" && (len(v.Country) != 2 || !asciiLetters(v.Country)) {
		return Identity{}, invalid("country must contain exactly two characters", nil)
	}
	return v, nil
}

func normalizeDisplayName(name string) (string, error) {
	for _, r := range name {
		if r != ' ' && (r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f') {
			return "", invalid("display name may contain spaces but no other whitespace", nil)
		}
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", invalid("display name is required", nil)
	}
	for _, r := range name {
		if !(r == ' ' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return "", invalid("display name may contain only ASCII letters, digits, and spaces", nil)
		}
	}
	if ctext.Slugify(name, "-", ctext.Lower) == "" {
		return "", invalid("display name has an empty storage identifier", nil)
	}
	return name, nil
}

type KeySpec string

const (
	ECDSAP256 KeySpec = "ecdsa-p256"
	ECDSAP384 KeySpec = "ecdsa-p384"
	Ed25519   KeySpec = "ed25519"
	RSA3072   KeySpec = "rsa-3072"
	RSA4096   KeySpec = "rsa-4096"
)

func (s KeySpec) valid() bool {
	return s == ECDSAP256 || s == ECDSAP384 || s == Ed25519 || s == RSA3072 || s == RSA4096
}

type IssuanceMode string

const (
	None   IssuanceMode = "none"
	Ledger IssuanceMode = "ledger"
)

func (m IssuanceMode) valid() bool { return m == None || m == Ledger }

type Profile string

const (
	TLSClient       Profile = "tls-client"
	TLSServer       Profile = "tls-server"
	TLSClientServer Profile = "tls-client-server"
)

func (p Profile) valid() bool { return p == TLSClient || p == TLSServer || p == TLSClientServer }
func (p Profile) usages() (x509.KeyUsage, []x509.ExtKeyUsage) {
	switch p {
	case TLSClient:
		return x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	case TLSServer:
		return x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	case TLSClientServer:
		return x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	default:
		return 0, nil
	}
}

type SANs struct {
	DNSNames       []string   `json:"dns_names,omitempty"`
	IPAddresses    []net.IP   `json:"ip_addresses,omitempty"`
	URIs           []*url.URL `json:"uris,omitempty"`
	EmailAddresses []string   `json:"email_addresses,omitempty"`
}

func (s SANs) clone() SANs {
	r := SANs{DNSNames: append([]string(nil), s.DNSNames...), EmailAddresses: append([]string(nil), s.EmailAddresses...)}
	for _, ip := range s.IPAddresses {
		r.IPAddresses = append(r.IPAddresses, append(net.IP(nil), ip...))
	}
	for _, u := range s.URIs {
		if u != nil {
			c := *u
			r.URIs = append(r.URIs, &c)
		} else {
			r.URIs = append(r.URIs, nil)
		}
	}
	return r
}

func normalizeSANs(s SANs) (SANs, error) {
	r := s.clone()
	seen := map[string]bool{}
	for i, d := range r.DNSNames {
		d = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d), "."))
		r.DNSNames[i] = d
		if !validDNSName(d, true) {
			return SANs{}, invalid("invalid DNS SAN", nil)
		}
		k := "d:" + d
		if seen[k] {
			return SANs{}, invalid("duplicate DNS SAN", nil)
		}
		seen[k] = true
	}
	for i, ip := range r.IPAddresses {
		if ip == nil || ip.To16() == nil {
			return SANs{}, invalid("invalid IP SAN", nil)
		}
		r.IPAddresses[i] = append(net.IP(nil), ip...)
		k := "i:" + ip.String()
		if seen[k] {
			return SANs{}, invalid("duplicate IP SAN", nil)
		}
		seen[k] = true
	}
	for _, u := range r.URIs {
		if u == nil || u.Scheme == "" || u.String() == "" || strings.IndexFunc(u.String(), unicode.IsControl) >= 0 {
			return SANs{}, invalid("invalid URI SAN", nil)
		}
		k := "u:" + u.String()
		if seen[k] {
			return SANs{}, invalid("duplicate URI SAN", nil)
		}
		seen[k] = true
	}
	for i, e := range r.EmailAddresses {
		e = strings.TrimSpace(e)
		r.EmailAddresses[i] = e
		at := strings.LastIndexByte(e, '@')
		if at <= 0 || at == len(e)-1 || strings.ContainsAny(e, " \x00\r\n") || !validDNSName(e[at+1:], false) {
			return SANs{}, invalid("invalid email SAN", nil)
		}
		k := "e:" + strings.ToLower(e)
		if seen[k] {
			return SANs{}, invalid("duplicate email SAN", nil)
		}
		seen[k] = true
	}
	return r, nil
}

func (s SANs) empty() bool {
	return len(s.DNSNames)+len(s.IPAddresses)+len(s.URIs)+len(s.EmailAddresses) == 0
}

type SANPolicy struct {
	DNSSuffixes  []string     `json:"dns_suffixes,omitempty"`
	IPRanges     []*net.IPNet `json:"ip_ranges,omitempty"`
	URIPrefixes  []string     `json:"uri_prefixes,omitempty"`
	EmailDomains []string     `json:"email_domains,omitempty"`
}

type IssuerDefinition struct {
	Name            string        `json:"name"`
	Slug            string        `json:"slug"`
	Validity        time.Duration `json:"validity"`
	AllowedProfiles []Profile     `json:"allowed_profiles"`
	SubjectOwner    string        `json:"subject_owner,omitempty"`
	RequiredSubject Identity      `json:"required_subject,omitempty"`
	SANPolicy       SANPolicy     `json:"san_policy,omitempty"`
}

// NewIssuerDefinition creates an immutable issuer policy declaration.
func NewIssuerDefinition(name string, opts ...Option) (IssuerDefinition, error) {
	n, err := normalizeDisplayName(name)
	if err != nil {
		return IssuerDefinition{}, err
	}
	o, err := parseOptions(scopeIssuerDefinition, opts)
	if err != nil {
		return IssuerDefinition{}, err
	}
	return finishIssuerDefinition(n, o)
}

// NewIssuer is a concise alias for NewIssuerDefinition.
func NewIssuer(name string, opts ...Option) (IssuerDefinition, error) {
	return NewIssuerDefinition(name, opts...)
}

func finishIssuerDefinition(name string, o optionValues) (IssuerDefinition, error) {
	d := IssuerDefinition{Name: name, Slug: ctext.Slugify(name, "-", ctext.Lower), Validity: 2 * 365 * 24 * time.Hour, AllowedProfiles: []Profile{TLSClient, TLSServer, TLSClientServer}}
	if o.issuerValiditySet {
		if o.issuerValidity <= 0 {
			return d, invalid("issuer validity must be positive", nil)
		}
		d.Validity = o.issuerValidity
	}
	if o.profilesSet {
		if len(o.profiles) == 0 {
			return d, invalid("at least one allowed profile is required", nil)
		}
		d.AllowedProfiles = append([]Profile(nil), o.profiles...)
	}
	for _, p := range d.AllowedProfiles {
		if !p.valid() {
			return d, invalid("unsupported profile", nil)
		}
	}
	seenProfiles := map[Profile]bool{}
	for _, p := range d.AllowedProfiles {
		if seenProfiles[p] {
			return d, invalid("duplicate allowed profile", nil)
		}
		seenProfiles[p] = true
	}
	d.SubjectOwner = strings.TrimSpace(o.subjectOwner)
	if strings.ContainsAny(d.SubjectOwner, "\x00\r\n") {
		return d, invalid("invalid subject owner", nil)
	}
	if o.requiredSubjectSet {
		var err error
		d.RequiredSubject, err = normalizeIdentity(o.requiredSubject)
		if err != nil {
			return d, err
		}
	}
	d.SANPolicy = cloneSANPolicy(o.sanPolicy)
	for i, suffix := range d.SANPolicy.DNSSuffixes {
		suffix = strings.ToLower(strings.Trim(strings.TrimSpace(suffix), "."))
		if !validDNSName(suffix, false) {
			return d, invalid("invalid DNS policy suffix", nil)
		}
		d.SANPolicy.DNSSuffixes[i] = suffix
	}
	for _, network := range d.SANPolicy.IPRanges {
		if network == nil || network.IP == nil {
			return d, invalid("invalid IP policy range", nil)
		}
	}
	for _, prefix := range d.SANPolicy.URIPrefixes {
		u, err := url.Parse(prefix)
		if err != nil || u.Scheme == "" {
			return d, invalid("invalid URI policy prefix", err)
		}
	}
	for i, domain := range d.SANPolicy.EmailDomains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" || strings.ContainsAny(domain, " @\x00\r\n") {
			return d, invalid("invalid email policy domain", nil)
		}
		d.SANPolicy.EmailDomains[i] = domain
	}
	return d, nil
}

func asciiLetters(v string) bool {
	for _, r := range v {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}
func validDNSName(name string, wildcard bool) bool {
	if wildcard && strings.HasPrefix(name, "*.") {
		name = strings.TrimPrefix(name, "*.")
	} else if strings.Contains(name, "*") {
		return false
	}
	if name == "" || len(name) > 253 || strings.Contains(name, "..") {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

func cloneSANPolicy(v SANPolicy) SANPolicy {
	out := SANPolicy{DNSSuffixes: append([]string(nil), v.DNSSuffixes...), URIPrefixes: append([]string(nil), v.URIPrefixes...), EmailDomains: append([]string(nil), v.EmailDomains...)}
	for _, network := range v.IPRanges {
		if network == nil {
			out.IPRanges = append(out.IPRanges, nil)
			continue
		}
		out.IPRanges = append(out.IPRanges, &net.IPNet{IP: append(net.IP(nil), network.IP...), Mask: append(net.IPMask(nil), network.Mask...)})
	}
	return out
}

type EnsureResult struct {
	Created        bool
	Pending        bool
	RootGeneration int
	Revision       uint64
	Issuers        []string
}
type PromotionResult struct {
	RootGeneration int
	Revision       uint64
}

type Inspection struct {
	AuthorityID           string
	Revision              uint64
	AuthorityName         string
	ActiveRoot            int
	ActiveRootFingerprint string
	PendingKind           string
	Issuers               map[string]int
}

type LedgerRecord struct {
	AuthorityRevision uint64    `json:"authority_revision"`
	Fingerprint       string    `json:"fingerprint"`
	Issuer            string    `json:"issuer"`
	IssuerVersion     int       `json:"issuer_version"`
	Serial            string    `json:"serial"`
	CertificateDER    []byte    `json:"certificate_der"`
	Subject           Identity  `json:"subject"`
	SANs              SANs      `json:"sans"`
	Profile           Profile   `json:"profile"`
	IssuedAt          time.Time `json:"issued_at"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func (s SANs) MarshalJSON() ([]byte, error) {
	type wire struct{ DNSNames, IPAddresses, URIs, EmailAddresses []string }
	w := wire{DNSNames: s.DNSNames, EmailAddresses: s.EmailAddresses}
	for _, ip := range s.IPAddresses {
		w.IPAddresses = append(w.IPAddresses, ip.String())
	}
	for _, u := range s.URIs {
		if u != nil {
			w.URIs = append(w.URIs, u.String())
		}
	}
	return json.Marshal(w)
}
func (s *SANs) UnmarshalJSON(b []byte) error {
	type wire struct{ DNSNames, IPAddresses, URIs, EmailAddresses []string }
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	s.DNSNames = w.DNSNames
	s.EmailAddresses = w.EmailAddresses
	for _, v := range w.IPAddresses {
		ip := net.ParseIP(v)
		if ip == nil {
			return fmt.Errorf("invalid IP %q", v)
		}
		s.IPAddresses = append(s.IPAddresses, ip)
	}
	for _, v := range w.URIs {
		u, err := url.Parse(v)
		if err != nil {
			return err
		}
		s.URIs = append(s.URIs, u)
	}
	return nil
}
