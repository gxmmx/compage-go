package config

import (
	"fmt"
	"strings"
)

// formatAdapter isolates format-specific parsing and rendering from file selection.
type formatAdapter interface {
	Decode([]byte) (map[string]any, error)
	Encode(map[string]any) ([]byte, error)
}

func formatForExtension(extension string) (formatAdapter, bool) {
	switch strings.ToLower(extension) {
	case ".json":
		return jsonFormat{}, true
	case ".toml":
		return tomlFormat{}, true
	case ".yaml", ".yml":
		return yamlFormat{}, true
	default:
		return nil, false
	}
}

// flatten turns a format document into leaf values while retaining invalid
// object values at scalar/unknown keys.  Retaining those values is important:
// dropping an empty object would make {"unknown": {}} indistinguishable from
// an absent key.
func flatten(prefix string, in map[string]any, out map[string]any, leaves, namespaces map[string]bool) error {
	for key, value := range in {
		if key == "" || strings.Contains(key, ".") {
			return fmt.Errorf("invalid key %q", key)
		}
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch nested := value.(type) {
		case map[string]any:
			if leaves[fullKey] || len(nested) == 0 && !namespaces[fullKey] {
				if _, exists := out[fullKey]; exists {
					return fmt.Errorf("duplicate key %q", fullKey)
				}
				out[fullKey] = nested
				continue
			}
			if err := flatten(fullKey, nested, out, leaves, namespaces); err != nil {
				return err
			}
		case map[any]any:
			return fmt.Errorf("non-string mapping key at %q", fullKey)
		default:
			if _, exists := out[fullKey]; exists {
				return fmt.Errorf("duplicate key %q", fullKey)
			}
			out[fullKey] = nested
		}
	}
	return nil
}
