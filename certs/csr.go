package certs

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"path/filepath"
)

type CSRBundle struct {
	csr        *x509.CertificateRequest
	csrDER     []byte
	key        crypto.Signer
	passphrase []byte
}

func NewCSR(opts ...Option) (*CSRBundle, error) {
	o, err := parseOptions(scopeCSR, opts)
	if err != nil {
		return nil, err
	}
	if !o.subjectSet {
		return nil, invalid("WithSubject is required", nil)
	}
	subject, err := normalizeIdentity(o.subject)
	if err != nil {
		return nil, err
	}
	sans, err := normalizeSANs(o.sans)
	if err != nil {
		return nil, err
	}
	spec := o.keySpec
	if !o.keySpecSet {
		spec = ECDSAP256
	}
	if !spec.valid() {
		return nil, invalid("unsupported key specification", nil)
	}
	key, err := generateSigner(spec)
	if err != nil {
		return nil, err
	}
	t := &x509.CertificateRequest{Subject: identityName(subject), DNSNames: sans.DNSNames, IPAddresses: sans.IPAddresses, URIs: sans.URIs, EmailAddresses: sans.EmailAddresses}
	der, err := x509.CreateCertificateRequest(rand.Reader, t, key)
	if err != nil {
		return nil, fmt.Errorf("certs: create CSR: %w", err)
	}
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		return nil, corrupt("generated CSR", err)
	}
	debugLog(o.logger, "key created", "key_type", spec, "purpose", "csr")
	return &CSRBundle{csr: csr, csrDER: der, key: key, passphrase: append([]byte(nil), o.keyPassphrase...)}, nil
}

func identityName(v Identity) pkix.Name {
	return pkix.Name{Organization: nonempty(v.Organization), OrganizationalUnit: nonempty(v.OrganizationalUnit), Country: nonempty(v.Country), Province: nonempty(v.Province), Locality: nonempty(v.Locality), CommonName: v.CommonName}
}
func identityFromName(v pkix.Name) Identity {
	return Identity{Organization: first(v.Organization), OrganizationalUnit: first(v.OrganizationalUnit), Country: first(v.Country), Province: first(v.Province), Locality: first(v.Locality), CommonName: v.CommonName}
}
func nonempty(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}
func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
func (b *CSRBundle) CSRPEM() []byte {
	if b == nil || len(b.csrDER) == 0 {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: append([]byte(nil), b.csrDER...)})
}
func (b *CSRBundle) KeyPEM() ([]byte, error) {
	if b == nil || signerIsNil(b.key) {
		return nil, &NotFoundError{Resource: "private key"}
	}
	return marshalKeyPEM(b.key, b.passphrase)
}
func (b *CSRBundle) CSR() *x509.CertificateRequest {
	if b == nil {
		return nil
	}
	return b.csr
}
func (b *CSRBundle) Key() crypto.Signer {
	if b == nil {
		return nil
	}
	return b.key
}
func (b *CSRBundle) SaveCSR(path string, opts ...Option) error {
	return atomicSave(path, b.CSRPEM(), 0644, opts...)
}
func (b *CSRBundle) SaveKey(path string, opts ...Option) error {
	v, err := b.KeyPEM()
	if err != nil {
		return err
	}
	return atomicSave(path, v, 0600, opts...)
}
func (b *CSRBundle) Save(directory string, opts ...Option) (map[string]string, error) {
	if err := validateDirectory(directory); err != nil {
		return nil, err
	}
	written := map[string]string{}
	csr := filepath.Join(directory, "request.pem")
	key := filepath.Join(directory, "key.pem")
	if err := b.SaveCSR(csr, opts...); err != nil {
		return nil, err
	}
	written["csr"] = csr
	if err := b.SaveKey(key, opts...); err != nil {
		return nil, err
	}
	written["key"] = key
	return written, nil
}
