package logger

import (
	"log/slog"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type LogController interface {
	GetLogger() *slog.Logger
	SetLevel(level string)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	opts *Opts

	level  *slog.LevelVar
	logger *slog.Logger
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(opts ...OptFunc) *Controller {
	options := defaultOpts()
	for _, opt := range opts {
		opt(options)
	}
	ctrl := &Controller{
		opts:  options,
		level: new(slog.LevelVar),
	}
	ctrl.SetLevel(options.level)
	return ctrl
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (m *Controller) GetLogger() *slog.Logger {
	if m.logger == nil {
		// create a slice of slog attributes
		attrs := []slog.Attr{
			slog.String("app", m.opts.name),
		}

		m.logger = slog.New(newLogHandler(m.level, m.opts.class, m.opts.unit, m.opts.service).WithAttrs(attrs))
	}
	return m.logger
}

func (m *Controller) SetLevel(level string) {
	m.level.Set(StringToLevel(level))
}
