# configv2 Implementation Plan

This is the implementation-ready plan for the `configv2/` package. It supersedes
`todo.md` once implementation begins. The former v1 implementation is retained in
`config-old/` as a behavior reference only; it is not a supported package and does
not impose a compatibility requirement on v2.

## 1. Product goals

`configv2` provides a typed, generic configuration loader with explicit precedence and
source provenance, TOML/YAML/JSON support, environment and generic flag sources,
runtime `Set`, curated persistence, fail-fast errors, and concurrency-safe access.

The package is optimized for correctness and ergonomics, not for repeatedly loading
configuration in a short-lived hot path. A future daemon may load it once and expose
an in-memory snapshot, but that is outside this package's initial scope.

## 2. Deliberate scope

The implementation will not include:

- Viper, mapstructure, or pflag in the core package;
- system/user configuration distinctions;
- configuration-file merging or multi-file composition;
- live watching, remote providers, or sub-configs;
- preservation of unknown keys, comments, or original formatting on save;
- a separate reflection-free `configlite` API.

Configuration errors participate in the repository-wide semantic error model defined
by the sibling `errors/` package. Go does not provide error inheritance, so config
errors implement the shared error contract and wrap shared semantic categories where
appropriate. This lets applications use generic error handling while retaining
config-specific context.

Configuration is explicitly passed through the application. The package does not
provide or manage a global configuration singleton. Applications should pass the
value returned by `Values()` to components that only need startup configuration,
preferably passing only the relevant sub-struct. Components that need to observe or
change configuration may receive the config object or a narrow application-defined
interface instead. This keeps dependencies explicit and makes testing independent
of process-global state.

## 3. Resolution model

Sources are layered from lowest to highest precedence:

```text
default < file < env < flag < set
```

Each invocation loads at most one file. This iteration accepts one explicit file
target and loads it when it exists. An optional environment-selected file path may
take precedence over that explicit target. File merging, ordered fallback candidates,
and multi-file composition are deferred to a future feature.

Configured paths expand environment variables and a leading `~`/`~/...` before
file evaluation and save-target selection. Expansion is limited to these forms; the
package does not perform general shell parsing.

Every resolved value carries its winning source label. Typical labels are:

```text
default
file:/path/config.toml
env
flag
set
```

The layer store is the source of truth for resolution and provenance; no second parse
or parallel source-tracking structure is used.

## 4. Schema and tags

The registry reflects the user's struct once and produces field metadata. Supported
tags are `cfg`, `env`, `flag`, `default`, `required`, `sensitive`, and `save`.

Rules:

- `cfg` is derived as acronym-aware snake_case when omitted.
- `cfg:"-"` excludes a field.
- `env` is derived from the full config path and may be disabled with `env:"-"`.
- `flag` is opt-in only; flags are never generated for every field automatically.
- Nested `cfg` structs create dotted namespaces.
- Anonymous embedded structs are flattened.
- `required` and `default` together are a registry-build error.
- Pointer optional fields are deferred until the second implementation phase.
- Duplicate config keys and duplicate explicit flags are registry errors.

`save:"true"` defines the normal curated file surface. Fields without `save` remain
loadable but are not included in ordinary generated configuration output. The tag
means that a field is eligible for normal persistence; source rules and sensitive
value safeguards still determine whether it is written.

## 5. File presence, generation, and required-field lifecycle

`required` remains strict for normal application startup. The package must not
automatically relax required validation merely because no file was loaded. A config
can be intentionally file-free and use only defaults, environment, and/or flags.

File presence is observable state:

```go
cfg.FileLoaded() // true when the selected file was found and successfully loaded
cfg.Path()       // selected file, or the configured save target when none was found
```

If the selected file does not exist, `FileLoaded()` is false even though a save target
is available. The application may then decide to generate a file. The configuration
package does not infer that file absence means generation is wanted.

When `WithConfigEnv` names a non-empty environment variable, its expanded path is
the selected file and the save target. Otherwise, `WithFile` supplies the selected
file and save target. If neither is configured, the package operates filelessly and
`Save()` returns `ErrNoConfigFile`. The file format is inferred from the path
extension; supported extensions are `.toml`, `.yaml`, `.yml`, and `.json`.

