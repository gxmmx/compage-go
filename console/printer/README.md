# console/printer

Human-facing terminal output for Go applications: level-gated messages with
semantic markers, color, indent nesting, tables, and interactive prompts.

```go
import "github.com/gxmmx/compage-go/console/printer"
```

## Quick start

```go
out := printer.New()
out.Info("starting up")
out.Success("done")
out.Warn("retrying")
out.Error("failed: %v", err)
```

## Printer

`New(...Option)` returns a `Printer`. Output is gated by level, styled with
semantic markers, and color-aware (auto-disabled when not writing to a TTY, or
when `NO_COLOR` is set).

| Method | Marker / behavior |
|--------|-------------------|
| `Info` / `Success` / `Warn` / `Error` / `Verbose` | leveled messages (✓ for success, `!` warn, `✗` error) |
| `Print` / `Printf` / `Println` | raw output, no marker |
| `Table(headers, rows)` | auto-width columnar output |
| `WithIndent(n)` | derived printer indented n levels |
| `WithTextColor(c)` | derived printer with a text color override |
| `SetLevel(lvl)` | runtime level change, shared across derived printers |
| `Slog()` | expose as `*slog.Logger` |

Derived printers (`WithIndent`, `WithTextColor`) share the parent's write mutex,
so concurrent goroutines never interleave partial lines.

### Options

| Option | Description |
|--------|-------------|
| `WithLevel(slog.Level)` | minimum level (default: Info) |
| `WithColor(bool)` | force color on/off (default: on; `NO_COLOR` always wins) |
| `WithOutToErr()` | route normal output to stderr instead of stdout |
| `WithErrToOut()` | route warn/error output to stdout instead of stderr |

### Stream model

A printer is a human interface, so it only ever writes to the two standard
streams — it cannot be pointed at a file or arbitrary writer (that is the
logger's job). By default:

- normal output (`Info`/`Success`/`Verbose`/`Print*`) → **stdout**
- warn/error output (`Warn`/`Error`) → **stderr**

`WithOutToErr()` and `WithErrToOut()` redirect between the two — useful, for
example, to keep stdout clean for a machine-readable payload while status still
reaches the user on stderr.

### Level gating

```
slog.LevelDebug → verbose + everything above
slog.LevelInfo  → info, success, warn, error, raw (default)
slog.LevelWarn  → only warn, error
slog.LevelError → only error
```

## Prompter

`NewPrompter(...Option)` returns a `Prompter` — a `Printer` that can also read
interactive input. It accepts the same options as `New`.

```go
pr := printer.NewPrompter()
name := pr.Prompt("Project name", "my-app")   // returns fallback on empty input
if pr.Continue("Proceed?") {                    // [y/N], default No
    // ...
}
```

Input always comes from **stdin**. Prompting only makes sense when the process is
attached to a terminal, so callers should check for a TTY before using a Prompter:

```go
if !term.IsTerminal(os.Stdin) {
    return fmt.Errorf("no terminal: pass --name to run non-interactively")
}
pr := printer.NewPrompter()
name := pr.Prompt("Project name", "")
```

If input isn't interactive, fail with instructions on which flags to pass rather
than blocking on a pipe.

## stdlib interop & presentation hints

`Slog()` exposes the printer as a `*slog.Logger`, so any code that takes a
`*slog.Logger` can produce formatted terminal output without importing this
package. Such code can pass presentation hints as standard slog attributes using
the `console.` key prefix:

```go
func doSomething(l *slog.Logger) {
    l.Info("loading config", "console.indent", 1)
    l.Info("enrolled", "console.success", true)
}
```

| Key | Type | Printer effect |
|-----|------|----------------|
| `console.indent` | int | add N indent levels |
| `console.success` | bool | render an Info record as Success (✓) |

The canonical key strings live in the root `console` package as `HintIndent` /
`HintSuccess`. Hints compose: `printer.WithIndent(2).Slog()` with
`console.indent=1` in a record produces indent level 3.

## What this package does NOT do

- **No arbitrary writers.** Output goes to stdout/stderr only (redirectable
  between them). Use `console/logger` to write structured records to files.
- **No prompting into pipes.** Input is stdin; check for a TTY before prompting.
- **No format options.** Output is formatted text with fixed markers.
