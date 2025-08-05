package platform

import (
	"os"
	"path/filepath"

	stringx "github.com/gxmmx/compage-go/utils/stringx"
)

// Returns sanitized name from binary being executed
func BinaryName() string {
	return stringx.SlugifyString(filepath.Base(os.Args[0]))
}
