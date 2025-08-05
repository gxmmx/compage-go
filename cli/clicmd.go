package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gxmmx/compage-go/config"
	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type CliCommandFunc func(cli CliContext) error

type CliCommand interface {
	App(name string)
	AddFlag(long string, short string, def any, help string, secret bool)
	AddArg(name string, required bool, multi bool)
	AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type CliCommandController struct {
	name string
	cli  *CliController
	cmd  *cobra.Command
	grp  []string
	args []cliArg
}

type cliArg struct {
	name     string
	required bool
	multi    bool
}

// -----------------------------------------------------------------------------
// Cli Command methods
// -----------------------------------------------------------------------------

func (cc *CliCommandController) App(name string) {
	if name == "" {
		name = "app"
	}
}

func (cc *CliCommandController) AddFlag(long string, short string, def any, help string, secret bool) {
	addFlag(cc, long, short, def, help, secret, false)
}

func (cc *CliCommandController) AddArg(name string, required bool, multi bool) {
	addArg(cc, name, required, multi)
}

func (cc *CliCommandController) AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand {
	return addCommand(cc, name, short, long, f)
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
		name: n,
		cli:  c.cli,
		cmd: &cobra.Command{
			Use:     n,
			Short:   short,
			Long:    long,
			GroupID: g,
			Args:    cobra.NoArgs,
		},
	}
	if f != nil {
		command.cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return f(c.cli)
		}
	}
	c.cmd.AddCommand(command.cmd)
	return command
}

// Adds a persistent flag and configuration option to a command controller
func addFlag(c *CliCommandController, long string, short string, def any, help string, secret bool, hide bool) {
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
		c.cli.cfg.Option(config.WithFlag(c.cmd.Flags().Lookup(long), secret))
	}
}

func addArg(c *CliCommandController, name string, required bool, multi bool) {
	if name == "" {
		return
	}
	for _, arg := range c.args {
		if arg.name == name {
			return // Argument already exists
		}
		if multi && arg.multi {
			return // Only one argument can have multiple values
		}
		if required && !arg.required {
			return // Cannot add a required argument after an optional one
		}
	}

	// append the new argument
	c.args = append(c.args, cliArg{name: name, required: required, multi: multi})
	usage := c.name
	requiredArgs := 0
	optionalArgs := 0
	haveMulti := false
	for _, arg := range c.args {
		if arg.multi {
			usage += fmt.Sprintf(" [<%s>]...", arg.name)
			haveMulti = true
		} else if arg.required {
			usage += fmt.Sprintf(" <%s>", arg.name)
		} else {
			usage += fmt.Sprintf(" [<%s>]", arg.name)
		}
		if arg.required {
			requiredArgs++
		} else {
			optionalArgs++
		}
	}
	c.cmd.Use = usage
	if haveMulti {
		c.cmd.Args = cobra.ArbitraryArgs
	} else if optionalArgs == 0 {
		c.cmd.Args = cobra.ExactArgs(requiredArgs)
	} else {
		c.cmd.Args = cobra.RangeArgs(requiredArgs, requiredArgs+optionalArgs)
	}
}

// func addCtx
