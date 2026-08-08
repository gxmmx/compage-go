// Package console provides structured logging and pre-init record queuing
// for Go applications. It wraps log/slog with opinionated defaults: JSON output,
// automatic run ID correlation, nested unit identity, and infallible construction.
//
// See README.md for usage patterns and the bootstrap sequence.
package console

import (
	"log/slog"
	"strings"
)

// ParseLevel maps a string to a slog.Level. Recognized values (case-insensitive):
// "debug", "info", "warn", "warning", "error". Unknown values default to LevelInfo.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug", "verbose":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
