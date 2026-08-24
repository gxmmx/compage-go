package certs

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CertificateBundle struct {
	certs         []*x509.Certificate
	leaf          *x509.Certificate
	intermediates []*x509.Certificate
	root          *x509.Certificate
	key           crypto.Signer
	keyPassphrase []byte
}

func LoadBundle(opts ...Option) (*CertificateBundle, error) {
	o, err := parseOptions(scopeLoad, opts)
	if err != nil {
		return nil, err
	}
	return loadCertificateBundle(o)
}

// NewCertificateBundle creates a certificate bundle from certificate objects,
// PEM data, or certificate files supplied through the certificate options.
func NewCertificateBundle(opts ...Option) (*CertificateBundle, error) {
	o, err := parseOptions(scopeLoad, opts)
	if err != nil {
		return nil, err
	}
	return loadCertificateBundle(o)
}

func loadCertificateBundle(o optionValues) (*CertificateBundle, error) {
	if o.keyPassphraseSet && !o.keyPathSet {
		return nil, invalid("WithKeyPassphrase requires WithKey", nil)
	}
	certificates, err := loadCertificateInputs(o)
	if err != nil {
		return nil, err
	}
	b, err := normalizeBundle(certificates, nil)
	if err != nil {
		return nil, err
	}
	if o.keyPathSet {
		data, err := readRegular(o.keyPath)
		if err != nil {
			return nil, err
		}
		next, err := b.AddKeyPEM(data, o.keyPassphrase)
		if err != nil {
			return nil, err
		}
		b = next
	}
	if len(certificates) == 0 && !o.keyPathSet {
		return nil, invalid("at least one certificate or key is required", nil)
	}
	return b, nil
}

func loadCertificateInputs(o optionValues) ([]*x509.Certificate, error) {
	var certificates []*x509.Certificate
	for _, cert := range o.certificates {
		if cert == nil {
			return nil, corrupt("nil certificate", nil)
		}
		certificates = append(certificates, cloneCertificate(cert))
	}
	for _, data := range o.certPEM {
		parsed, err := parseCertificatesPEM(data)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, parsed...)
	}
	for _, path := range o.certPaths {
		data, err := readRegular(path)
		if err != nil {
			return nil, err
		}
		parsed, err := parseCertificatesPEM(data)
		if err != nil {
			return nil, fmt.Errorf("certs: load %s: %w", path, err)
		}
		certificates = append(certificates, parsed...)
	}
	return certificates, nil
}

func readRegular(path string) ([]byte, error) {
	if err := rejectSymlinkParents(filepath.Dir(path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &NotFoundError{Resource: path, Cause: err}
		}
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, invalid("input must be a regular non-symlink file", nil)
	}
	return os.ReadFile(path)
}

func cloneCertificate(cert *x509.Certificate) *x509.Certificate {
	if cert == nil {
		return nil
	}
	copy := *cert
	return &copy
}

func parseCertificatesPEM(data []byte) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	rest := data
	for len(bytes.TrimSpace(rest)) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			return nil, corrupt("malformed certificate PEM", nil)
		}
		if block.Type != "CERTIFICATE" {
			return nil, corrupt("non-certificate PEM block in certificate input", nil)
		}
		parsed, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			return nil, corrupt("invalid certificate DER", err)
		}
		if len(parsed) != 1 {
			return nil, corrupt("certificate PEM block must contain one certificate", nil)
		}
		out = append(out, parsed[0])
		rest = next
	}
	if len(out) == 0 {
		return nil, corrupt("certificate input is empty", nil)
	}
	return out, nil
}

func (b *CertificateBundle) AddCert(cert *x509.Certificate) (*CertificateBundle, error) {
	if b == nil {
		return nil, invalid("nil certificate bundle", nil)
	}
	return b.addCertificates([]*x509.Certificate{cert})
}
func (b *CertificateBundle) AddCertPEM(data []byte) (*CertificateBundle, error) {
	if b == nil {
		return nil, invalid("nil certificate bundle", nil)
	}
	add, err := parseCertificatesPEM(data)
	if err != nil {
		return b, err
	}
	return b.addCertificates(add)
}
func (b *CertificateBundle) AddCertPath(path string) (*CertificateBundle, error) {
	if b == nil {
		return nil, invalid("nil certificate bundle", nil)
	}
	data, err := readRegular(path)
	if err != nil {
		return b, err
	}
	return b.AddCertPEM(data)
}
func (b *CertificateBundle) addCertificates(add []*x509.Certificate) (*CertificateBundle, error) {
	all := append([]*x509.Certificate(nil), b.certs...)
	all = append(all, add...)
	next, err := normalizeBundle(all, b.key)
	if err != nil {
		return b, err
	}
	next.keyPassphrase = append([]byte(nil), b.keyPassphrase...)
	return next, nil
}
func (b *CertificateBundle) AddKeyPEM(data, passphrase []byte) (*CertificateBundle, error) {
	if b != nil && !signerIsNil(b.key) {
		return b, &ConflictError{Message: "bundle already has a private key"}
	}
	key, err := parseKeyPEM(data, passphrase)
	if err != nil {
		return b, err
	}
	var certs []*x509.Certificate
	if b != nil {
		certs = b.certs
	}
	next, err := normalizeBundle(certs, key)
	if err != nil {
		return b, err
	}
	if block, _ := pem.Decode(data); block != nil && block.Type == "ENCRYPTED PRIVATE KEY" {
		next.keyPassphrase = append([]byte(nil), passphrase...)
	}
	return next, nil
}
func (b *CertificateBundle) AddKey(path string, passphrase []byte) (*CertificateBundle, error) {
	data, err := readRegular(path)
	if err != nil {
		return b, err
	}
	return b.AddKeyPEM(data, passphrase)
}