The package does not distinguish system-owned from user-owned configuration paths,
does not elevate privileges, and does not change ownership. Applications choose an
appropriate path; normal filesystem permissions determine whether saving succeeds.

`New()` is an error-free construction step: it stores options and creates the
configuration object but does not build the registry or load any source. The first
phase of `Load()` validates option-level configuration, including validator names,
nil validator functions, duplicate validator registrations, and file options. These
errors are added to the initialization log buffer and returned from `Load()` before
source loading begins. Schema-dependent errors, such as an unknown
`validate:"name"` tag, are also detected during the registry phase of `Load()`.

The high-level load sequence is:

1. Build and validate the struct registry.
2. Create the defaults layer.
3. Resolve the selected file path and load it if it exists.
4. Record `FileLoaded`, the selected file path, and the file layer provenance.
5. Read environment values into the environment layer.
6. Read changed flags into the flag layer.
7. Add any initial runtime `Set` layer values.
8. Resolve each field using `default < file < env < flag < set`.
9. Decode resolved values into the typed configuration.
10. Run semantic validators for present values and required validation according to
    the selected load mode.
11. Publish the completed state atomically.

Structural errors, file errors, missing required values, present-but-invalid type
conversions, and failed semantic validators always fail the load. There is no
incomplete-load mode in this iteration; applications that need a missing value must
report the required flag/env/input and retry on a later process invocation. A failed
load never partially mutates an already loaded configuration.

Required validation is source-based, not zero-value-based. A supplied `0`, `false`, or
empty string counts as supplied when its source is present. A required field without a
source is missing.

The default `save:"true"` tag is sufficient to define the normal generated file
surface; a separate generation tag is not needed. On an explicit `Save`, fields marked
`save:"true"` are eligible when their source is `default`, `file`, or `set`. Values
only supplied by `env` or `flag` remain transient. A field explicitly changed through
`Set` is eligible even without `save:"true"`, which supports commands such as
`myapp config set key value`.

## 6. Save semantics

`Save` always produces a clean document. It discards unknown keys, comments, original
whitespace/order, and fields no longer registered.

The normal save selection rule is:

- include fields marked `save:"true"` when their effective source is `default`, `file`,
  or `set`;
- exclude values sourced only from `env` or `flag`, since those are transient;
- preserve the existing sensitive-value safeguards.

`Save()` always writes a fresh document containing all currently eligible fields; it
is not a partial patch operation. A transient value is never persisted merely because
it won resolution. If an application wants to persist a value obtained from an env
variable, flag, query, or another external source, it must explicitly call `Set()`
with that value and then call `Save()`. This explicit promotion prevents transient
secrets from silently becoming durable configuration.

Validation is divided into three operations:

- `Load()` performs type coercion, semantic validation for every present value, and
  required-field validation before publishing the completed state.
- `Set(key, value)` coerces and semantically validates only the changed field. The
  layer and resolved value are updated only after validation succeeds; invalid input
  leaves the existing configuration unchanged.
- `Validate()` performs a read-only full validation of the current configuration,
  rerunning semantic validators and required-field checks without mutating state.

Semantic validators are not called for absent optional fields. Required validation is
source-based, so a present zero value is valid when its source is present.

When no file was loaded, generated output contains default-backed curated fields, while excluding
environment- and flag-only values. Values obtained during bootstrap with `Set` are
persisted when the field is marked `save:"true"`, and an explicit CLI `config set`
can persist a single writable field.

Sensitive values are only written when sourced from `file` or explicitly changed
through `Set`, never from env, flag, or an implicit default. Permissions are based on
whether a sensitive value is actually written.

Writes use an atomic temporary-file-and-rename sequence. Permissions are `0600` when
sensitive data is written and `0644` otherwise, subject to platform behavior.

## 7. Planned components

### 7.1 Config log buffer

Rename the v1 diagnostic buffer to a config log buffer. Buffer `slog.Record`s during
load, set, validation, and save; flush to a caller-provided logger. Redact sensitive
values and retain caller-PC capture.

### 7.2 Registry

Reflect the type into deterministic `fieldMeta` records. Validate tags, derived names,
supported types, nesting, and duplicate keys. Registry construction is done once per
configuration instance or cached safely by type.

### 7.3 Format adapters

