# `config`: Implementation Plan for a Typed, Provenance-Aware Go Configuration Package

## 1. Purpose

`config` loads one application-defined Go struct from explicit sources, resolves
their values deterministically, exposes the winning source for each field, permits
validated runtime overrides, and writes a deliberately curated replacement file.

It is a startup/configuration package, not a distributed configuration system. A
successfully loaded `Config[T]` is a valid immutable snapshot to readers; every
state-changing operation validates before publication.

## 2. Goals and non-goals

### Goals

- Typed configuration declared by one non-pointer struct type `T`.
- Deterministic precedence: `default < file < env < flag < set`.
- TOML, YAML, and JSON input and fresh-document output.
- Explicit file selection, including an operator override environment variable.
- Per-field winning provenance and per-layer presence inspection.
- Strict schema, type-conversion, unknown-file-key, required-value, and validation failures.
- Source-aware, secret-safe persistence with atomic replacement.
- Race-free reads and mutations, with defensive copies for reference values.
- Config-owned contextual errors classified using the existing `errx` package.

### Non-goals

- Viper, mapstructure, pflag, or any CLI library in the core package.
- Multiple files, search paths, fallback candidates, merging, remote sources, or live reload.
- Preservation of comments, ordering, unknown keys, aliases, or formatting on save.
- Pointer fields, maps, arbitrary slices, interfaces, custom decode hooks, or reflection-free schemas.
- Global configuration state, directory creation, ownership changes, or privilege elevation.

## 3. Design principles and invariants

### One schema, one selected file, five layers

The reflected schema is the authority for every source. An invocation selects zero or
one file. A file is a single layer, never a merge input. A source is present even when
its decoded value is `0`, `false`, `""`, or an empty list.

### Explicit promotion prevents accidental persistence

Environment and flag values are transient. `Set` is the sole way to promote an
externally acquired value into the runtime/persistable layer. `save:"true"` is always
a persistence allow-list: `Set` never bypasses it.

### Fail before publish

`Load` builds a candidate state privately and publishes it only after schema, source,
conversion, required, and semantic checks pass. `Set` validates a full candidate
snapshot before replacing active state. A failed later `Load` preserves prior state.

### Public state cannot be mutated indirectly

`Values` returns a defensive copy. Version 1 supports `[]string`, so copying the
outer struct alone is insufficient: supported slices are cloned recursively on return.
The implementation never retains caller-owned mutable slice storage.

### Error meaning is separate from causes

`config` owns contextual error types and reports their `errx.Kind`. It does not wrap
imaginary shared `FileNotFound`, `Required`, or transport-status errors. Every
`Unwrap` decision is documented and tested as a compatibility behavior.

### Security defaults are intentional

Sensitive values never appear in diagnostics, error messages, generated defaults, or
provenance detail. They are written only under the persistence rules below. A selected
file may be read through a symlink, but `Save` refuses to replace a symlink or any
existing non-regular file.

## 4. Public API

The initial exported surface is deliberately small. These signatures are contractual.

```go
package config

type Config[T any] struct { /* unexported */ }
type Option func(*options) error

func New[T any](opts ...Option) *Config[T]
func Load[T any](opts ...Option) (*Config[T], error)

func (c *Config[T]) Load() error
func (c *Config[T]) Values() T
func (c *Config[T]) Source(key string) (Source, bool)
func (c *Config[T]) Origin(key string) (Origin, bool)
func (c *Config[T]) HasSource(key string, source Source) bool
func (c *Config[T]) Set(key string, value any) error
func (c *Config[T]) Validate() error
func (c *Config[T]) Save() error
func (c *Config[T]) FileLoaded() bool
func (c *Config[T]) Path() string
func (c *Config[T]) FlushLog(logger *slog.Logger)

func WithFile(path string) Option
func WithConfigEnv(envVar string) Option
func WithEnvPrefix(prefix string) Option
func WithFlagSource(source FlagSource) Option
func WithValidator(name string, fn FieldValidator) Option
func WithInitial(key string, value any) Option
```

`New` stores options only and never panics for invalid user input. Options are applied
at the beginning of `Load`, so construction errors are returned and available in
initialization diagnostics. `Load[T]` is exactly `New[T](opts...).Load()` and returns
its config object even on error so diagnostics can be flushed.

A nil option, a nil/typed-nil flag source or validator, duplicate `WithFile`, duplicate
`WithConfigEnv`, duplicate validator name, duplicate `WithInitial` key, invalid env
prefix, and malformed option argument are all `OptionError`s from `Load`. `WithInitial`
keys are checked against the schema and their values converted after registry creation.
No exported sentinel errors are provided; callers use `errors.As` for the documented
config-owned types and `errx.IsKind` for broad handling.

