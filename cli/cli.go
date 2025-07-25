package cli

import (
	"fmt"

	cmpcfg "github.com/gxmmx/compage-go/config"
	cmplog "github.com/gxmmx/compage-go/logger"
	"github.com/gxmmx/compage-go/utils/stringutils"

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

	// AddConfigCommands()

	AddFlag(string, string, any, string)
	AddConf(string, any, string)

	AddCommand(string, CliCommandFunc) CliCommand

	Execute()
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type CliController struct {
	name    string
	version string

	root   *CliCommandController
	log    cmplog.Logger
	cfg    cmpcfg.Config
	logctl *cmplog.Controller
	cfgctl *cmpcfg.Controller
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewCli() Cli {
	name := stringutils.AppNameFromBin()
	c := &CliController{}
	c.name = name
	c.version = "0.1.0"
	c.cfgctl = cmpcfg.NewConfig()
	c.logctl = cmplog.NewLogger()
	c.root = &CliCommandController{}
	c.root.cli = c
	c.root.cmd = &cobra.Command{}
	c.root.cmd.Use = name
	c.root.cmd.Version = c.version
	c.root.cmd.Short = "Compage Application"
	c.root.cmd.Long = "App built using the Compage framework."

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
	c.cfgctl.FileName(name)
}

func (c *CliController) ConfigFileDir(dir string) {
	c.cfgctl.FileDir(dir)
}

func (c *CliController) ConfigFileType(t string) {
	c.cfgctl.FileType(t)
}

func (c *CliController) ConfigFilePath(path string) {
	c.cfgctl.FilePath(path)
}

func (c *CliController) ConfigEnvPrefix(prefix string) {
	c.cfgctl.EnvPrefix(prefix)
}

func (c *CliController) AddFlag(long string, short string, def any, help string) {
	addFlag(c.root, long, short, def, help, false)
}

func (c *CliController) AddConf(name string, def any, help string) {
	addFlag(c.root, name, "", def, help, true)
}

func (c *CliController) AddCommand(name string, f CliCommandFunc) CliCommand {
	return addCommand(c.root, name, "short", "long", f)
}

func (c *CliController) Execute() {
	c.bootstrapOnExecute()

	if err := c.root.cmd.Execute(); err != nil {
		fmt.Println("Main error wrapper:", err)
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
	c.AddConf("log-level", "info", "Log level of the application")

	// Persistent initialization
	c.root.cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		c.bootstrapOnPre()
	}

	c.root.cmd.SilenceErrors = true
}

// Initializes config and logging
// Called in the PersistentPreRun of the root command
func (c *CliController) bootstrapOnPre() {
	// Set the config file override from command line
	cnfPathFromCmdLine, _ := c.root.cmd.Flags().GetString("config")
	if cnfPathFromCmdLine != "" {
		c.cfgctl.FilePath(cnfPathFromCmdLine)
	}

	if c.root.cmd.Flags().Changed("loglevel") {
		logLevelFromCmdLine, _ := c.root.cmd.Flags().GetString("loglevel")
		c.cfgctl.Set("log.level", logLevelFromCmdLine)
	} else if c.root.cmd.Flags().Changed("verbose") {
		c.cfgctl.Set("log.level", "debug")
	} else if c.root.cmd.Flags().Changed("quiet") {
		c.cfgctl.Set("log.level", "error")
	}

	// Parse the flags and config
	cfg, err := c.cfgctl.Parse()
	if err != nil {
		panic(err)
	}
	c.cfg = cfg

	// Initialize the logger
	log, err := c.logctl.Parse()
	if err != nil {
		panic(err)
	}
	c.log = log
}

func (c *CliController) bootstrapOnExecute() {
	// Add Help group
	_, helpGroup := c.root.handleGroup("help:help")
	c.root.cmd.SetCompletionCommandGroupID(helpGroup)
	c.root.cmd.SetHelpCommandGroupID(helpGroup)
}
