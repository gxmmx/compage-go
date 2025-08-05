package config

import (
	"os"
	"sync"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	cmpplt "github.com/gxmmx/compage-go/utils/platform"
	stringx "github.com/gxmmx/compage-go/utils/stringx"
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
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(opts ...Option) Config {
	appName := cmpplt.BinaryName()
	ctl := &Controller{
		cnf:       viper.New(),
		filecnf:   viper.New(),
		cnfName:   appName,
		cnfDir:    defaultCnfDir,
		cnfType:   defaultCnfType,
		cmfPerms:  defaultCnfPerms,
		envPrefix: stringx.EnvifyString(appName),
		flags:     make(map[string]*pflag.Flag),
		secret:    []string{},
	}

	for _, opt := range opts {
		opt(ctl)
	}

	return ctl
}

// -----------------------------------------------------------------------------
// Controller methods
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
