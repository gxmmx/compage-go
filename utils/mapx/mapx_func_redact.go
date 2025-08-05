package maputils

import (
	"strings"
)

// -----------------------------------------------------------------------------
// Redact from Map
// -----------------------------------------------------------------------------

// RedactFromMap returns a new map with sensitive keys (dot-notated) redacted as "<redacted>".
func RedactFromMap(m map[string]any, sensitive []string) {
	for _, path := range sensitive {
		deepRedact(m, strings.Split(path, "."))
	}
}

// -----------------------------------------------------------------------------
// Internal functions
// -----------------------------------------------------------------------------

func deepRedact(current map[string]any, path []string) {
	if path[0] == "" {
		return
	}

	key := path[0]
	rest := path[1:]

	for k, v := range current {
		if k == key {
			if len(rest) == 0 {
				current[k] = "<redacted>"
			} else if nested, ok := v.(map[string]any); ok {
				deepRedact(nested, rest)
			}
		}

		// Recurse into nested maps regardless of key match
		if nested, ok := v.(map[string]any); ok {
			deepRedact(nested, path)
		}
	}
}
