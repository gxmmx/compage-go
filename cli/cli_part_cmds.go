package cli

import (
	"fmt"

	cmpcfg "github.com/gxmmx/compage-go/config"
	cmpstr "github.com/gxmmx/compage-go/utils/stringx"

	"gopkg.in/yaml.v3"
)

// -----------------------------------------------------------------------------
// Config Management Commands
// -----------------------------------------------------------------------------

func (ctl *Controller) addConfigCommands() {
	// Base config managment command
	cfgCmd := ctl.AddCommand("system:config", "Config management", "Configuration management commands", nil)
	// List command
	cfgCmd.AddCommand("list", "List configuration", "List all configuration options", cfgList)
	// Get command
	cfgGetCmd := cfgCmd.AddCommand("get", "Get configuration value", "Get a specific configuration value", cfgGet)
	cfgGetCmd.AddArg("key", true, false)
	// Env command
	cfgEnvCmd := cfgCmd.AddCommand("env", "Get environment variable", "Get a specific configuration values compatible environment variable", cfgEnv)
	cfgEnvCmd.AddArg("key", true, false)
	// Set command
	cfgSetCmd := cfgCmd.AddCommand("set", "Set configuration value", "Set a specific configuration value", cfgSet)
	cfgSetCmd.AddArg("key", true, false)
	cfgSetCmd.AddArg("value", true, false)
}

func cfgList(cli CliContext) error {
	logger := cli.GetLogger()
	cliController, _ := cli.(*Controller)
	configController, _ := cliController.cfg.(*cmpcfg.Controller)
	redacted := configController.GetRedactedMap()

	logger.Info("Configuration options:")
	yamlData, err := yaml.Marshal(redacted)
	if err == nil {
		fmt.Print(string(yamlData))
	}
	return nil
}

func cfgGet(cli CliContext) error {
	logger := cli.GetLogger()
	args := cli.GetArgs()
	cliController, _ := cli.(*Controller)
	configController, _ := cliController.cfg.(*cmpcfg.Controller)
	envPrefix := configController.GetEnvPrefix()
	key := args[0]

	sanitizedKey := cmpstr.KeyifyString(cmpstr.StripPrefix(key, envPrefix))
	response := fmt.Sprintf("%s: not found", sanitizedKey)
	value, ok := configController.GetRedactedValue(sanitizedKey)
	if !ok {
		logger.Info(response)
		return nil
	}
	response = fmt.Sprintf("%s: %s", sanitizedKey, value)
	logger.Info(response)
	return nil
}

func cfgEnv(cli CliContext) error {
	logger := cli.GetLogger()
	args := cli.GetArgs()
	cliController, _ := cli.(*Controller)
	configController, _ := cliController.cfg.(*cmpcfg.Controller)
	envPrefix := configController.GetEnvPrefix()
	key := args[0]

	sanitizedKey := cmpstr.KeyifyString(cmpstr.StripPrefix(key, envPrefix))
	response := fmt.Sprintf("%s: not found", sanitizedKey)
	value, ok := configController.GetRedactedValue(sanitizedKey)
	if !ok {
		logger.Info(response)
		return nil
	}
	response = fmt.Sprintf("export %s_%s=%s", envPrefix, cmpstr.EnvifyString(sanitizedKey), value)
	logger.Info(response)
	return nil
}

func cfgSet(cli CliContext) error {
	logger := cli.GetLogger()
	args := cli.GetArgs()
	key := args[0]
	value := args[1]

	cliController, _ := cli.(*Controller)
	configController, _ := cliController.cfg.(*cmpcfg.Controller)
	envPrefix := configController.GetEnvPrefix()
	sanitizedKey := cmpstr.KeyifyString(cmpstr.StripPrefix(key, envPrefix))
	if err := configController.Save(sanitizedKey, value); err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("%s: saved", sanitizedKey))
	return nil
}
