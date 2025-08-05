package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func (ctl *Controller) parse() {
	// Set config file settings
	cnfFileEnvName := fmt.Sprintf("%s_%s", ctl.envPrefix, "CONFIG")
	if ctl.cnfPath != "" {
		ctl.cnfCurrentPath = ctl.cnfPath
		ctl.cnf.SetConfigFile(ctl.cnfPath)
		ctl.filecnf.SetConfigFile(ctl.cnfPath)
	} else if envPath := os.Getenv(cnfFileEnvName); envPath != "" {
		ctl.cnfCurrentPath = envPath
		ctl.cnf.SetConfigFile(envPath)
		ctl.filecnf.SetConfigFile(envPath)
	} else {
		ctl.cnfCurrentPath = fmt.Sprintf("%s/%s.%s", ctl.cnfDir, ctl.cnfName, ctl.cnfType)
		ctl.cnf.SetConfigName(ctl.cnfName)
		ctl.cnf.SetConfigType(ctl.cnfType)
		ctl.cnf.AddConfigPath(ctl.cnfDir)
		ctl.filecnf.SetConfigName(ctl.cnfName)
		ctl.filecnf.SetConfigType(ctl.cnfType)
		ctl.filecnf.AddConfigPath(ctl.cnfDir)
		if ctl.cnfDir != "/etc" {
			ctl.cnf.AddConfigPath("/etc")
			ctl.filecnf.AddConfigPath("/etc")
		}
		if ctl.cnfDir != "." {
			ctl.cnf.AddConfigPath(".")
			ctl.filecnf.AddConfigPath(".")
		}
	}

	// Environment variable settings
	ctl.cnf.SetEnvPrefix(ctl.envPrefix)
	ctl.cnf.AutomaticEnv()
	ctl.cnf.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Add flags
	for n, f := range ctl.flags {
		_ = ctl.cnf.BindPFlag(n, f)
		// No need checking for error,
		// as WithFlag already checks for nil, when assigning name
	}

	// Read config
	err := ctl.cnf.ReadInConfig()
	var notFoundErr viper.ConfigFileNotFoundError
	var pathErr *fs.PathError
	if err != nil && !errors.As(err, &notFoundErr) && !errors.As(err, &pathErr) {
		panic(fmt.Sprintf("failed to read config: %v", err))
	}

	// Safe write config if not exists
	if ctl.cnfCreate {
		cnfExists := true
		if _, err := os.Stat(ctl.cnfCurrentPath); os.IsNotExist(err) {
			cnfExists = false
		}
		if !cnfExists {
			if err := ctl.cnf.SafeWriteConfigAs(ctl.cnfCurrentPath); err != nil {
				panic(fmt.Sprintf("failed to write config file: %v", err))
			}
			if err := osChmodFunc(ctl.cnfCurrentPath, ctl.cmfPerms); err != nil {
				panic(fmt.Sprintf("failed to set permissions on config file: %v", err))
			}
		}
	}

	// Read file config
	_ = ctl.filecnf.ReadInConfig()
	// No need to check for errors here,
	// as we already checked for errors in the main config read.
}
