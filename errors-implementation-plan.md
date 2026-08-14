# `errx`: Implementation Plan for a Transport-Neutral Go Error Package

## 1. Purpose

This document defines a repository-wide error model for Go packages and applications. The design keeps Go's standard error behavior, adds stable semantic classification, lets domain packages retain typed context, and supports HTTP, gRPC, CLI, logging, and other adapters without coupling the core package to any transport.

The package is named `errx` so it can be imported alongside the standard library's `errors` package without aliases:

```go
import (
	"errors"

	"example.com/project/errx"
)
```

## 2. Goals and non-goals

### Goals

- Preserve useful causes through standard Go error trees.
- Expose a small, stable set of transport-neutral semantic kinds.
- Let an outer abstraction deliberately reclassify an inner failure.
- Keep causal structure separate from semantic taxonomy.
- Make structured errors immutable after construction.
- Keep domain-specific context in the package that owns the domain.
- Work naturally with `errors.Is`, `errors.As`, `errors.Unwrap`, and `errors.Join`.
- Make safe, common usage concise while leaving extension points orthogonal.
- Support transport and presentation adapters outside the core package.
- Document when exposing a cause through `Unwrap` becomes part of an API contract.

### Non-goals

- Replacing Go's built-in `error` interface.
- Defining every domain error in one shared package.
- Assigning HTTP or gRPC status codes in the core package.
- Owning logging, telemetry, localization, response bodies, or retry loops.
- Treating formatted error text as a stable machine-readable API.
- Automatically inferring classifications from arbitrary third-party errors.

## 3. Design principles

### 3.1 Causality and classification are different dimensions

An error tree describes causes and contextual wrapping. A kind describes how a layer interprets a failure. Do not create ceremonial wrappers such as `NotFound -> FileNotFound -> ConfigFileNotFound` merely to encode an “is-a” hierarchy.

A domain error can carry its semantic kind directly:

```text
LoadConfigError
  -> ConfigFileNotFoundError { path }
    -> *os.PathError
      -> syscall.ENOENT
```

`ConfigFileNotFoundError` can report `errx.NotFound`; no physical `NotFound` wrapper is required.

### 3.2 The nearest classification to the caller wins

Classification is an interpretation at an abstraction boundary. An outer error may intentionally override an inner classification:

```text
ProviderUnavailableError  Kind: Unavailable
  -> context.DeadlineExceeded  Kind: Timeout
```

`KindOf` returns the outermost applicable kind. This permits an application or domain boundary to expose a meaning appropriate to its callers while retaining the lower-level cause for diagnostics.

### 3.3 Wrapping is an API decision

When a type implements `Unwrap`, callers can depend on the exposed cause through `errors.Is` and `errors.As`. Public packages must unwrap only errors whose identities or types callers are intentionally allowed to observe.

Internal packages may expose implementation causes more freely. Public SDKs should commonly retain a cause internally for diagnostics but omit `Unwrap`, or translate it into a stable domain-owned cause.

### 3.4 Structured errors are immutable

Constructors validate state. Fields that participate in invariants remain unexported and are exposed through read-only accessors. All structured error types are pointer types, constructors return pointers, and methods use pointer receivers.

### 3.5 Text is for people; fields are for programs

`Error()` is human-readable but unstable. Classification, concrete types, accessors, and optional capability interfaces are the machine-readable API. Sensitive or high-cardinality values should not automatically appear in messages.

## 4. Recommended package structure

```text
project/
  errx/
    doc.go                 package documentation and invariants
    kind.go                Kind, Classified, validation, String
    inspect.go             KindOf, KindsOf, IsKind
    error.go               optional immutable generic wrapper
    capabilities.go        small optional interfaces
    *_test.go

  errxhttp/
    status.go              Kind -> HTTP status mapping
    response.go            optional application-specific response policy
    status_test.go

  errxgrpc/                optional; add only when needed
    code.go

  configv2/
    errors.go              config-owned error types and constructors
    errors_test.go

  internal/app/
    errors.go              application-boundary errors, if useful
```

