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
- No cross-signed or branching managed-CA chains in v1. Generic bundles may
  represent one connected partial chain, but do not model alternate paths.

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
    certs.WithAuthorityName("My Organization"), // optional when Identity is set
    certs.WithIdentity(identity),
    certs.WithOwner("ca-admin"),
    certs.WithGroup("issuer"),
    certs.WithRootKeySpec(certs.ECDSAP256),
    certs.WithIssuanceMode(certs.Ledger),
    certs.WithIssuer(enrollment),
)

result, err := authority.Ensure(ctx, certs.WithRootRotateBefore(20*24*time.Hour))
_, err = authority.PromotePending(ctx) // explicit; Ensure never promotes implicitly

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

`NewIdentity` centralizes normalization and validation, but `Identity` remains a
value type so callers may use literals. Manager validation always runs at the
boundary. `AuthorityManager` accepts either `WithAuthorityName` or
`WithIdentity`, or both; it rejects construction when neither is supplied for
a new authority.

```text
My Organization G1 Root CA
My Organization G1 Foo Agent Issuer V2
```

When only `AuthorityName` is supplied, CA certificates contain the generated
CN and no additional identity fields. When only `Identity` is supplied,
`AuthorityName` is resolved from `Identity.Organization`. When both are
supplied, the values may intentionally differ, allowing multiple CAs for one
organization. The identity supplies O/OU/C/ST/L; the manager never uses
`Identity.CommonName` for a root or issuer CN. Leaf CSRs may use a full leaf
identity independently.

When deriving `AuthorityName` from `Identity.Organization`, the organization
must satisfy the authority-name display-name rules. An organization that is
valid as an X.509 O value but not as an authority display name requires an
explicit valid `AuthorityName`.

The generated names are exact:

```text
<AuthorityName> G<generation> Root CA
<AuthorityName> G<generation> <IssuerName> Issuer V<version>
```

On an existing authority, omitted configuration is loaded from `ca.json` and
supplied configuration is an assertion that must match the persisted value.
The root certificate subject and generated CN are cross-checked against the
stored identity and authority name.

Authority and issuer display names:

- permit `A-Z`, `a-z`, `0-9`, and spaces;
- trim outer whitespace and collapse internal spaces;
- preserve normalized display text in certificates and stored records;
- derive storage IDs using `text.Slugify(name, "-", text.Lower)`;
- reject empty slugs and collisions such as `Foo Agent` and `foo-agent`.

Never derive paths, SQLite keys, or authority identity directly from display text. Persist a random stable authority ID and certificate fingerprints. A
`FileStore` location is the authority directory supplied by the caller; the
package does not add an authority-name-derived path component.

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

`IssuanceMode` is immutable. An existing authority provisioned with `None`
cannot later switch to `Ledger`, because historical issuance cannot be
reconstructed. A new authority manager requesting a different mode returns
`DriftError`.

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

Default leaf validity is 90 days; maximum is one year. A shorter request is accepted. Reject, rather than shorten, a request exceeding that maximum or the issuer's remaining lifetime. Managed CA issuance enforces child expiry before parent expiry; generic bundle validation does not impose this creator-side policy.

Use `crypto/rand` to generate positive random 159-bit leaf serials. In ledger mode, retry the negligible collision.

## Authority lifecycle

### Create and load

`Ensure` creates `G1` when absent. Before mutation it validates existing certificates, matching keys, immutable records, owner/group requirements, and topology. A missing or malformed `ca.json` is not treated as a fresh authority.

The authority owner alone administers roots and issuers. Issuer-group users may load the active issuer and sign leaves, but cannot rotate roots or issuers.

### Issuer reconciliation

`Ensure` creates each declared issuer as `V1` under the active root. `IssuerManager` only loads and signs; it never creates or rotates issuer certificates.

Normal issuer rotation prepares `V(n+1)` under the active root. All due issuer rotations in one reconciliation are one pending transaction and are promoted together. Compute the new expiry from immutable issuer validity, current time, and root expiry. Require child `NotAfter <= parent.NotAfter - safetyMargin`; fail when the configured lifetime cannot fit and never silently shorten it.

After promotion, delete the retired issuer private key immediately. Retain all public issuer certificates until authority purge, so chains remain constructible. A cached issuer process may continue using an in-memory retired key until it reloads or restarts.

### Staged root rotation

Root rotation changes the trust anchor and must be staged:

