package cli

import (
	"fmt"
	"strconv"

	cmpcfg "github.com/gxmmx/compage-go/config"
	"github.com/gxmmx/compage-go/utils/stringutils"

	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Config commands
// -----------------------------------------------------------------------------

func cfgList(cli CliContext) error {
	logger := cli.Logger()
	config := cli.Config()
	configmanager, _ := config.(cmpcfg.ConfigManager)
	redacted := configmanager.GetRawRedacted()
	logger.Info("Configuration options:")
	yamlData, err := yaml.Marshal(redacted)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	fmt.Print(string(yamlData))
	return nil
}

func cfgGet(cli CliContext) error {
	args := cli.Args()
	config := cli.Config()
	configmanager, _ := config.(cmpcfg.ConfigManager)
	envprefix := configmanager.GetEnvPrefix()
	key := args[0]
	sanitizedKey := stringutils.KeyifyString(stringutils.StripPrefix(key, envprefix))
	value := configmanager.GetRedacted(sanitizedKey)
	fmt.Printf("%s", value)
	return nil
}

func cfgEnv(cli CliContext) error {
	args := cli.Args()
	config := cli.Config()
	configmanager, _ := config.(cmpcfg.ConfigManager)
	envprefix := configmanager.GetEnvPrefix()
	key := args[0]
	sanitizedKey := stringutils.KeyifyString(stringutils.StripPrefix(key, envprefix))
	value := configmanager.GetRedacted(sanitizedKey)
	if value == "<redacted>" {
		fmt.Printf("export %s_%s=<redacted>\n", envprefix, stringutils.EnvifyString(sanitizedKey))
	} else {
		fmt.Printf("export %s_%s=%s\n", envprefix, stringutils.EnvifyString(sanitizedKey), value)
	}
	return nil
}

func cfgSet(cli CliContext) error {
	args := cli.Args()
	config := cli.Config()
	configmanager, _ := config.(cmpcfg.ConfigManager)
	envprefix := configmanager.GetEnvPrefix()
	key := args[0]
	value := args[1]
	sanitizedKey := stringutils.KeyifyString(stringutils.StripPrefix(key, envprefix))
	currentValue := configmanager.GetFromFile(sanitizedKey)
	// Empty value expects string
	if currentValue == "" {
		if err := configmanager.WriteKey(sanitizedKey, value); err != nil {
			return fmt.Errorf("failed to set value for key '%s': %w", sanitizedKey, err)
		}
		fmt.Printf("Configuration key '%s' set\n", sanitizedKey)
		return nil
	}
	// If current value is boolean or integer, return boolean
	if currentValue == "true" || currentValue == "false" {
		if boolval, err := strconv.ParseBool(value); err == nil {
			if err := configmanager.WriteKey(sanitizedKey, boolval); err != nil {
				return fmt.Errorf("failed to set boolean value for key '%s': %w", sanitizedKey, err)
			}
			fmt.Printf("Configuration key '%s' set\n", sanitizedKey)
			return nil
		}
		return fmt.Errorf("failed to set boolean value for key '%s'", sanitizedKey)
	}
	// If current value is integer, return integer
	if _, err := strconv.Atoi(currentValue); err == nil {
		if intval, err := strconv.Atoi(value); err == nil {
			if err := configmanager.WriteKey(sanitizedKey, intval); err != nil {
				return fmt.Errorf("failed to set integer value for key '%s': %w", sanitizedKey, err)
			}
			fmt.Printf("Configuration key '%s' set\n", sanitizedKey)
			return nil
		}
		return fmt.Errorf("failed to set integer value for key '%s'", sanitizedKey)
	}
	// Default treat as string
	if err := configmanager.WriteKey(sanitizedKey, value); err != nil {
		return fmt.Errorf("failed to set value for key '%s': %w", sanitizedKey, err)
	}
	fmt.Printf("Configuration key '%s' set\n", sanitizedKey)
	return nil
}

func cfgWrite(cli CliContext) error {
	config := cli.Config()
	configmanager, _ := config.(cmpcfg.ConfigManager)
	if err := configmanager.Write(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Println("Configuration written successfully.")
	return nil
}

func (c *CliController) addConfigCommands() {
	// Base configuration command
	cfgcmd := c.AddCommand("system:config", "Config management", "Configuration management commands.", nil)
	// List command
	cfgcmd.AddCommand("list", "List configuration", "List all configuration options", cfgList)
	// Get command
	getcmd := cfgcmd.AddCommand("get", "Get configuration value", "Get a specific configuration value", cfgGet)
	getcmd.AddArg("key", true, false)
	// Environment variable command
	envcmd := cfgcmd.AddCommand("env", "Get environment variable", "Get a specific configuration values compatible environment variable", cfgEnv)
	envcmd.AddArg("key", true, false)
	// Set command
	setcmd := cfgcmd.AddCommand("set", "Set configuration value", "Set a specific configuration value in config file", cfgSet)
	setcmd.AddArg("key", true, false)
	setcmd.AddArg("value", true, false)
	// Write command
	cfgcmd.AddCommand("write", "Write all configuration", "Writes the current configuration to config file", cfgWrite)
}
