package cli

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type CliCommandFunc func(cli CliContext) error

type CliCommand interface {
	// Usage(string)
	// Short(string)
	// Long(string)
	// Example(string)

	// AddConf()
	AddFlag(string, string, any, string)
	AddCommand(string, string, string, CliCommandFunc) CliCommand
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type CliCommandController struct {
	cli *CliController
	cmd *cobra.Command
	grp []string
}

// -----------------------------------------------------------------------------
// Cli Command methods
// -----------------------------------------------------------------------------

func (cc *CliCommandController) AddFlag(long string, short string, def any, help string) {
	addFlag(cc, long, short, def, help, false)
}

func (cc *CliCommandController) AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand {
	return addCommand(cc, name, "", "", f)
}

// -----------------------------------------------------------------------------
// Cli command internal methods
// -----------------------------------------------------------------------------

// Handles the group:name syntax for commands
func (c *CliCommandController) handleGroup(name string) (string, string) {
	// Handle group:name syntax
	var g, n string
	if idx := strings.Index(name, ":"); idx != -1 {
		g = name[:idx]
		n = name[idx+1:]
	} else {
		g = "commands"
		n = name
	}
	// Add group if it doesn't exist
	if !slices.Contains(c.grp, g) {
		var title string
		if g == "commands" {
			title = "Commands:"
		} else {
			title = strings.ToUpper(g[:1]) + g[1:] + " Commands:"
		}
		c.cmd.AddGroup(
			&cobra.Group{
				ID:    g,
				Title: title,
			},
		)
		c.grp = append(c.grp, g)
	}
	return n, g
}

// -----------------------------------------------------------------------------
// Cli command helper methods
// -----------------------------------------------------------------------------

func addCommand(c *CliCommandController, name string, short string, long string, f CliCommandFunc) CliCommand {
	n, g := c.handleGroup(name)

	command := &CliCommandController{
		cli: c.cli,
		cmd: &cobra.Command{
			Use:   n,
			Short: short,
			Long:  long,
			RunE: func(cmd *cobra.Command, args []string) error {
				return f(c.cli)
			},
			GroupID: g,
		},
	}
	c.cmd.AddCommand(command.cmd)
	return command
}

// Adds a persistent flag and configuration option to a command controller
func addFlag(c *CliCommandController, long string, short string, def any, help string, hide bool) {
	// Skip if flag name is empty
	if long == "" {
		return
	}
	// Skip if short flag is longer than 1 character
	if len(short) > 1 {
		return
	}

	// Set as valid flag, and if no case matches, set invalid
	flagIsValid := true

	switch v := def.(type) {
	case string:
		if short == "" {
			c.cmd.Flags().String(long, v, help)
		} else {
			c.cmd.Flags().StringP(long, short, v, help)
		}
		if hide {
			c.cmd.Flags().MarkHidden(long)
		}
	case int:
		if short == "" {
			c.cmd.Flags().Int(long, v, help)
		} else {
			c.cmd.Flags().IntP(long, short, v, help)
		}
	case bool:
		if short == "" {
			c.cmd.Flags().Bool(long, v, help)
		} else {
			c.cmd.Flags().BoolP(long, short, v, help)
		}
	default:
		flagIsValid = false
	}
	// If the flag is valid, add it to the config controller
	if flagIsValid {
		c.cli.cfgctl.AddFlag(c.cmd.Flags().Lookup(long))
	}
}