1. A due root rotation, including threshold zero, prepares pending `G(n+1)` and a pending successor version for every known issuer.
2. `TrustBundlePEM()` includes the active and pending roots, plus retired roots still within the configured trust window or required by the ledger.
3. The product distributes that trust set to all endpoints.
4. After validating every pending certificate and key, `PromotePending` atomically makes the pending root and all cascaded issuer versions active.
5. Delete the old root and replaced issuer private keys; retain every public root and issuer certificate.

Only one pending transaction is allowed. A pending root cannot sign and is never selected by an issuer. A second prepare returns `ConflictError`. The owner may explicitly discard an unactivated pending transaction. An existing pending issuer transaction must be promoted or discarded before root preparation. A root transaction promotes its cascaded pending issuers together; an issuer transaction may be promoted independently under the current root.

While a root is pending, current issuers remain under the active root and continue signing. Issuance never selects pending material. While any transaction is pending, `Ensure` does not reconcile additional issuer changes; it returns a conflict until the pending operation is promoted or discarded. `Ensure` prepares but never promotes; applications wanting immediate rollover call `PromotePending` explicitly.

### Purge

One FileStore authority directory or SQLite file represents exactly one authority. `Purge` is owner-only, requires the expected authority ID/fingerprint from the caller, and deletes all authority data: roots, issuers, ledger, metadata, pending state, and local-unlock material.

## Root-key security and authorization

Every root private key is encrypted at rest. Use standard encrypted PKCS#8 PEM (`ENCRYPTED PRIVATE KEY`) with modern PBES2/scrypt-based encryption and a vetted implementation that supports RSA, ECDSA, and Ed25519. Do not use deprecated legacy PEM encryption. Every root generation gets a fresh key pair and a fresh high-entropy unlock secret.

On root creation or root rotation the authority creates a high-entropy binary unlock secret. It is kept only by the creating manager unless the owner explicitly exports or stores it:

```go
authority.Ensure(ctx)
authority.StoreLocalUnlock()                    // optional owner-only storage
unlock, err := authority.ExportGeneratedUnlock() // optional external handling
authority.Close()
```

- `Ensure` never returns the unlock secret.
- `ExportGeneratedUnlock` is available only for a newly generated secret and returns a copy.
- `StoreLocalUnlock` stores only active and pending generation secrets, keyed by generation. It atomically updates the file and refuses replacement without explicit authorization.
- During promotion, retain both active and pending secrets until the new active state is durable and validated; then remove the retired generation's secret with its private key.
- `PromotePending` accepts the pending generation unlock when local unlock is absent. If local unlock is enabled, it tolerates an extra active/pending entry after a crash and reconciles it owner-only.
- Reopened root operations use configured local unlock material or a caller-supplied secret. Missing material returns an `errx` not-found-style error so callers can prompt.
- Never log a secret. The library accepts bytes; a caller CLI must prompt or use a deliberate secure source, never command arguments or environment variables.

Issuer private keys remain group-readable for unattended leaf signing; `localunlock` never protects or exposes them.

On Linux and Darwin, strict stores validate effective owner, issuer group membership, modes, regular files, and every parent path without following symlinks. Split owner/group mode is unsupported elsewhere. The authority manager requires the configured owner; the issuer manager requires group membership. `localunlock` is owner-only and must never be readable by the issuer group. A separate administrative user may be granted access deliberately for unattended root administration.

## Storage

Both stores implement one transactional model:

- immutable authority and issuer records;
- active/pending topology and monotonic revision;
- encrypted root keys and current issuer keys;
- certificates and optional separate issuance ledger;
- owner-aware local unlock;
- exclusive administration locking;
- inspection and narrowly scoped explicit repair.

Cryptographic material is authoritative for signatures, subjects, generated names,
key algorithms, and validity. Metadata is authoritative for policy, topology,
pending-operation intent, revisions, and ledger mode. Normal reads cross-check
both. A valid but inconsistent document returns `DriftError`; malformed or
cryptographically invalid data returns `CorruptError` classified as `errx.Invalid`.

`Inspect` is read-only. With a single JSON document and no history, `Repair`
may rebuild only derived fields that can be proved from the same document; it
must not invent issuer policy or silently choose a replacement topology.

### FileStore

FileStore requires owner and group. Its parent location must already exist; the store does not create it.

```text
<authority-directory>/
  ca.json
  ca.lock
  localunlock
  ledger/                         # only in Ledger mode
    ledger.jsonl
    ledger.lock
```

