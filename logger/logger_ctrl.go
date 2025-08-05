package logger

import (
	"io"
	"log/slog"
	"sync"
)

// -----------------------------------------------------------------------------
// Controller
// -----------------------------------------------------------------------------

type Controller struct {
	app       string
	level     *slog.LevelVar
	logger    *slog.Logger
	outWriter io.Writer
	errWriter io.Writer
	once      sync.Once
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

// Creates a new logger controller with the specified options.
func New(opts ...Option) Logger {
	ctl := &Controller{
		level:     new(slog.LevelVar),
		outWriter: defaultOutWriter,
		errWriter: defaultErrWriter,
		app:       "",
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
	})
	return ctl.logger
}
