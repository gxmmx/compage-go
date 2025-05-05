package core

import (
	"context"
	"fmt"
	"log/slog"
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
	// Structure
	AddCommand(cmd *cobra.Command)
	AddConfig(name string, f *pflag.Flag)
	AddConfigs(fs *pflag.FlagSet)
	AddUnit(unit Unit)

	// State
	Run()
	End(rc AppReturnCode)
	Execute() error

	// Internal
	initialize()
	runUnits()
	logStartMessage()
	logStopMessage()
	waitLoop()
	shutdown()
}

type ApplicationRuntime interface {
	GetLogger() *slog.Logger
	GetConfig(sub string, unmarshal any) error
	GetHandle(name string) (any, bool)
	End(rc AppReturnCode)

	// Internal
	getConfigString(key string) string
	getConfigInt(key string) int
	getConfigBool(key string) bool
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
	rc  AppReturnCode

	sigChan chan os.Signal
	endChan chan AppReturnCode

	logger *applogger.Controller
	config *appconfig.Controller

	flags   map[string]*pflag.Flag
	command *cobra.Command

	units *registeredUnits
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewApp(settings *AppSettings) Application {
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

	app.units = newRegisteredUnits()

	if settings.AddConfigCmd {
		configCmd := genConfigCommand(app)
		app.command.AddCommand(configCmd)
	}

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

func (a *App) AddUnit(unit Unit) {
	if a.state > appInitialized {
		panic("Unit '" + unit.Name() + "' can not be added after the application has started")
	}
	if unit == nil {
		return
	}
	// Add unit to registered units
	a.units.set(unit.Name(), unit)

	ctx, end := appctx.Child(a.ctx, unit.Name())
	unit.setApp(a)
	unit.setCtx(ctx)
	unit.setEnd(end)
}

// -----------------------------------------------------------------------------
// Runtime functions
// -----------------------------------------------------------------------------

// Gets the logger for the application
func (a *App) GetLogger() *slog.Logger {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching logger")
	}
	return a.logger.GetLogger()
}

// Gets the config for a given sub-structure
func (a *App) GetConfig(sub string, unmarshal any) error {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching config")
	}
	return a.config.GetConfig(sub, unmarshal)
}

// Gets handle (any) of a named unit - should be wrapped with a type assertion
func (a *App) GetHandle(name string) (any, bool) {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching handle")
	}
	unit, ok := a.units.get(name)
	if !ok {
		return nil, false
	}
	return unit.getHandle()
}

// Ends the application with a given return code
func (a *App) End(rc AppReturnCode) {
	if a.state != appRunning {
		return
	}
	a.rc = rc
	a.endChan <- rc
}

// Gets the config value for a given key as a string
func (a *App) getConfigString(key string) string {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching config")
	}
	return a.config.GetString(key)
}

// Gets the config value for a given key as an int
func (a *App) getConfigInt(key string) int {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching config")
	}
	return a.config.GetInt(key)
}

// Gets the config value for a given key as a bool
func (a *App) getConfigBool(key string) bool {
	if a.state < appInitialized {
		panic("Application must be initialized before fetching config")
	}
	return a.config.GetBool(key)
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

	for _, unit := range a.units.Values() {
		a.wg.Add(1)
		switch unit.getKind() {
		case UnitKindFunc:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Running: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Completed unsuccessfully: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Completed: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		case UnitKindPeer:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Peering starting: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Peering stopped unsuccessfully: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Peering stopped: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		case UnitKindData:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Data connection init: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Data connection unsuccessful: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Data connection ready: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		case UnitKindRepo:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Repository init: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Repository unavailable: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Repository ready: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		case UnitKindServ:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Service init: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Service unavailable: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Service ready: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		case UnitKindPort:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Transport starting: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Transport stopped unsuccessfully: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Transport stopped: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		default:
			go func(u Unit) {
				defer a.wg.Done()
				a.logger.GetLogger().DebugContext(a.ctx, "Running: "+appctx.GetUnitName(u.GetCtx()))
				err := u.run(u)
				if err != nil {
					a.logger.GetLogger().DebugContext(a.ctx, "Completed unsuccessfully: "+appctx.GetUnitName(u.GetCtx()))
				} else {
					a.logger.GetLogger().DebugContext(a.ctx, "Completed: "+appctx.GetUnitName(u.GetCtx()))
				}
			}(unit)
		}
	}
}

// Logs the start message
func (a *App) logStartMessage() {
	a.logger.GetLogger().DebugContext(a.ctx, "Application started")
}

// Logs the stop message based on the return code
func (a *App) logStopMessage() {
	if a.rc == 0 {
		a.logger.GetLogger().DebugContext(a.ctx, "Application stopped successfully")
	} else {
		a.logger.GetLogger().ErrorContext(a.ctx, "Application stopped with error code: "+string(a.rc))
	}
}

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
