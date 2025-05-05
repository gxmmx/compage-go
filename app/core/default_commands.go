package core

import (
	"fmt"

	apperrors "github.com/gxmmx/compage-go/errors"
	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Config Commands
// -----------------------------------------------------------------------------

func configWriteFunc(u Unit) error {
	err := u.getApp().(*App).config.WriteConfig()
	if err != nil {
		u.GetLogger().InfoContext(u.GetCtx(), "failed to write config")
		return apperrors.Internal(err, "failed to write config")
	}
	u.GetLogger().InfoContext(u.GetCtx(), "config written successfully")
	u.getApp().End(0)
	return nil
}

func configReadFunc(u Unit) error {
	conf := u.getApp().(*App).config.GetRawConfig()
	u.GetLogger().InfoContext(u.GetCtx(), "config read successfully", "config", conf)
	u.getApp().End(0)
	return nil
}

func genConfigCommand(a *App) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration commands",
		Long:  "Configuration commands",
	}

	configWriteCmd := &cobra.Command{
		Use:   "write",
		Short: "Write current config to file",
		Long:  fmt.Sprintf("Persist current config to file (path: %s)", a.settings.ConfigDir),
		Run: func(cmd *cobra.Command, args []string) {
			unit := NewUnit("config:write", UnitKindFunc, configWriteFunc)
			a.AddUnit(unit)
			a.Run()
		},
	}

	configReadCmd := &cobra.Command{
		Use:   "read",
		Short: "Read current config",
		Long:  "Read current config from file, env, and flags",
		Run: func(cmd *cobra.Command, args []string) {
			unit := NewUnit("config:read", UnitKindFunc, configReadFunc)
			a.AddUnit(unit)
			a.Run()
		},
	}

	configCmd.AddCommand(configWriteCmd)
	configCmd.AddCommand(configReadCmd)
	return configCmd
}
