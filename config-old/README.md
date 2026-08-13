# config

Multi-source configuration for Go applications. A single struct definition with
tags declares all configuration fields. The package resolves values from four
tiers (flag > env > file > default), provides typed access via generics,
supports runtime mutation with `Set()`, and persists configuration with `Save()`.

## Why this package exists

Applications wire viper + pflag + env + defaults + validation separately,
duplicate default values, and handle write-back ad-hoc. This package
consolidates all of that into a single struct definition.

## Quick start

```go
import "github.com/gxmmx/compage-go/config"

type AppConfig struct {
    LogLevel string `cfg:"log_level" save:"true" flag:"log-level" env:"LOG_LEVEL" default:"info"`
    Port     int    `cfg:"port"      save:"true" flag:"port"      env:"PORT"      default:"8080"`
}

cfg, err := config.Load[AppConfig](
    config.WithPath("/etc/myapp"),
    config.WithEnvPrefix("MYAPP"),
)
if err != nil {
    log.Fatal(err)
}

fmt.Println(cfg.Values().Port) // resolved from flag > env > file > default
```

## Struct tags

| Tag | Purpose | When absent |
|-----|---------|-------------|
| `cfg:"key"` | Config key and viper key path | Field excluded from config entirely |
| `flag:"name"` | pflag name to bind | No flag tier for this field |
| `env:"SUFFIX"` | Env var suffix (auto-prefixed) | No env tier for this field |
| `default:"val"` | Fallback value | Zero value if unresolved |
| `required:"true"` | Must be non-zero post-resolution | Optional |
| `sensitive:"true"` | Excluded from Save() unless explicitly Set() | Normal |
| `save:"true"` | Field participates in Save() output | Never written to file |

Every field that participates in configuration **must** have a `cfg` tag — it is
the field's identity in the system.

## Resolution order

For each field, highest wins:

1. **Set** — explicit `Set()` call at runtime
2. **Flag** — only if `flag.Changed == true` (user explicitly passed it)
3. **Env** — `PREFIX_SUFFIX` variable set in environment
4. **File** — value from the TOML/YAML config file
5. **Default** — from struct tag
6. **Zero value** — if none of the above

Unchanged flags (cobra/pflag defaults) do NOT override env or file values.

## Nested structs

Parent `cfg` tags prefix children with dot notation:

```go
type Config struct {
    Network NetworkConfig `cfg:"network"`
}

type NetworkConfig struct {
    Addr string `cfg:"addr" save:"true" flag:"addr" env:"ADDR" default:"localhost"`
    Port int    `cfg:"port" save:"true" flag:"port" env:"PORT" default:"9090"`
}
```

| Layer | Representation |
|-------|---------------|
| Viper key | `network.addr` |
| TOML | `[network]` section, `addr` key |
| YAML | `network:` → `addr:` |
| Go access | `cfg.Values().Network.Addr` |
| Query | `cfg.Source("network.addr")` |
| Env var | `PREFIX_NETWORK_ADDR` |
| Flag | `--addr` (flat, as declared on leaf) |

## Flag binding

The `flag` tag is a **flat reference** to an externally-owned flag name. Unlike
`env` (where config constructs the full variable from prefix + path), config does
not create flags — it binds to them by exact name in the supplied `*pflag.FlagSet`.

Key points:

- Flag names are flat strings matching the CLI definition, not nested paths.
- The CLI code (cobra or manual pflag) owns flag creation. Config only binds.
- Flag names must be unique **within a single config struct**. Two fields
  referencing the same flag name is a load-time error (duplicate detection).
- If nested structs both need a "port" flag in the same struct, the CLI must
  define distinct names (e.g., `--network-port`, `--db-port`) and the struct
  references those explicitly.
- Different subcommands can reuse the same flag name freely — each subcommand
  has its own FlagSet and passes only that FlagSet to config. The same `cfg`
  struct field can be populated by `--port` in one subcommand and `--port` in
  another; the implementations handle the value differently.
- A flag that doesn't exist in the supplied FlagSet is silently skipped (logged
  as a diagnostic).

