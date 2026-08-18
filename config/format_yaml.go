package config

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

type yamlFormat struct{}

func (yamlFormat) Decode(data []byte) (map[string]any, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if err := checkYAMLNode(&document); err != nil {
		return nil, err
	}
	var root map[string]any
	err := yaml.Unmarshal(data, &root)
	return root, err
}
func (yamlFormat) Encode(root map[string]any) ([]byte, error) { return yaml.Marshal(root) }

// YAML's generic Go decoding turns some tagged scalar forms (notably binary)
// into ordinary strings. Reject them while tag information is still available,
// rather than accidentally treating them as supported config strings.
func checkYAMLNode(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are unsupported")
	}
	if node.Tag == "!!binary" || node.Tag == "!!timestamp" {
		return fmt.Errorf("unsupported YAML value %q", node.Tag)
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Kind != yaml.ScalarNode || node.Content[i].Tag != "!!str" {
				return fmt.Errorf("YAML mapping key is not a string")
			}
		}
	}
	for _, child := range node.Content {
		if err := checkYAMLNode(child); err != nil {
			return err
		}
	}
	return nil
}
