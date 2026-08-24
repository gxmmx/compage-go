package certs

import "log/slog"

func debugLog(logger *slog.Logger, message string, args ...any) {
	if logger != nil {
		logger.Debug(message, args...)
	}
}
