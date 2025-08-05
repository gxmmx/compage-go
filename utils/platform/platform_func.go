package platform

import (
	"os"
	"path/filepath"

	cmpstr "github.com/gxmmx/compage-go/utils/stringx"
)

// Returns sanitized name from binary being executed
func BinaryName() string {
	return cmpstr.SlugifyString(filepath.Base(os.Args[0]))
}
