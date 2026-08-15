package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestStrictFileObjectsAndYAMLValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.json")
	if err := os.WriteFile(path, []byte(`{"unknown":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load[struct{ Value string }](WithFile(path))
	var unknown *UnknownKeyError
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %T, want UnknownKeyError", err)
	}
	for _, contents := range []string{`{"value":{}}`, `{"value":{"nested":true}}`, `{"value":1}`} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load[struct{ Value string }](WithFile(path))
		var decode *DecodeError
		if !errors.As(err, &decode) {
			t.Fatalf("error = %T, want DecodeError", err)
		}
	}
	for _, contents := range []string{"value: .nan\n", "value: !!binary aGVsbG8=\n", "value: 2026-08-15T12:00:00Z\n", "value: &value 1\nother: *value\n"} {
		yamlPath := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(yamlPath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load[struct{ Value float64 }](WithFile(yamlPath)); err == nil {
			t.Fatal("Load accepted invalid YAML")
		}
	}
}

func TestFileSelectionAndReadFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.toml")
	c, err := Load[struct {
		Value string `default:"x" save:"true"`
	}](WithFile(path))
	if err != nil || c.FileLoaded() || c.Path() != path {
		t.Fatalf("fileless config = %#v, %v", c, err)
	}
	override := filepath.Join(t.TempDir(), "override.json")
	if err := os.WriteFile(override, []byte(`{"value":"override"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_CONFIG_PATH", override)
	c2, err := Load[struct{ Value string }](WithFile(path), WithConfigEnv("TEST_CONFIG_PATH"))
	if err != nil || !c2.FileLoaded() || c2.Path() != override {
		t.Fatalf("override = %#v, %v", c2, err)
	}
	link := filepath.Join(t.TempDir(), "broken.json")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.json"), link); err != nil {
		t.Skip(err)
	}
	_, err = Load[struct{ Value string }](WithFile(link))
	var read *FileReadError
	if !errors.As(err, &read) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("broken symlink error = %T %v", err, err)
	}
}

func TestStrictDuplicateKeysAcrossFormats(t *testing.T) {
	for _, test := range []struct{ extension, contents string }{
		{".json", `{"value":1,"value":2}`},
		{".yaml", "value: 1\nvalue: 2\n"},
		{".toml", "value = 1\nvalue = 2\n"},
	} {
		path := filepath.Join(t.TempDir(), "config"+test.extension)
		if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load[struct{ Value int }](WithFile(path)); err == nil {
			t.Fatalf("duplicate keys accepted for %s", test.extension)
		}
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
