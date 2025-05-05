package core

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Creates a new map of registered units
func newRegisteredUnits() *registeredUnits {
	return &registeredUnits{
		ukeys: make([]string, 0),
		units: make(map[string]Unit),
	}
}

// Creates an end channel for the application
func newAppEndChan() chan AppReturnCode {
	return make(chan AppReturnCode, 1)
}

// Creates a signal channel for the application and listens for OS signals
func newAppSigChan() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}

// Creates a wait group for the application
func newAppWaitGroup() *sync.WaitGroup {
	return &sync.WaitGroup{}
}

func newRootCommand(settings *AppSettings) *cobra.Command {
	rootCmd := &cobra.Command{}
	rootCmd.Use = settings.Name
	rootCmd.Short = settings.Short
	rootCmd.Long = settings.Long
	rootCmd.Version = settings.Version
	rootCmd.PersistentFlags().String("config", "", "config file path")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Enable quiet mode (loglevel:error)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (loglevel:debug)")
	rootCmd.PersistentFlags().String("loglevel", "info", "Set log level (debug|info|warn|error)")
	rootCmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "loglevel")
	return rootCmd
}

func newFlags() map[string]*pflag.Flag {
	return make(map[string]*pflag.Flag)
}

func newUnitSink() unitSink {
	return &sink{
		mu:    sync.RWMutex{},
		data:  nil,
		isSet: false,
	}
}
