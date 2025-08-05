package logger

import "log/slog"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Logger interface {
	// Set options for the logger controller.
	Option(opt Option)
	// SetLevel sets the logging level for the controller.
	SetLevel(lvl string)
	// Get returns the logger instance for the controller.
	Get() *slog.Logger
}
