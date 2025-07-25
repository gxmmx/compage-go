package cli

import (
	cmpcfg "github.com/gxmmx/compage-go/config"
	cmplog "github.com/gxmmx/compage-go/logger"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type CliContext interface {
	GetConfig() cmpcfg.Config
	GetLogger() cmplog.Logger
}

// -----------------------------------------------------------------------------
// Cli Controller context methods
// -----------------------------------------------------------------------------

func (c *CliController) GetConfig() cmpcfg.Config {
	return c.cfg
}

func (c *CliController) GetLogger() cmplog.Logger {
	return c.log
}
