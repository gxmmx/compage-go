package config

import (
	// Compage
	utils "github.com/gxmmx/compage-go/utils"

	// Third party
	"github.com/spf13/pflag"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type OptFunc func(*Opts)

type Opts struct {
	cnfName   string
	cnfDir    string
	cnfType   string
	envPrefix string
	cnfPath   string
	logLevel  string
	flags     map[string]*pflag.Flag
}

// -----------------------------------------------------------------------------
// Internal Functions
// -----------------------------------------------------------------------------

func defaultOpts() *Opts {
	appName := utils.AppNameFromBin()
	return &Opts{
		cnfName:   appName,
		cnfDir:    ".",
		cnfType:   "yaml",
		envPrefix: utils.EnvifyString(appName),
		cnfPath:   "",
		logLevel:  "",
		flags:     make(map[string]*pflag.Flag),
	}
}

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

// Sets the config file name to be used.
func WithName(name string) OptFunc {
	return func(opts *Opts) {
		opts.cnfName = name
	}
}

// Sets the config file directory to be used.
func WithDir(dir string) OptFunc {
	return func(opts *Opts) {
		opts.cnfDir = dir
	}
}

// Sets the config file type to be used.
// yaml, json, toml, etc.
func WithType(t string) OptFunc {
	return func(opts *Opts) {
		opts.cnfType = t
	}
}

// Sets the environment variable prefix to be used.
func WithEnvPrefix(prefix string) OptFunc {
	return func(opts *Opts) {
		opts.envPrefix = utils.EnvifyString(prefix)
	}
}

// Sets the config file path to be used.
// Overrides the name, directory, and type passed.
func WithPath(path string) OptFunc {
	return func(opts *Opts) {
		opts.cnfPath = path
	}
}

// Sets the log level to be used.
// Overrides the log level found in config.
func WithLogLevel(level string) OptFunc {
	return func(opts *Opts) {
		opts.logLevel = level
	}
}

// Sets command line flags to be used for config.
// This should set config default values.
func WithFlags(flags map[string]*pflag.Flag) OptFunc {
	return func(opts *Opts) {
		opts.flags = flags
	}
}