Keep `errx` dependency-light. It should normally import only standard packages such as `errors` and `fmt`; it must not import `net/http`, gRPC, a logger, or domain packages. Dependency direction is:

```text
errx <- domain packages <- application <- transport adapters
```

Adapters may import `errx`; `errx` never imports adapters.

## 5. Core semantic model

### 5.1 `Kind`

Start with a deliberately small set:

```go
package errx

type Kind uint8

const (
	Unknown Kind = iota
	Invalid
	Validation
	Unauthenticated
	Forbidden
	NotFound
	Conflict
	RateLimited
	Unavailable
	Timeout
	Internal
)
```

Meanings:

| Kind | Meaning |
|---|---|
| `Unknown` | No single semantic classification is available. |
| `Invalid` | Input could not be decoded, parsed, or interpreted. |
| `Validation` | Input was understood but violates a domain rule. |
| `Unauthenticated` | Valid authentication is missing or failed. |
| `Forbidden` | The caller is known but not permitted to act. |
| `NotFound` | The requested domain resource does not exist. |
| `Conflict` | The operation conflicts with current state or invariants. |
| `RateLimited` | Work was rejected because an enforced rate was exceeded. |
| `Unavailable` | A required service or resource is temporarily unavailable. |
| `Timeout` | The operation exceeded its deadline. |
| `Internal` | A classified unexpected implementation or system failure. |

`Unknown` means “unclassified or ambiguous”; `Internal` is an explicit classification. Keep those distinct so inspection does not silently convert missing metadata into a claim about the failure.

Do not add kinds for package-specific concepts such as `ConfigFileMissing`. Represent those with concrete domain types or, if an externally stable identifier is required, a separate domain-owned string code.

### 5.2 Core interface

```go
type Classified interface {
	error
	Kind() Kind
}
```

`Kind()` must return a valid declared value. Unknown future numeric values are treated as `Unknown` at inspection boundaries.

### 5.3 Inspection API

Recommended public API:

```go
func KindOf(err error) (Kind, bool)
func KindsOf(err error) []Kind
func IsKind(err error, want Kind) bool
```

Contracts:

- `KindOf(nil)` returns `(Unknown, false)`.
- For a linear chain, `KindOf` returns the first valid `Classified` value encountered from outermost to innermost.
- `IsKind` tests the effective kind returned by `KindOf`; it does not search for any inner kind.
- `KindsOf` returns all distinct valid kinds found throughout the tree in deterministic depth-first, outer-to-inner order.
- Inspection must be cycle-safe if custom error implementations create malformed cycles.

Returning `(Kind, bool)` prevents `Unknown`, “no classification,” and conflicting classifications from being confused with `Internal`.

## 6. Joined error behavior

Go errors form trees because a value may implement `Unwrap() []error`, including values returned by `errors.Join`. A joined error can contain incompatible classifications, so `KindOf` must not guess.

The policy is:

1. An outer error's own valid `Kind()` wins, even if its wrapped subtree contains multiple kinds. The outer layer explicitly resolved the abstraction.
2. Otherwise inspect all non-nil children.
3. If no child has an effective kind, return `(Unknown, false)`.
4. If every classified child resolves to the same kind, return that kind and `true`.
5. If classified children resolve to different kinds, return `(Unknown, false)`.
6. `KindsOf` remains available when callers need the complete set.

Example:

```go
joined := errors.Join(notFoundErr, unavailableErr)

kind, ok := errx.KindOf(joined)
// kind == errx.Unknown, ok == false

kinds := errx.KindsOf(joined)
// []errx.Kind{errx.NotFound, errx.Unavailable}
```

An application may wrap that join in an explicitly classified outer error when its boundary has a meaningful single interpretation.

