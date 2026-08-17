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
