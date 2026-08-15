package config

import "go.yaml.in/yaml/v3"

type yamlFormat struct{}

func (yamlFormat) Decode(data []byte) (map[string]any, error) {
	var root map[string]any
	err := yaml.Unmarshal(data, &root)
	return root, err
}
func (yamlFormat) Encode(root map[string]any) ([]byte, error) { return yaml.Marshal(root) }
