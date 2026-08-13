package config

import (
	"strings"

	"github.com/spf13/pflag"
)

type options struct {
	paths     []string
	name      string
	fileType  string
	envPrefix string
	configEnv string
	flags     *pflag.FlagSet
}

// Option configures a Config instance during construction.
type Option func(*options)

// WithPath adds a directory to the config file search path. Paths are searched
// in the order added. The first path is also the write target for Save().
func WithPath(dir string) Option {
	return func(o *options) {
		o.paths = append(o.paths, dir)
	}
}

// WithName sets the config file name without extension (e.g., "config").
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithType sets the config file format. Defaults to "toml".
func WithType(t string) Option {
	return func(o *options) {
		o.fileType = t
	}
}

// WithEnvPrefix sets the prefix for environment variable binding.
// For prefix "MYAPP" and env tag "PORT", the bound variable is "MYAPP_PORT".
func WithEnvPrefix(prefix string) Option {
	return func(o *options) {
		o.envPrefix = prefix
	}
}

// WithConfigEnv sets the name of an environment variable whose value is the
// full path to the config file. When set and the variable is non-empty,
// it takes precedence over Paths — the file is loaded directly via
// viper.SetConfigFile(). If the path has no recognized extension, WithType
// is used as a fallback.
func WithConfigEnv(envVar string) Option {
	return func(o *options) {
		o.configEnv = envVar
	}
}

// WithFlags binds a pflag.FlagSet for the flag resolution tier. Only flags
// with Changed == true override env/file values. Pass nil to disable flags.
func WithFlags(fs *pflag.FlagSet) Option {
	return func(o *options) {
		o.flags = fs
	}
}

func (o *options) sanitize() {
	o.fileType = strings.TrimPrefix(o.fileType, ".")
	o.fileType = strings.ToLower(o.fileType)

	if o.fileType == "" {
		o.fileType = "toml"
	}

	if o.name == "" {
		o.name = "config"
	}
	ext := "." + o.fileType
	o.name = strings.TrimSuffix(o.name, ext)
}
