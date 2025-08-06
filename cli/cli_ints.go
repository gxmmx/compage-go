package cli

import (
	"context"
	"log/slog"

	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Cli interface {
	// Set options for the config controller.
	Option(opt Option)
	// Add a config entry to the cli.
	AddConf(key string, def any, help string, secret bool)
	// Add a flag to the cli.
	AddFlag(long string, short string, def any, help string, secret bool)
	// Add a command to the cli.
	AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand
	// Execute the cli command.
	Execute()
}

type CliCommand interface {
	// Add a flag to the command.
	AddFlag(long string, short string, def any, help string, secret bool)
	// Add an argument to the command.
	AddArg(name string, required bool, multi bool)
	// Add a sub-command to the command.
	AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand
}

type CliContext interface {
	// Get the command line arguments.
	Args() []string
	// Get the context of the command.
	Context() context.Context
	// Get the configuration.
	Config() *viper.Viper
	// Get the logger.
	Logger() *slog.Logger
}
