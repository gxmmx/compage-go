package utils

import (
	"errors"
	"fmt"
	"os"
)

// -----------------------------------------------------------------------------
// File functions
// -----------------------------------------------------------------------------

// Fetches a string from a file.
// Returns an error if the file does not exist or could not be read.
// Returns an empty string if the path is empty.
func FetchStringFromFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("file '%s' does not exist: %w", path, err)
		}
		return "", fmt.Errorf("file '%s' could not be read: %w", path, err)
	}
	return string(data), nil
}
