package cli

import (
	cmperr "github.com/gxmmx/compage-go/errors"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Exit methods
// -----------------------------------------------------------------------------

// Gets the command to be run from the CLI arguments.
func (ctl *Controller) getCmdFromPassedArgs() *cobra.Command {
	current, _, err := ctl.root.cmd.Find(ctl.cliArgs)
	if err != nil {
		return ctl.root.cmd
	}
	return current
}

func (ctl *Controller) isRunnable(cmd *cobra.Command) bool {
	if !cmd.Runnable() {
		cmd.Help()
		return false
	}
	return true
}

func (ctl *Controller) handleError(cmd *cobra.Command, err error) int {
	if err == nil {
		return 0
	}
	if aerr, ok := err.(cmperr.ApplicationError); ok {
		msg, fields := aerr.Slog()
		ctl.log.Get().Error(msg, fields...)
		return aerr.ExitCode()
	} else {
		ctl.log.Get().Error("Error: " + err.Error())
		cmd.Help()
		return 1
	}
}