```go
// CLI defines the flags:
cmd.Flags().Int("network-port", 9090, "network listener port")
cmd.Flags().Int("db-port", 5432, "database port")

// Config struct references them by exact name:
type Config struct {
    Network struct {
        Port int `cfg:"port" flag:"network-port" default:"9090"`
    } `cfg:"network"`
    DB struct {
        Port int `cfg:"port" flag:"db-port" default:"5432"`
    } `cfg:"db"`
}
```

## Config file resolution

The file tier is entirely optional. Three modes:

**No file (fileless mode):**
No `WithPath` or `WithConfigEnv` provided. Config loads from flags + env +
defaults only. `Path()` returns `""`. `Save()` returns `ErrNoConfigFile`.

**ConfigEnv (operator override):**
`WithConfigEnv("MY_CONFIG_FILE")` names an env var whose value is the full path
to the config file. When set and non-empty, it takes precedence over all paths.
Useful for container orchestration, systemd, or non-standard deployments.

```go
// Operator sets: MY_CONFIG_FILE=/opt/custom/agent.toml
cfg, _ := config.Load[AppConfig](
    config.WithConfigEnv("MY_CONFIG_FILE"),
    config.WithPath("/etc/myapp"),       // fallback if env var is empty
)
```

**Path search (application default):**
Paths are searched in order. The first directory containing `name.type` wins.

```go
cfg, _ := config.Load[AppConfig](
    config.WithPath("/etc/myapp"),
    config.WithPath("$HOME/.myapp"),
    config.WithPath("."),
)
```

Missing files are graceful — load continues with other tiers. Malformed files
return an error.

## Options

| Option | Description |
|--------|-------------|
| `WithPath(dir)` | Add search path (accumulates, searched in order) |
| `WithName(name)` | Config file name without extension (default: "config") |
| `WithType(t)` | File format: "toml", "yaml" (default: "toml") |
| `WithEnvPrefix(prefix)` | Env var prefix (e.g., "MYAPP" → MYAPP_PORT) |
| `WithConfigEnv(envVar)` | Env var holding the full config file path |
| `WithFlags(fs)` | Bind a pflag.FlagSet for the flag tier |

## Source tracking

After load, query where each value came from:

```go
cfg.Source("log_level") // "flag", "env", "file", "default", "set", or ""
cfg.Source("api_key")   // "env"
```

## Runtime mutation

```go
cfg.Set("api_key", enrolledKey)  // source becomes "set"
cfg.Values().APIKey              // reflects the new value immediately
```

`Set()` returns `ErrUnknownKey` if the key doesn't exist in the registry.

## Save

`Save()` writes a config file containing only fields with `save:"true"`:

```go
err := cfg.Save()
```

**What gets written:**

| Source | sensitive? | Written? |
|--------|-----------|----------|
| `"set"` | any | Yes |
| `"file"` | any | Yes |
| `"default"` | no | Yes |
| `"default"` | yes | No |
| `"env"` | any | No (transient) |
| `"flag"` | any | No (transient) |

Fields without `save:"true"` are never written, regardless of source.

**Write target:**
1. ConfigEnv resolved path (if set and non-empty)
2. `paths[0]/name.type` (first configured path)
3. `ErrNoConfigFile` (if neither available)

**File permissions:** 0644 by default, 0600 if any `save`-tagged field is
`sensitive:"true"`.

## Preseed pattern

For fields that start empty and get seeded at runtime (API keys, discovered
URLs). These do NOT use `required:"true"` — they are expected to be absent on
first run:

```go
type AgentConfig struct {
    Name   string `cfg:"name"    save:"true" default:"agent-1"`
    APIKey string `cfg:"api_key" save:"true" sensitive:"true"`
}

// First run: no config file exists
cfg, _ := config.Load[AgentConfig](config.WithPath(dir))
// cfg.Values().APIKey == "" (no error, not required)

cfg.Set("api_key", enrolledKey)  // source: "set"
cfg.Save()                       // writes: name + api_key

// Subsequent runs: config file exists
cfg, _ = config.Load[AgentConfig](config.WithPath(dir))
// cfg.Values().APIKey == enrolledKey (source: "file")
```

## Diagnostic logging

Config runs before a logger exists. Debug messages are buffered internally and
flushed to any `*slog.Logger` after initialization:

