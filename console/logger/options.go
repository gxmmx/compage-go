package logger

import (
	"io"
	"log/slog"
)

// Option configures a Logger during construction.
type Option func(*config)

// FileOption configures file writer behavior. Reserved for future use (rotation params).
type FileOption func(*fileConfig)

type config struct {
	level          slog.Level
	stderr         bool
	filePath       string
	fileOpts       []FileOption
	runID          string
	fallbackWriter io.Writer // override stderr fallback (testing only)
}

type fileConfig struct {
	// Reserved for rotation parameters (max size, max backups, etc.)
}

// WithLevel sets the minimum log level for the Logger. Default is LevelInfo.
func WithLevel(lvl slog.Level) Option {
	return func(c *config) {
		c.level = lvl
	}
}

// WithStderr enables stderr as an output target.
func WithStderr() Option {
	return func(c *config) {
		c.stderr = true
	}
}

// WithFile enables a file as an output target. If the file cannot be opened,
// the logger falls back to stderr and logs a warning.
// FileOption params are reserved for future rotation configuration.
func WithFile(path string, opts ...FileOption) Option {
	return func(c *config) {
		c.filePath = path
		c.fileOpts = opts
	}
}

// WithRunID sets an explicit run ID for correlation. If not set, a 16-char
// hex ID is auto-generated at logger creation time.
func WithRunID(id string) Option {
	return func(c *config) {
		c.runID = id
	}
}

// withFallbackWriter overrides the stderr fallback target. Used in tests to
// capture output without touching real stderr.
func withFallbackWriter(w io.Writer) Option {
	return func(c *config) {
		c.fallbackWriter = w
	}
}
