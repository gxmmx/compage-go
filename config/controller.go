package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/gxmmx/compage-go/errors"
	stringutils "github.com/gxmmx/compage-go/utils/stringutils"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ConfigController interface {
	WriteConfig() error
	GetLogLevel() string
	GetUnmarshalledConfig(sub string, m any) error
	GetRaw() map[string]any
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	Set(key string, value any)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	opts *Opts

	config *viper.Viper
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func Parse(opts ...OptFunc) (*Controller, error) {
	options := defaultOpts()
	for _, opt := range opts {
		opt(options)
	}

	ctrl := &Controller{
		opts:   options,
		config: viper.New(),
	}

	return ctrl.initialize()
}

// -----------------------------------------------------------------------------
// Internal methods
// -----------------------------------------------------------------------------

// Binds passed command line flags to the config.
// Run by initialize method.
func (c *Controller) bindFlags(flags map[string]*pflag.Flag) error {
	for n, f := range flags {
		err := c.config.BindPFlag(n, f)
		if err != nil {
			return apperrors.Internal(err, "failed to bind flag to config")
		}
	}
	return nil
}

// Reads config from the configured sources.
// Run by initialize method.
func (c *Controller) readConfig() error {
	// Read the config from a file
	err := c.config.ReadInConfig()
	if err != nil && !isMissingConfigFileErr(err) {
		return apperrors.Internal(err, "failed to read config file")
	}

	// Set log level if passed to controller
	if c.opts.logLevel != "" {
		c.config.Set("log.level", c.opts.logLevel)
	}
	return nil
}

// Initialize the config controller.
// Run when a config is parsed.
func (c *Controller) initialize() (*Controller, error) {
	cnfFileEnvName := fmt.Sprintf("%s_%s", c.opts.envPrefix, "CONFIG")

	// If direct path is provided, possibly from command line, use it.
	if c.opts.cnfPath != "" {
		c.config.SetConfigFile(c.opts.cnfPath)
	} else if os.Getenv(cnfFileEnvName) != "" {
		envPath := os.Getenv(cnfFileEnvName)
		c.config.SetConfigFile(envPath)
	} else {
		c.config.SetConfigName(stringutils.SlugifyString(c.opts.cnfName))
		c.config.SetConfigType(c.opts.cnfType)
		c.config.AddConfigPath(c.opts.cnfDir)
		if c.opts.cnfDir != "/etc" {
			c.config.AddConfigPath("/etc")
		}
		if c.opts.cnfDir != "." {
			c.config.AddConfigPath(".")
		}
	}

	c.config.SetEnvPrefix(c.opts.envPrefix)
	c.config.AutomaticEnv()
	c.config.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	c.config.SetDefault("log.level", "info")

	err := c.bindFlags(c.opts.flags)
	if err != nil {
		return c, err
	}
	err = c.readConfig()
	if err != nil {
		return c, err
	}
	return c, nil
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

// Write the config to the configured file
func (c *Controller) WriteConfig() error {
	// Write the config to a file
	file := fmt.Sprintf("%s.%s", c.opts.cnfName, c.opts.cnfType)
	path := filepath.Join(c.opts.cnfDir, file)
	if c.opts.cnfPath != "" {
		path = c.opts.cnfPath
	}
	err := c.config.WriteConfigAs(path)
	if err != nil {
		return apperrors.Internal(err, "failed to write config file")
	}
	return nil
}

// Get the log level from the config
func (c *Controller) GetLogLevel() string {
	return c.config.GetString("log.level")
}

// Unmarshalls the config into the given struct
func (c *Controller) GetConfigStruct(sub string, m any) error {
	var cnf *viper.Viper
	if sub == "" {
		cnf = c.config
	} else {
		cnf = c.config.Sub(sub)
		if cnf == nil {
			return apperrors.Internal(nil, fmt.Sprintf("failed to get config sub '%s'", sub))
		}
	}
	err := cnf.Unmarshal(m)
	if err != nil {
		return apperrors.Internal(err, "failed to unmarshal config")
	}
	return nil
}

// Get all config values as a map
func (c *Controller) GetRaw() map[string]any {
	// Get the raw config
	rawConfig := c.config.AllSettings()
	return rawConfig
}

// Get a string value from the config
func (c *Controller) GetString(key string) string {
	return c.config.GetString(key)
}

// Get an int value from the config
func (c *Controller) GetInt(key string) int {
	return c.config.GetInt(key)
}

// Get a bool value from the config
func (c *Controller) GetBool(key string) bool {
	return c.config.GetBool(key)
}

// Set a value in the config
func (c *Controller) Set(key string, value any) {
	c.config.Set(key, value)
}