```go
cfg, err := config.Load[AppConfig](opts...)

// Logger is now constructable (we have log level from config)
logger := console.New("agent", console.WithLogLevel(level))
cfg.FlushDiagnostics(logger.Slog())
```

Diagnostics include: registry build (each field discovered), file resolution
(paths searched, outcome), tier binding (env vars, flags, defaults), and
per-field resolution result (source + value, redacted for sensitive fields).

## Error handling

```go
cfg, err := config.Load[AppConfig](opts...)

var ve *config.ValidationError
var pe *config.ParseError

switch {
case errors.As(err, &ve):
    // ve.Missing contains required keys that are zero
    fmt.Println("missing:", ve.Missing)

case errors.As(err, &pe):
    // pe.Field, pe.Key, pe.Err — struct tag malformed or duplicate flag
    fmt.Println("parse error:", pe)

case err != nil:
    // Wrapped viper error — malformed config file
    fmt.Println("config file error:", err)
}
```

`Load()` returns nil when a config file is not found — this is graceful, not an
error.

## Usage patterns

### Daemon with config file

```go
type ServerConfig struct {
    ListenAddr string `cfg:"listen_addr" save:"true" flag:"listen" env:"LISTEN_ADDR" default:":9090"`
    LogLevel   string `cfg:"log_level"   save:"true" flag:"log-level" env:"LOG_LEVEL" default:"info"`
    TLSCert    string `cfg:"tls_cert"    save:"true" flag:"tls-cert"  env:"TLS_CERT"`
    APIKey     string `cfg:"api_key"     save:"true" sensitive:"true"`
}

cfg, err := config.Load[ServerConfig](
    config.WithConfigEnv("SERVER_CONFIG_FILE"),
    config.WithPath("/etc/server"),
    config.WithPath("$HOME/.server"),
    config.WithEnvPrefix("SERVER"),
    config.WithFlags(cmd.Flags()),
)
```

### CLI tool (no config file)

```go
type CLIConfig struct {
    Output  string `cfg:"output"  flag:"output"  env:"OUTPUT"  default:"stdout"`
    Format  string `cfg:"format"  flag:"format"  env:"FORMAT"  default:"text"`
    Verbose bool   `cfg:"verbose" flag:"verbose" default:"false"`
}

cfg, err := config.Load[CLIConfig](
    config.WithEnvPrefix("MYTOOL"),
    config.WithFlags(cmd.Flags()),
)
// No paths, no file — flags + env + defaults only
```

### Mixed fields (some persisted, some transient)

```go
type AgentConfig struct {
    // Persisted to config file
    Name     string `cfg:"name"      save:"true" flag:"name"      env:"NAME"      default:"agent-1"`
    LogLevel string `cfg:"log_level" save:"true" flag:"log-level" env:"LOG_LEVEL" default:"info"`
    APIKey   string `cfg:"api_key"   save:"true" sensitive:"true"`

    // Runtime-only (never written to file)
    Debug    bool   `cfg:"debug"    flag:"debug"    default:"false"`
    PIDFile  string `cfg:"pid_file" flag:"pid-file" env:"PID_FILE"`
}
```

Fields without `save:"true"` can still be read from a config file (if someone
puts them there manually), but Save() will never write them.

## What this package does NOT do

- **No flag registration.** Config binds to an existing `*pflag.FlagSet`. The
  caller (cobra, manual pflag setup) creates the flags.
- **No cobra dependency.** Only `pflag` is used.
- **No hot reload.** Config is loaded once. File watching is a future plan.
- **No custom type decoders.** Standard types (string, int, bool, duration,
  slices) are supported via mapstructure coercion.
- **No consumer migration.** Hooking this into existing applications is a
  separate follow-up.

## Public API

```go
func New[T any](opts ...Option) *Config[T]
func Load[T any](opts ...Option) (*Config[T], error)

func (c *Config[T]) Load() error
func (c *Config[T]) Values() *T
func (c *Config[T]) Set(key string, val any) error
func (c *Config[T]) Source(key string) string
func (c *Config[T]) Save() error
func (c *Config[T]) Path() string
func (c *Config[T]) FlushDiagnostics(target *slog.Logger)
```
