package logger

import (
	"log/slog"

	apputils "github.com/gxmmx/compage-go/utils"
)

type Settings struct {
	AppName string
	Class   string
	Level   string
	Service LoggerService
}

type Controller struct {
	settings *Settings
	level    *slog.LevelVar
	logger   *slog.Logger
}

func NewSettings() *Settings {
	return &Settings{
		AppName: apputils.AppNameFromBin(),
		Class:   "app",
		Level:   "info",
		Service: nil,
	}
}

func NewController(settings *Settings) *Controller {
	if settings == nil {
		settings = NewSettings()
	}
	lvl := new(slog.LevelVar)
	lvl.Set(StringToLevel(settings.Level))

	return &Controller{
		settings: settings,
		level:    lvl,
	}
}

func (m *Controller) SetLevel(level string) {
	m.level.Set(StringToLevel(level))
}

func (m *Controller) GetLogger() *slog.Logger {
	if m.logger == nil {
		// create a slice of slog attributes
		attrs := []slog.Attr{
			slog.String("app", m.settings.AppName),
		}

		m.logger = slog.New(newLogHandler(m.level, m.settings.Class, m.settings.Service).WithAttrs(attrs))
	}
	return m.logger
}
