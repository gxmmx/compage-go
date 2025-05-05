package app

import apputils "github.com/gxmmx/compage-go/utils"

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type AppSettings struct {
	Name         string
	Version      string
	Short        string
	Long         string
	ConfigDir    string
	EnvPrefix    string
	AddConfigCmd bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewAppSettings() *AppSettings {
	appName := apputils.AppNameFromBin()
	return &AppSettings{
		Name:         appName,
		Version:      "1.0.0",
		Short:        "Compage Application",
		Long:         "App built using the Compage framework.",
		ConfigDir:    ".",
		EnvPrefix:    apputils.EnvifyString(appName),
		AddConfigCmd: true,
	}
}
