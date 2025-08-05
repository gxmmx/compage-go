package config

import "github.com/spf13/viper"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Config interface {
	// Set options for the config controller.
	Option(opt Option)
	// Get the config instance for the controller.
	Get() *viper.Viper
	// Get the parse errors encountered during config parsing.
	GetParseErrors() []error
}
