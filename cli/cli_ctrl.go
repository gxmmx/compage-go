package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	cmpcfg "github.com/gxmmx/compage-go/config"
	cmperr "github.com/gxmmx/compage-go/errors"
	cmplog "github.com/gxmmx/compage-go/logger"
	cmpplt "github.com/gxmmx/compage-go/utils/platform"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

type Controller struct {
	// Cli metadata
	name    string
	version string

	// Cli data
	root *CommandController
	cfg  cmpcfg.Config
	log  cmplog.Logger

	// Default commands
	addConfigCmds bool

	// Currently executed command
	cmdCmd *cobra.Command
	// Args passed to the CLI
	cliArgs []string
	// Args passed with the current command
	cmdArgs []string
}

type CommandController struct {
	name string
	cli  *Controller
	cmd  *cobra.Command

	// Command parsed groups and args
	grps []string
	args []cliArg
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(opts ...Option) Cli {
	name := cmpplt.BinaryName()
	ctl := &Controller{}
	ctl.name = name
	ctl.version = "0.1.0"

	ctl.cfg = cmpcfg.New()
	ctl.log = cmplog.New()

	ctl.root = &CommandController{}
	ctl.root.cli = ctl
	ctl.root.cmd = &cobra.Command{}
	ctl.root.cmd.Use = name
	ctl.root.cmd.Version = ctl.version
	ctl.root.cmd.Short = "Compage Application"
	ctl.root.cmd.Long = "App built using the Compage framework."

	ctl.cliArgs = os.Args[1:]
	ctl.cmdArgs = []string{}

	ctl.bootstrapOnCreate()

	for _, opt := range opts {
		opt(ctl)
	}
	return ctl
}

// -----------------------------------------------------------------------------
// Cli Execution
// -----------------------------------------------------------------------------

func (ctl *Controller) Execute() (exitCode int) {
	// Handle panics gracefully
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic: %v\n", r)
			exitCode = panicExitCode
		}
	}()

	// Bootstrap the CLI
	ctl.bootstrapOnExecute()
	// Get the command to execute, if not runnable, show usage
	cmd := ctl.getCmdFromPassedArgs()

	// Return if not runnable
	runnable := ctl.isRunnable(cmd)
	if !runnable {
		exitCode = 0
		return
	}

	// Run the CLI
	err := ctl.root.cmd.Execute()
	// Set exit code based on error
	exitCode = ctl.handleError(cmd, err)
	return
}

// -----------------------------------------------------------------------------
// Cli methods
// -----------------------------------------------------------------------------

// Apply an option to the controller after it has been created.
// After Execute(), adding options does not change state.
func (ctl *Controller) Option(opt Option) {
	opt(ctl)
}

func (ctl *Controller) AddConf(key string, def any, help string, secret bool) {
	addFlag(ctl.root, key, "", def, help, secret, true)
}

func (ctl *Controller) AddFlag(long string, short string, def any, help string, secret bool) {
	addFlag(ctl.root, long, short, def, help, secret, false)
}

func (ctl *Controller) AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand {
	return addCommand(ctl.root, name, short, long, f)
}

func (ctl *Controller) SetRootCommand(f CliCommandFunc) {
	if f != nil {
		ctl.root.cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return f(ctl)
		}
	}
}

// -----------------------------------------------------------------------------
// Cli Command methods
// -----------------------------------------------------------------------------

func (cc *CommandController) AddFlag(long string, short string, def any, help string, secret bool) {
	addFlag(cc, long, short, def, help, secret, false)
}

func (cc *CommandController) AddArg(name string, required bool, multi bool) {
	addArg(cc, name, required, multi)
}

func (cc *CommandController) AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand {
	return addCommand(cc, name, short, long, f)
}

// -----------------------------------------------------------------------------
// Cli Context methods
// -----------------------------------------------------------------------------

func (ctl *Controller) GetArgs() []string {
	return ctl.cmdArgs
}

func (ctl *Controller) GetContext() context.Context {
	return context.Background()
}

func (ctl *Controller) GetLogger() *slog.Logger {
	return ctl.log.Get()
}

func (ctl *Controller) GetConfig() *viper.Viper {
	return ctl.cfg.Get()
}

func (ctl *Controller) SaveConfig(key string, value any) error {
	// Todo
	return nil
}

// -----------------------------------------------------------------------------
// Cli internal methods
// -----------------------------------------------------------------------------

func (ctl *Controller) bootstrapOnCreate() {
	// Config flag
	ctl.root.cmd.PersistentFlags().String("config", "", "config file path")

	// Logging flags
	ctl.root.cmd.PersistentFlags().BoolP("quiet", "q", false, "Enable quiet mode (loglevel:error)")
	ctl.root.cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (loglevel:debug)")
	ctl.root.cmd.PersistentFlags().String("loglevel", "info", "Set log level (debug|info|warn|error)")
	ctl.root.cmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "loglevel")
	ctl.AddConf("log-level", "info", "Log level of the application", false)

	// Persistent initialization
	ctl.root.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return ctl.bootstrapOnPreCommand(cmd, args)
	}
	// Silence usage and errors, will be handled in Execute
	ctl.root.cmd.SilenceErrors = true
	ctl.root.cmd.SilenceUsage = true
}

func (ctl *Controller) bootstrapOnExecute() {
	// Add System group
	_, helpGroup := ensureCommandGroup(ctl.root, "system:system")
	ctl.root.cmd.SetCompletionCommandGroupID(helpGroup)
	ctl.root.cmd.SetHelpCommandGroupID(helpGroup)
	if ctl.addConfigCmds {
		ctl.addConfigCommands()
	}
	// Set cli arguments
	ctl.root.cmd.SetArgs(ctl.cliArgs)
}

func (ctl *Controller) bootstrapOnPreCommand(cmd *cobra.Command, args []string) error {
	// Set the called command and arguments
	ctl.cmdCmd = cmd
	ctl.cmdArgs = args

	// Set the config file override from command line
	cnfPathFromCmdLine, _ := ctl.root.cmd.Flags().GetString("config")
	if cnfPathFromCmdLine != "" {
		ctl.cfg.Option(cmpcfg.WithFilePath(cnfPathFromCmdLine))
	}

	// Parse the config
	config := ctl.cfg.Get()

	// Set log level from command line flags
	if ctl.root.cmd.Flags().Changed("loglevel") {
		logLevelFromCmdLine, _ := ctl.root.cmd.Flags().GetString("loglevel")
		config.Set("log.level", logLevelFromCmdLine)
	} else if ctl.root.cmd.Flags().Changed("verbose") {
		config.Set("log.level", "debug")
	} else if ctl.root.cmd.Flags().Changed("quiet") {
		config.Set("log.level", "error")
	}

	ctl.log.SetLevel(config.GetString("log.level"))

	if parseErrors := ctl.cfg.GetParseErrors(); len(parseErrors) > 0 {
		for _, err := range parseErrors {
			// Return if corruped config data
			if cmperr.IsOfKind(err, cmperr.KindCorruptedData) {
				return err
			}
			// Add pre-warning to the logger
			ctl.log.Option(cmplog.WithPreWarning(err))
		}
	}
	return nil
}
