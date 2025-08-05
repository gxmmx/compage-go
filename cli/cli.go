package cli

import (
	"fmt"
	"os"

	cmpcfg "github.com/gxmmx/compage-go/config"
	cmperr "github.com/gxmmx/compage-go/errors"
	cmplog "github.com/gxmmx/compage-go/logger"
	cmpplt "github.com/gxmmx/compage-go/utils/platform"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Cli interface {
	Name(string)
	Version(string)
	Short(string)
	Long(string)

	ConfigFileName(string)
	ConfigFileDir(string)
	ConfigFileType(string)
	ConfigFilePath(string)
	ConfigEnvPrefix(string)
	ConfigCreate(bool)

	AddConfigCommands()

	AddFlag(string, string, any, string, bool)
	AddConf(string, any, string, bool)

	AddCommand(string, string, string, CliCommandFunc) CliCommand

	Execute()
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type CliController struct {
	name    string
	version string

	root *CliCommandController
	args []string
	cfg  cmpcfg.Config
	log  cmplog.Logger

	enablecfgcmds bool

	calledcmd *cobra.Command
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewCli() Cli {
	name := cmpplt.BinaryName()
	c := &CliController{}
	c.name = name
	c.version = "0.1.0"
	c.cfg = cmpcfg.New()
	c.log = cmplog.New()
	c.root = &CliCommandController{}
	c.root.cli = c
	c.root.cmd = &cobra.Command{}
	c.root.cmd.Use = name
	c.root.cmd.Version = c.version
	c.root.cmd.Short = "Compage Application"
	c.root.cmd.Long = "App built using the Compage framework."
	c.args = []string{}

	c.bootstrapOnCreate()

	return c
}

// -----------------------------------------------------------------------------
// Cli methods
// -----------------------------------------------------------------------------

func (c *CliController) Name(name string) {
	c.name = name
	c.root.cmd.Use = name
}

func (c *CliController) Version(version string) {
	c.version = version
	c.root.cmd.Version = version
}

func (c *CliController) Usage(usage string) {
	c.root.cmd.Use = usage
}

func (c *CliController) Short(short string) {
	c.root.cmd.Short = short
}

func (c *CliController) Long(long string) {
	c.root.cmd.Long = long
}

func (c *CliController) ConfigFileName(name string) {
	c.cfg.Option(cmpcfg.WithFileName(name))
}

func (c *CliController) ConfigFileDir(dir string) {
	c.cfg.Option(cmpcfg.WithFileDir(dir))
}

func (c *CliController) ConfigFileType(t string) {
	c.cfg.Option(cmpcfg.WithFileType(t))
}

func (c *CliController) ConfigFilePath(path string) {
	c.cfg.Option(cmpcfg.WithFilePath(path))
}

func (c *CliController) ConfigEnvPrefix(prefix string) {
	c.cfg.Option(cmpcfg.WithEnvPrefix(prefix))
}

func (c *CliController) ConfigCreate(sensitive bool) {
	c.cfg.Option(cmpcfg.WithCreateConfig(sensitive))
}

func (c *CliController) AddConfigCommands() {
	c.enablecfgcmds = true
}

func (c *CliController) AddFlag(long string, short string, def any, help string, secret bool) {
	addFlag(c.root, long, short, def, help, secret, false)
}

func (c *CliController) AddConf(name string, def any, help string, secret bool) {
	addFlag(c.root, name, "", def, help, secret, true)
}

func (c *CliController) AddCommand(name string, short string, long string, f CliCommandFunc) CliCommand {
	return addCommand(c.root, name, short, long, f)
}

func (c *CliController) Execute() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v\n", r)
			os.Exit(1)
		}
	}()

	c.bootstrapOnExecute()

	if err := c.root.cmd.Execute(); err != nil {
		if aerr, ok := err.(cmperr.ApplicationError); ok {
			msg, fields := aerr.Slog()
			c.log.Get().Error(msg, fields...)
		} else {
			c.log.Get().Error(err.Error())
		}
	}
}

// -----------------------------------------------------------------------------
// Cli Controller Bootstrap methods (internal)
// -----------------------------------------------------------------------------

// Bootstraps the root cli command
// Called in the NewCli constructor
func (c *CliController) bootstrapOnCreate() {
	// Config flag
	c.root.cmd.PersistentFlags().String("config", "", "config file path")

	// Logging flags
	c.root.cmd.PersistentFlags().BoolP("quiet", "q", false, "Enable quiet mode (loglevel:error)")
	c.root.cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (loglevel:debug)")
	c.root.cmd.PersistentFlags().String("loglevel", "info", "Set log level (debug|info|warn|error)")
	c.root.cmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "loglevel")
	c.AddConf("log-level", "info", "Log level of the application", false)

	// Persistent initialization
	c.root.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return c.bootstrapOnPre(cmd, args)
	}

	// c.root.cmd.SilenceErrors = true
}

func (c *CliController) bootstrapOnExecute() {
	// Add System group
	_, helpGroup := c.root.handleGroup("system:system")
	c.root.cmd.SetCompletionCommandGroupID(helpGroup)
	c.root.cmd.SetHelpCommandGroupID(helpGroup)
	if c.enablecfgcmds {
		c.addConfigCommands()
	}
}

// Initializes config and logging
// Called in the PersistentPreRunE of the root command
func (c *CliController) bootstrapOnPre(cmd *cobra.Command, args []string) error {
	// Set the called command and arguments
	c.calledcmd = cmd
	c.args = args

	// Set the config file override from command line
	cnfPathFromCmdLine, _ := c.root.cmd.Flags().GetString("config")
	if cnfPathFromCmdLine != "" {
		c.cfg.Option(cmpcfg.WithFilePath(cnfPathFromCmdLine))
	}

	// Parse the config
	config := c.cfg.Get()

	// Set log level from command line flags
	if c.root.cmd.Flags().Changed("loglevel") {
		logLevelFromCmdLine, _ := c.root.cmd.Flags().GetString("loglevel")
		config.Set("log.level", logLevelFromCmdLine)
	} else if c.root.cmd.Flags().Changed("verbose") {
		config.Set("log.level", "debug")
	} else if c.root.cmd.Flags().Changed("quiet") {
		config.Set("log.level", "error")
	}

	c.log.SetLevel(config.GetString("log.level"))

	if parseErrors := c.cfg.GetParseErrors(); len(parseErrors) > 0 {
		for _, err := range parseErrors {
			// Return if corruped config data
			if cmperr.IsOfKind(err, cmperr.KindCorruptedData) {
				return err
			}
			// Add pre-warning to the logger
			c.log.Option(cmplog.WithPreWarning(err))
		}
	}
	return nil
}
