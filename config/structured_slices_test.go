package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type StructuredItem struct {
	Key   string `cfg:"key"`
	Value string `cfg:"value"`
}

func TestStructuredSliceSchema(t *testing.T) {
	tests := []struct {
		name string
		load func() error
	}{
		{"valid", func() error {
			_, err := Load[struct{ Items []StructuredItem }]()
			return err
		}},
		{"nested slice", func() error {
			type Item struct{ Values []string }
			_, err := Load[struct{ Items []Item }]()
			return err
		}},
		{"element default tag", func() error {
			type Item struct {
				Key   string `default:"key"`
				Value string
			}
			_, err := Load[struct{ Items []Item }]()
			return err
		}},
		{"element environment tag", func() error {
			type Item struct {
				Key   string `env:"ITEM_KEY"`
				Value string
			}
			_, err := Load[struct{ Items []Item }]()
			return err
		}},
		{"unexported element field", func() error {
			type Item struct {
				Key    string
				secret string
			}
			_ = Item{secret: "private"}
			_, err := Load[struct{ Items []Item }]()
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.load()
			if test.name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var schema *SchemaError
			if !errors.As(err, &schema) {
				t.Fatalf("error = %T %v, want SchemaError", err, err)
			}
		})
	}
}

func TestStructuredSliceSourcesAndCloning(t *testing.T) {
	type Config struct {
		Items []StructuredItem `default:"[{\"key\":\"default\",\"value\":\"d\"}]" env:"STRUCTURED_ITEMS" flag:"items" save:"true"`
	}
	initial := []StructuredItem{{Key: "initial", Value: "i"}}
	c, err := Load[Config](WithInitial("items", initial))
	if err != nil {
		t.Fatal(err)
	}
	initial[0].Key = "mutated"
	if got := c.Values().Items[0].Key; got != "initial" {
		t.Fatalf("initial value = %q, want initial", got)
	}
	if source, ok := c.Source("items"); !ok || source != SourceInitial {
		t.Fatalf("source = %v, %v", source, ok)
	}

	set := []StructuredItem{{Key: "set", Value: "s"}}
	if err := c.Set("items", set); err != nil {
		t.Fatal(err)
	}
	set[0].Key = "mutated"
	if got := c.Values().Items[0].Key; got != "set" {
		t.Fatalf("set value = %q, want set", got)
	}
	if err := c.Set("items", `[{"key":"json","value":"j"}]`); err != nil {
		t.Fatal(err)
	}
	if got := c.Values().Items[0]; got != (StructuredItem{Key: "json", Value: "j"}) {
		t.Fatalf("JSON set = %#v", got)
	}
	returned := c.Values()
	returned.Items[0].Key = "changed"
	if got := c.Values().Items[0].Key; got != "json" {
		t.Fatalf("returned value aliased snapshot: %q", got)
	}
}

func TestStructuredSliceFiles(t *testing.T) {
	tests := []struct {
		ext, content, want string
	}{
		{"json", `{"items":[{"key":"json","value":"v"}]}`, "json"},
		{"yaml", "items:\n  - key: yaml\n    value: v\n", "yaml"},
		{"toml", "[[items]]\nkey = 'toml'\nvalue = 'v'\n", "toml"},
	}
	for _, test := range tests {
		t.Run(test.ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config."+test.ext)
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			c, err := Load[struct {
				Items []StructuredItem `save:"true"`
			}](WithFile(path))
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Values().Items; !reflect.DeepEqual(got, []StructuredItem{{Key: test.want, Value: "v"}}) {
				t.Fatalf("items = %#v", got)
			}
		})
	}
}

func TestStructuredSliceSourcePrecedenceAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"items":[{"key":"file","value":"f"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STRUCTURED_PRECEDENCE", `[{"key":"env","value":"e"}]`)
	seen := []string{}
	validate := func(_ FieldContext, value any) error {
		items, ok := value.([]StructuredItem)
		if !ok || len(items) != 1 {
			return errors.New("wrong structured value")
		}
		seen = append(seen, items[0].Key)
		return nil
	}
	type Config struct {
		Items []StructuredItem `default:"[{\"key\":\"default\",\"value\":\"d\"}]" env:"STRUCTURED_PRECEDENCE" flag:"items" validate:"items"`
	}
	c, err := Load[Config](WithFile(path), WithInitial("items", []StructuredItem{{Key: "initial", Value: "i"}}), WithFlagSource(testFlags{"items": {`[{"key":"flag","value":"g"}]`, true}}), WithValidator("items", validate))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Values().Items[0].Key; got != "flag" {
		t.Fatalf("winning value = %q, want flag", got)
	}
	if source, _ := c.Source("items"); source != SourceFlag {
		t.Fatalf("source = %v, want flag", source)
	}
	if err := c.Set("items", `[{"key":"set","value":"s"}]`); err != nil {
		t.Fatal(err)
	}
	if got := c.Values().Items[0].Key; got != "set" {
		t.Fatalf("set value = %q", got)
	}
	if !reflect.DeepEqual(seen, []string{"flag", "set"}) {
		t.Fatalf("validator calls = %#v", seen)
	}
}

func TestStructuredSliceRejectsInvalidValues(t *testing.T) {
	type Config struct{ Items []StructuredItem }
	for _, raw := range []any{
		`{"key":"not-an-array"}`,
		`[1]`,
		`[{"key":"missing-value"}]`,
		`[{"key":"unknown","value":"v","other":"x"}]`,
		`[{"key":1,"value":"v"}]`,
		[]any{"not an object"},
	} {
		c, err := Load[Config]()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Set("items", raw); err == nil {
			t.Errorf("Set accepted %#v", raw)
		}
	}
}

func TestStructuredSliceSaveUsesNativeObjects(t *testing.T) {
	tests := []struct {
		ext, marker string
	}{
		{"json", `"items": [`},
		{"yaml", "items:\n    - key:"},
		{"toml", "[[items]]"},
	}
	for _, test := range tests {
		t.Run(test.ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config."+test.ext)
			c, err := Load[struct {
				Items []StructuredItem `default:"[{\"key\":\"saved\",\"value\":\"v\"}]" save:"true"`
			}](WithFile(path))
			if err != nil || c.Save() != nil {
				t.Fatalf("load/save failed: %v", err)
			}
			contents, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(contents), test.marker) || !strings.Contains(string(contents), "saved") {
				t.Fatalf("saved %s = %q, err = %v", test.ext, contents, err)
			}
		})
	}
}

func TestStructuredSliceTransientSourcesAreNotSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("STRUCTURED_TRANSIENT", `[{"key":"environment","value":"e"}]`)
	c, err := Load[struct {
		Items []StructuredItem `env:"STRUCTURED_TRANSIENT" flag:"items" save:"true"`
	}](WithFile(path), WithFlagSource(testFlags{"items": {`[{"key":"flag","value":"f"}]`, true}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "flag") || strings.Contains(string(contents), "environment") {
		t.Fatalf("transient value saved: %q", contents)
	}
}
