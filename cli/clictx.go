package cli

import (
	"context"
	"log/slog"

	cmpcfg "github.com/gxmmx/compage-go/config"
	cmplog "github.com/gxmmx/compage-go/logger"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ctxKey string

const appKey ctxKey = "app"

type CliContext interface {
	Args() []string
	Context() context.Context
	Config() *viper.Viper
	Logger() *slog.Logger
	LoggerLevel(lvl string)
}

type CliAppContext interface {
	App() string
	Args() []string
	LoggerLevel(lvl string)
	LoggerParser() cmplog.Parser
}

// -----------------------------------------------------------------------------
// Cli Context methods
// -----------------------------------------------------------------------------

func (c *CliController) Args() []string {
	return c.args
}

func (c *CliController) Context() context.Context {
	if c.calledcmd != nil {
		if ctx := c.calledcmd.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (c *CliController) Config() cmpcfg.Config {
	return c.cfg
}

func (c *CliController) Logger() *slog.Logger {
	return c.log.Get()
}

func (c *CliController) LoggerLevel(lvl string) {
	c.logctl.SetLevel(lvl)
}

// -----------------------------------------------------------------------------
// Cli App Context methods
// -----------------------------------------------------------------------------

func (c *CliController) App() string {
	return c.calledcmd.DisplayName()
}

func (c *CliController) LoggerParser() cmplog.Parser {
	return c.logctl
}
