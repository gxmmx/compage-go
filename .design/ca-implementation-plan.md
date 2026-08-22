# Certificate Authority and Certificate Management Plan

## Purpose and scope

Add one public `certs` package to `github.com/gxmmx/compage-go`.

It manages a private, product-local mTLS PKI with exactly this maximum chain:

```text
root CA → issuer CA → leaf certificate
```

The package provides three independent facilities:

1. Authority lifecycle: roots, issuers, storage, trust bundles, and rotation.
2. Issuer signing: load the active issuer and sign validated CSRs.
3. Generic CSR/key and certificate-bundle tooling, with no dependency on a managed CA.

The generic bundle type is shared: a bundle returned by signing and one loaded from arbitrary PEM files have the same representation.

### v1 non-goals

- Root CAs never sign leaf certificates.
- No CRL/OCSP generation, responder, URL, or runtime status enforcement.
- No networked DB, KMS/HSM, remote signer, or background daemon.
- No automatic FileStore↔SQLite migration.
- No custom leaf key usages or arbitrary certificate profiles.
- No cross-signed, branching, or multiple certificate chains per bundle.

CA certificates still include `CertSign | CRLSign`, preserving the option of adding revocation later without reissuing the CA hierarchy.

## Package shape

Use one package, rather than `ca`, `issuer`, and `crt` subpackages.

```go
package certs

type AuthorityManager struct{ /* ... */ }
type IssuerManager struct{ /* ... */ }
type CertificateBundle struct{ /* ... */ }
type CSRBundle struct{ /* ... */ }

type FileStore struct{ /* ... */ }
type SQLiteStore struct{ /* ... */ }
```

Illustrative lifecycle API:

```go
authority, err := certs.NewAuthorityManager(
    certs.WithStore(store),
    certs.WithAuthorityName("My Organization"),
    certs.WithIdentity(identity),
    certs.WithOwner("ca-admin"),
    certs.WithGroup("issuer"),
    certs.WithRootKeySpec(certs.ECDSAP256),
    certs.WithIssuer(enrollment),
)

result, err := authority.Ensure(ctx, certs.WithRootRotateBefore(20*24*time.Hour))

issuer, err := certs.NewIssuerManager(
    certs.WithStore(store),
    certs.WithIssuerName("Enrollment"),
)
bundle, err := issuer.SignCSR(ctx, csrPEM, certs.WithProfile(certs.TLSClient))
```

Functional options are strict: nil, conflicting, unsupported, and irrelevant options return validation errors. Construction validates static configuration; operations accept per-call policy options.

FileStore and SQLiteStore share one internal storage interface. Do not promise a public third-party store contract in v1.

## Identity, names, and identifiers

Use a complete subject model:

```go
type Identity struct {
    Organization       string // O
    OrganizationalUnit string // OU
    Country            string // C
    Province           string // ST
    Locality           string // L
    CommonName         string // CN
}
```

`AuthorityName` is separate immutable display text used for CA CN derivation. It is preserved after validation; do not title-case it.

```text
My Organization G1 Root CA
My Organization G1 Foo Agent Issuer V2
```

`Identity` supplies the remaining subject attributes. Leaf CSRs may use a full leaf identity independently.

Authority and issuer display names:

- permit `A-Z`, `a-z`, `0-9`, and spaces;
- trim outer whitespace and collapse internal spaces;
- preserve normalized display text in certificates and stored records;
- derive storage IDs using `text.Slugify(name, "-", text.Lower)`;
- reject empty slugs and collisions such as `Foo Agent` and `foo-agent`.

Never derive paths, SQLite keys, or authority identity directly from display text. Persist a random stable authority ID and certificate fingerprints.

## Immutable specifications and operation policy

Persist immutable specifications at first provisioning. Any incompatible later input returns `DriftError`.

### AuthoritySpec

- stable authority ID and format version;
- authority name and identity;
- root validity: default 10 years;
- immutable CA signing key specification;
- mandatory encrypted-root-key policy;
- issuance mode: `None` or `Ledger`;
- backend metadata.

### IssuerDefinition

- display name and slug;
- validity: default 2 years;
- allowed leaf profiles;
- optional exact Organization policy: `WithSubjectOwner(org)`;
- optional required subject fields: `WithRequiredSubject(identity)` requires equality for every non-empty field;
- optional SAN policy: DNS suffixes, IP ranges, URI prefixes, and email domains.

Issuer definitions are immutable once provisioned. Owners may add issuers later. Omitting an existing issuer from a later manager configuration never changes or deletes it.

### RotationPolicy

Rotation policy is per operation, not certificate identity:

- omitted rotate-before: 20 days;
- non-zero: rotate at or below the duration to expiry, including expiry in the past;
- zero: request rotation immediately.