func normalizeBundle(certs []*x509.Certificate, key crypto.Signer) (*CertificateBundle, error) {
	if len(certs) == 0 {
		return &CertificateBundle{key: key}, nil
	}
	byRaw := map[string]bool{}
	for _, c := range certs {
		if c == nil {
			return nil, corrupt("nil certificate", nil)
		}
		k := string(c.Raw)
		if byRaw[k] {
			return nil, &ConflictError{Message: "duplicate certificate"}
		}
		byRaw[k] = true
	}
	parents := make(map[*x509.Certificate]*x509.Certificate)
	children := make(map[*x509.Certificate][]*x509.Certificate)
	for _, child := range certs {
		self := bytes.Equal(child.RawSubject, child.RawIssuer) && child.IsCA && child.CheckSignatureFrom(child) == nil
		if self {
			continue
		}
		var matches []*x509.Certificate
		for _, candidate := range certs {
			if candidate == child || !bytes.Equal(child.RawIssuer, candidate.RawSubject) {
				continue
			}
			if child.CheckSignatureFrom(candidate) == nil {
				matches = append(matches, candidate)
			}
		}
		if len(matches) > 1 {
			return nil, &ConflictError{Message: "ambiguous or cross-signed certificate chain"}
		}
		if len(matches) == 1 {
			parent := matches[0]
			if !parent.IsCA || !parent.BasicConstraintsValid || parent.KeyUsage&x509.KeyUsageCertSign == 0 {
				return nil, corrupt("issuer certificate is not CA-capable", nil)
			}
			parents[child] = parent
			children[parent] = append(children[parent], child)
		}
	}
	for _, v := range children {
		if len(v) > 1 {
			return nil, &ConflictError{Message: "branching certificate chain"}
		}
	}
	var bottoms []*x509.Certificate
	for _, c := range certs {
		if len(children[c]) == 0 {
			bottoms = append(bottoms, c)
		}
	}
	if len(bottoms) != 1 {
		return nil, &ConflictError{Message: "disconnected or ambiguous certificate material"}
	}
	ordered := []*x509.Certificate{}
	seen := map[*x509.Certificate]bool{}
	for c := bottoms[0]; c != nil; c = parents[c] {
		if seen[c] {
			return nil, corrupt("certificate cycle", nil)
		}
		seen[c] = true
		ordered = append(ordered, c)
	}
	if len(ordered) != len(certs) {
		return nil, &ConflictError{Message: "disconnected certificate material"}
	}
	r := &CertificateBundle{certs: ordered, key: key}
	top := ordered[len(ordered)-1]
	if bytes.Equal(top.RawSubject, top.RawIssuer) && top.IsCA && top.CheckSignatureFrom(top) == nil {
		r.root = top
	}
	if !ordered[0].IsCA {
		r.leaf = ordered[0]
		end := len(ordered)
		if r.root != nil {
			end--
		}
		if end > 1 {
			r.intermediates = append([]*x509.Certificate(nil), ordered[1:end]...)
		}
	} else {
		end := len(ordered)
		if r.root != nil {
			end--
		}
		if end > 0 {
			r.intermediates = append([]*x509.Certificate(nil), ordered[:end]...)
		}
	}
	if !signerIsNil(key) && r.leaf != nil && !publicKeysEqual(key.Public(), r.leaf.PublicKey) {
		return nil, invalid("private key does not match terminal certificate", nil)
	}
	return r, nil
}

