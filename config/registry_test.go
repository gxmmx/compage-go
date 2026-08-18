package config

import (
	"errors"
	"testing"
)

func TestSchemaContract(t *testing.T) {
	type alias string
	tests := []struct {
		name string
		load func() error
	}{
		{"non_struct", func() error { _, err := Load[int](); return err }},
		{"alias", func() error { _, err := Load[struct{ Value alias }](); return err }},
		{"required_default", func() error {
			_, err := Load[struct {
				Value string `required:"true" default:"x"`
			}]()
			return err
		}},
		{"bad_tag", func() error {
			_, err := Load[struct {
				Value string `save:"yes"`
			}]()
			return err
		}},
		{"excluded_with_tag", func() error {
			_, err := Load[struct {
				Value string `cfg:"-" env:"VALUE"`
			}]()
			return err
		}},
		{"nested_leaf_tag", func() error {
			_, err := Load[struct {
				Nested struct{ Value string } `env:"NESTED"`
			}]()
			return err
		}},
		{"scalar_namespace_collision", func() error {
			_, err := Load[struct {
				Scalar string                `cfg:"network"`
				Nested struct{ Host string } `cfg:"network"`
			}]()
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.load()
			var schema *SchemaError
			if !errors.As(err, &schema) {
				t.Fatalf("error = %T, want SchemaError", err)
			}
		})
	}
}

func TestOptionContract(t *testing.T) {
	for _, opts := range [][]Option{
		{WithFile("")},
		{WithFile("a.json"), WithFile("b.json")},
		{WithConfigEnv("A"), WithConfigEnv("B")},
		{WithEnvPrefix("_APP")},
		{WithFlagSource(nil)},
		{WithInitial("value", "a"), WithInitial("value", "b")},
	} {
		_, err := Load[struct{ Value string }](opts...)
		var option *OptionError
		if !errors.As(err, &option) {
			t.Fatalf("error = %T, want OptionError", err)
		}
	}
}
