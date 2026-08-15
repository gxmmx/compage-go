package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceStrings(t *testing.T) {
	for source, want := range map[Source]string{
		SourceNone: "none", SourceDefault: "default", SourceFile: "file", SourceEnv: "env", SourceFlag: "flag", SourceSet: "set",
	} {
		if got := source.String(); got != want {
			t.Fatalf("Source(%d).String() = %q, want %q", source, got, want)
		}
	}
	if got := Source(99).String(); got != "none" {
		t.Fatalf("unknown source string = %q", got)
	}
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
	if origin, ok := c.Origin("port"); !ok || origin != (Origin{Source: SourceFlag, Detail: "port"}) {
		t.Fatalf("Origin(port) = %#v, %v", origin, ok)
	}
	if !c.HasSource("port", SourceFile) || !c.HasSource("port", SourceEnv) || c.HasSource("missing", SourceFile) {
		t.Fatal("per-layer presence was incorrect")
	}
	values := c.Values()
	values.Tags[0] = "mutated"
	if c.Values().Tags[0] != "file" {
		t.Fatal("Values exposed slice storage")
	}
	unknownPath := filepath.Join(dir, "unknown.json")
	if err := os.WriteFile(unknownPath, []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load[testConfig](WithFile(unknownPath))
	var unknown *UnknownKeyError
	if !errors.As(err, &unknown) || unknown.Source() != "file" {
		t.Fatalf("unknown file error = %T %v", err, err)
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
