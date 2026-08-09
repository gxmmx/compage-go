package printer

import (
	"io"
	"log/slog"
)

// Option configures a Printer (or Prompter) during construction.
type Option func(*config)

type config struct {
	// outWriter/errWriter, when non-nil, override the resolved stream targets.
	// They are for testing only and cannot be set through the public API — the
	// printer is a human interface that always writes to the standard streams.
	outWriter io.Writer
	errWriter io.Writer
	// input, when non-nil, overrides the Prompter's stdin. Testing only.
	input io.Reader

	outToErr      bool
	errToOut      bool
	level         slog.Level
	suppressColor bool
}

// WithLevel sets the minimum level for printer output. Uses slog.Level directly
// so one console.ParseLevel() call can feed both a logger and a printer.
//
//	slog.LevelDebug → shows verbose + everything above
//	slog.LevelInfo  → default (info, success, warn, fail, error, raw)
//	slog.LevelWarn  → only warn, fail, error
//	slog.LevelError → only fail, error
func WithLevel(lvl slog.Level) Option {
	return func(c *config) {
		c.level = lvl
	}
}

// WithColor explicitly controls color output. Passing false suppresses color
// (equivalent to an app's --no-color flag). Color is on by default.
// Note: NO_COLOR env var always wins regardless of this setting.
func WithColor(enabled bool) Option {
	return func(c *config) {
		c.suppressColor = !enabled
	}
}

// WithOutToErr routes normal (info/success/verbose/raw) output to stderr.
// By default normal output goes to stdout. Useful when stdout must stay clean
// for a machine-readable payload while status still reaches the user.
func WithOutToErr() Option {
	return func(c *config) {
		c.outToErr = true
	}
}

// WithErrToOut routes warn/error output to stdout instead of stderr.
// By default error output goes to stderr.
func WithErrToOut() Option {
	return func(c *config) {
		c.errToOut = true
	}
}

// withWriters overrides both stream targets. Testing only; unexported so the
// public API keeps the printer bound to the standard streams.
func withWriters(out, err io.Writer) Option {
	return func(c *config) {
		c.outWriter = out
		c.errWriter = err
	}
}

// withInput overrides the Prompter's input reader. Testing only.
func withInput(r io.Reader) Option {
	return func(c *config) {
		c.input = r
	}
}
