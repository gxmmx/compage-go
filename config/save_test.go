package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

func TestSavePromotesOnlySetValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
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
	if len(b) == 0 || contains(string(b), "env-secret") {
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

func TestSaveSensitiveAndNestedValues(t *testing.T) {
	type Config struct {
		Network struct {
			Host string `default:"localhost" save:"true"`
		} `cfg:"network"`
		Token string `sensitive:"true" save:"true"`
	}
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load[Config](WithFile(path))
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
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %#o, want 0600", info.Mode().Perm())
	}
	contents, err := os.ReadFile(path)
	if err != nil || !contains(string(contents), `"network"`) || !contains(string(contents), "persisted-secret") {
		t.Fatalf("saved nested content = %q, err = %v", contents, err)
	}
}

func TestSaveNeverWritesSensitiveDefault(t *testing.T) {
	type Config struct {
		Name   string `default:"normal" save:"true"`
		Secret string `default:"default-secret" sensitive:"true" save:"true"`
	}
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load[Config](WithFile(path))
	if err != nil || c.Save() != nil {
		t.Fatalf("load/save failed: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || contains(string(contents), "default-secret") {
		t.Fatalf("sensitive default was saved: %q, %v", contents, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %#o, want 0644", info.Mode().Perm())
	}
}

func TestSaveFailuresAreTypedAndDoNotPublish(t *testing.T) {
	type Config struct {
		Value string `default:"value" save:"true"`
	}
	path := filepath.Join(t.TempDir(), "missing", "config.json")
	c, err := Load[Config](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Save()
	var save *SaveError
	if !errors.As(err, &save) || save.Path() != path || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("save error = %T %v", err, err)
	}
	if got := c.Values().Value; got != "value" {
		t.Fatalf("failed Save changed value to %q", got)
	}
}

func TestSaveRefusesSymlinkTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"value":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "config.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	c, err := Load[struct {
		Value string `default:"value" save:"true"`
	}](WithFile(link))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Save()
	var save *SaveError
	if !errors.As(err, &save) || save.Kind() != errx.Internal {
		t.Fatalf("Save error = %T (%v), want internal SaveError", err, err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || !contains(string(contents), "file") {
		t.Fatalf("symlink target changed: %q, %v", contents, err)
	}
}
