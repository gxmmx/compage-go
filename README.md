# compage-go

Foundational packages for building Go applications. Removes the need to
re-implement common program infrastructure — logging, configuration, CLI
output — from scratch in every project.

Opinionated where it matters (structured JSON logs, tiered config resolution),
generic where it doesn't (no framework lock-in, no runtime magic).

## Packages

| Package | Purpose |
|---------|---------|
| [`console`](./console/) | Structured logging (JSON/slog), terminal output with semantic markers, pre-init record queuing, interactive prompts |
| [`config`](./config/) | Multi-source configuration from a single struct definition (flag > env > file > default), source tracking, runtime mutation, persistence |

## Install

```bash
go get github.com/gxmmx/compage-go
```

## Development checks

With [Task](https://taskfile.dev/) installed, run the complete verification workflow:

```bash
task check
```

`task test` runs uncached tests across the repository and `task test:race` runs the
same suite with Go's race detector. Target one package directory with a wildcard task:
`task test:config` or `task test:race:config`. `task check` additionally verifies
formatting and tidy module metadata, runs vet and race tests, and executes the
config fuzz targets. The fuzz step is config-only because it is currently the
repository's only package with fuzz tests.

## Quick start

### Logging

```go
import "github.com/gxmmx/compage-go/console"

log := console.New("myapp", console.WithStderr())
log.Info("started", "port", 8080)

// Nested units for subsystem identity
db := log.For("storage")
db.Info("connected", "path", "/var/lib/app.db")
// Output: {..., "unit": "myapp.storage", ...}
```

### Configuration

```go
import "github.com/gxmmx/compage-go/config"

type AppConfig struct {
    LogLevel string `cfg:"log_level" save:"true" flag:"log-level" env:"LOG_LEVEL" default:"info"`
    Port     int    `cfg:"port"      save:"true" flag:"port"      env:"PORT"      default:"8080"`
}

cfg, err := config.Load[AppConfig](
    config.WithFile("/etc/myapp/config.toml"),
    config.WithEnvPrefix("MYAPP"),
)

fmt.Println(cfg.Values().Port)       // resolved value
source, _ := cfg.Source("port")
fmt.Println(source)                  // flag, env, file, default, or set
```

### Terminal output

```go
import "github.com/gxmmx/compage-go/console"

p := console.NewPrinter()
p.Success("enrolled agent %s", name)
p.Warn("certificate expires in %d days", days)
p.WithIndent(1).Info("listening on %s", addr)
```

## Design principles

- **Infallible construction.** Loggers and printers never fail to create. File
  errors degrade gracefully to stderr.
- **Single definition.** One struct with tags defines all configuration fields,
  their sources, defaults, and persistence rules.
- **No global state.** No package-level vars, no init functions, no singletons.
- **Stdlib first.** Built on `log/slog`, `context`, and standard interfaces.
  External dependencies are kept minimal.
- **Test-friendly.** All I/O goes through interfaces. Writers, readers, and flag
  sets are injectable.

## Documentation

Each package has its own README with full API documentation, usage patterns, and
design rationale:

- [console/README.md](./console/README.md)
- [config/README.md](./config/README.md)

## License

MIT