Implement thin TOML, YAML, and JSON adapters. Decode into a common nested-map shape,
normalize scalar/list representations, flatten for the layer store, and encode a fresh
nested map for Save. Do not implement a parser from scratch.

### 7.4 Layer store

Store ordered labeled layers and resolve keys from high to low precedence. Keep source
presence distinct from zero values. Provide atomic replacement for a completed load.
The store must retain per-layer presence independently of the winning source, so the
API can distinguish a value winning from `flag` while also being present in `file`.

### 7.5 Loaders

Implement defaults, one selected file, environment, generic flags, and runtime `Set` as
separate layer producers. Only changed flags enter the flag layer.

### 7.6 Decoder

Initially support `string`, `bool`, `int`, `int64`, `uint`, `uint64`, `float64`,
`time.Duration`, and `[]string`. Invalid present values are hard errors containing
field, key, source, actual value, and expected type. Missing values use defaults where
defined.

### 7.7 Validation

Applications define semantic validators and register them with `WithValidator(name,
fn)`. A field selects one with `validate:"name"`. Duplicate validator names, invalid
validator registrations, and unknown validator names are configuration errors and the
unknown or duplicate validator is never run. Validator failures include field, key,
and validator name context.

The log buffer is an initialization diagnostic channel. It buffers debug, warning,
and error records produced while constructing the registry and loading configuration,
including errors that are also returned to the caller. Buffered logging does not
replace error returns. `Load()` returns an error for any blocker and must not publish
an invalid state. It may return the constructed `*Config` alongside the error so
callers can flush diagnostics before handling the failure.

Runtime methods do not append to the initialization log buffer. `Set()`, `Save()`,
`Values()`, `Source()`, `HasSource()`, and `Validate()` return or expose their runtime
results directly. `Set()` and `Save()` return errors; failed operations leave
configuration state unchanged. `Set()` is rejected before a successful load.

Applications must handle returned errors and decide whether to retry, report, or exit.
The package does not silently continue after unrecoverable configuration errors. A
caller may construct a logger after a failed load and call `FlushLog`; if the logger
itself depends on valid configuration, diagnostics must instead be flushed to an
earlier/preconfigured logger or other fallback sink. `FlushLog` is idempotent and
clears the buffered records after a successful flush.

### 7.8 Error model

The sibling `errors/` package defines the repository-wide semantic error contract and
stable categories/codes. Config-specific errors wrap the nearest shared domain error,
such as `FileNotFound`, while retaining structured context such as field, key, source,
expected type, actual value, validator, and path. Shared errors own their semantic
classification: `FileNotFound` itself wraps or exposes `NotFound`. Config does not
need to know how that semantic parent is implemented.

Examples include `ConfigFileNotFound` wrapping shared `FileNotFound`, a config decode
error wrapping shared `Invalid`, and a required-field error wrapping shared
`Required`. An application may add an outer application wrapper without losing
`errors.Is`/`errors.As` access to either the config context or shared semantic
category. See
`errors-implementation-plan.md` for the repository-wide contract and chain rules.

Callers use `errors.Is` for stable sentinel conditions and `errors.As` for structured
config errors or the shared semantic error interface. Initialization errors are both
returned and recorded in the config log buffer; runtime errors are returned directly
and are not buffered. Failed operations never partially mutate configuration state.

The shared error model may provide transport mappings such as HTTP status codes, but
the config package remains transport-agnostic. HTTP or other protocol adapters map
semantic categories to transport responses outside the config package.

### 7.9 Persistence

Select the configured file as the save target, select the format from its extension,
construct only eligible output fields, encode a fresh document, and atomically
replace the target.

### 7.10 Public API

Provide the v1-shaped API: `New[T]`, `Load[T]`, `Values`, `Source`, `HasSource`, `Set`, `Save`,
`FileLoaded`, `Path`, `Validate`, and `FlushLog`. File configuration consists of
`WithFile(path)` and `WithConfigEnv(envVar)`; there are no directory-search,
name, or type options in this iteration. `Values()` is a point-in-time read: each
call returns its own copy of the currently resolved values and never exposes the
configuration object's internally stored value for mutation. The package does not
provide automatic change notifications or hot reload in this iteration.

The intended application wiring is dependency-injected rather than global:

```go
cfg, err := configv2.Load[AppConfig](...)
if err != nil {
    return err
}

app := NewApp(cfg.Values())
```

