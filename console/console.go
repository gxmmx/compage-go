// Package console is the shared root for compage-go's console output packages.
// It holds primitives common to its subpackages and nothing component-specific:
//
//   - console/logger — structured JSON logging for machine consumption.
//   - console/printer — human-facing terminal output (and interactive prompts).
//
// This package itself only exposes level parsing and the slog-attribute hint
// contract that both subpackages share. See the subpackage READMEs for usage.
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
