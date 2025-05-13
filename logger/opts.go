package logger

import (
	// Compage
	stringutils "github.com/gxmmx/compage-go/utils/stringutils"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type OptFunc func(*Opts)

type Opts struct {
	name    string
	unit    string
	class   string
	level   string
	service LoggerService
}

// -----------------------------------------------------------------------------
// Internal Functions
// -----------------------------------------------------------------------------

func defaultOpts() *Opts {
	appName := stringutils.AppNameFromBin()
	return &Opts{
		name:    appName,
		unit:    appName,
		class:   "app",
		level:   "info",
		service: nil,
	}
}

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

// Sets the app name to be used.
func WithName(name string) OptFunc {
	return func(opts *Opts) {
		opts.name = name
	}
}

// Sets the default unit name to be used.
func WithUnit(unit string) OptFunc {
	return func(opts *Opts) {
		opts.unit = unit
	}
}

// Sets the default class name to be used.
func WithClass(class string) OptFunc {
	return func(opts *Opts) {
		opts.class = class
	}
}

// Sets the default log level to be used.
func WithLevel(level string) OptFunc {
	return func(opts *Opts) {
		opts.level = level
	}
}

// Sets the logger service to be used.
func WithService(service LoggerService) OptFunc {
	return func(opts *Opts) {
		opts.service = service
	}
}
