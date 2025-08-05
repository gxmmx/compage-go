package logger

import (
	"io"
)

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

type Option func(*Controller)

// -----------------------------------------------------------------------------
// Options
// -----------------------------------------------------------------------------

// Enables an application logger with the given name.
func WithApp(name string) Option {
	return func(ctl *Controller) {
		ctl.app = name
	}
}

// Sets the logging level for the controller.
func WithLevel(level string) Option {
	return func(ctl *Controller) {
		ctl.level.Set(stringToLevel(level))
	}
}

// Sets the output writer for standard output logs. (default is os.Stdout)
func WithOutWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.outWriter = w
	}
}

// Sets the output writer for error logs. (default is os.Stderr)
func WithErrWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.errWriter = w
	}
}

// Adds a pre-warning to the controller. These warnings are logged on first get of logger.
func WithPreWarning(err error) Option {
	return func(ctl *Controller) {
		ctl.preWarnings = append(ctl.preWarnings, err)
	}
}
