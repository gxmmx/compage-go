package logger

import (
	"io"
	"log/slog"
	"sync"

	cmperr "github.com/gxmmx/compage-go/errors"
)

// -----------------------------------------------------------------------------
// Controller
// -----------------------------------------------------------------------------

type Controller struct {
	app         string
	level       *slog.LevelVar
	logger      *slog.Logger
	outWriter   io.Writer
	errWriter   io.Writer
	once        sync.Once
	preWarnings []error
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// Creates a new logger controller with the specified options.
func New(opts ...Option) Logger {
	ctl := &Controller{
		level:       new(slog.LevelVar),
		outWriter:   defaultOutWriter,
		errWriter:   defaultErrWriter,
		app:         "",
		preWarnings: []error{},
	}
	ctl.level.Set(slog.LevelInfo)

	for _, opt := range opts {
		opt(ctl)
	}

	return ctl
}

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

// Apply an option to the controller after it has been created.
// After creation, adding options does not change state.
func (ctl *Controller) Option(opt Option) {
	opt(ctl)
}

// SetLevel sets the logging level for the controller.
// This method can be used to change the logging level dynamically.
func (ctl *Controller) SetLevel(level string) {
	ctl.level.Set(stringToLevel(level))
}

// Get returns the logger instance for the controller.
func (ctl *Controller) Get() *slog.Logger {
	ctl.once.Do(func() {
		if ctl.app == "" {
			ctl.newCliLogger()
		} else {
			ctl.newAppLogger()
		}
		// Log pre-warnings if any
		if len(ctl.preWarnings) > 0 {
			for _, err := range ctl.preWarnings {
				if aerr, ok := err.(cmperr.ApplicationError); ok {
					msg, fields := aerr.Slog()
					ctl.logger.Warn(msg, fields...)
				} else {
					ctl.logger.Warn(err.Error())
				}
			}
		}
	})
	return ctl.logger
}
