// Package certs manages a small, product-local mTLS certificate authority and
// provides generic CSR and certificate bundle utilities.
//
// Managed authorities have one fixed topology: a root signs issuers and an
// issuer signs leaves. Root rotation is staged so callers can distribute both
// trust anchors before explicitly promoting the pending generation. Root keys
// are always encrypted PKCS#8; issuer keys are available only through an
// IssuerManager and are never returned by its signing API.
// Constructors accept WithLogger for concise optional DEBUG lifecycle logging;
// passing nil disables logging.
//
// FileStore and SQLiteStore are local security boundaries, not general-purpose
// third-party storage interfaces. Generic CSRBundle and CertificateBundle use
// no managed authority and may be used independently.
package certs
