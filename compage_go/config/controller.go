package config

import (
	"fmt"
	"os"
	"strings"

	apperrors "github.com/gxmmx/compage-go/errors"
	apputils "github.com/gxmmx/compage-go/utils"

	"github.com/spf13/viper"
)

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
	warnings []error

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
		warnings: make([]error, 0),
	}
}

// -----------------------------------------------------------------------------
// Controller public functions
// -----------------------------------------------------------------------------

func (c *Controller) Init() {
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
	c.config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	c.config.SetDefault("loglevel", "info")
}

func (c *Controller) SetDefault(key string, value interface{}) {
	c.config.SetDefault(key, value)
}

func (c *Controller) ReadConfig() {
	// Read the config from a file
	err := c.config.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore
			fmt.Println("Config file not found, using defaults")
		} else {
			c.warnings = append(c.warnings, apperrors.Internal(err, "failed to read config file"))
		}
	}

	// Set log level from command line
	if c.settings.LogFromCmdLine != "" {
		// fmt.Println("Log level set from command line inside read:", c.settings.LogFromCmdLine)
		c.config.Set("loglevel", c.settings.LogFromCmdLine)
	}
}

func (c *Controller) WriteConfig() {
	// Write the config to a file
	err := c.config.WriteConfig()
	if err != nil {
		c.warnings = append(c.warnings, apperrors.Internal(err, "failed to write config file"))
	}
}

func (c *Controller) HasWarning() bool {
	if len(c.warnings) > 0 {
		return true
	}
	return false
}

func (c *Controller) GetWarnings() []error {
	return c.warnings
}

func (c *Controller) GetConfig(dest any) error {
	err := c.config.Unmarshal(dest)
	if err != nil {
		c.warnings = append(c.warnings, apperrors.Internal(err, "failed to unmarshal config"))
		return err
	}
	return nil
}
