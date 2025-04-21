package appold

import (
	"context"
	"log/slog"

	appconfig "github.com/gxmmx/compage-go/config"
	applogger "github.com/gxmmx/compage-go/logger"
)

func (a *Application) WriteConfig() error {
	if a.config == nil {
		a.config = appconfig.NewController(nil)
	}
	err := a.config.WriteConfig()
	if err != nil {
		return err
	}
	return nil
}

func (a *Application) GetLogger() *slog.Logger {
	if a.logger == nil {
		a.logger = applogger.NewController(nil)
	}
	return a.logger.GetLogger()
}

func (a *Application) GetConfig(dest any) error {
	if a.config == nil {
		a.config = appconfig.NewController(nil)
	}
	return a.config.GetConfig(dest)
}

func (a *Application) GetRawConfig() map[string]any {
	if a.config == nil {
		a.config = appconfig.NewController(nil)
	}
	return a.config.GetRawConfig()
}

func (a *Application) GetContext() context.Context {
	return a.ctx
}
