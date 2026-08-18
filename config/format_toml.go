package config

import "github.com/pelletier/go-toml/v2"

type tomlFormat struct{}

func (tomlFormat) Decode(data []byte) (map[string]any, error) {
	var root map[string]any
	err := toml.Unmarshal(data, &root)
	return root, err
}
func (tomlFormat) Encode(root map[string]any) ([]byte, error) { return toml.Marshal(root) }