Do not define arbitrary priority such as “Internal always wins.” Transport adapters must handle an ambiguous join using their own conservative fallback.

## 7. Generic immutable wrapper

A single generic wrapper is useful for adding context or reclassifying at a boundary. Avoid a large inheritance-like family of shared error structs.

```go
type Error struct {
	message string
	kind    Kind
	cause   error
	unwrap  bool
}

type Option func(*options) error

func New(message string, opts ...Option) *Error
func WithKind(kind Kind) Option
func WithCause(cause error) Option
func WithoutUnwrap() Option

func (e *Error) Error() string
func (e *Error) Kind() Kind
func (e *Error) Unwrap() error
```

Recommended constructor invariants:

- Reject or panic on programmer errors consistently; preferably return an error from a separate validating constructor if inputs can be dynamic.
- Require a non-empty message or a non-nil cause.
- Accept only declared kinds; omitted kind means `Unknown` and the type should not accidentally claim `Internal`.
- Do not expose mutable option or field state after construction.
- `WithoutUnwrap` causes `Unwrap()` to return `nil`; the retained cause may be available only to trusted internal diagnostics, if such access is truly needed.

There is an important interface nuance: if every `*Error` has a `Kind()` method, an unclassified wrapper is still `Classified`. Its `Kind()` can return `Unknown`, and traversal must continue inward rather than stopping. `KindOf` therefore skips `Unknown` and looks at the wrapped subtree.

### 7.1 Message composition

Choose one predictable rule:

```go
func (e *Error) Error() string {
	switch {
	case e == nil:
		return "<nil>"
	case e.message == "" && e.cause != nil:
		return e.cause.Error()
	case e.cause == nil || !e.unwrap:
		return e.message
	default:
		return e.message + ": " + e.cause.Error()
	}
}
```

Context wrappers should add useful text. Classification alone should not add repetitive text such as `not found: file not found: resource not found`. Prefer one domain error that supplies both context and a kind.

If redaction rules differ from unwrapping rules, introduce an explicit private/public rendering policy in an application layer rather than overloading `Error()`.

### 7.2 Compile-time assertions

Use pointer receivers consistently and assert intended contracts:

```go
var _ error = (*Error)(nil)
var _ Classified = (*Error)(nil)
var _ interface{ Unwrap() error } = (*Error)(nil)
```

Every domain error should have equivalent assertions in its owning package.

## 8. Domain-owned contextual errors

The package that understands a concept owns its error type. `errx` should not contain `FileNotFoundError`, `ConfigParseError`, database errors, or business-rule errors.

```go
package configv2

type FileNotFoundError struct {
	path  string
	cause error
}

func NewFileNotFoundError(path string, cause error) *FileNotFoundError {
	return &FileNotFoundError{path: path, cause: cause}
}

func (e *FileNotFoundError) Error() string {
	return "configuration file was not found"
}

func (e *FileNotFoundError) Kind() errx.Kind { return errx.NotFound }
func (e *FileNotFoundError) Path() string    { return e.path }
func (e *FileNotFoundError) Unwrap() error   { return e.cause }

var _ error = (*FileNotFoundError)(nil)
var _ errx.Classified = (*FileNotFoundError)(nil)
```

The path is structured context. Whether it appears in logs, traces, or a response is decided by consumers. It need not appear in `Error()`.

If exposing `*os.PathError` is not a supported public contract, omit `Unwrap` or return a stable domain-owned cause instead. Document the choice for each public error type.

### 8.1 Domain codes, if needed

Stable application codes are separate from kinds:

```go
type Coded interface {
	error
	Code() string
}

const CodeConfigFileMissing = "CONFIG_FILE_MISSING"
```

Codes are owned by the domain or public API that promises their stability. They are not HTTP status numbers and should not be required of every error.

## 9. Orthogonal capabilities

Do not turn `Error` into a giant base class. Add small capability interfaces only when a concrete consumer exists:

