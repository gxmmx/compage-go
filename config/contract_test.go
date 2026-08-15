package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSchemaContract(t *testing.T) {
	type alias string
	tests := []struct {
		name string
		load func() error
	}{
		{"non_struct", func() error { _, e := Load[int](); return e }},
		{"alias", func() error { _, e := Load[struct{ Value alias }](); return e }},
		{"required_default", func() error {
			_, e := Load[struct {
				Value string `required:"true" default:"x"`
			}]()
			return e
		}},
		{"bad_tag", func() error {
			_, e := Load[struct {
				Value string `save:"yes"`
			}]()
			return e
		}},
		{"empty_required_tag", func() error {
			_, e := Load[struct {
				Value string `required:""`
			}]()
			return e
		}},
		{"empty_sensitive_tag", func() error {
			_, e := Load[struct {
				Value string `sensitive:""`
			}]()
			return e
		}},
		{"empty_validate_tag", func() error {
			_, e := Load[struct {
				Value string `validate:""`
			}]()
			return e
		}},
		{"nested_leaf_tag", func() error {
			_, e := Load[struct {
				Nested struct {
					Value string
				} `env:"NESTED"`
			}]()
			return e
		}},
		{"scalar_namespace_collision", func() error {
			_, e := Load[struct {
				Scalar string `cfg:"network"`
				Nested struct {
					Host string
				} `cfg:"network"`
			}]()
			return e
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.load()
			if err == nil {
				t.Fatal("Load succeeded")
			}
			var target *SchemaError
			if !errors.As(err, &target) {
				t.Fatalf("error = %T, want SchemaError", err)
			}
		})
	}
}

func TestOptionContract(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"empty_file", []Option{WithFile("")}},
		{"duplicate_file", []Option{WithFile("a.json"), WithFile("b.json")}},
		{"bad_prefix", []Option{WithEnvPrefix("_APP")}},
		{"empty_config_env", []Option{WithConfigEnv("")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load[struct{ Value string }](test.opts...)
			if err == nil {
				t.Fatal("Load accepted invalid options")
			}
			var option *OptionError
			if !errors.As(err, &option) {
				t.Fatalf("error = %T, want OptionError", err)
			}
		})
	}
}

func TestLifecycleAndRequiredPresence(t *testing.T) {
	c := New[struct {
		Value int `required:"true"`
	}]()
	if err := c.Save(); err == nil {
		t.Fatal("Save before Load succeeded")
	} else {
		var notLoaded *NotLoadedError
		if !errors.As(err, &notLoaded) {
			t.Fatalf("Save error = %T", err)
		}
	}
	if err := c.Load(); err == nil {
		t.Fatal("missing required value loaded")
	}
	c, err := Load[struct {
		Value int `required:"true"`
	}](WithInitial("value", 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Values().Value; got != 0 {
		t.Fatalf("required zero value = %d", got)
	}
	noFile, err := Load[struct{ Value string }]()
	if err != nil {
		t.Fatal(err)
	}
	if err := noFile.Save(); err == nil {
		t.Fatal("fileless Save succeeded")
	} else {
		var noTarget *NoConfigFileError
		if !errors.As(err, &noTarget) {
			t.Fatalf("fileless save error = %T", err)
		}
	}
}

func TestStrictFileContract(t *testing.T) {
	tests := []struct{ name, extension, contents string }{
		{"unknown_json", ".json", `{"unknown":true}`},
		{"duplicate_json", ".json", `{"value":1,"value":2}`},
		{"duplicate_yaml", ".yaml", "value: 1\nvalue: 2\n"},
		{"duplicate_toml", ".toml", "value = 1\nvalue = 2\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config"+test.extension)
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load[struct{ Value int }](WithFile(path))
			if err == nil {
				t.Fatal("Load accepted invalid file")
			}
		})
	}
}

func TestFileSelectionAndAbsentFileLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.toml")
	c, err := Load[struct {
		Value string `default:"x" save:"true"`
	}](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if c.FileLoaded() || c.Path() != path {
		t.Fatalf("FileLoaded/Path = %v/%q", c.FileLoaded(), c.Path())
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(t.TempDir(), "override.json")
	if err := os.WriteFile(override, []byte(`{"value":"override"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_CONFIG_PATH", override)
	c2, err := Load[struct{ Value string }](WithFile(path), WithConfigEnv("TEST_CONFIG_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	if !c2.FileLoaded() || c2.Path() != override || c2.Values().Value != "override" {
		t.Fatalf("override config = %#v path=%q", c2.Values(), c2.Path())
	}
}

func TestLeadingHomeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	c, err := Load[struct{ Value string }](WithFile("~/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "config.json"); c.Path() != want {
		t.Fatalf("Path() = %q, want %q", c.Path(), want)
	}
}

func TestBrokenSymlinkIsNotAnAbsentConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.Symlink(filepath.Join(filepath.Dir(path), "missing.json"), path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := Load[struct{ Value string }](WithFile(path))
	if err == nil {
		t.Fatal("broken symlink was treated as absent")
	}
	var read *FileReadError
	if !errors.As(err, &read) {
		t.Fatalf("error = %T, want FileReadError", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("FileReadError did not expose the filesystem cause")
	}
	if read.Path() != path {
		t.Fatalf("FileReadError.Path() = %q, want %q", read.Path(), path)
	}
}

func TestFailedSetAndLoadDoNotPublish(t *testing.T) {
	type Config struct {
		Port int    `default:"1"`
		Name string `validate:"nonempty"`
	}
	c, err := Load[Config](WithValidator("nonempty", func(_ FieldContext, value any) error {
		if value.(string) == "" {
			return errors.New("empty")
		}
		return nil
	}), WithInitial("name", "good"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("name", ""); err == nil {
		t.Fatal("Set accepted invalid state")
	}
	if c.Values().Name != "good" {
		t.Fatal("failed Set published state")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"port":2,"name":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fileConfig, err := Load[Config](WithFile(path), WithValidator("nonempty", func(_ FieldContext, value any) error {
		if value.(string) == "" {
			return errors.New("empty")
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"port":"bad","name":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fileConfig.Load(); err == nil {
		t.Fatal("invalid reload succeeded")
	}
	if got := fileConfig.Values().Port; got != 2 {
		t.Fatalf("failed reload changed value to %d", got)
	}
}

func TestConversionContract(t *testing.T) {
	type Config struct {
		String   string        `default:"value"`
		Bool     bool          `default:"true"`
		Int      int           `default:"-1"`
		Int64    int64         `default:"2"`
		Uint     uint          `default:"3"`
		Uint64   uint64        `default:"4"`
		Float    float64       `default:"1.5"`
		Duration time.Duration `default:"2s"`
		Tags     []string      `default:"[\"a\",\"b\"]"`
	}
	c, err := Load[Config]()
	if err != nil {
		t.Fatal(err)
	}
	v := c.Values()
	if v.String != "value" || !v.Bool || v.Int != -1 || v.Int64 != 2 || v.Uint != 3 || v.Uint64 != 4 || v.Float != 1.5 || v.Duration != 2*time.Second || len(v.Tags) != 2 {
		t.Fatalf("converted values = %#v", v)
	}
	_, err = Load[struct {
		Value int `default:"not-a-number"`
	}]()
	var defaultError *DefaultValueError
	if !errors.As(err, &defaultError) {
		t.Fatalf("default error = %T", err)
	}
}

func TestSetDoesNotRetainCallerSlice(t *testing.T) {
	type Config struct{ Tags []string }
	c, err := Load[Config]()
	if err != nil {
		t.Fatal(err)
	}
	input := []string{"first"}
	if err := c.Set("tags", input); err != nil {
		t.Fatal(err)
	}
	input[0] = "mutated"
	if got := c.Values().Tags[0]; got != "first" {
		t.Fatalf("stored caller slice mutated to %q", got)
	}
}

func TestSetLayerSurvivesReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"value":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load[struct {
		Value string `default:"default"`
	}](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("value", "set"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"value":"new-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	if got := c.Values().Value; got != "set" {
		t.Fatalf("reload value = %q, want set", got)
	}
	if source, ok := c.Source("value"); !ok || source != SourceSet {
		t.Fatalf("source=%v,%v", source, ok)
	}
}

func TestConcurrentReadAndMutation(t *testing.T) {
	type Config struct {
		Value int      `default:"1"`
		Tags  []string `default:"[\"x\"]"`
	}
	c, err := Load[Config]()
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			for j := 0; j < 100; j++ {
				_ = c.Values()
				_, _ = c.Source("value")
				if i == 0 {
					if err := c.Set("value", j); err != nil {
						t.Error(err)
					}
				}
			}
		}(i)
	}
	group.Wait()
}
