package appold

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	appconfig "github.com/gxmmx/compage-go/config"
	appctx "github.com/gxmmx/compage-go/ctx"
	applogger "github.com/gxmmx/compage-go/logger"
	apputils "github.com/gxmmx/compage-go/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ApplicationController interface {
	BindFlags(*pflag.FlagSet)
	BindFlag(string, *pflag.Flag)
	Execute() error
	GetLogger() *slog.Logger
	GetConfig(dest any) error
	GetRawConfig() map[string]any
	GetContext() context.Context
	WriteConfig() error
}

// -----------------------------------------------------------------------------
// Application functions
// -----------------------------------------------------------------------------

type Settings struct {
	Name      string
	Version   string
	Short     string
	Long      string
	ConfigDir string
	EnvPrefix string
}

type Application struct {
	RootCmd *cobra.Command

	ctx     context.Context
	end     context.CancelFunc
	awg     *sync.WaitGroup
	sigChan chan os.Signal
	// units   []AppUnit

	config *appconfig.Controller
	logger *applogger.Controller

	flags    map[string]*pflag.Flag
	settings *Settings
}

func NewSettings() *Settings {
	appName := apputils.AppNameFromBin()
	return &Settings{
		Name:      appName,
		Version:   "0.0.0",
		Short:     "Short description of the application.",
		Long:      "Long descriptuin of the application.",
		ConfigDir: ".",
		EnvPrefix: apputils.EnvifyString(appName),
	}
}

func NewApplication(settings *Settings) *Application {
	if settings == nil {
		settings = NewSettings()
	}
	// New Application
	app := &Application{}
	app.settings = settings

	// Context
	ctx, cancel := appctx.New(settings.Name)
	awg := &sync.WaitGroup{}
	app.ctx = ctx
	app.end = cancel
	app.awg = awg
	app.sigChan = make(chan os.Signal, 1)
	// app.units = make([]AppUnit, 0)

	// Root Command
	var (
		cfgFile string
		logL    string
		logQ    bool
		logV    bool
	)
	rootCmd := &cobra.Command{}
	rootCmd.Use = settings.Name
	rootCmd.Short = settings.Short
	rootCmd.Long = settings.Long
	rootCmd.Version = settings.Version
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&logQ, "quiet", "q", false, "Enable quiet mode (loglevel:error)")
	rootCmd.PersistentFlags().BoolVarP(&logV, "verbose", "v", false, "Enable verbose output (loglevel:debug)")
	rootCmd.PersistentFlags().StringVar(&logL, "loglevel", "info", "Set log level (debug|info|warn|error)")
	rootCmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "loglevel")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		app.initialize()
	}

	app.RootCmd = rootCmd
	return app
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (a *Application) initialize() {
	// Config settings
	confset := appconfig.NewSettings()
	confset.CnfName = a.settings.Name
	confset.EnvPrefix = a.settings.EnvPrefix
	confset.CnfDir = a.settings.ConfigDir
	confset.CnfType = "yaml"
	confset.CnfFromCmdLine, _ = a.RootCmd.Flags().GetString("config")
	if a.RootCmd.Flags().Changed("loglevel") {
		confset.LogFromCmdLine, _ = a.RootCmd.Flags().GetString("loglevel")
	} else if a.RootCmd.Flags().Changed("verbose") {
		confset.LogFromCmdLine = "debug"
	} else if a.RootCmd.Flags().Changed("quiet") {
		confset.LogFromCmdLine = "error"
	} else {
		confset.LogFromCmdLine = ""
	}

	a.config = appconfig.NewController(confset)
	errs := a.config.Init(a.flags)

	// Log settings
	logset := applogger.NewSettings()
	logset.AppName = a.settings.Name
	logset.Level = a.config.GetLogLevel()
	logset.Class = "app"
	a.logger = applogger.NewController(logset)

	for _, err := range errs {
		if err != nil {
			a.logger.GetLogger().ErrorContext(a.ctx, fmt.Sprintf("error: %v", err))
		}
	}
}

// -----------------------------------------------------------------------------
// Bind flags for configuration
// -----------------------------------------------------------------------------

func (a *Application) BindFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}
	if a.flags == nil {
		// create a new map of string to pflag.Flag
		a.flags = make(map[string]*pflag.Flag)
	}
	// append flagset to the map using visitall
	fs.VisitAll(func(f *pflag.Flag) {
		if f == nil {
			return
		}
		if _, ok := a.flags[f.Name]; !ok {
			a.flags[f.Name] = f
		}
	})
}

func (a *Application) BindFlag(name string, f *pflag.Flag) {
	if f == nil {
		return
	}
	if a.flags == nil {
		a.flags = make(map[string]*pflag.Flag)
	}
	if _, ok := a.flags[name]; !ok {
		a.flags[name] = f
	}
}

// -----------------------------------------------------------------------------
// Execute the application
// -----------------------------------------------------------------------------

// Run from main
func (a *Application) Execute() error {
	return a.RootCmd.Execute()
}

// -----------------------------------------------------------------------------
// Flow control
// -----------------------------------------------------------------------------

// Runs a long running application. Run from the Cobra run functions
func (a *Application) Run() {
}
