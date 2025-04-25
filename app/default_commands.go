package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Config Commands
// -----------------------------------------------------------------------------

func configWriteFunc(u ApplicationUnit) {
	err := u.GetApp().config.WriteConfig()
	if err != nil {
		u.GetLogger().InfoContext(u.GetCtx(), "failed to write config")
		return
	}
	u.GetLogger().InfoContext(u.GetCtx(), "config written successfully")
	u.GetApp().End(0)
}

func genConfigCommand(a *App) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration commands",
		Long:  "Configuration commands",
	}

	configWriteCmd := &cobra.Command{
		Use:   "write",
		Short: "Write config to file",
		Long:  fmt.Sprintf("Persist current config to file (path: %s)", a.settings.ConfigDir),
		Run: func(cmd *cobra.Command, args []string) {
			unit := NewUnit("config", configWriteFunc)
			a.AddUnit(unit)
			a.Run()
		},
	}
	configCmd.AddCommand(configWriteCmd)
	return configCmd
}