```go
type Retryable interface {
	error
	Retryable() bool
}

type PublicMessenger interface {
	error
	PublicMessage() string
}

type Fieldser interface {
	error
	Fields() map[string]any
}
```

Guidelines:

- Retryability is not implied solely by kind. An `Unavailable` error may be permanent for a given operation, and a `Conflict` may be retryable after rereading state.
- Prefer typed accessors on domain errors over a generic `map[string]any` when callers know the type.
- If `Fields()` is added, return a fresh map or otherwise prevent mutation of internal state.
- Public messages must be safe for untrusted clients and must never default to raw cause text.
- Add `Temporary`, severity, user-action, or telemetry capabilities only after a real use case establishes their semantics.

## 10. `errors.Is` and `errors.As`

Use standard Go traversal for identity and concrete type inspection:

```go
if errors.Is(err, context.DeadlineExceeded) {
	// Stable underlying identity was intentionally exposed.
}

var missing *configv2.FileNotFoundError
if errors.As(err, &missing) {
	path := missing.Path()
	_ = path
}
```

Use `errx.IsKind` for semantic classification:

```go
if errx.IsKind(err, errx.NotFound) {
	// Broad semantic handling.
}
```

Do not manufacture category sentinels solely to force classification through `errors.Is`. If a domain has a genuine stable sentinel, it may expose one normally. Custom `Is(target error) bool` implementations must perform shallow equivalence checks only; they must not recursively call `errors.Is` or `Unwrap` themselves.

## 11. Transport adapters

### 11.1 HTTP

HTTP mapping belongs in `errxhttp` or the application transport layer:

```go
func Status(err error) int {
	kind, ok := errx.KindOf(err)
	if !ok {
		return http.StatusInternalServerError
	}

	switch kind {
	case errx.Invalid:
		return http.StatusBadRequest
	case errx.Validation:
		return http.StatusUnprocessableEntity
	case errx.Unauthenticated:
		return http.StatusUnauthorized
	case errx.Forbidden:
		return http.StatusForbidden
	case errx.NotFound:
		return http.StatusNotFound
	case errx.Conflict:
		return http.StatusConflict
	case errx.RateLimited:
		return http.StatusTooManyRequests
	case errx.Unavailable:
		return http.StatusServiceUnavailable
	case errx.Timeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
```

The exact mapping is application policy. For example, a server may map its own operation timeout to 504, 503, or 408 depending on where the timeout occurred. Allow an application-supplied mapping or wrapper around the default adapter.

Response serialization should use a safe public message and optional stable domain code. Never serialize `err.Error()` or structured diagnostic fields by default.

### 11.2 gRPC, CLI, jobs, and messaging

- A gRPC adapter maps kinds independently to `codes.Code`.
- A CLI maps kinds to exit codes and user-facing guidance.
- A worker uses kind plus a `Retryable` capability to decide acknowledgment or retry behavior.
- A GraphQL adapter maps kinds and domain codes into its error extensions.

None of these policies belongs in `errx`.

## 12. API examples

### 12.1 Creating a domain failure

```go
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, NewFileNotFoundError(path, err)
		}
		return nil, errx.New(
			"read configuration file",
			errx.WithKind(errx.Internal),
			errx.WithCause(err),
		)
	}

	return parse(b)
}
```

### 12.2 Reclassifying at an abstraction boundary

```go
func Authenticate(ctx context.Context, token string) error {
	user, err := users.LookupByToken(ctx, token)
	if err != nil {
		if errx.IsKind(err, errx.NotFound) {
			return errx.New(
				"authentication failed",
				errx.WithKind(errx.Unauthenticated),
				errx.WithCause(err),
				errx.WithoutUnwrap(), // avoid exposing account existence/cause
			)
		}
		return errx.New(
			"authentication service unavailable",
			errx.WithKind(errx.Unavailable),
			errx.WithCause(err),
		)
	}

	_ = user
	return nil
}
```

