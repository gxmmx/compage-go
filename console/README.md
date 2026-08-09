# console

Console output for Go applications, split into two focused subpackages plus a
slim shared root.

| Package | Import | Audience | Purpose |
|---------|--------|----------|---------|
| [`logger`](./logger) | `console/logger` | machines | structured JSON logging with run-id correlation, nested unit identity, and infallible construction |
| [`printer`](./printer) | `console/printer` | humans | level-gated terminal output with markers, color, indentation, tables, and interactive prompts |
| `console` (root) | `console` | — | shared primitives only: `ParseLevel` and the slog-attribute hint contract |

The two subpackages are independent — neither imports the other — and each owns
its own `New` and its own options. Because they live in separate packages, the
short option names are reused without collision:

```go
import (
    "github.com/gxmmx/compage-go/console"
    "github.com/gxmmx/compage-go/console/logger"
    "github.com/gxmmx/compage-go/console/printer"
)

log := logger.New("myapp", logger.WithLevel(console.ParseLevel(cfg.LogLevel)))
out := printer.New(printer.WithLevel(slog.LevelInfo))
```

## Root package

The `console` package itself is deliberately small:

- **`ParseLevel(string) slog.Level`** — case-insensitive mapping of
  `"debug"`/`"info"`/`"warn"`/`"error"` (and `"verbose"`/`"warning"`) to a level,
  defaulting to `LevelInfo`. One call can feed both a logger and a printer.
- **Hint contract** — `HintPrefix`, `HintIndent`, `HintSuccess` are the
  `console.`-prefixed slog attribute keys that the logger strips from JSON and the
  printer consumes as rendering directives. They live here as the single source of
  truth shared by both subpackages; callers rarely reference them directly.

See the subpackage READMEs for full usage:

- **[console/logger](./logger/README.md)** — Logger, PreLogger, correlation IDs, bootstrap pattern
- **[console/printer](./printer/README.md)** — Printer, Prompter, stream model, presentation hints
