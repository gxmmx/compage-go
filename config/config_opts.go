package config

import (
	"slices"

	stringx "github.com/gxmmx/compage-go/utils/stringx"
	"github.com/spf13/pflag"
)

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

type Option func(*Controller)

// -----------------------------------------------------------------------------
// Options
// -----------------------------------------------------------------------------

// Enables an application logger with the given name.
func WithFileName(name string) Option {
	return func(ctl *Controller) {
		ctl.cnfName = name
	}
}

// Sets the directory where configuration file will be stored.
func WithFileDir(dir string) Option {
	return func(ctl *Controller) {
		ctl.cnfDir = dir
	}
}

// Sets the type of configuration file (e.g., yaml, json).
func WithFileType(t string) Option {
	return func(ctl *Controller) {
		ctl.cnfType = t
	}
}

// Sets the direct override path to configuration file.
func WithFilePath(path string) Option {
	return func(ctl *Controller) {
		ctl.cnfPath = path
	}
}

// Sets the environment variable prefix.
func WithEnvPrefix(prefix string) Option {
	return func(ctl *Controller) {
		ctl.envPrefix = prefix
	}
}

// Enables creation of the configuration file if it does not exist.
// Sensitive argument indicates strict file permissions.
func WithCreateConfig(sensitive bool) Option {
	return func(ctl *Controller) {
		ctl.cnfCreate = true
		if sensitive {
			ctl.cmfPerms = 0600
		} else {
			ctl.cmfPerms = 0644
		}
	}
}

// Adds a flag to the controller.
// The flag can be marked as secret, which will prevent it from being logged.
func WithFlag(flag *pflag.Flag, secret bool) Option {
	return func(ctl *Controller) {
		if flag == nil {
			panic("failed to add flag: nil flag provided")
		}
		name := stringx.KeyifyString(flag.Name)
		ctl.flags[name] = flag
		if secret {
			if !slices.Contains(ctl.secret, name) {
				ctl.secret = append(ctl.secret, name)
			}
		}
	}
}
