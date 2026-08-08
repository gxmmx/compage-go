package console

import (
	"io"
	"log/slog"
)

// PrinterOption configures a Printer during construction.
type PrinterOption func(*printerConfig)

// PrompterOption configures a Prompter during construction.
// It is a superset of PrinterOption.
type PrompterOption func(*prompterConfig)

type printerConfig struct {
	outWriter     io.Writer
	errWriter     io.Writer
	level         slog.Level
	suppressColor bool
}

type prompterConfig struct {
	printerConfig
	input io.Reader
}

// WithOutTo routes all output to w. Default is os.Stdout.
func WithOutTo(w io.Writer) PrinterOption {
	return func(c *printerConfig) {
		c.outWriter = w
	}
}

// WithErrTo overrides the destination for warn/fail/error output.
// If not set, error-level output goes to the out writer.
func WithErrTo(w io.Writer) PrinterOption {
	return func(c *printerConfig) {
		c.errWriter = w
	}
}

// WithLevel sets the minimum level for printer output. Uses slog.Level directly
// so one ParseLevel() call can feed both Logger and Printer.
//
//	slog.LevelDebug → shows verbose + everything above
//	slog.LevelInfo  → default (info, success, warn, fail, error, raw)
//	slog.LevelWarn  → only warn, fail, error
//	slog.LevelError → only fail, error
func WithLevel(lvl slog.Level) PrinterOption {
	return func(c *printerConfig) {
		c.level = lvl
	}
}

// WithColor explicitly controls color output. Passing false suppresses color
// (equivalent to app's --no-color flag). Color is on by default.
// Note: NO_COLOR env var always wins regardless of this setting.
func WithColor(enabled bool) PrinterOption {
	return func(c *printerConfig) {
		c.suppressColor = !enabled
	}
}

// WithInput sets the input reader for Prompter. Default is os.Stdin.
func WithInput(r io.Reader) PrompterOption {
	return func(c *prompterConfig) {
		c.input = r
	}
}

// PrinterOptFor wraps a PrinterOption as a PrompterOption.
func PrinterOptFor(opt PrinterOption) PrompterOption {
	return func(c *prompterConfig) {
		opt(&c.printerConfig)
	}
}
