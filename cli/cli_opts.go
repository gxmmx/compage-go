package cli

import (
	"io"

	cmpcfg "github.com/gxmmx/compage-go/config"
	"github.com/gxmmx/compage-go/logger"
	cmpstx "github.com/gxmmx/compage-go/utils/stringx"
)

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

type Option func(*Controller)

// -----------------------------------------------------------------------------
// Info Options
// -----------------------------------------------------------------------------

// Enables an application logger with the given name.
func SetName(name string) Option {
	return func(ctl *Controller) {
		sanitizedName := cmpstx.SlugifyString(name)
		ctl.name = sanitizedName
		ctl.root.cmd.Use = sanitizedName
	}
}

func SetVersion(version string) Option {
	return func(ctl *Controller) {
		ctl.version = version
		ctl.root.cmd.Version = version
	}
}

// func SetUsage(usage string) Option {
// 	return func(ctl *Controller) {
// 		ctl.root.cmd.Use = usage
// 	}
// }

func SetShort(short string) Option {
	return func(ctl *Controller) {
		ctl.root.cmd.Short = short
	}
}

func SetLong(long string) Option {
	return func(ctl *Controller) {
		ctl.root.cmd.Long = long
	}
}

// -----------------------------------------------------------------------------
// Config Options
// -----------------------------------------------------------------------------

// Sets the name of the config file.
func WithConfigFileName(name string) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithFileName(name))
	}
}

// Sets the directory for the config file.
func WithConfigFileDir(dir string) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithFileDir(dir))
	}
}

// Sets the type of the config file.
func WithConfigFileType(t string) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithFileType(t))
	}
}

// Sets the path for the config file.
// Overrides the file name, directory, and type.
func WithConfigFilePath(path string) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithFilePath(path))
	}
}

// Sets the environment variable prefix for the config.
func WithConfigEnvPrefix(prefix string) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithEnvPrefix(prefix))
	}
}

// Enables intitial creation of a config file.
// If sensitive is true, the config file permissions will be restricted.
func WithConfigCreate(sensitive bool) Option {
	return func(ctl *Controller) {
		ctl.cfg.Option(cmpcfg.WithCreateConfig(sensitive))
	}
}

// Enables default config management commands.
func WithConfigCommands() Option {
	return func(ctl *Controller) {
		ctl.addConfigCmds = true
	}
}

// -----------------------------------------------------------------------------
// Testing overrides
// -----------------------------------------------------------------------------

// Sets output writer for the logger.
func withOutWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.log.Option(logger.WithOutWriter(w))
	}
}

// Sets error writer for the logger.
func withErrWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.log.Option(logger.WithErrWriter(w))
	}
}

func withArgs(args []string) Option {
	return func(ctl *Controller) {
		ctl.cliArgs = args
	}
}