`Ensure` takes `WithRootRotateBefore` and per-issuer `WithIssuerRotateBefore` options.

## Certificate policy

### Key algorithms

Support ECDSA P-256 (default), ECDSA P-384, Ed25519, RSA 3072, and RSA 4096. Reject weaker/other RSA sizes.

The root and issuer signing key specification is shared and immutable for one authority. Leaf keys may use any supported algorithm independently. Use `crypto.Signer` internally so Ed25519 value handling stays in key codec code.

### CA certificates

- root: self-signed, `IsCA`, `MaxPathLen=1`;
- issuer: root-signed, `IsCA`, `MaxPathLen=0`;
- both: valid basic constraints, SKI/AKI, random positive serials, `CertSign|CRLSign` key usage;
- CA certificates do not enumerate leaf EKUs.

### Leaf profiles

Signing always requires an explicit issuer-permitted profile:

| Profile | Key usage | Extended key usage |
| --- | --- | --- |
| `TLSClient` | `DigitalSignature` | `ClientAuth` |
| `TLSServer` | `DigitalSignature` | `ServerAuth` |
| `TLSClientServer` | `DigitalSignature` | `ClientAuth`, `ServerAuth` |

Default leaf validity is 90 days; maximum is one year. A shorter request is accepted. Reject, rather than shorten, a request exceeding that maximum or the issuer's remaining lifetime. Every child must expire before its parent.

Use `crypto/rand` to generate positive random 159-bit leaf serials. In ledger mode, retry the negligible collision.

## Authority lifecycle

### Create and load

`Ensure` creates `G1` when absent. Before mutation it validates existing certificates, matching keys, immutable records, owner/group requirements, and topology.

The authority owner alone administers roots and issuers. Issuer-group users may load the active issuer and sign leaves, but cannot rotate roots or issuers.

### Issuer reconciliation

`Ensure` creates each declared issuer as `V1` under the active root. `IssuerManager` only loads and signs; it never creates or rotates issuer certificates.

Normal issuer rotation creates `V(n+1)` under the active root. Compute the new expiry from immutable issuer validity, current time, and root expiry. Fail when the configured lifetime cannot fit before root expiry by the safety margin; never silently shorten it.

After publishing a successor, delete the retired issuer private key immediately. Retain all public issuer certificates until authority purge, so chains remain constructible.

### Staged root rotation

Root rotation changes the trust anchor and must be staged:

1. A due root rotation, including threshold zero, prepares pending `G(n+1)` only.
2. `TrustBundlePEM()` includes all retained roots, including the pending root, oldest to newest.
3. The product distributes that trust set to all endpoints.
4. `Ensure(...WithActivatePendingRoot())` creates next versions of every known issuer under the pending root.
5. After validating all new issuer versions, atomically publish the new topology.
6. Delete old root and replaced issuer private keys; retain every public root and issuer certificate.

Only one pending root is allowed. A pending root cannot sign and is never selected by an issuer. A second prepare returns `ConflictError`. The owner may explicitly discard an unactivated pending root.

While a root is pending, current issuers remain under the active root. Any issuer renewal that cannot fit under that root fails; it must not silently move to the pending anchor.

### Purge

One FileStore authority directory or SQLite file represents exactly one authority. `Purge` is owner-only, requires the expected authority ID/fingerprint from the caller, and deletes all authority data: roots, issuers, ledger, metadata, pending state, and local-unlock material.

## Root-key security and authorization

Every root private key is encrypted at rest. Use standard encrypted PKCS#8 PEM (`ENCRYPTED PRIVATE KEY`) with modern PBES2/scrypt-based encryption and a vetted implementation that supports RSA, ECDSA, and Ed25519. Do not use deprecated legacy PEM encryption.

On first root creation the authority creates a high-entropy binary unlock secret, kept only by the creating manager for its short lifetime:

```go
authority.Ensure(ctx)
authority.StoreLocalUnlock()                 // optional owner-only storage
unlock, err := authority.ExportGeneratedUnlock() // optional external handling
authority.Close()
```

- `Ensure` never returns the unlock secret.
- `ExportGeneratedUnlock` is available only for a newly generated secret and returns a copy.
- `StoreLocalUnlock` atomically writes the pending secret and refuses replacement without explicit authorization.
- Reopened root operations use configured local unlock material or a caller-supplied secret. Missing material returns an `errx` not-found-style error so callers can prompt.
- Never log a secret. The library accepts bytes; a caller CLI must prompt or use a deliberate secure source, never command arguments or environment variables.

Issuer private keys remain group-readable for unattended leaf signing; `localunlock` never protects or exposes them.