Here the outer classification intentionally replaces `NotFound`, and the cause is not exposed because doing so would leak implementation or security-sensitive information.

### 12.3 Generic handling plus typed context

```go
if errx.IsKind(err, errx.NotFound) {
	var e *configv2.FileNotFoundError
	if errors.As(err, &e) {
		logger.Info("optional config absent", "path", e.Path())
	}
}
```

### 12.4 Multiple failures

```go
err := errors.Join(validateName(name), validatePort(port))
if err != nil {
	// If both children are Validation, KindOf resolves to Validation.
	// If their kinds differ, KindOf reports no single effective kind.
	return err
}
```

## 13. Usage rules

1. Return `nil`, not a typed nil pointer stored in an `error` interface.
2. Use `%w` only when the wrapped cause is intentionally part of the observable API; use `%v` or a non-unwrapping domain error otherwise.
3. Use `errx.IsKind` for broad semantics, `errors.As` for typed context, and `errors.Is` for stable identities.
4. Never compare or parse `Error()` strings.
5. Add context once per meaningful abstraction boundary; avoid repetitive wrappers.
6. Reclassify only when the outer abstraction has a meaning that differs from the inner failure.
7. Domain packages own their error types and codes. Do not add domain vocabulary to `errx`.
8. Do not include secrets, tokens, full payloads, or sensitive identifiers in messages or default fields.
9. Log or render an error once at the boundary responsible for handling it; intermediate layers should generally return it.
10. Preserve cancellation deliberately. Do not casually reclassify or hide `context.Canceled` and `context.DeadlineExceeded` when callers need to coordinate cancellation.
11. Treat kind values, public domain codes, exported error types, accessors, and exposed unwrap targets as compatibility commitments.
12. Prefer constructors over struct literals for every structured error.

## 14. Common pitfalls

### Transport coupling in disguise

`StatusCode() int` with values such as 404 or 503 is still HTTP coupling even if `net/http` is not imported. Keep kind and transport mapping separate.

### Confusing `Unknown` with `Internal`

Unclassified and explicitly internal failures are different. Adapters may map both to a safe 500 response, but inspection should preserve the distinction.

### Encoding taxonomy as wrapper depth

Extra category wrappers create allocations, repetitive messages, and fragile invariants. Put semantic metadata on the error that owns the context.

### Forbidding valid reclassification

Inner timeout and outer unavailable classifications can both be correct at their respective layers. Effective classification is caller-relative; outermost wins.

### Exported mutable fields

Public `Message`, `Err`, `Kind`, maps, or slices let callers invalidate constructor guarantees and introduce races. Use private fields and read-only accessors or defensive copies.

### Accidental implementation leaks

Unwrapping `sql.ErrNoRows`, filesystem types, or vendor SDK types lets callers depend on them. Decide exposure deliberately for each public boundary.

### Ambiguous joined errors

A join does not always have one kind. Do not select a child based on order or hidden priority. Return no single kind when classified branches conflict.

### Typed nils

Returning `(*MyError)(nil)` as `error` produces a non-nil interface. Constructors and return sites must avoid this.

### Pointer/value method-set surprises

Embedding and mixed receivers make it unclear which values implement `error`. Standardize on pointer receivers and add compile-time assertions.

### Overloaded kinds

Do not encode retryability, severity, visibility, transport status, and domain code into `Kind`. These are separate dimensions.

### Unsafe presentation

Cause strings are often unsuitable for clients. Transport adapters must use safe messages and stable public codes rather than raw error text.

### Cyclic custom trees

Bad third-party implementations can return themselves from `Unwrap`. Tree inspection helpers should avoid infinite traversal where practical and document their safety limits.

## 15. Implementation phases

### Phase E0 — Contract and semantics

- Write package documentation and freeze the definitions of each initial kind.
- Implement `Kind`, `String`, validity checks, and `Classified`.
- Specify `Unknown` versus `Internal` behavior.
- Decide the supported Go version and confirm joined-error requirements.