- `ca.json` contains authority metadata, root generations, issuer definitions and versions, active/pending topology, encrypted root keys, current issuer keys, and revisions. Certificates and keys are base64-encoded DER/PKCS#8 values; PEM is used at API boundaries.
- One pending manifest in `ca.json` identifies the operation kind, expected revision, affected generations/issuer versions, and required promotion set.
- Every `ca.json` update is written to a temp file in the authority directory, chmod/chown'd, synced, closed, atomically renamed, and followed by directory sync. Readers see either the old or new complete document.
- `ca.json` is owner-write/group-read (`0640`); the authority directory is owner-write/group-read/traverse (`0750`-style). The issuer group cannot replace or mutate authority state.
- `ledger/` is created only in `Ledger` mode. Its directory is owner/group writable but outside the authority state's write boundary; `ledger.jsonl` is append-oriented JSON Lines with a group-writable lock. Group members may write ledger data but cannot replace `ca.json` or read `localunlock`.
- Reject symlinks, non-regular files, unsafe paths, bad ownership, and bad modes. Ledger data is validated against certificates and issuer versions loaded from `ca.json`.
- `IssuanceMode=None` does not create or update a ledger. It is intended for a small CA and permits an issuer manager to load and cache its signer with one authority-state read, followed by explicit `Reload` when desired.

### SQLiteStore

SQLite is local-only in v1, one authority per file. Use `modernc.org/sqlite` for CGO-free, cross-platform compilation.

- Split owner/group mode: authority database `0640`, directory `0750`; owner writes, group reads.
- Issuer connections use SQLite read-only mode for authority state. Filesystem permissions are the security boundary: issuer code cannot open an alternate write connection or replace the authority DB.
- Mandate rollback journal (`DELETE`) mode, never WAL, to avoid writable `-wal`/`-shm` requirements for group readers.
- Administration uses a context-aware write transaction; issuer reads only active revision and current issuer data.
- In `Ledger` mode, use a separate group-writable ledger database/file under a ledger-only directory, or an equivalent isolated writer store. Never grant issuer code write access to authority topology.
- A local unlock file is permitted only with owner-only protection. Without it the caller provides an unlock secret.
- SQLite without owner/group is allowed only for a private single-principal `0600` deployment, never for a separate issuer service.

Use versioned migrations and tables for authority records, root generations, issuer definitions/versions, topology state, and encrypted keys. If the ledger is stored in SQLite, use a separate ledger database/schema with equivalent access separation. Store public certificates as DER blobs; encode PEM at API boundaries.

### Issuance ledger

`IssuanceMode=None` persists no leaf history. `Ledger` persists public records for every certificate returned by `SignCSR`:

- certificate DER/PEM and SHA-256 fingerprint;
- issuer version and serial;
- subject, SANs, profile, issued/expires timestamps.

Never persist leaf private keys. The ledger is issuer-writable audit/query data; direct use of an issuer private key outside `IssuerManager.SignCSR` is unsupported misuse and is not tracked. Validate every record's certificate signature and issuer relationship. A malformed ledger blocks ledger-mode signing and lifecycle calculations.

Support exact `(issuer version, serial)` lookup and scan-based paginated FileStore listing; do not add arbitrary subject/SAN search in v1. Ledger mode makes a durable record before returning a certificate. If ledger persistence fails, do not return the signed bundle.

Ledger-derived retired-root trust may be used for package-issued certificates. Root pruning remains explicit and owner-authorized. Without a ledger, persist a fixed retired-root trust deadline of promotion time plus maximum leaf validity and the clock-skew margin. Retain retired public roots and issuer certificates for reconstruction even after removing a root from the default trust bundle.

## Issuer signing

`SignCSR` must:

1. Decode one valid PEM CSR and verify its signature.
2. Require a supported RSA/ECDSA/Ed25519 public key.
3. Reject requested CA capabilities, arbitrary requested extensions, and unknown critical extensions. Preserve only the supported SAN request; generate all other certificate extensions from the selected profile and issuer policy.
4. Validate the CSR subject against issuer policy.
5. Preserve CSR subject/SANs by default. `WithAdditionalSANs` merges; explicit `WithSubject` or `WithSANs` replaces its field.
6. Validate final subject and SANs against policy again.
7. Require an allowed explicit profile and valid lifetime.
8. Resolve the active issuer revision, sign, write the ledger when enabled, and return a keyless bundle containing leaf, issuer, and root. In `None` mode, use the cached signer; in `Ledger` mode, refresh enough authority state to record the active revision consistently.

SANs are typed DNS, IP, URI, and email values. The issuer controls final content, but the package does not prove real-world DNS/IP/URI/email ownership.

Issuer managers cache a signer with its authority revision. In `Ledger` mode, each sign performs a lightweight revision check and reloads on change, without a goroutine. In `None` mode, the manager may use one initial read until explicit `Reload` or restart. A process that already loaded a retired key may sign until reload; deleting its stored key cannot revoke in-memory key data.

