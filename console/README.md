# console

Structured logging for Go applications. Wraps `log/slog` with opinionated defaults:
JSON output, automatic run ID correlation, nested unit identity, and infallible
construction.

## Why this package exists

Applications need logging before their configuration is fully loaded. They need
correlation IDs to trace requests across components. They need nested identity to
understand which subsystem produced a message. And they need all of this without
the logger itself ever failing.

`console` solves these problems with two interfaces:

- **PreLogger** — queues structured records before the logger exists
- **Logger** — emits structured JSON records to configured targets

## Quick start

```go
import "github.com/gxmmx/compage-go/console"

log := console.New("myapp", console.WithStderr())
log.Info("started", "port", 8080)
```

## Bootstrap pattern

Most applications need configuration before the logger can be fully initialized.
The PreLogger bridges this gap:

```go
func main() {
    pre := console.Pre()

    cfg, err := loadConfig()
    if err != nil {
        pre.Warn("config load failed, using defaults", "err", err)
        cfg = defaults()
    }

    log := console.New("myapp",
        console.WithLogLevel(console.ParseLevel(cfg.LogLevel)),
        console.WithFile(cfg.LogFile),
    )

    pre.Flush(log.Slog())
    if pre.Count(slog.LevelError) > 0 {
        os.Exit(1)
    }

    // Application runs with fully initialized logger
    run(ctx, log, cfg)
}
```

**When to skip PreLogger:** If your application always gets valid configuration
(e.g., `LoadOrDefault()` that cannot fail), initialize the logger directly. Not
every application needs the pre-init queue.

## PreLogger

Captures structured log records with their original timestamps before the Logger
is ready.

```go
pre := console.Pre()
pre.Info("loaded config", "path", "/etc/app.toml")
pre.Error("missing required field", "field", "api_key")

// Check severity before proceeding
if pre.Count(slog.LevelError) > 0 {
    // Handle startup failure
}

// Replay through the real logger (preserves original timestamps)
pre.Flush(log.Slog())
```

**Lifecycle rules:**
- PreLogger is not safe for concurrent use. Startup is single-goroutine.
- After `Flush()`, the PreLogger is spent. Any subsequent call panics.
- `Count(level)` returns the number of records at that level or above.

## Logger

Emits structured JSON records. Safe for concurrent use.

```go
log := console.New("myapp",
    console.WithLogLevel(slog.LevelDebug),
    console.WithStderr(),
    console.WithFile("/var/log/myapp.log"),
    console.WithRunID("custom-run-id"),
)
```

### Construction options

| Option | Description |
|--------|-------------|
| `WithLogLevel(slog.Level)` | Minimum log level (default: Info) |
| `WithStderr()` | Enable stderr output |
| `WithFile(path, ...FileOption)` | Enable file output (graceful fallback on failure) |
| `WithRunID(string)` | Set explicit run ID (default: auto-generated 16-char hex) |

### Writer resolution

The logger never fails to construct. Writer fallback rules:

1. `WithStderr()` + `WithFile(path)`: tries to open file; if it fails, warns to stderr, continues with stderr only.
2. `WithFile(path)` only: tries to open file; if it fails, falls back to stderr with a warning (logger must never be mute).
3. `WithStderr()` only: writes to stderr.
4. No writers specified: defaults to stderr.

**The logger always has at least one working writer.**

## Unit nesting

Units identify which subsystem produced a log message. They nest automatically
via dot notation:

```go
root := console.New("agent", console.WithStderr())
// program=agent, unit=agent

storage := root.For("storage")
// program=agent, unit=agent.storage

sqlite := storage.For("sqlite")
// program=agent, unit=agent.storage.sqlite
```

`program` is immutable and set once at construction. `unit` appends with each
`For()` call. Children inherit all parent fields.

## Correlation IDs

### run_id

Every logger has a `run_id` stamped on all records. It identifies a single
process invocation and is inherited by all child loggers.

```go
// Auto-generated (default)
log := console.New("agent", console.WithStderr())
// run_id = <16-char random hex>

// Explicit (for parent-child process correlation)
log := console.New("agent",
    console.WithStderr(),
    console.WithRunID(os.Getenv("MY_RUN_ID")),
)
```

Use explicit run IDs when a parent process launches a child and needs to
correlate their log output (e.g., a `start` command querying whether its
spawned daemon started successfully).

### request_id

Set at any depth to trace a request across nested subsystems:

```go
reqLog := log.WithRequest("req-abc-123")
// All children inherit request_id

dbLog := reqLog.For("db")
dbLog.Info("query executed")
// Output: {..., "unit": "agent.db", "request_id": "req-abc-123", ...}
```

### trace_id

Correlates an action across service boundaries (e.g., panel → controller → agent):

```go
traceLog := log.WithTrace("trace-xyz-789")
// All children inherit trace_id
```

### Context-aware logging

Instead of creating a child logger, you can store correlation IDs in context and
let the handler extract them automatically:

