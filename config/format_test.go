package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupportedFormatsAndStrictJSON(t *testing.T) {
	for _, test := range []struct{ name, content string }{
		{"json", `{"port":9000,"name":"json","tags":["x"]}`},
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
			if err != nil || c.Values().Port != 9000 || c.Values().Name != test.name {
				t.Fatalf("values = %#v, err = %v", c.Values(), err)
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
