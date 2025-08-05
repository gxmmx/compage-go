package config

import (
	"io/fs"
	"os"
)

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

// Default configuration directory
var defaultCnfDir string = "."

// Default configuration file type
var defaultCnfType string = "yaml"

// Default configuration file mode
var defaultCnfPerms os.FileMode = 0644

// -----------------------------------------------------------------------------
// Variables
// -----------------------------------------------------------------------------

// Testing override
var osChmodFunc func(string, fs.FileMode) error = os.Chmod
