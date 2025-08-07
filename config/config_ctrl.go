package config

import (
	"os"
	"slices"
	"sync"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	cmperr "github.com/gxmmx/compage-go/errors"
	cmpmap "github.com/gxmmx/compage-go/utils/mapx"
	cmpplt "github.com/gxmmx/compage-go/utils/platform"
	cmpstr "github.com/gxmmx/compage-go/utils/stringx"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

type Controller struct {
	cnf     *viper.Viper
	filecnf *viper.Viper

	cnfName   string
	cnfDir    string
	cnfType   string
	cnfPath   string
	cmfPerms  os.FileMode
	envPrefix string

	cnfCreate      bool
	cnfCurrentPath string

	flags  map[string]*pflag.Flag
	secret []string

	once sync.Once

	parseErrors []error
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(opts ...Option) Config {
	appName := cmpplt.BinaryName()
	ctl := &Controller{
		cnf:         viper.New(),
		filecnf:     viper.New(),
		cnfName:     appName,
		cnfDir:      defaultCnfDir,
		cnfType:     defaultCnfType,
		cmfPerms:    defaultCnfPerms,
		envPrefix:   cmpstr.EnvifyString(appName),
		flags:       make(map[string]*pflag.Flag),
		secret:      []string{},
		parseErrors: []error{},
	}

	for _, opt := range opts {
		opt(ctl)
	}

	return ctl
}

// -----------------------------------------------------------------------------
// Config methods
// -----------------------------------------------------------------------------

// Apply an option to the controller after it has been created.
// After creation, adding options does not change state.
func (ctl *Controller) Option(opt Option) {
	opt(ctl)
}

// Get returns the config instance for the controller.
func (ctl *Controller) Get() *viper.Viper {
	ctl.once.Do(func() {
		ctl.parse()
	})
	return ctl.cnf
}

// GetParseErrors returns the list of errors encountered during parsing.
func (ctl *Controller) GetParseErrors() []error {
	return ctl.parseErrors
}

// Save saves a configuration key value pair.
func (ctl *Controller) Save(key string, value any) error {
	ctl.once.Do(func() {
		ctl.parse()
	})
	// Only save if the key is in the file config, not set after parsing.
	if !ctl.filecnf.IsSet(key) {
		return cmperr.New(cmperr.KindNotFound, "config-file", "key not found in config", nil)
	}
	// Save to file config
	ctl.filecnf.Set(key, value)
	// Write the file config to disk
	if err := ctl.filecnf.WriteConfigAs(ctl.cnfCurrentPath); err != nil {
		return cmperr.New(cmperr.KindInternal, "config-file", "failed to write config file", err)
	}
	// Update in-memory config as well
	ctl.cnf.Set(key, value)
	return nil
}

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

// Get the environment key prefix used.
func (ctl *Controller) GetEnvPrefix() string {
	return ctl.envPrefix
}

// Get a redacted value for a specific configuration key.
func (ctl *Controller) GetRedactedValue(key string) (string, bool) {
	str := ctl.cnf.GetString(key)
	if str == "" {
		return str, false
	}
	if slices.Contains(ctl.secret, key) {
		return "<redacted>", true
	}
	return str, true
}

// Get a map of all configuration options with secrets redacted.
func (ctl *Controller) GetRedactedMap() map[string]any {
	raw := ctl.cnf.AllSettings()
	cmpmap.RedactFromMap(raw, ctl.secret)
	return raw
}