On Linux and Darwin, strict stores validate effective owner, issuer group membership, modes, regular files, and every parent path without following symlinks. Split owner/group mode is unsupported elsewhere. The authority manager requires the configured owner; the issuer manager requires group membership.

## Storage

Both stores implement one transactional model:

- immutable authority and issuer records;
- active/pending topology and monotonic revision;
- encrypted root keys and current issuer keys;
- certificates and optional issuance ledger;
- owner-aware local unlock;
- exclusive administration locking;
- inspection and explicit repair.

Certificates and keys are source material. Metadata is an atomic topology index, not unquestioned truth. Normal reads cross-check it. Mismatch returns `DriftError`; `Inspect` reports it and owner-only `Repair` may rebuild only derived state it can prove valid.

### FileStore

FileStore requires owner and group. Its parent location must already exist; the store does not create it.

```text
<location>/<authority-slug>/
  state.json
  localunlock
  roots/
    g1.crt
    g1.key
    g2.pending.crt
    g2.pending.key
  issuers/<issuer-slug>/
    v1.crt
    v1.key
    issued/<serial>.pem
    issued/<serial>.json
  .lock
```

- `*.pending` makes staged root/issuer material independently inspectable.
- Write and fsync all pending files, validate them, atomically publish `state.json`, then rename pending files as cleanup.
- Normal reads require names and state to agree. A failure before publishing retains the prior active topology; a failure after publishing has complete validated source files.
- Every write is temp-in-target-dir, chmod/chown, write, sync, close, rename, and directory sync. Reject symlinks, non-regular files, unsafe paths, bad ownership, and bad modes.
- Public certs/metadata are owner-group readable; roots and `localunlock` are owner-only; current issuer private keys are owner-group readable. Directories permit group read/traverse, not group mutation.

### SQLiteStore

SQLite is local-only in v1, one authority per file. Use `modernc.org/sqlite` for CGO-free, cross-platform compilation.

- Split owner/group mode: database `0640`, directory `0750`; owner writes, group reads.
- Issuer connections use SQLite read-only mode. Filesystem permissions are the security boundary: compromised issuer code cannot open an alternate write connection or replace the DB.
- Mandate rollback journal (`DELETE`) mode, never WAL, to avoid writable `-wal`/`-shm` requirements for group readers.
- Administration uses a context-aware write transaction; issuer reads only active revision and current issuer data.
- A local unlock file is permitted only with strict owner/group local protection. Without it the caller provides an unlock secret.
- SQLite without owner/group is allowed only for a private single-principal `0600` deployment, never for a separate issuer service.

Use versioned migrations and tables for authority records, root generations, issuer definitions/versions, topology state, encrypted keys, and ledger records. Store public certificates as DER blobs; encode PEM at API boundaries.

### Issuance ledger

`IssuanceMode=None` persists no leaf history. `Ledger` persists immutable public records in either backend:

- certificate DER/PEM and SHA-256 fingerprint;
- issuer version and serial;
- subject, SANs, profile, issued/expires timestamps.

Never persist leaf private keys. Support exact `(issuer version, serial)` lookup and scan-based paginated FileStore listing; do not add arbitrary subject/SAN search in v1. A ledger is audit/query data, not revocation.

## Issuer signing

`SignCSR` must:

1. Decode one valid PEM CSR and verify its signature.
2. Require a supported RSA/ECDSA/Ed25519 public key.
3. Reject requested CA capabilities and unknown critical extensions.
4. Validate the CSR subject against issuer policy.
5. Preserve CSR subject/SANs by default. `WithAdditionalSANs` merges; explicit `WithSubject` or `WithSANs` replaces its field.
6. Validate final subject and SANs against policy again.
7. Require an allowed explicit profile and valid lifetime.
8. Resolve active issuer revision, sign, write the ledger when enabled, and return a keyless bundle containing leaf, issuer, and root.

SANs are typed DNS, IP, URI, and email values. The issuer controls final content, but the package does not prove real-world DNS/IP/URI/email ownership.

Issuer managers cache a signer with its authority revision. Each sign performs a lightweight revision check and reloads on change, without a goroutine. A process that already loaded a retired key may sign until reload; deleting its file cannot revoke in-memory key data.

## Generic CSR and certificate bundles

### CSRBundle

`NewCSR` accepts complete leaf identity, typed SANs, supported key spec, and optional key passphrase. It outputs a standards-compliant CSR and PKCS#8 private key.

- `CSRPEM()` returns `CERTIFICATE REQUEST` PEM.
- `KeyPEM()` returns unencrypted PKCS#8 by default or encrypted PKCS#8 on request.
- `SaveCSR(path)` and `SaveKey(path)` write explicit files.
- `Save(directory)` writes `request.pem` and `key.pem` and returns paths.

