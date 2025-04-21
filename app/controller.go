package app

import (
	"context"
	"log/slog"
	"sync"

	appconfig "github.com/gxmmx/compage-go/config"
	appctx "github.com/gxmmx/compage-go/ctx"
	applogger "github.com/gxmmx/compage-go/logger"
	apputils "github.com/gxmmx/compage-go/utils"

	"github.com/spf13/cobra"
)

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

	ctx context.Context
	end context.CancelFunc
	awg *sync.WaitGroup

	config *appconfig.Controller
	logger *applogger.Controller

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
	a.config.Init()
	a.config.ReadConfig()

	// Log settings
	logset := applogger.NewSettings()
	logset.AppName = a.settings.Name
	logset.Level = a.config.GetString("loglevel")
	logset.Class = "app"
	a.logger = applogger.NewController(logset)
}

func (a *Application) Execute() error {
	return a.RootCmd.ExecuteContext(a.ctx)
}

func (a *Application) GetLogger() *slog.Logger {
	if a.logger == nil {
		a.logger = applogger.NewController(nil)
	}
	return a.logger.GetLogger()
}

func (a *Application) GetContext() context.Context {
	return a.ctx
}
