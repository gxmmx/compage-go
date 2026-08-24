package certs

import (
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"time"
)

type optionScope uint64

const (
	scopeAuthority optionScope = 1 << iota
	scopeEnsure
	scopeIssuerManager
	scopeIssuerDefinition
	scopeSign
	scopeCSR
	scopeLoad
	scopeSave
	scopeStore
	scopePromote
)

// Option is accepted by certs constructors and operations. Options are strict:
// an option used with an unrelated operation is rejected.
type Option interface {
	apply(*optionValues) error
	validFor(optionScope) bool
	optionName() string
}
type option struct {
	name   string
	scopes optionScope
	fn     func(*optionValues) error
}

func (o option) apply(v *optionValues) error { return o.fn(v) }
func (o option) validFor(s optionScope) bool { return o.scopes&s != 0 }
func (o option) optionName() string          { return o.name }

type optionValues struct {
	store                                                        backend
	logger                                                       *slog.Logger
	authorityName                                                string
	authorityNameSet                                             bool
	identity                                                     Identity
	identitySet                                                  bool
	owner, group                                                 string
	ownerSet, groupSet                                           bool
	rootKeySpec                                                  KeySpec
	rootKeySpecSet                                               bool
	issuanceMode                                                 IssuanceMode
	issuanceModeSet                                              bool
	issuers                                                      []IssuerDefinition
	rootValidity                                                 time.Duration
	rootValiditySet                                              bool
	rootRotateBefore                                             time.Duration
	rootRotateBeforeSet                                          bool
	issuerRotateBefore                                           map[string]time.Duration
	issuerName                                                   string
	issuerNameSet                                                bool
	rootUnlock                                                   []byte
	rootUnlockSet                                                bool
	profile                                                      Profile
	profileSet                                                   bool
	lifetime                                                     time.Duration
	lifetimeSet                                                  bool
	subject                                                      Identity
	subjectSet                                                   bool
	sans                                                         SANs
	sansSet                                                      bool
	additionalSANs                                               SANs
	additionalSANsSet                                            bool
	keySpec                                                      KeySpec
	keySpecSet                                                   bool
	keyPassphrase                                                []byte
	keyPassphraseSet                                             bool
	certPaths                                                    []string
	keyPath                                                      string
	keyPathSet                                                   bool
	replace                                                      bool
	issuerValidity                                               time.Duration
	issuerValiditySet                                            bool
	profiles                                                     []Profile
	profilesSet                                                  bool
	subjectOwner                                                 string
	requiredSubject                                              Identity
	requiredSubjectSet                                           bool
	sanPolicy                                                    SANPolicy
	dnsSuffixesSet, ipRangesSet, uriPrefixesSet, emailDomainsSet bool
}

func parseOptions(scope optionScope, opts []Option) (optionValues, error) {
	v := optionValues{issuerRotateBefore: map[string]time.Duration{}}
	seen := map[string]bool{}
	for _, o := range opts {
		if optionNil(o) {
			return v, invalid("nil option", nil)
		}
		if !o.validFor(scope) {
			return v, invalid(o.optionName()+" is not valid for this operation", nil)
		}
		// WithCert, WithIssuer, and per-issuer rotation are intentionally repeatable.
		if seen[o.optionName()] && o.optionName() != "WithCert" && o.optionName() != "WithIssuer" && o.optionName() != "WithIssuerRotateBefore" {
			return v, invalid(o.optionName()+" specified more than once", nil)
		}
		seen[o.optionName()] = true
		if err := o.apply(&v); err != nil {
			return v, invalid(o.optionName(), err)
		}
	}
	return v, nil
}
func optionNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return (rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface || rv.Kind() == reflect.Func) && rv.IsNil()
}
func makeOption(name string, scopes optionScope, fn func(*optionValues) error) Option {
	return option{name: name, scopes: scopes, fn: fn}
}

func WithStore(store any) Option {
	return makeOption("WithStore", scopeAuthority|scopeIssuerManager, func(o *optionValues) error {
		b, ok := store.(backend)
		if !ok || optionNil(b) {
			return fmt.Errorf("supported store is required")
		}
		o.store = b
		return nil
	})
}

