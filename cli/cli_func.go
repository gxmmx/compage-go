package cli

import (
	"fmt"
	"slices"
	"strings"

	cmpcfg "github.com/gxmmx/compage-go/config"
	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Command controller internal functions
// -----------------------------------------------------------------------------

func ensureCommandGroup(cc *CommandController, name string) (string, string) {
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
	if !slices.Contains(cc.grps, g) {
		var title string
		if g == "commands" {
			title = "Commands:"
		} else {
			title = strings.ToUpper(g[:1]) + g[1:] + " Commands:"
		}
		cc.cmd.AddGroup(
			&cobra.Group{
				ID:    g,
				Title: title,
			},
		)
		cc.grps = append(cc.grps, g)
	}
	return n, g
}

func addFlag(cc *CommandController, long string, short string, def any, help string, secret bool, hide bool) {
	// Ensure long and short names are valid
	if long == "" {
		panic("cli: flag long name cannot be empty")
	}
	if len(short) > 1 {
		panic("cli: flag short name must be a single character")
	}

	// Set as valid flag, and if no case matches, set invalid
	flagIsValid := true

	// Add the flag as the correct type
	switch v := def.(type) {
	case string:
		if short == "" {
			cc.cmd.Flags().String(long, v, help)
		} else {
			cc.cmd.Flags().StringP(long, short, v, help)
		}
	case int:
		if short == "" {
			cc.cmd.Flags().Int(long, v, help)
		} else {
			cc.cmd.Flags().IntP(long, short, v, help)
		}
	case bool:
		if short == "" {
			cc.cmd.Flags().Bool(long, v, help)
		} else {
			cc.cmd.Flags().BoolP(long, short, v, help)
		}
	default:
		flagIsValid = false
	}
	// If the flag is valid, add it to the config controller
	if flagIsValid {
		// If hide (config only, no cli flag), mark the flag as hidden
		if hide {
			cc.cmd.Flags().MarkHidden(long)
		}
		cc.cli.cfg.Option(cmpcfg.WithFlag(cc.cmd.Flags().Lookup(long), secret))
	}
}

func addCommand(cc *CommandController, name string, short string, long string, f CliCommandFunc) CliCommand {
	// Ensure name is valid
	if name == "" {
		panic("cli: command name cannot be empty")
	}

	// Ensure the command group exists
	n, g := ensureCommandGroup(cc, name)

	// Create the command controller
	command := &CommandController{
		name: n,
		cli:  cc.cli,
		cmd: &cobra.Command{
			Use:     n,
			Short:   short,
			Long:    long,
			GroupID: g,
			Args:    cobra.NoArgs,
		},
	}

	// If a function is provided, set it to run on execution
	if f != nil {
		command.cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return f(cc.cli)
		}
	}
	// Add the command to the parent command
	cc.cmd.AddCommand(command.cmd)
	return command
}

func addArg(cc *CommandController, name string, required bool, multi bool) {
	// Ensure argument name is valid
	if name == "" {
		panic("cli: argument name cannot be empty")
	}

	// Ensure argument coexistence rules
	for _, arg := range cc.args {
		if arg.name == name {
			panic(fmt.Sprintf("cli: argument '%s' already exists", name))
		}
		if multi && arg.multi {
			panic(fmt.Sprintf("cli: argument '%s' cannot be multi-valued after '%s'", name, arg.name))
		}
		if required && !arg.required {
			panic(fmt.Sprintf("cli: argument '%s' cannot be required after optional '%s'", name, arg.name))
		}
	}

	// append the new argument
	cc.args = append(cc.args, cliArg{name: name, required: required, multi: multi})
	usage := cc.name
	requiredArgs := 0
	optionalArgs := 0
	haveMulti := false
	for _, arg := range cc.args {
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
	// Set usage message to contain arguments
	cc.cmd.Use = usage
	// Set expected arguments for the command
	if haveMulti {
		cc.cmd.Args = cobra.ArbitraryArgs
	} else if optionalArgs == 0 {
		cc.cmd.Args = cobra.ExactArgs(requiredArgs)
	} else {
		cc.cmd.Args = cobra.RangeArgs(requiredArgs, requiredArgs+optionalArgs)
	}
}
