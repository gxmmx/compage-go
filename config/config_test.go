package config

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Port  int      `default:"8080" env:"TEST_CONFIG_PORT" flag:"port" save:"true"`
	Name  string   `default:"agent" save:"true"`
	Token string   `env:"TEST_CONFIG_TOKEN" sensitive:"true" save:"true"`
	Tags  []string `default:"[\"a\"]" save:"true"`
}

func TestTypedErrors(t *testing.T) {
	_, err := Load[int]()
	var schema *SchemaError
	if !errors.As(err, &schema) || schema.Kind().String() != "invalid" {
		t.Fatalf("schema error = %T %v", err, err)
	}
	c := New[testConfig]()
	err = c.Set("name", "x")
	var notLoaded *NotLoadedError
	if !errors.As(err, &notLoaded) {
		t.Fatalf("not loaded error = %T", err)
	}
}

func TestFlushLogDrainsInitializationDiagnostics(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	c, err := Load[testConfig]()
	if err != nil {
		t.Fatal(err)
	}
	c.FlushLog(logger)
	if !contains(output.String(), "config load completed") {
		t.Fatalf("diagnostic output = %q", output.String())
	}
	output.Reset()
	c.FlushLog(logger)
	if output.Len() != 0 {
		t.Fatalf("FlushLog did not drain records: %q", output.String())
	}
}

type testFlags map[string]struct {
	value   string
	changed bool
}

func (f testFlags) Lookup(name string) (string, bool, bool) {
	v, ok := f[name]
	return v.value, v.changed, ok
}

func TestLoadPrecedenceAndProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"port":9000,"name":"file","tags":["file"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_CONFIG_PORT", "9100")
	c, err := Load[testConfig](WithFile(path), WithFlagSource(testFlags{"port": {"9200", true}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Values(); got.Port != 9200 || got.Name != "file" {
		t.Fatalf("Values() = %#v", got)
	}
	if source, ok := c.Source("port"); !ok || source != SourceFlag {
		t.Fatalf("Source(port) = %v, %v", source, ok)
	}
	if !c.HasSource("port", SourceFile) || !c.HasSource("port", SourceEnv) {
		t.Fatal("per-layer presence was lost")
	}
	values := c.Values()
	values.Tags[0] = "mutated"
	if c.Values().Tags[0] != "file" {
		t.Fatal("Values exposed slice storage")
	}
}

func TestSetValidatesAndSaveDoesNotPersistEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("TEST_CONFIG_TOKEN", "env-secret")
	c, err := Load[testConfig](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("name", "saved"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" || contains(string(b), "env-secret") {
		t.Fatalf("Save wrote unexpected content: %q", b)
	}
	c2, err := Load[testConfig](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if c2.Values().Name != "saved" {
		t.Fatalf("saved name = %q", c2.Values().Name)
	}
}

func TestSaveSensitiveSetValueUsesPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load[testConfig](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("token", "persisted-secret"); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %#o, want 0600", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(contents), "persisted-secret") {
		t.Fatal("Set sensitive value was not saved")
	}
}

func TestSaveNeverWritesSensitiveDefault(t *testing.T) {
	type Config struct {
		Name   string `default:"normal" save:"true"`
		Secret string `default:"default-secret" sensitive:"true" save:"true"`
	}
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load[Config](WithFile(path))
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
	if contains(string(contents), "default-secret") {
		t.Fatalf("sensitive default was saved: %q", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %#o, want 0644", got)
	}
}

func TestSaveFailureDoesNotChangePublishedState(t *testing.T) {
	type Config struct {
		Value string `default:"value" save:"true"`
	}
	path := filepath.Join(t.TempDir(), "missing", "config.json")
	c, err := Load[Config](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Save()
	if err == nil {
		t.Fatal("Save created a missing parent directory")
	}
	var save *SaveError
	if !errors.As(err, &save) {
		t.Fatalf("save error = %T", err)
	}
	if save.Path() != path {
		t.Fatalf("save path = %q, want %q", save.Path(), path)
	}
	if got := c.Values().Value; got != "value" {
		t.Fatalf("failed Save changed value to %q", got)
	}
}

func TestSaveRefusesSymlinkTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"original":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	type Config struct {
		Value string `default:"value" save:"true"`
	}
	c, err := Load[Config](WithFile(link))
	if err == nil {
		t.Fatal("unknown target key should fail strict load")
	}
	// Use an empty symlink target that matches the schema for the actual save assertion.
	if err := os.WriteFile(target, []byte(`{"value":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load[Config](WithFile(link))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err == nil {
		t.Fatal("Save replaced a symlink target")
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(contents), "file") {
		t.Fatalf("symlink target changed: %q", contents)
	}
}

func TestSupportedFormatsAndStrictJSON(t *testing.T) {
	for _, test := range []struct{ name, content string }{
		{"json", `{"port": 9000, "name":"json", "tags":["x"]}`},
		{"toml", "port = 9000\nname = 'toml'\ntags = ['x']\n"},
		{"yaml", "port: 9000\nname: yaml\ntags: [x]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config."+test.name)
			if test.name == "yaml" {
				path = filepath.Join(filepath.Dir(path), "config.yaml")
			}
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			c, err := Load[testConfig](WithFile(path))
			if err != nil {
				t.Fatal(err)
			}
			if c.Values().Port != 9000 || c.Values().Name != test.name {
				t.Fatalf("values = %#v", c.Values())
			}
		})
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"port":1,"port":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load[testConfig](WithFile(path)); err == nil {
		t.Fatal("duplicate JSON key loaded")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