### CertificateBundle

`LoadBundle` accepts repeatable `WithCert(path)` values. Every input may contain one or more PEM certificate blocks; malformed PEM and non-certificate blocks are errors. Files may be supplied in any order.

Build exactly one linear cryptographically verified chain:

```text
leaf → intermediate(s) → self-signed root (optional)
```

Reject duplicates, disconnected material, ambiguity, cross-signing, and branching. Detect a root only when it is CA-capable, has subject equal to issuer, and validates its own signature; never infer root merely from list position.

Accept a partial chain when root is omitted: `Root() == nil` and `Complete() == false`. A strict option requires a root-terminated chain. Loading validates parse/order/signatures/constraints and optional leaf-key match, but does not reject expiration or validate hostname/time until explicit verification.

```go
Certificate() *x509.Certificate
Intermediates() []*x509.Certificate
Root() *x509.Certificate
Certificates() []*x509.Certificate // leaf through root, when present
TLSChainPEM() []byte               // leaf + intermediates
FullChainPEM() []byte              // leaf + intermediates + root
Key() crypto.Signer                // optional
```

When supplied, parse a supported PKCS#8 key and require exact match to leaf public key. `WithKeyPassphrase` unlocks encrypted keys. Issued keyless bundles are valid: `SaveKey` returns `NotFoundError` and directory save omits `key.pem`.

Provide explicit `SaveCertificate`, `SaveTLSChain`, `SaveFullChain`, and `SaveKey`. `Save(directory)` writes `cert.pem`, `chain.pem`, `fullchain.pem`, and `key.pem` when present.

All generic saves are atomic/no-symlink writes. Existing targets fail unless an explicit replace option is supplied. Private-key mode is `0600`; public files are safely readable.

## Concurrency, repair, and errors

`Ensure` takes a context-aware exclusive authority lock: strict file lock for FileStore and serialized write transaction for SQLite. Validate before mutation. If a staged multi-object operation fails, return a partial-result error and preserve the prior active topology.

Follow this repository's `errx` conventions and never include secrets in errors:

- validation: options, names, paths, SANs, profiles, lifetimes;
- not found: authority, issuer, key, local unlock, missing bundle key;
- forbidden: owner/group/authentication/safe-access failure;
- conflict: pending root, slug collision, overwrite refusal, lock policy;
- drift: immutable spec, metadata, key/certificate, or filesystem mismatch;
- unavailable: unsupported platform/security mode;
- unexpected OS/crypto failures: wrapped safely with `errx.New`.

`Inspect` is read-only. `Repair` is owner-only and explicit: scan, validate cryptographically, then rebuild only derived state.

## Implementation phases

1. Add package docs, models, naming/identity validation, errors, clock/filesystem abstractions, and PEM/key codecs.
2. Implement `CSRBundle` and `CertificateBundle`: generation, normalization, validation, key matching, and safe save/load.
3. Implement internal store model and FileStore: strict ownership, atomic writes, locks, state, pending roots, ledger files, repair.
4. Implement authority reconciliation: initial provisioning, issuer lifecycle, staged root prepare/activate/discard, trust bundle, purge.
5. Implement issuer signing, subject/SAN policies, profiles, ledger writes, and revision-aware issuer reload.
6. Implement SQLiteStore: `modernc.org/sqlite`, migrations, read-only issuer access, DELETE journaling, transactions, local-unlock rules, parity tests.
7. Add examples, security/lifecycle documentation, and migration/export-import design notes without implementing automatic migration.

## Verification and acceptance criteria

Use standard `testing`, table-driven cases, `t.TempDir`, real filesystem security tests, dependency injection for clock/files, and fuzzing for PEM/order/DER/name boundaries.

Required coverage:

- key algorithms, encrypted-root round trips, missing/wrong unlock, Ed25519 codecs;
- immutable drift, additive issuer definitions, slug collisions, display-name preservation;
- initial provisioning, issuer rotation, staged root prepare/activate/discard, trust order, interrupted activation recovery;
- validity boundaries, parent/child expiry, serial collision retry;
- owner/group enforcement, symlink rejection, exact modes, local-unlock access, read-only SQLite issuer access;
- FileStore/SQLiteStore parity, SQLite migrations/DELETE journal behavior, lock cancellation, read-only inspection;
- CSR signature/key validation, subject/SAN policy before and after overrides, profiles, leaf validity;
- arbitrary PEM ordering, root validation, partial/strict chains, ambiguity rejection, encrypted key loading, key mismatch;
- ledger writes/lookups proving no leaf private key persists;
- atomic save/replace behavior and keyless issued bundle behavior.

Run `go test ./...`, race tests for concurrent ensure/sign/load paths, and Linux/Darwin ownership tests.
