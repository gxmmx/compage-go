package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// -----------------------------------------------------------------------------
// String functions
// -----------------------------------------------------------------------------

// Sanitizes a string for use in a slug.
// Converts the string to lowercase and
// replaces non-alphanumeric characters with underscores.
func SlugifyString(input string) string {
	// Convert the string to lowercase
	lower := strings.ToLower(input)

	// Replace non-alphanumeric characters with single underscores
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(lower, "-")

	// Return the sanitized string
	return slug
}

func EnvifyString(input string) string {
	// Convert the string to uppercase
	upper := strings.ToUpper(input)

	// Replace non-alphanumeric characters with single underscores
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	env := re.ReplaceAllString(upper, "_")

	// Return the sanitized string
	return env
}

// Returns sanitized app name from binary being executed
func AppNameFromBin() string {
	return SlugifyString(filepath.Base(os.Args[0]))
}