`T` must be a non-pointer struct. This is checked during `Load`; any other `T` returns
`SchemaError`. The zero value of `Config[T]` is unsupported.

`Path`, `FileLoaded`, `Values`, `Source`, `Origin`, and `HasSource` describe the last
successfully published state. Before the first success: `Path` is `""`, `FileLoaded`
is false, `Values` returns the zero `T`, and inspection returns `false`. `Set`,
`Validate`, and `Save` return `*NotLoadedError` before success.

### Sources and provenance

```go
type Source uint8

func (s Source) String() string

const (
    SourceNone Source = iota
    SourceDefault
    SourceFile
    SourceEnv
    SourceFlag
    SourceSet
)

type Origin struct {
    Source Source
    Detail string // file: selected path; env: variable; flag: flag name; otherwise empty
}
```

`Source(key)` reports the winning layer for a known, resolved key. `HasSource(key,
source)` reports whether a key is present in a layer, whether or not it wins.
`SourceNone` is not valid for `HasSource`. Unknown keys return `false`, not an error;
`Set` rejects an unknown key. Stable string forms are `none`, `default`, `file`, `env`,
`flag`, and `set`. Callers use `Origin`, never parse a formatted source label.

### Environment, flags, and validators

```go
type FlagSource interface {
    Lookup(name string) (value string, changed bool, found bool)
}

type FieldValidator func(FieldContext, any) error

type FieldContext struct {
    Field  string
    Key    string
    Origin Origin
}

type Validatable interface { ValidateConfig() error }
```

Only a changed flag is present. If `WithFlagSource` is supplied, every declared flag
must be found; a missing declared flag is `FlagBindingError`. If no source is supplied,
the whole flag layer is absent. This catches misspelled CLI wiring.

Field validators receive a decoded value for every present field, including defaults.
Their returned error is wrapped in `ValidationError`; the package never recovers a
validator panic. For cross-field rules, a value `T` may implement `Validatable`. It
runs after field validators in lexical key order on a private candidate copy.

## 5. Schema and tag contract

Only exported fields participate. Every exported field must be supported or explicitly
excluded with `cfg:"-"`; silently ignoring an exported field is a schema error.
Unexported fields are ignored. A field with any config tag and `cfg:"-"` is an error.

Supported leaves are exactly:

```text
string, bool, int, int64, uint, uint64, float64, time.Duration, []string
```

Named aliases are unsupported except `time.Duration`. A struct is a nested namespace
only if it is not `time.Duration`; it must contain only supported/excluded exported
fields. Pointers, maps, arrays, interfaces, funcs, and arbitrary slices are errors.

| Tag | Exact contract |
|---|---|
| `cfg` | Non-empty single path component or `-`. Omitted derives one from Go field name. Dots are forbidden. |
| `env` | `-` disables env. Explicit non-empty value is the exact variable name. Omitted derives one from prefix and full path. |
| `flag` | Exact non-empty flat flag name. Omitted means no flag binding. |
| `default` | Raw value decoded with the string grammar. Empty default is valid. |
| `required` | Must be exactly `true`; omitted means optional. It conflicts with `default`. |
| `sensitive` | Must be exactly `true`; omitted means false. |
| `save` | Must be exactly `true`; omitted means false. |
| `validate` | Exact registered validator name; omitted means none. |

Derived names are lower snake case at Go initialism boundaries: `URLValue` →
`url_value`, `TLS` → `tls`, `UserID` → `user_id`, and `HTTPServerURL` →
`http_server_url`. Derived variables join optional prefix and all path components with
`_` in uppercase; `MYAPP` plus `network.http_server_url` becomes
`MYAPP_NETWORK_HTTP_SERVER_URL`. Prefixes may contain only uppercase ASCII letters,
digits, and `_`, and may not start or end with `_`.

Named nested structs use their `cfg` component. Anonymous embedded supported structs
are flattened and may not carry `cfg`. Duplicate config keys, derived/explicit env
variables, and explicit flag names are errors. Fields are ordered lexically by full
key for deterministic behavior.

## 6. Resolution, conversion, and validation

For each leaf, resolution is:

```text
default < file < env < flag < set
```

There is no implicit zero-value layer. An optional unresolved field decodes as its Go
zero value and has `SourceNone`; a required unresolved field fails. Present empty
strings, zero numbers, false booleans, and empty arrays satisfy `required`.

