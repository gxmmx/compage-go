package app

import (
	"context"
	"os"
	"sync"

	appconfig "github.com/gxmmx/compage-go/config"
	applogger "github.com/gxmmx/compage-go/logger"
	apputils "github.com/gxmmx/compage-go/utils"
)

type App struct {
	state    appState
	settings *AppSettings

	ctx context.Context
	end context.CancelFunc
	wg  *sync.WaitGroup

	sigChan chan os.Signal
	endChan chan struct{}

	loggerCtrl *applogger.Controller
	configCtrl *appconfig.Controller
}

func NewAppSettings() *AppSettings {
	appName := apputils.AppNameFromBin()
	return &AppSettings{
		Name:      appName,
		Version:   "1.0.0",
		Short:     "Compage Application",
		Long:      "App built using the Compage framework.",
		ConfigDir: ".",
		EnvPrefix: apputils.EnvifyString(appName),
	}
}