Exit criteria: exported semantics are reviewed independently of any transport.

### Phase E1 — Tree inspection

- Implement `KindOf`, `KindsOf`, and `IsKind`.
- Support both `Unwrap() error` and `Unwrap() []error`.
- Implement outermost-wins behavior and conflict handling for joins.
- Make traversal deterministic and cycle-safe.

Exit criteria: exhaustive table tests cover nil, linear, nested, joined, repeated, unclassified, explicitly classified, conflicting, and malformed cyclic errors.

### Phase E2 — Immutable wrapper

- Implement `Error`, constructors, and options.
- Validate messages, kinds, causes, and unwrap policy.
- Define nil-safe and non-repetitive message behavior.
- Add compile-time interface assertions.

Exit criteria: mutation is impossible through exported API, and tests cover all option combinations and typed-nil hazards.

### Phase E3 — First domain integration

- Add domain-owned error types to `configv2` or another pilot package.
- Give each type private fields, constructors, typed accessors, a kind, and a documented unwrap contract.
- Replace string matching and transport codes at call sites.
- Confirm `errors.As`, `errors.Is`, and `errx.IsKind` behave across real trees.

Exit criteria: the pilot package exposes no HTTP concept and callers can perform both generic and typed handling.

### Phase E4 — Transport adapters

- Implement default HTTP mapping in `errxhttp`.
- Add safe response serialization only if the application has a defined public response schema.
- Add gRPC or CLI adapters only for active consumers.
- Permit application-level mapping overrides without mutating global state.

Exit criteria: adapter tests cover every kind, unknown and ambiguous errors, public-message safety, and overrides.

### Phase E5 — Repository migration

- Inventory existing sentinels, typed errors, status helpers, and string comparisons.
- Map existing errors to kinds and domain-owned types.
- Migrate one package boundary at a time.
- Provide temporary compatibility helpers only where necessary and mark them deprecated.
- Update package examples and contributor guidance.

Exit criteria: no core/domain package returns transport status codes or relies on error-string parsing.

### Phase E6 — Optional capabilities

- Add `Retryable`, public-message, structured-field, or stable-code support only when required by a real consumer.
- Define precedence and tree traversal semantics for each capability before exporting it.
- Keep capabilities independent of `Kind`.

Exit criteria: every capability has a documented consumer, clear semantics, and contract tests.

## 16. Testing strategy

### Unit tests

Use table-driven tests for:

- Every kind's value, validity, and string representation.
- `KindOf` on nil and unclassified errors.
- Outer classified errors overriding inner classified errors.
- Unknown outer wrappers allowing inner classification to resolve.
- Identical and conflicting kinds across joins.
- Nested joins and nil join members.
- Duplicate removal and deterministic order in `KindsOf`.
- `errors.As` reaching domain context when unwrapping is enabled.
- `errors.Is` reaching intentionally exposed sentinels.
- Non-unwrapping errors hiding their retained cause.
- Message formatting with empty message, nil cause, and both present.
- Typed nil pointer behavior.
- Defensive copies for any exported map or slice accessors.
- Cycle handling for custom malformed errors.

### Contract tests

Create reusable tests that domain packages can invoke against error constructors:

- Constructor returns a non-nil pointer.
- Returned pointer implements `error` and `Classified` as intended.
- Kind is stable.
- Accessors return constructor values without exposing mutation.
- Unwrap exposure matches documented policy.
- Message contains no fields designated sensitive.

### Adapter tests

- Every known kind maps explicitly.
- Unknown, unclassified, and conflicting joins use a conservative fallback.
- Application overrides take precedence without process-wide mutable globals.
- Client responses never contain raw cause text by default.

### Fuzz tests

Fuzz inspection over generated wrapper/join trees to verify:

- No panic or infinite loop.
- Deterministic results.
- A classified outer node always wins.
- A homogeneous join resolves to its common kind.
- A heterogeneous join does not invent a single kind.