```go
// At the gRPC/HTTP boundary:
ctx = console.WithRequestID(ctx, "req-abc-123")
ctx = console.WithTraceID(ctx, "trace-xyz-789")

// Deep in the call stack — no logger threading needed:
log.InfoContext(ctx, "query executed", "rows", 42)
// Output: {..., "request_id": "req-abc-123", "trace_id": "trace-xyz-789", ...}
```

Both patterns coexist. If `WithRequest()` was used to stamp `request_id` on a
child logger, context extraction skips that key (no duplication).

**Context helpers:**

| Function | Description |
|----------|-------------|
| `WithRequestID(ctx, id)` | Store request ID in context |
| `RequestIDFrom(ctx)` | Extract request ID (or "") |
| `WithTraceID(ctx, id)` | Store trace ID in context |
| `TraceIDFrom(ctx)` | Extract trace ID (or "") |

### Arbitrary fields

```go
tagged := log.With("version", "1.2.0", "env", "production")
```

## Runtime level changes

Both Logger and Printer support `SetLevel(slog.Level)` for runtime level changes
without reconstructing the instance. Children created via `For()`, `WithRequest()`,
`WithTrace()`, `With()`, `WithIndent()`, or `WithTextColor()` share the same
level — one `SetLevel` call changes the entire tree.

```go
log := console.New("agent", console.WithLogLevel(slog.LevelInfo))
child := log.For("storage")

log.SetLevel(slog.LevelDebug)
// Both log and child now emit debug records

printer := console.NewPrinter(console.WithLevel(slog.LevelInfo))
indented := printer.WithIndent(2)

printer.SetLevel(slog.LevelDebug)
// Both printer and indented now show verbose output
```

This enables config-reload → level-change without restarting the application or
replacing logger instances across goroutines.

## Output format

Always JSON. Always structured. No configuration for format.

```json
{
  "time": "2025-01-15T10:30:00.000Z",
  "level": "INFO",
  "msg": "request handled",
  "program": "agent",
  "unit": "agent.api",
  "run_id": "a1b2c3d4e5f67890",
  "request_id": "req-xyz",
  "duration_ms": 42
}
```

Fields always present: `time`, `level`, `msg`, `program`, `unit`, `run_id`.
Fields present when set: `request_id`, `trace_id`, and any custom attributes.

## stdlib interop

Both Logger and Printer expose `Slog() *slog.Logger`, making `*slog.Logger` the
universal type for "something that can receive log records":

```go
// Logger — structured JSON output
stdlogger := log.Slog()

// Printer — formatted terminal output
stdlogger := printer.Slog()
```

Pass `*slog.Logger` to any package that needs diagnostic output without coupling
it to a specific output format. The caller decides the backend.

### Presentation hints

Code receiving a `*slog.Logger` can pass presentation hints as standard slog attrs
using the `console.` key prefix. No console package import needed:

```go
func doSomething(l *slog.Logger) {
    l.Info("loading config", "console.indent", 1)
    l.Info("enrolled", "console.success", true)
    l.Error("connection failed", "err", err)
}
```

| Key | Type | Printer effect | Logger effect |
|-----|------|---------------|--------------|
| `console.indent` | int | Add N indent levels to base | Silently dropped |
| `console.success` | bool | Info → Success (✓ marker) | Silently dropped |

Hints compose: `printer.WithIndent(2).Slog()` with `console.indent=1` in a record
produces indent level 3. Hints via `slog.With("console.indent", 1)` persist across
all subsequent records from that logger.

Logger's JSON handler never includes `console.*` keys in output — they are
filtered before reaching the JSON encoder.

## Concurrency model

- **PreLogger**: not goroutine-safe. Used only during single-goroutine startup.
- **Logger**: fully goroutine-safe. Share freely across goroutines.

## What this package does NOT do

- **No format options.** Logger output is always JSON. Printer output is always
  formatted text with markers. No custom formats.
- **No buffering.** Records are written immediately. PreLogger is not a buffer —
  it's a one-time startup queue.
- **No runtime health monitoring.** If a file writer fails at runtime (e.g.,
  permissions changed), write errors are silently dropped. Robustness is at init
  time only.
- **No context cancellation.** The logger is always available, including during
  shutdown. It outlives all contexts.
- **No extensible context extraction.** Only `request_id` and `trace_id` are
  extracted from context. Application-specific values use `With()` on a child
  logger.
- **No extensible hint keys.** Only `console.indent` and `console.success` are
  recognized as presentation hints. Other `console.*` keys are dropped silently.

## ParseLevel

Maps strings to `slog.Level` (case-insensitive):

| Input | Level |
|-------|-------|
| `"debug"` | `slog.LevelDebug` |
| `"info"` | `slog.LevelInfo` |
| `"warn"` / `"warning"` | `slog.LevelWarn` |
| `"error"` | `slog.LevelError` |
| anything else | `slog.LevelInfo` |
