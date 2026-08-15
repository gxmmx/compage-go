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

func flatten(prefix string, in map[string]any, out map[string]any) error {
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
			if err := flatten(fullKey, nested, out); err != nil {
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