func (b *CertificateBundle) Certificate() *x509.Certificate {
	if b == nil {
		return nil
	}
	return b.leaf
}
func (b *CertificateBundle) Intermediates() []*x509.Certificate {
	if b == nil {
		return nil
	}
	return append([]*x509.Certificate(nil), b.intermediates...)
}
func (b *CertificateBundle) Root() *x509.Certificate {
	if b == nil {
		return nil
	}
	return b.root
}
func (b *CertificateBundle) Certificates() []*x509.Certificate {
	if b == nil {
		return nil
	}
	return append([]*x509.Certificate(nil), b.certs...)
}
func (b *CertificateBundle) Complete() bool { return b != nil && b.leaf != nil && b.root != nil }
func (b *CertificateBundle) HasKey() bool   { return b != nil && !signerIsNil(b.key) }
func (b *CertificateBundle) KeyMatchesLeaf() bool {
	return b != nil && !signerIsNil(b.key) && b.leaf != nil && publicKeysEqual(b.key.Public(), b.leaf.PublicKey)
}
func (b *CertificateBundle) Key() crypto.Signer {
	if b == nil {
		return nil
	}
	return b.key
}
func (b *CertificateBundle) Validate() error {
	if b == nil {
		return invalid("nil certificate bundle", nil)
	}
	_, err := normalizeBundle(b.certs, b.key)
	return err
}
func (b *CertificateBundle) UsableAt(at time.Time) error {
	if err := b.Validate(); err != nil {
		return err
	}
	for _, c := range b.certs {
		if at.Before(c.NotBefore) || at.After(c.NotAfter) {
			return invalid("certificate is outside its validity window", nil)
		}
	}
	return nil
}
func (b *CertificateBundle) Verify(opts x509.VerifyOptions) error {
	if b == nil || b.leaf == nil {
		return &NotFoundError{Resource: "terminal certificate"}
	}
	if opts.Intermediates == nil && len(b.intermediates) > 0 {
		opts.Intermediates = x509.NewCertPool()
		for _, cert := range b.intermediates {
			opts.Intermediates.AddCert(cert)
		}
	}
	_, err := b.leaf.Verify(opts)
	if err != nil {
		return invalid("certificate verification failed", err)
	}
	return nil
}

func certsPEM(certs []*x509.Certificate) []byte {
	var out []byte
	for _, c := range certs {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	return out
}
func (b *CertificateBundle) SaveCertificate(path string, opts ...Option) error {
	if b == nil || b.leaf == nil {
		return &NotFoundError{Resource: "terminal certificate"}
	}
	return atomicSave(path, certsPEM([]*x509.Certificate{b.leaf}), 0644, opts...)
}
func (b *CertificateBundle) SaveTLSChain(path string, opts ...Option) error {
	if b == nil || b.leaf == nil {
		return &NotFoundError{Resource: "terminal certificate"}
	}
	v := append([]*x509.Certificate{b.leaf}, b.intermediates...)
	return atomicSave(path, certsPEM(v), 0644, opts...)
}
func (b *CertificateBundle) SaveFullChain(path string, opts ...Option) error {
	if b == nil || b.leaf == nil || b.root == nil {
		return &NotFoundError{Resource: "complete certificate chain"}
	}
	return atomicSave(path, certsPEM(b.certs), 0644, opts...)
}
func (b *CertificateBundle) SaveCertificates(path string, opts ...Option) error {
	if b == nil || len(b.certs) == 0 {
		return &NotFoundError{Resource: "certificates"}
	}
	return atomicSave(path, certsPEM(b.certs), 0644, opts...)
}
func (b *CertificateBundle) SaveKey(path string, opts ...Option) error {
	if b == nil || signerIsNil(b.key) {
		return &NotFoundError{Resource: "private key"}
	}
	v, err := marshalKeyPEM(b.key, b.keyPassphrase)
	if err != nil {
		return err
	}
	return atomicSave(path, v, 0600, opts...)
}
func (b *CertificateBundle) Save(directory string, opts ...Option) (map[string]string, error) {
	if err := validateDirectory(directory); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if len(b.certs) > 0 {
		p := filepath.Join(directory, "certificates.pem")
		if err := b.SaveCertificates(p, opts...); err != nil {
			return nil, err
		}
		out["certificates"] = p
	}
	if b.leaf != nil {
		p := filepath.Join(directory, "chain.pem")
		if err := b.SaveTLSChain(p, opts...); err != nil {
			return nil, err
		}
		out["chain"] = p
	}
	if b.root != nil && b.leaf != nil {
		p := filepath.Join(directory, "fullchain.pem")
		if err := b.SaveFullChain(p, opts...); err != nil {
			return nil, err
		}
		out["fullchain"] = p
	}
	if !signerIsNil(b.key) {
		p := filepath.Join(directory, "key.pem")
		if err := b.SaveKey(p, opts...); err != nil {
			return nil, err
		}
		out["key"] = p
	}
	if len(out) == 0 {
		return nil, &NotFoundError{Resource: "bundle material"}
	}
	return out, nil
}
