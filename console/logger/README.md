# console/logger

Structured logging for Go applications. Wraps `log/slog` with opinionated
defaults: JSON output, automatic run ID correlation, nested unit identity, and
infallible construction.

```go
import "github.com/gxmmx/compage-go/console/logger"
```

## Why this package exists

Applications need logging before their configuration is fully loaded. They need
correlation IDs to trace requests across components. They need nested identity to
understand which subsystem produced a message. And they need all of this without
the logger itself ever failing.

This package provides two interfaces:

- **PreLogger** — queues structured records before the logger exists
- **Logger** — emits structured JSON records to configured targets

## Quick start

```go
log := logger.New("myapp", logger.WithStderr())
log.Info("started", "port", 8080)
```

## Bootstrap pattern

Most applications need configuration before the logger can be fully initialized.
The PreLogger bridges this gap:

```go
func main() {
    pre := logger.Pre()

    cfg, err := loadConfig()
    if err != nil {
        pre.Warn("config load failed, using defaults", "err", err)
        cfg = defaults()
    }

    log := logger.New("myapp",
        logger.WithLevel(console.ParseLevel(cfg.LogLevel)),
        logger.WithFile(cfg.LogFile),
    )

    pre.Flush(log.Slog())
    if pre.Count(slog.LevelError) > 0 {
        os.Exit(1)
    }

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
pre := logger.Pre()
pre.Info("loaded config", "path", "/etc/app.toml")
pre.Error("missing required field", "field", "api_key")

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
log := logger.New("myapp",
    logger.WithLevel(slog.LevelDebug),
    logger.WithStderr(),
    logger.WithFile("/var/log/myapp.log"),
    logger.WithRunID("custom-run-id"),
)
```

### Construction options

| Option | Description |
|--------|-------------|
| `WithLevel(slog.Level)` | Minimum log level (default: Info) |
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
root := logger.New("agent", logger.WithStderr())
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
log := logger.New("agent", logger.WithStderr())
// run_id = <16-char random hex>

// Explicit (for parent-child process correlation)
log := logger.New("agent",
    logger.WithStderr(),
    logger.WithRunID(os.Getenv("MY_RUN_ID")),
)
```

### request_id

Set at any depth to trace a request across nested subsystems:

```go
reqLog := log.WithRequest("req-abc-123")
dbLog := reqLog.For("db")
dbLog.Info("query executed")
// Output: {..., "unit": "agent.db", "request_id": "req-abc-123", ...}
```

### trace_id

Correlates an action across service boundaries:

```go
traceLog := log.WithTrace("trace-xyz-789")
```

### Context-aware logging

Store correlation IDs in context and let the handler extract them automatically:

```go
// At the gRPC/HTTP boundary:
ctx = logger.WithRequestID(ctx, "req-abc-123")
ctx = logger.WithTraceID(ctx, "trace-xyz-789")

// Deep in the call stack — no logger threading needed:
log.InfoContext(ctx, "query executed", "rows", 42)
// Output: {..., "request_id": "req-abc-123", "trace_id": "trace-xyz-789", ...}
```

If `WithRequest()` was used to stamp `request_id` on a child logger, context
extraction skips that key (no duplication).

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

`SetLevel(slog.Level)` changes the level without reconstructing the instance.
Children created via `For()`, `WithRequest()`, `WithTrace()`, or `With()` share
the same level — one `SetLevel` call changes the entire tree.

```go
log := logger.New("agent", logger.WithLevel(slog.LevelInfo))
child := log.For("storage")

log.SetLevel(slog.LevelDebug)
// Both log and child now emit debug records
```

## Output format

Always JSON. Always structured.

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

The JSON handler never includes `console.*` hint keys in output — they are
filtered before reaching the encoder (see the printer package for how hints are
produced and consumed).

## Concurrency model

- **PreLogger**: not goroutine-safe. Used only during single-goroutine startup.
- **Logger**: fully goroutine-safe. Share freely across goroutines.

## What this package does NOT do

- **No format options.** Output is always JSON.
- **No buffering.** Records are written immediately. PreLogger is a one-time
  startup queue, not a buffer.
- **No runtime health monitoring.** If a file writer fails at runtime, write
  errors are silently dropped. Robustness is at init time only.
- **No context cancellation.** The logger is always available, including during
  shutdown.
- **No extensible context extraction.** Only `request_id` and `trace_id` are
  extracted from context. Application-specific values use `With()`.