// WithLogger enables concise DEBUG lifecycle logging for the manager or CSR
// constructor receiving the option. A nil logger disables logging.
func WithLogger(logger *slog.Logger) Option {
	return makeOption("WithLogger", scopeAuthority|scopeIssuerManager|scopeCSR, func(o *optionValues) error {
		o.logger = logger
		return nil
	})
}
func WithAuthorityName(name string) Option {
	return makeOption("WithAuthorityName", scopeAuthority, func(o *optionValues) error { o.authorityName = name; o.authorityNameSet = true; return nil })
}
func WithIdentity(v Identity) Option {
	return makeOption("WithIdentity", scopeAuthority, func(o *optionValues) error { o.identity = v; o.identitySet = true; return nil })
}
func WithOwner(v string) Option {
	return makeOption("WithOwner", scopeAuthority|scopeStore, func(o *optionValues) error { o.owner = v; o.ownerSet = true; return nil })
}
func WithGroup(v string) Option {
	return makeOption("WithGroup", scopeAuthority|scopeStore, func(o *optionValues) error { o.group = v; o.groupSet = true; return nil })
}
func WithRootKeySpec(v KeySpec) Option {
	return makeOption("WithRootKeySpec", scopeAuthority, func(o *optionValues) error { o.rootKeySpec = v; o.rootKeySpecSet = true; return nil })
}
func WithIssuanceMode(v IssuanceMode) Option {
	return makeOption("WithIssuanceMode", scopeAuthority, func(o *optionValues) error { o.issuanceMode = v; o.issuanceModeSet = true; return nil })
}
func WithIssuer(v IssuerDefinition) Option {
	return makeOption("WithIssuer", scopeAuthority, func(o *optionValues) error { o.issuers = append(o.issuers, v); return nil })
}
func WithRootValidity(v time.Duration) Option {
	return makeOption("WithRootValidity", scopeAuthority, func(o *optionValues) error { o.rootValidity = v; o.rootValiditySet = true; return nil })
}
func WithRootRotateBefore(v time.Duration) Option {
	return makeOption("WithRootRotateBefore", scopeEnsure, func(o *optionValues) error { o.rootRotateBefore = v; o.rootRotateBeforeSet = true; return nil })
}
func WithIssuerRotateBefore(name string, v time.Duration) Option {
	return makeOption("WithIssuerRotateBefore", scopeEnsure, func(o *optionValues) error {
		if _, ok := o.issuerRotateBefore[name]; ok {
			return fmt.Errorf("issuer specified more than once")
		}
		o.issuerRotateBefore[name] = v
		return nil
	})
}
func WithIssuerName(v string) Option {
	return makeOption("WithIssuerName", scopeIssuerManager, func(o *optionValues) error { o.issuerName = v; o.issuerNameSet = true; return nil })
}
func WithRootUnlock(v []byte) Option {
	return makeOption("WithRootUnlock", scopeAuthority|scopePromote, func(o *optionValues) error {
		if len(v) == 0 {
			return fmt.Errorf("unlock secret is empty")
		}
		o.rootUnlock = append([]byte(nil), v...)
		o.rootUnlockSet = true
		return nil
	})
}
func WithProfile(v Profile) Option {
	return makeOption("WithProfile", scopeSign, func(o *optionValues) error { o.profile = v; o.profileSet = true; return nil })
}
func WithLifetime(v time.Duration) Option {
	return makeOption("WithLifetime", scopeSign, func(o *optionValues) error { o.lifetime = v; o.lifetimeSet = true; return nil })
}
func WithSubject(v Identity) Option {
	return makeOption("WithSubject", scopeSign|scopeCSR, func(o *optionValues) error { o.subject = v; o.subjectSet = true; return nil })
}
func WithSANs(v SANs) Option {
	return makeOption("WithSANs", scopeSign|scopeCSR, func(o *optionValues) error { o.sans = v; o.sansSet = true; return nil })
}
func WithAdditionalSANs(v SANs) Option {
	return makeOption("WithAdditionalSANs", scopeSign, func(o *optionValues) error { o.additionalSANs = v; o.additionalSANsSet = true; return nil })
}
func WithKeySpec(v KeySpec) Option {
	return makeOption("WithKeySpec", scopeCSR, func(o *optionValues) error { o.keySpec = v; o.keySpecSet = true; return nil })
}
func WithKeyPassphrase(v []byte) Option {
	return makeOption("WithKeyPassphrase", scopeCSR|scopeLoad, func(o *optionValues) error {
		if len(v) == 0 {
			return fmt.Errorf("passphrase is empty")
		}
		o.keyPassphrase = append([]byte(nil), v...)
		o.keyPassphraseSet = true
		return nil
	})
}
func WithCert(path string) Option {
	return makeOption("WithCert", scopeLoad, func(o *optionValues) error {
		if path == "" {
			return fmt.Errorf("path is empty")
		}
		o.certPaths = append(o.certPaths, path)
		return nil
	})
}
func WithKey(path string) Option {
	return makeOption("WithKey", scopeLoad, func(o *optionValues) error {
		if path == "" {
			return fmt.Errorf("path is empty")
		}
		o.keyPath = path
		o.keyPathSet = true
		return nil
	})
}
func WithReplace() Option {
	return makeOption("WithReplace", scopeSave, func(o *optionValues) error { o.replace = true; return nil })
}
func WithIssuerValidity(v time.Duration) Option {
	return makeOption("WithIssuerValidity", scopeIssuerDefinition, func(o *optionValues) error { o.issuerValidity = v; o.issuerValiditySet = true; return nil })
}
func WithAllowedProfiles(v ...Profile) Option {
	return makeOption("WithAllowedProfiles", scopeIssuerDefinition, func(o *optionValues) error {
		o.profiles = append([]Profile(nil), v...)
		o.profilesSet = true
		return nil
	})
}
func WithSubjectOwner(v string) Option {
	return makeOption("WithSubjectOwner", scopeIssuerDefinition, func(o *optionValues) error { o.subjectOwner = v; return nil })
}
func WithRequiredSubject(v Identity) Option {
	return makeOption("WithRequiredSubject", scopeIssuerDefinition, func(o *optionValues) error { o.requiredSubject = v; o.requiredSubjectSet = true; return nil })
}
func WithDNSPolicy(v ...string) Option {
	return makeOption("WithDNSPolicy", scopeIssuerDefinition, func(o *optionValues) error {
		if o.dnsSuffixesSet {
			return fmt.Errorf("DNS suffix policy already specified")
		}
		o.dnsSuffixesSet = true
		o.sanPolicy.DNSSuffixes = append([]string(nil), v...)
		return nil
	})
}
func WithDNSSuffixes(v ...string) Option {
	return makeOption("WithDNSSuffixes", scopeIssuerDefinition, func(o *optionValues) error {
		if o.dnsSuffixesSet {
			return fmt.Errorf("DNS suffix policy already specified")
		}
		o.dnsSuffixesSet = true
		o.sanPolicy.DNSSuffixes = append([]string(nil), v...)
		return nil
	})
}
func WithIPPolicy(v ...*net.IPNet) Option {
	return makeOption("WithIPPolicy", scopeIssuerDefinition, func(o *optionValues) error {
		if o.ipRangesSet {
			return fmt.Errorf("IP range policy already specified")
		}
		o.ipRangesSet = true
		o.sanPolicy.IPRanges = append([]*net.IPNet(nil), v...)
		return nil
	})
}
func WithIPRanges(v ...*net.IPNet) Option {
	return makeOption("WithIPRanges", scopeIssuerDefinition, func(o *optionValues) error {
		if o.ipRangesSet {
			return fmt.Errorf("IP range policy already specified")
		}
		o.ipRangesSet = true
		o.sanPolicy.IPRanges = append([]*net.IPNet(nil), v...)
		return nil
	})
}
func WithURIPolicy(v ...string) Option {
	return makeOption("WithURIPolicy", scopeIssuerDefinition, func(o *optionValues) error {
		if o.uriPrefixesSet {
			return fmt.Errorf("URI prefix policy already specified")
		}
		o.uriPrefixesSet = true
		o.sanPolicy.URIPrefixes = append([]string(nil), v...)
		return nil
	})
}
func WithURIPrefixes(v ...string) Option {
	return makeOption("WithURIPrefixes", scopeIssuerDefinition, func(o *optionValues) error {
		if o.uriPrefixesSet {
			return fmt.Errorf("URI prefix policy already specified")
		}
		o.uriPrefixesSet = true
		o.sanPolicy.URIPrefixes = append([]string(nil), v...)
		return nil
	})
}
func WithEmailPolicy(v ...string) Option {
	return makeOption("WithEmailPolicy", scopeIssuerDefinition, func(o *optionValues) error {
		if o.emailDomainsSet {
			return fmt.Errorf("email domain policy already specified")
		}
		o.emailDomainsSet = true
		o.sanPolicy.EmailDomains = append([]string(nil), v...)
		return nil
	})
}
func WithEmailDomains(v ...string) Option {
	return makeOption("WithEmailDomains", scopeIssuerDefinition, func(o *optionValues) error {
		if o.emailDomainsSet {
			return fmt.Errorf("email domain policy already specified")
		}
		o.emailDomainsSet = true
		o.sanPolicy.EmailDomains = append([]string(nil), v...)
		return nil
	})
}