Defaults decode while building the defaults layer. Invalid defaults fail as
`DefaultValueError` before file/env/flag processing. File values retain format-native
scalar/list form; env and flags are strings. `Set` accepts either a string under the
same grammar or the exact target Go type; `[]string` is cloned.

| Target | Accepted string representation |
|---|---|
| `string` | Literal string. |
| `bool` | `strconv.ParseBool` grammar. |
| signed/unsigned integer | Base-10 only, full-width range checked. |
| `float64` | `strconv.ParseFloat`, rejecting NaN and infinities. |
| `time.Duration` | `time.ParseDuration` grammar. |
| `[]string` | JSON string array, e.g. `["a","b"]`; no delimiter shorthand. |

Format-native numbers must fit exactly: integers reject fractions; signedness and
width are checked; floats reject non-finite values. YAML/TOML timestamps, maps,
binary values, and mixed-type arrays are unsupported.

`Load`, `Set`, and `Validate` always perform full semantic validation. `Set` creates a
prospective resolved snapshot, runs all field validators and `ValidateConfig`, and
publishes only if all pass. Cross-field invariants therefore remain true after runtime
mutation.

`WithInitial` entries form the initial `set` layer. Duplicates are option errors.
They survive later successful `Load` calls until replaced by `Set`; v1 has no unset or
revert API.

## 7. File selection and formats

`WithFile` accepts at most one non-empty path. `WithConfigEnv` accepts an env-variable
name whose non-empty value overrides `WithFile`; an unset or empty variable falls back
to `WithFile`. With neither path, loading is fileless and `Save` returns
`*NoConfigFileError`.

The selected value is expanded via `os.ExpandEnv`, then leading `~`/`~/` via
`os.UserHomeDir`; `~user` is never expanded. An empty result, unresolved leading `~`,
or an extension outside `.toml`, `.yaml`, `.yml`, `.json` is a load error even if the
target does not yet exist. `Path` and file provenance use the expanded cleaned path;
the package does not make it absolute.

Only `fs.ErrNotExist` is an absent file and allows loading to continue. Permission
errors, directories, broken symlinks, unsupported formats, and parse/decode failures
fail `Load`. File absence is represented by `FileLoaded() == false`, not an error.

Decoders create common trees of strings, bools, finite numbers, and scalar `[]any`.
They reject duplicate/non-string mapping keys, aliases that make cycles, unsupported
values, and keys that do not exactly match a schema leaf/namespace. Unknown keys are
errors. Scalars cannot collide with namespaces. Decoding must not lose integer
precision through `map[string]any`.

Output is a fresh nested document in lexical key order. JSON is indented with a
trailing newline. TOML/YAML use deterministic lexical ordering to the degree their
selected encoders support it; adapter tests pin output.

## 8. Persistence contract

`Save` writes only the selected path and never creates parent directories. It makes a
fresh document from an immutable snapshot. A leaf is written exactly when all hold:

1. it has `save:"true"`;
2. its winning source is `default`, `file`, or `set`; and
3. if sensitive, its winning source is `file` or `set`.

Env/flag values never become durable merely because they win; callers must `Set` to
promote them. Sensitive defaults are never emitted. A field without `save:"true"` is
never emitted, including after `Set`.

Write procedure:

1. Acquire a stable snapshot under the read lock, then release it.
2. `Lstat` an existing target; reject symlink, directory, device, or non-regular file.
3. Create a temp file in the target directory with `0600` if sensitive output exists,
   otherwise `0644`; explicitly chmod it so umask cannot weaken the mode.
4. Encode, write all bytes, `Sync`, close, and rename over the target.
5. Where supported, sync the parent directory.

Failure removes only a created temporary file and cannot change in-memory state.
Unsupported directory sync is a documented platform durability limit. Concurrent
external writers are out of scope: rename prevents torn files, not lost updates.

## 9. Lifecycle and concurrency

The object has `new`, `loading`, and `loaded` states. One mutex serializes `Load`,
`Set`, `Validate`, and `Save` snapshot acquisition; readers use a read lock. A second
`Load` waits for the first rather than observing partial state.

Later `Load` calls re-read default/file/env/flag layers, retain `set`, validate a full
replacement, and publish atomically. Failure preserves previous values and path
metadata. `Save` writes the snapshot present when it began, even if `Set` succeeds
before disk I/O ends.

No caller code runs under a config lock: validators run on private candidates, and
logging drains outside locks. This prevents lock inversion/deadlock.

