package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/gxmmx/compage-go/errors"
	apputils "github.com/gxmmx/compage-go/utils"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ConfigController interface {
	Init(map[string]*pflag.Flag) []error
	WriteConfig() error
	GetLogLevel() string
	GetConfig(dest any) error
	GetRawConfig() map[string]any
}

// -----------------------------------------------------------------------------
// Controller
// -----------------------------------------------------------------------------

type Settings struct {
	CnfFromCmdLine string
	LogFromCmdLine string
	EnvPrefix      string
	CnfName        string
	CnfDir         string
	CnfType        string
}

type Controller struct {
	settings *Settings

	config *viper.Viper
}

func NewSettings() *Settings {
	appName := apputils.AppNameFromBin()
	return &Settings{
		CnfFromCmdLine: "",
		LogFromCmdLine: "",
		EnvPrefix:      apputils.EnvifyString(appName),
		CnfName:        appName,
		CnfDir:         ".",
		CnfType:        "yaml",
	}
}

func NewController(settings *Settings) *Controller {
	if settings == nil {
		settings = NewSettings()
	}
	return &Controller{
		config:   viper.New(),
		settings: settings,
	}
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (c *Controller) bindFlags(flags map[string]*pflag.Flag) error {
	for n, f := range flags {
		err := c.config.BindPFlag(n, f)
		if err != nil {
			return apperrors.Internal(err, "failed to bind flag to config")
		}
	}
	return nil
}

func (c *Controller) readConfig() error {
	// Read the config from a file
	err := c.config.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore
			// fmt.Println("Config file not found, using defaults")
		} else {
			return apperrors.Internal(err, "failed to read config file")
		}
	}

	// Set log level from command line
	if c.settings.LogFromCmdLine != "" {
		// fmt.Println("Log level set from command line inside read:", c.settings.LogFromCmdLine)
		c.config.Set("log.level", c.settings.LogFromCmdLine)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Public functions
// -----------------------------------------------------------------------------

func (c *Controller) Init(flags map[string]*pflag.Flag) error {
	cnfFileEnvName := fmt.Sprintf("%s_%s", c.settings.EnvPrefix, apputils.EnvifyString(c.settings.CnfName))

	if c.settings.CnfFromCmdLine != "" {
		c.config.SetConfigFile(c.settings.CnfFromCmdLine)
	} else if os.Getenv(cnfFileEnvName) != "" {
		c.config.SetConfigFile(cnfFileEnvName)
	} else {
		c.config.SetConfigName(apputils.SlugifyString(c.settings.CnfName))
		c.config.SetConfigType(c.settings.CnfType)
		c.config.AddConfigPath(c.settings.CnfDir)
		if c.settings.CnfDir != "/etc" {
			c.config.AddConfigPath("/etc")
		}
		if c.settings.CnfDir != "." {
			c.config.AddConfigPath(".")
		}
	}

	c.config.SetEnvPrefix(c.settings.EnvPrefix)
	c.config.AutomaticEnv()
	c.config.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	c.config.SetDefault("log.level", "info")

	err := c.bindFlags(flags)
	if err != nil {
		return err
	}
	err = c.readConfig()
	if err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Usage functions
// -----------------------------------------------------------------------------

func (c *Controller) WriteConfig() error {
	// Write the config to a file
	file := fmt.Sprintf("%s.%s", c.settings.CnfName, c.settings.CnfType)
	path := filepath.Join(c.settings.CnfDir, file)
	err := c.config.WriteConfigAs(path)
	if err != nil {
		// wrap error with message
		return apperrors.Internal(err, "failed to write config file")
	}
	return nil
}

func (c *Controller) GetLogLevel() string {
	logLevel := c.config.GetString("log.level")
	if logLevel == "" {
		return "info"
	}
	return logLevel
}

func (c *Controller) GetConfig(dest any) error {
	err := c.config.Unmarshal(dest)
	if err != nil {
		return apperrors.Internal(err, "failed to unmarshal config")
	}
	return nil
}

func (c *Controller) GetRawConfig() map[string]any {
	// Get the raw config
	rawConfig := c.config.AllSettings()
	return rawConfig
}

// func (c *Controller) GetString(key string) string {
// 	return c.config.GetString(key)
// }

// func (c *Controller) GetInt(key string) int {
// 	return c.config.GetInt(key)
// }
