package text

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Case controls case conversion in Slugify.
type Case int

const (
	Lower    Case = iota // lowercase
	Upper                // UPPERCASE
	Preserve             // no conversion
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// Slugify sanitizes input for use as an identifier.
// Converts case, replaces runs of non-alphanumeric characters with sep,
// and trims leading/trailing separators. Defaults to Lower if no Case is provided.
func Slugify(input string, sep string, c ...Case) string {
	casing := Lower
	if len(c) > 0 {
		casing = c[0]
	}
	switch casing {
	case Lower:
		input = strings.ToLower(input)
	case Upper:
		input = strings.ToUpper(input)
	}
	result := nonAlnum.ReplaceAllString(input, sep)
	return strings.Trim(result, sep)
}

// StripPrefix removes prefix from input. If the remaining string starts
// with a common separator (_, -, .), that separator is also removed.
// Returns input unchanged if prefix is not present.
func StripPrefix(input string, prefix string) string {
	if !strings.HasPrefix(input, prefix) {
		return input
	}
	s := strings.TrimPrefix(input, prefix)
	if len(s) > 0 && (s[0] == '_' || s[0] == '-' || s[0] == '.') {
		s = s[1:]
	}
	return s
}

// EnsurePrefix returns s with prefix prepended if not already present.
func EnsurePrefix(s string, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}
	return prefix + s
}

// EnsureSuffix returns s with suffix appended if not already present.
func EnsureSuffix(s string, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s
	}
	return s + suffix
}

// Truncate shortens s to max runes. If truncated, tail is appended
// (included in the max count). Returns s unchanged if already within max.
func Truncate(s string, max int, tail string) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	tailLen := utf8.RuneCountInString(tail)
	if max <= tailLen {
		return string([]rune(tail)[:max])
	}
	runes := []rune(s)
	return string(runes[:max-tailLen]) + tail
}
