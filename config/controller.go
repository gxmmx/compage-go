package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/gxmmx/compage-go/utils/stringutils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Config interface {
	// WriteConfig() error
	// GetLogLevel() string
	// GetUnmarshalledConfig(sub string, m any) error
	GetRaw() map[string]any
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	Set(key string, value any)
}

type Parser interface {
	FileName(string)
	FileDir(string)
	FileType(string)
	FilePath(string)
	EnvPrefix(string)

	AddFlag(*pflag.Flag)
	Parse() (Config, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type Controller struct {
	cnf *viper.Viper

	cnfName   string
	cnfDir    string
	cnfType   string
	cnfPath   string
	envPrefix string

	flags map[string]*pflag.Flag
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewConfig() *Controller {
	appName := stringutils.AppNameFromBin()
	c := &Controller{
		cnf:       viper.New(),
		cnfName:   appName,
		cnfDir:    ".",
		cnfType:   "yaml",
		cnfPath:   "",
		envPrefix: stringutils.EnvifyString(appName),
		flags:     make(map[string]*pflag.Flag),
	}

	return c
}

// -----------------------------------------------------------------------------
// Parser methods
// -----------------------------------------------------------------------------

func (c *Controller) FileName(name string) {
	c.cnfName = stringutils.SlugifyString(name)
}

func (c *Controller) FileDir(dir string) {
	c.cnfDir = dir
}

func (c *Controller) FileType(t string) {
	c.cnfType = t
}

func (c *Controller) FilePath(path string) {
	c.cnfPath = path
}
func (c *Controller) EnvPrefix(prefix string) {
	c.envPrefix = prefix
}

func (c *Controller) AddFlag(flag *pflag.Flag) {
	c.flags[flag.Name] = flag
}

func (c *Controller) Parse() (Config, error) {
	// Set config file settings
	cnfFileEnvName := fmt.Sprintf("%s_%s", c.envPrefix, "CONFIG")
	if c.cnfPath != "" {
		c.cnf.SetConfigFile(c.cnfPath)
	} else if envPath := os.Getenv(cnfFileEnvName); envPath != "" {
		c.cnf.SetConfigFile(envPath)
	} else {
		c.cnf.SetConfigName(c.cnfName)
		c.cnf.SetConfigType(c.cnfType)
		c.cnf.AddConfigPath(c.cnfDir)
		if c.cnfDir != "/etc" {
			c.cnf.AddConfigPath("/etc")
		}
		if c.cnfDir != "." {
			c.cnf.AddConfigPath(".")
		}
	}

	// Set environment variable settings
	c.cnf.SetEnvPrefix(c.envPrefix)
	c.cnf.AutomaticEnv()
	c.cnf.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// c.cnf.SetDefault("log.level", "info")

	// Add flags
	for n, f := range c.flags {
		err := c.cnf.BindPFlag(confNameFromFlagName(n), f)
		if err != nil {
			return nil, fmt.Errorf("failed to bind flag %s: %w", n, err)
		}
	}

	// Read config
	err := c.cnf.ReadInConfig()
	if err != nil && !isMissingConfigFileErr(err) {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// return config
	return c, nil
}

// -----------------------------------------------------------------------------
// Config methods
// -----------------------------------------------------------------------------

func (c *Controller) GetRaw() map[string]any {
	rawConfig := c.cnf.AllSettings()
	if rawConfig == nil {
		return make(map[string]any)
	}
	return rawConfig
}

func (c *Controller) GetString(key string) string {
	return c.cnf.GetString(key)
}

func (c *Controller) GetInt(key string) int {
	return c.cnf.GetInt(key)
}

func (c *Controller) GetBool(key string) bool {
	return c.cnf.GetBool(key)
}

func (c *Controller) Set(key string, value any) {
	c.cnf.Set(key, value)
}
