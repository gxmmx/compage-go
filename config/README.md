# config

`config` resolves a typed struct from `default < file < env < flag < set`.
It loads at most one TOML, YAML, or JSON file; unknown file keys and invalid values
are errors. `Values` returns a defensive snapshot, while `Source`, `Origin`, and
`HasSource` expose provenance.

```go
type AppConfig struct {
    Port int `default:"8080" env:"MYAPP_PORT" save:"true"`
    Token string `env:"MYAPP_TOKEN" sensitive:"true" save:"true"`
}

cfg, err := config.Load[AppConfig](config.WithFile("./config.toml"))
if err != nil { return err }

port := cfg.Values().Port
_ = port
```

`Save` writes only fields marked `save:"true"`. Values whose winning source is env or
flag are transient and are not written. To persist an externally supplied value, call
`Set` first. Sensitive defaults are never written; sensitive file or set values use
mode `0600`.

Use `configpflag.Source{Flags: fs}` with `WithFlagSource` when adapting a pflag flag
set. The core package does not import pflag.

## Schema and tags

`T` must be a non-pointer struct. Exported fields must either be supported leaves or
be excluded with `cfg:"-"`; unsupported exported fields are rejected. Nested structs
form dotted namespaces, while anonymous embedded structs are flattened.

Supported leaf types are `string`, `bool`, `int`, `int64`, `uint`, `uint64`,
`float64`, `time.Duration`, and `[]string`. Named aliases are not supported (except
`time.Duration` itself). Values for `[]string` use a JSON string array, such as
`["one","two"]`.

| Tag | Meaning |
| --- | --- |
| `cfg` | File key component. Omitted names derive from the Go field name as lower snake case; `-` excludes the field. |
| `env` | Environment variable. Omitted names derive from `WithEnvPrefix` and the full dotted key; `-` disables env loading. |
| `flag` | Flat flag name supplied by `FlagSource`. Omitted means there is no flag binding. |
| `default` | String-form default value. |
| `required:"true"` | Requires a value from any layer and cannot be combined with `default`. |
| `sensitive:"true"` | Redacts value data from diagnostics and prevents default values from being saved. |
| `save:"true"` | Allows the winning default, file, or set value to be written by `Save`. |
| `validate` | Name of a validator registered with `WithValidator`. |

Duplicate config keys, environment variables, and flags are schema errors. File
documents are strict: keys must match the schema exactly, and duplicate mappings,
non-string mapping keys, unsupported values, and scalar/namespace collisions fail
loading.

## Lifecycle, mutation, and files

`New` stores options only. `Load` applies them, selects a single optional file, and
publishes a snapshot only after conversion, required-field checks, and validation
succeed. A later failed `Load` leaves the previous snapshot untouched. Before a
successful load, `Values` returns the zero struct and `Set`, `Validate`, and `Save`
return `NotLoadedError`.

`WithFile` selects one path. A non-empty value from `WithConfigEnv` overrides it.
Paths expand environment variables and a leading `~` or `~/`; the file is optional
only when it is genuinely absent. JSON, TOML, YAML, and YML extensions are accepted.
`Save` never creates parent directories and refuses an existing symlink or non-regular
target. It writes a fresh, atomically replaced document; comments and formatting are
not preserved. Directory syncing is attempted where the platform supports it.

`WithInitial` supplies values for the initial `set` layer. It is useful for supplying
trusted bootstrap values—such as values constructed by embedding code or a secret
provider—that must override file, environment, and flags and remain across reloads.
`Set` is the runtime equivalent and validates the whole prospective configuration
before publishing it.

Readers may call `Values`, `Source`, `Origin`, `HasSource`, `Path`, and `FileLoaded`
concurrently with loads and mutations. Returned `[]string` fields are copied. `Save`
uses the snapshot that existed when it began; concurrent mutations may publish a newer
in-memory snapshot while that file write finishes.

## Errors and diagnostics

Errors are package-owned immutable types. Use `errors.As` for types such as
`OptionError`, `SchemaError`, `DecodeError`, `ValidationError`, `FileReadError`, and
`SaveError`; use `errx.IsKind` for broad handling. Error text intentionally omits raw
values and sensitive data. `FileReadError` and `SaveError` unwrap their OS cause for
`errors.Is`; parser and validator causes are retained without being unwrapped.

Initialization diagnostics are retained in a bounded buffer. Call
`cfg.FlushLog(logger)` after logging is configured to drain them; passing nil is a
no-op. Runtime operation errors are returned directly and are not buffered.
