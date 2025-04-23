package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	appconfig "github.com/gxmmx/compage-go/config"
	appctx "github.com/gxmmx/compage-go/ctx"
	applogger "github.com/gxmmx/compage-go/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Application interface {
	AddCommand(cmd *cobra.Command)
	AddConfig(name string, f *pflag.Flag)
	AddConfigs(fs *pflag.FlagSet)
	AddUnit(unit ApplicationUnit)
	Initialize()
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type App struct {
	state    appState
	settings *AppSettings

	ctx context.Context
	end context.CancelFunc
	wg  *sync.WaitGroup
	rc  appReturnCode

	sigChan chan os.Signal
	endChan chan appReturnCode

	logger *applogger.Controller
	config *appconfig.Controller

	flags   map[string]*pflag.Flag
	command *cobra.Command
	units   map[string]ApplicationUnit
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewApp(settings *AppSettings) *App {
	// Create default settings if not provided
	if settings == nil {
		settings = NewAppSettings()
	}
	// Create the app instance
	app := &App{}
	app.state = appCreated
	app.settings = settings

	ctx, end := appctx.New(settings.Name)
	app.ctx = ctx
	app.end = end
	app.wg = newAppWaitGroup()
	app.rc = 0
	app.sigChan = newAppSigChan()
	app.endChan = newAppEndChan()

	app.flags = newFlags()
	app.command = newRootCommand(settings)
	app.command.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		app.initialize()
	}

	app.logger = nil
	app.config = nil

	app.units = newAppUnits()

	return app
}

// -----------------------------------------------------------------------------
// Build functions
// -----------------------------------------------------------------------------

func (a *App) AddCommand(cmd *cobra.Command) {
	if a.state != appCreated {
		panic("Commands can not be added after the application is initialized")
	}
	a.command.AddCommand(cmd)
}

func (a *App) AddConfig(name string, f *pflag.Flag) {
	if a.state != appCreated {
		panic("Config flags can not be added after the application is initialized")
	}
	if f == nil {
		return
	}
	if _, ok := a.flags[name]; !ok {
		a.flags[name] = f
	}
}

func (a *App) AddConfigs(fs *pflag.FlagSet) {
	if a.state != appCreated {
		panic("Config flags can not be added after the application is initialized")
	}
	if fs == nil {
		return
	}
	fs.VisitAll(func(f *pflag.Flag) {
		if f == nil {
			return
		}
		if _, ok := a.flags[f.Name]; !ok {
			a.flags[f.Name] = f
		}
	})
}

func (a *App) AddUnit(unit ApplicationUnit) {
	if a.state > appInitialized {
		panic("Unit '" + unit.Name() + "' can not be added after the application has started")
	}
	if unit == nil {
		return
	}
	if _, ok := a.units[unit.Name()]; ok {
		panic("Unit with the name '" + unit.Name() + "' is already in use")
	}
	ctx, end := appctx.Child(a.ctx, unit.Name())
	unit.setApp(a)
	unit.setCtx(ctx)
	unit.setEnd(end)
	a.units[unit.Name()] = unit
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func (a *App) initialize() {
	if a.state != appCreated {
		panic("Application can only be initialized once")
	}
	a.state = appInitialized

	// Initialize config
	cnfSet := appconfig.NewSettings()
	cnfSet.CnfType = "yaml"
	cnfSet.CnfDir = a.settings.ConfigDir
	cnfSet.CnfName = a.settings.Name
	cnfSet.EnvPrefix = a.settings.EnvPrefix
	cnfSet.CnfFromCmdLine, _ = a.command.Flags().GetString("config")
	if a.command.Flags().Changed("loglevel") {
		cnfSet.LogFromCmdLine, _ = a.command.Flags().GetString("loglevel")
	} else if a.command.Flags().Changed("verbose") {
		cnfSet.LogFromCmdLine = "debug"
	} else if a.command.Flags().Changed("quiet") {
		cnfSet.LogFromCmdLine = "error"
	}

	a.config = appconfig.NewController(cnfSet)
	if err := a.config.Init(a.flags); err != nil {
		panic(err)
	}

	// Initialize logger
	logSet := applogger.NewSettings()
	logSet.AppName = a.settings.Name
	logSet.Level = a.config.GetLogLevel()
	logSet.Class = "app"
	a.logger = applogger.NewController(logSet)

	a.state = appInitialized
}

func (a *App) runUnits() {
	if a.state < appInitialized {
		panic("Application must be initialized before running units")
	}

	for _, unit := range a.units {
		a.wg.Add(1)
		go func(u ApplicationUnit) {
			defer a.wg.Done()
			cancel := u.GetEnd()
			defer cancel()
			a.logger.GetLogger().DebugContext(a.ctx, "Starting unit: "+u.Name())
			unit.run(a)
		}(unit)
	}
}

func (a *App) logStartMessage() {
	a.logger.GetLogger().DebugContext(a.ctx, "Application started")
}

func (a *App) logStopMessage() {
	if a.rc == 0 {
		a.logger.GetLogger().DebugContext(a.ctx, "Application stopped successfully")
	} else {
		a.logger.GetLogger().ErrorContext(a.ctx, "Application stopped with error code: "+string(a.rc))
	}
}

// func (a *App) running() {

// 	if a.state != appRunning {
// 		panic("Application must be running to wait for completion")
// 	}
// 	a.wg.Wait()
// 	a.state = appCompleted
// }

// Waits for the application to be signaled to stop
func (a *App) waitLoop() {
	a.state = appRunning
	select {
	case <-a.sigChan:
		a.logger.GetLogger().DebugContext(a.ctx, "Application stopping (signal received)")
	case rc := <-a.endChan:
		a.rc = rc
		a.logger.GetLogger().DebugContext(a.ctx, "Application stopping (termination received)")
	}
}

// Shuts down the Application
func (a *App) shutdown() {
	a.state = appStopping
	if r := recover(); r != nil {
		a.logger.GetLogger().ErrorContext(a.ctx, "Application panic: "+fmt.Sprint(r))
		a.rc = 1
	}

	a.end()
	a.wg.Wait()
	a.state = appCompleted
	a.logStopMessage()
	close(a.sigChan)
	close(a.endChan)
	os.Exit(int(a.rc))
}

// -----------------------------------------------------------------------------
// Run functions
// -----------------------------------------------------------------------------

func (a *App) End(rc appReturnCode) {
	if a.state != appRunning {
		return
	}
	a.rc = rc
	a.endChan <- rc
}

// Run the application from the run function of commands
func (a *App) Run() {
	defer a.shutdown()
	if a.state > appInitialized {
		panic("Application must be initialized before running")
	}

	a.logStartMessage()
	a.state = appRunning

	// Run all units
	a.runUnits()

	// Wait for all units to finish or termination
	a.waitLoop()
}

// Main execution function
func (a *App) Execute() error {
	return a.command.Execute()
}
