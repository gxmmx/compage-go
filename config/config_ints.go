package config

import "github.com/spf13/viper"

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Config interface {
	Option(opt Option)
	Get() *viper.Viper
}