## Generic CSR and certificate bundles

### CSRBundle

`NewCSR` accepts complete leaf identity, typed SANs, supported key spec, and optional key passphrase. It outputs a standards-compliant CSR and PKCS#8 private key.

- `CSRPEM()` returns `CERTIFICATE REQUEST` PEM.
- `KeyPEM()` returns unencrypted PKCS#8 by default or encrypted PKCS#8 on request.
- `SaveCSR(path)` and `SaveKey(path)` write explicit files.
- `Save(directory)` writes `request.pem` and `key.pem` and returns paths.

### CertificateBundle

`LoadBundle` accepts repeatable `WithCert(path)` values and an optional `WithKey(path)` value. Every input may contain one or more PEM certificate blocks; malformed PEM and non-certificate blocks are errors. Files may be supplied in any order.

Build one connected, linear certificate component, which may be partial and may begin at any point:

```text
leaf → intermediate(s) → self-signed root (optional)
intermediate → self-signed root
root
leaf
```

Reject duplicates, disconnected material, ambiguity, cross-signing, and branching. Detect a root only when it is CA-capable, has subject equal to issuer, and validates its own signature; never infer root merely from list position. A key-only bundle is valid material until a certificate is added; a key always belongs to the terminal leaf once one exists.

Certificate additions and key additions are transactional: an error returns the original bundle unchanged. Loading and additions validate parsing, connectivity, signatures, CA constraints, and key matching where applicable, but do not reject expiration or validate hostname/time until explicit verification. Generic validation does not enforce managed-CA parent/child expiry policy.

```go
Certificate() *x509.Certificate // terminal leaf, or nil when none exists
Intermediates() []*x509.Certificate
Root() *x509.Certificate
Certificates() []*x509.Certificate // supplied connected certificates
Complete() bool                     // leaf through validated root
HasKey() bool
KeyMatchesLeaf() bool
Key() crypto.Signer // optional
```

When supplied, parse a supported PKCS#8 key and require exact match to the terminal leaf when one exists. `WithKeyPassphrase` unlocks encrypted keys. Issued keyless bundles are valid: `SaveKey` returns `NotFoundError` and directory save omits `key.pem`.

Provide explicit `SaveCertificate`, `SaveTLSChain`, `SaveFullChain`, and `SaveKey`. Missing required material returns `NotFoundError`; for example, `SaveTLSChain` requires a leaf and `SaveFullChain` requires both a leaf and root. Add a generic certificate export for arbitrary partial bundles. `Save(directory)` writes only applicable outputs and reports the files written.

All generic saves are atomic/no-symlink writes. Existing targets fail unless an explicit replace option is supplied. Private-key mode is `0600`; public files are safely readable.

Separate structural validation from usability checks:

- `Validate` checks parseability, connected topology, signatures, constraints, and key matching;
- `UsableAt(time)` checks that every included certificate is within its validity window and that supplied links are usable, without claiming trust for a rootless bundle;
- `Verify(x509.VerifyOptions)` performs explicit trust, hostname, key-usage, and time verification and returns only an error.

No certificate bundle method enforces leaf-before-parent expiry; that is a managed-CA issuance rule.

## Concurrency, repair, and errors

`Ensure`, `PromotePending`, `DiscardPending`, root pruning, and purge take a context-aware exclusive authority lock: `ca.lock` for FileStore and a serialized write transaction for SQLite. Validate before mutation. A staged operation publishes either the prior active topology or the complete validated pending topology; never expose a mixed generation.

Follow this repository's `errx` conventions and never include secrets in errors:

- validation: options, names, paths, SANs, profiles, lifetimes;
- not found: authority, issuer, key, local unlock, missing bundle key;
- forbidden: owner/group/authentication/safe-access failure;
- conflict: pending root, slug collision, overwrite refusal, lock policy;
- drift: valid immutable-spec or policy mismatch;
- corruption/integrity: malformed JSON/ledger, invalid DER/PKCS#8, bad signature, impossible topology, or missing required active key; classify as `errx.Invalid`;
- unavailable: unsupported platform/security mode;
- unexpected OS/crypto failures: wrapped safely with `errx.New`.

`Inspect` is read-only. `Repair` is owner-only and explicit, and may rebuild only derived state it can prove from valid material. It cannot reconstruct a corrupt authority or invent missing policy. Ledger repair may only remove an incomplete final append after validation.

## Implementation and code-organization guidelines