Startup-only components should receive the relevant configuration value or
sub-struct. Components that need runtime reads or mutation may receive `Config[T]`
or, preferably, a narrow application-defined interface such as:

```go
type ConfigReader interface {
    Values() AppConfig
    Source(string) string
}
```

The package must not require callers to retrieve configuration through global
state.

The construction and loading API is:

```go
cfg := configv2.New[AppConfig](opts...)
if err := cfg.Load(); err != nil {
    // Flush initialization diagnostics, then handle the returned blocker.
}
```

The package-level convenience function is equivalent to constructing and loading:

```go
func Load[T any](opts ...Option) (*Config[T], error) {
    cfg := New[T](opts...)
    return cfg, cfg.Load()
}
```

`New()` does not perform validation that could fail. All option, registry, source,
decoding, and initialization validation begins in `Load()`, ensuring failures can be
returned and recorded in one initialization diagnostic sequence.

## 8. Implementation order

### P0 — Package scaffolding

- Create `configv2/` as the replacement package; retain `config-old/` only as a
  behavior reference.
- Define package boundaries and public error types.
- Define source labels, field metadata, layer, and candidate-path types.
- Add baseline tests and decide whether format adapters live in subpackages.

### P1 — Logging

- Implement buffered records, flush, caller-PC capture, and redaction.
- Test disabled logging, flush ordering, and sensitive-value handling.

### P2 — Registry and tags

- Implement field walking and key derivation.
- Implement nesting and anonymous-field flattening.
- Validate tags, duplicate keys, duplicate flags, required/default conflicts, and types.
- Add table-driven tests for acronym conversion and edge cases.

### P3 — Format adapters

- Add TOML, YAML, and JSON decoding.
- Normalize values into the common representation.
- Add fresh-document encoding tests for each format.
- Decide deterministic output ordering and document format limitations.

### P4 — Layer store

- Implement ordered insertion and high-to-low resolution.
- Preserve source presence for zero values.
- Add provenance and precedence tests.

### P5 — Defaults, environment, and flags

- Build the default layer.
- Read env values using `os.LookupEnv`.
- Define the generic flag-source interface.
- Add changed-flag filtering and coercion tests.

### P6 — File selection and loading

- Implement selection between the environment-provided file and the explicit file
  option.
- Load at most one existing file.
- Track `FileLoaded`, selected file path, and file provenance.
- Infer the format from the file extension.
- Define behavior for missing paths, unsupported extensions, and parse errors.
- Expand environment variables and a leading home-directory shorthand in paths.

### P7 — Decoder

- Implement supported scalar and slice conversions.
- Add fail-fast error context.
- Test file, env, flag, default, and set representations independently.

### P8 — Orchestration, required validation, and concurrency

- Implement strict load behavior.
- Add read-only `Validate()` for explicit whole-configuration checks and future
  extensibility.
- Add mutex protection and atomic state replacement.
- Ensure failed loads do not partially mutate active configuration.

### P9 — Save

- Implement source-aware field selection.
- Implement default persistence when no file was loaded.
- Implement explicit `Set` persistence regardless of `save` tag.
- Implement clean re-encoding and atomic replacement.
- Test permissions, sensitive fields, missing files, and save failures.

### P10 — pflag adapter

- Add a separate adapter package importing pflag.
- Convert pflag values into the generic flag-source interface.
- Add optional registry-to-flag-spec generation.

### P11 — API polish and adoption

- Finalize names and option constructors.
- Write README and migration notes.
- Adopt the new package in downstream projects as needed; no in-repository v1
  compatibility layer is required.
- Add integration tests covering file-absent generation, normal loads, env/flag overrides, CLI Set,
  Save, and reload.

## 9. Decisions still requiring confirmation

- A non-empty environment-provided file path takes precedence over `WithFile`.
- A parse error or unsupported format on the selected existing file stops loading
  immediately.
- Whether ordered fallback candidates should be added in a later iteration.
- Exact shape of per-layer source inspection such as `HasSource`.
- Exact shape of `Values`, `Source`, and `FileLoaded`.
- Generic flag-source interface shape.

The former v1 implementation in `config-old/` is reference material for behavioral
comparisons only. It is intentionally excluded from builds, tests, vet, and lint.
