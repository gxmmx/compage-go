package app

import (
	"log/slog"

	cmpcli "github.com/gxmmx/compage-go/cli"
	cmplog "github.com/gxmmx/compage-go/logger"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type App interface {
	Name() string
	Logger(unit string) *slog.Logger
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	name string
	log  cmplog.Logger
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewApp(cli cmpcli.CliContext) App {
	climgr, _ := cli.(cmpcli.CliAppContext)
	name := climgr.App()
	logparser := climgr.LoggerParser()
	logparser.AppLogger(name)
	log := logparser.AppLogger(climgr.App())

	c := &Controller{}
	c.name = name
	c.log = log
	return c
}

// -----------------------------------------------------------------------------
// App methods
// -----------------------------------------------------------------------------

func (c *Controller) Name() string {
	return c.name
}

func (c *Controller) Logger(unit string) *slog.Logger {
	return nil
}
