package stringx

import (
	"regexp"
	"strings"
)

// -----------------------------------------------------------------------------
// String functions
// -----------------------------------------------------------------------------

// Sanitizes a string for use in a slug.
// Converts the string to lowercase and
// replaces non-alphanumeric characters with hyphens.
func SlugifyString(input string) string {
	// Convert the string to lowercase
	lower := strings.ToLower(input)

	// Replace non-alphanumeric characters with single hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(lower, "-")

	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	// Return the sanitized string
	return slug
}

// Sanitizes a string for use in an environment variable.
// Converts the string to uppercase and
// replaces non-alphanumeric characters with underscores.
func EnvifyString(input string) string {
	// Convert the string to uppercase
	upper := strings.ToUpper(input)

	// Replace non-alphanumeric characters with single underscores
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	env := re.ReplaceAllString(upper, "_")

	// Remove leading and trailing underscores
	env = strings.Trim(env, "_")

	// Return the sanitized string
	return env
}

// Sanitizes a string for use as a key.
// Converts the string to lowercase and
// replaces non-alphanumeric characters with dots.
func KeyifyString(input string) string {
	// Convert the string to lowercase
	lower := strings.ToLower(input)

	// Replace non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-z0-9]+`)
	key := re.ReplaceAllString(lower, ".")

	// Remove leading and trailing separators
	key = strings.Trim(key, ".")

	// Return the sanitized string
	return key
}

func StripPrefix(input string, prefix string) string {
	if !strings.HasPrefix(input, prefix) {
		return input
	}
	str := strings.TrimPrefix(input, prefix)
	str = strings.TrimPrefix(str, "_")
	str = strings.TrimPrefix(str, "-")
	str = strings.TrimPrefix(str, ".")
	return str
}

func EnsureLeadingSlash(s string) string {
	if !strings.HasPrefix(s, "/") {
		return "/" + s
	}
	return s
}
