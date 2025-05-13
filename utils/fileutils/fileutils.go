package fileutils

import (
	"errors"
	"fmt"
	"os"

	// Compage
	apperror "github.com/gxmmx/compage-go/errors"
)

// -----------------------------------------------------------------------------
// File functions
// -----------------------------------------------------------------------------

// Fetches bytes from a file.
// Returns an error if the file does not exist or could not be read.
// Returns nil if the path is empty.
func FetchBytesFromFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperror.NotFound(err, fmt.Sprintf("file '%s' does not exist", path))
		}
		return nil, apperror.Internal(err, fmt.Sprintf("file '%s' could not be read", path))
	}
	return data, nil
}

// Fetches a string from a file.
// Returns an error if the file does not exist or could not be read.
// Returns an empty string if the path is empty.
func FetchStringFromFile(path string) (string, error) {
	data, err := FetchBytesFromFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