## 10. Errors and diagnostics

All structured errors are immutable pointer types with private fields, constructors,
typed accessors, compile-time assertions, and redacted `Error` text.

| Error | `errx.Kind` | Cause exposure |
|---|---|---|
| `OptionError`, `SchemaError`, `DefaultValueError`, `DecodeError`, `UnknownKeyError` | `Invalid` | none |
| `RequiredFieldError`, `ValidationError` | `Validation` | validator cause retained but not unwrapped |
| `FlagBindingError`, `NotLoadedError`, `NoConfigFileError` | `Conflict` | none |
| `FileReadError`, `SaveError` | `Internal` | unwrap OS/filesystem cause |
| `FileFormatError`, `FileParseError` | `Invalid` | parser/library cause not unwrapped |

`FileReadError` is not used for an absent optional file. It deliberately unwraps an
OS cause, allowing `errors.Is(err, fs.ErrPermission)`. Parser library types are not
exposed. Secrets and raw sensitive values are available through no error accessor.

Initialization diagnostics are a bounded 256-record ring: options, schema, file
selection, source discovery, and load results. It never records raw sensitive values.
Overflow produces one warning with a discarded count. Runtime methods do not buffer.

`FlushLog(nil)` is a no-op. With a logger it atomically drains, clears, then emits
records in order. `slog.Logger` has no flush-success signal, so retention is not
conditional. Caller PCs are captured when records are created.

## 11. Package layout and dependencies

```text
config/
  doc.go           package contract and examples
  config.go        lifecycle, API, snapshots
  options.go       options
  source.go        layers and provenance
  registry.go      reflection and tags
  decode.go        conversion
  validate.go      field and struct validation
  file.go           selection and secure persistence
  format*.go       TOML/YAML/JSON adapters
  errors.go         config-owned typed errors
  log.go            initialization diagnostics
  flag.go           FlagSource contract

configpflag/
  source.go         optional pflag adapter
```

Core imports `errx`, `log/slog`, standard packages, and selected TOML/YAML encoders.
It never imports a CLI framework. `configpflag` imports pflag and `config`, never
the reverse.

## 12. Implementation phases

### C0 — Freeze contracts

- Add package documentation with lifecycle, tags, persistence matrix, and unwrap rules.
- Select/pin TOML and YAML implementations; confirm supported Go version.
- Compile API examples, especially `FlagSource` and `Set`.

Exit: sections 4–10 have review approval.

### C1 — Errors, sources, diagnostics

- Implement source/origin, config errors, `errx` assertions, and bounded logging.
- Test redaction, overflow, flush, and unwrap contracts.

Exit: no error type uses transport concepts or message parsing.

### C2 — Registry and conversion

- Implement deterministic schema walking, tags, naming, collisions, and defensive cloning.
- Implement the full conversion grammar before external source loaders.

Exit: table tests cover every tag and source representation.

### C3 — Layers, validation, lifecycle

- Implement immutable candidates, precedence/provenance, defaults, initial sets, and validators.
- Add race tests for reads, `Load`, `Set`, and `Validate`.

Exit: failed operations cannot change observable state.

### C4 — Files and formats

- Implement selection, strict decoding, format adapters, deterministic encoding.
- Test absent/malformed/permission/symlink/duplicate/unknown/collision behavior.

Exit: all formats round-trip supported field types into a fresh document.

### C5 — Persistence

- Implement output selection and secure atomic replacement.
- Test modes, sensitive/default/env/flag/set combinations, cleanup, and concurrent save snapshots.

Exit: the whole persistence matrix is tested on supported platforms.

### C6 — pflag adapter and adoption

- Add the adapter only after `FlagSource` stabilizes.
- Write README, migration notes, examples, and integration tests.

Exit: core has no pflag, Viper, mapstructure, or application imports.

## 13. Testing and release criteria

Tests must cover schema/tag validity, all collision classes, precedence/per-layer
presence, zero/empty supplied values, reload with retained set values, every conversion
boundary, redaction, validator ordering, file expansion/error distinctions, strict
format behavior, persistence modes and cleanup, error inspection, defensive copying,
state transitions, and `go test -race` concurrency.

Fuzz format normalization and conversion to prove no panic, integer precision loss, or
secret leakage in errors. Use filesystem integration tests plus fault-injection seams
for write/rename/sync failures.

Version 1 is ready only when every public behavior has a contract test, the race
detector is clean, all formats pass the same behavioral suite, and docs explain
precedence, tags, errors, sensitive-data rules, concurrency, and durability limits.