Keep the package readable through clean logical separation of concerns. The
public managers orchestrate domain operations; policy validation, certificate
construction, key/PEM codecs, storage, locking, and OS-specific security
checks remain separate components. Shared logic belongs in small, named
helpers with one responsibility. Do not create abstractions merely to avoid a
few lines of duplication, and do not make one function responsible for option
parsing, policy validation, cryptography, storage mutation, and error mapping.

Use one internal backend interface for the transactional authority model. It
is an implementation detail, not a public third-party store contract. Keep the
JSON FileStore and SQLiteStore implementations in separate files and isolate
backend-specific locking, transactions, serialization, migrations, and
filesystem/database access from shared authority reconciliation logic. The
shared layer must not depend on JSON fields, SQLite queries, or filesystem
paths.

Organize public APIs by responsibility rather than accumulating them in one
large file. Keep authority lifecycle, issuer signing, CSR generation, bundle
loading/validation, saving, and options in clearly named files. Keep the
backend interface and shared storage model separate from the public manager
types. Keep PEM/key codecs, identity/name policy, SAN policy, certificate
profiles, clocks, and filesystem/security adapters independently testable.

Centralize package errors in `errors.go`. Export the domain error types needed
by callers, such as validation, not-found, conflict, drift, corruption, and
partial-operation errors, and consistently classify/wrap them using this
repository's `errx` conventions. Never duplicate error-kind mapping in each
backend or leak backend-specific errors and secrets through public APIs.

Prefer narrow public functions and explicit wrappers for reusable operations:
for example, separate prepare, promote, discard, inspect, repair, and purge
operations instead of a flag-heavy lifecycle function. Keep option validation
strict and close to the option set it configures, while keeping shared domain
validation reusable by all callers. Public methods should preserve the
documented invariants on every return path, leave objects unchanged on failed
mutations where promised, and make context cancellation and locking behavior
explicit.

Production code should favor straightforward control flow, immutable or
transactionally updated state, descriptive names, small cohesive files, and
tests that mirror the package boundaries. Avoid hidden goroutines, global
state, clever reflection, and backend leakage. Add complexity only when it
protects a stated security, consistency, portability, or performance
requirement.

## Implementation phases

1. Add package docs, models, naming/identity validation, errors, clock/filesystem abstractions, and PEM/key codecs.
2. Implement `CSRBundle` and `CertificateBundle`: generation, normalization, validation, key matching, and safe save/load.
3. Implement internal store model and JSON FileStore: strict ownership, atomic `ca.json` writes, `ca.lock`, pending manifests, optional group-writable JSONL ledger, local-unlock generation map, and narrowly scoped inspection/repair.
4. Implement authority reconciliation: initial provisioning, issuer lifecycle, staged root prepare/promote/discard, trust bundle, pruning, and purge.
5. Implement issuer signing, subject/SAN policies, profiles, ledger writes, and revision-aware issuer reload.
6. Implement SQLiteStore: `modernc.org/sqlite`, migrations, read-only issuer access, DELETE journaling, separate ledger writer storage, transactions, local-unlock rules, parity tests.
7. Add examples, security/lifecycle documentation, and migration/export-import design notes without implementing automatic migration.

## Verification and acceptance criteria

Use standard `testing`, table-driven cases, `t.TempDir`, real filesystem security tests, dependency injection for clock/files, and fuzzing for PEM/order/DER/name boundaries.

Required coverage:

- key algorithms, encrypted-root round trips, missing/wrong unlock, Ed25519 codecs;
- immutable drift, additive issuer definitions, slug collisions, display-name preservation;
- initial provisioning, issuer rotation, staged root prepare/promote/discard, cascaded root/issuer promotion, fixed-window and ledger-based trust retention, interrupted activation recovery;
- validity boundaries, parent/child expiry, serial collision retry;
- owner/group enforcement, symlink rejection, exact modes, local-unlock access, group-writable isolated ledger access, read-only SQLite authority access;
- FileStore/SQLiteStore parity, SQLite migrations/DELETE journal behavior, lock cancellation, read-only inspection;
- CSR signature/key validation, subject/SAN policy before and after overrides, profiles, leaf validity;
- arbitrary PEM ordering, root-only/intermediate-only/key-only/partial bundles, ambiguity rejection, encrypted key loading, transactional key mismatch;
- ledger writes/lookups proving no leaf private key persists, malformed final-record handling, and issuance failure when ledger durability fails;
- atomic save/replace behavior and keyless issued bundle behavior.

Run `go test ./...`, race tests for concurrent ensure/sign/load paths, and Linux/Darwin ownership tests.
