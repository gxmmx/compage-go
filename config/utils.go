package config

import (
	"errors"
	"io/fs"
	"strings"

	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Functions
// -----------------------------------------------------------------------------

func confNameFromFlagName(name string) string {
	return strings.ReplaceAll(name, "-", ".")
}

func isMissingConfigFileErr(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	var pathErr *fs.PathError
	return errors.As(err, &notFound) || errors.As(err, &pathErr)
}