### Compatibility tests

Once published, pin numeric `Kind` values only if they are serialized or otherwise externally observable. Prefer not to serialize them; serialize stable names or domain codes instead. Add API-surface checks for exported types and methods where compatibility matters.

## 17. Migration guide

### From numeric status errors

Before:

```go
return errors.NewCoded(404, "config missing", err)
```

After:

```go
return configv2.NewFileNotFoundError(path, err)
```

The HTTP handler calls `errxhttp.Status(err)`.

### From category wrapper stacks

Before:

```go
return NewNotFound(NewFileNotFound(NewConfigFileError(path, err)))
```

After:

```go
return configv2.NewFileNotFoundError(path, err)
```

The concrete domain error reports `NotFound` directly.

### From string matching

Before:

```go
if strings.Contains(err.Error(), "not found") { /* ... */ }
```

After:

```go
if errx.IsKind(err, errx.NotFound) { /* ... */ }
```

For domain data, use `errors.As` and accessors.

### From leaked vendor errors

Translate third-party failures into stable domain types. Expose the vendor cause only if it is part of the promised API; otherwise retain it privately or log it at the owning boundary.

## 18. Documentation requirements

Package documentation must state:

- Exact meaning of every kind.
- Outermost-wins classification behavior.
- Ambiguous `errors.Join` behavior.
- The difference between `Unknown` and `Internal`.
- That `Error()` text is not stable or safe for automatic client exposure.
- That unwrapping exposes an observable compatibility surface.
- Pointer-only construction and immutability conventions.
- How to choose between `errors.Is`, `errors.As`, `errx.IsKind`, and `KindsOf`.

Every exported domain error must document:

- Its semantic kind.
- Its stable accessors and any stable code.
- Whether it unwraps, and which underlying identities/types are supported.
- Whether its message is safe for logs, users, or neither.
- Whether it implements optional capabilities such as retryability.

## 19. Recommended initial public surface

Keep version 1 intentionally small:

```go
package errx

type Kind uint8

const (
	Unknown Kind = iota
	Invalid
	Validation
	Unauthenticated
	Forbidden
	NotFound
	Conflict
	RateLimited
	Unavailable
	Timeout
	Internal
)

func (k Kind) String() string

type Classified interface {
	error
	Kind() Kind
}

func KindOf(error) (Kind, bool)
func KindsOf(error) []Kind
func IsKind(error, Kind) bool

type Error struct { /* unexported fields */ }
type Option func(*options) error

func New(string, ...Option) *Error
func WithKind(Kind) Option
func WithCause(error) Option
func WithoutUnwrap() Option

func (*Error) Error() string
func (*Error) Kind() Kind
func (*Error) Unwrap() error
```

Before committing to the exact `Option` API, prototype its validation ergonomics. If invalid options must be reported at runtime, prefer either `New(...) (*Error, error)` or distinct constructors over hidden panics. Do not export convenience functions for every kind until repeated usage proves they improve readability.

## 20. Acceptance criteria

The first stable release is complete when:

- Core `errx` has no transport or domain imports.
- Kinds are documented and used consistently.
- Causal trees and classification hierarchies are not conflated.
- Outer boundaries can intentionally reclassify inner failures.
- Joined errors have deterministic, tested ambiguity semantics.
- Structured errors cannot be mutated through public fields.
- Pointer receiver conventions and compile-time assertions are consistent.
- Every public unwrap decision is documented.
- Domain errors own domain context and codes.
- HTTP and other mappings live outside the core.
- Tests cover linear chains, trees, overrides, hidden causes, and safe presentation.
- Repository guidance clearly explains construction, inspection, wrapping, migration, and security rules.

This design keeps the happy path small—construct a domain error, classify with `IsKind`, inspect with `errors.As`—while preserving enough structure for transports, operations, and future capabilities to evolve independently.
