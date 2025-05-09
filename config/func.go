package config

import (
	"errors"
	"io/fs"

	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

func isMissingConfigFileErr(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	var pathErr *fs.PathError
	return errors.As(err, &notFound) || errors.As(err, &pathErr)
}
