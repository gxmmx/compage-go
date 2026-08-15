package config

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLifecycleAndRequiredPresence(t *testing.T) {
	c := New[struct {
		Value int `required:"true"`
	}]()
	if err := c.Save(); err == nil {
		t.Fatal("Save before Load succeeded")
	}
	if err := c.Load(); err == nil {
		t.Fatal("missing required value loaded")
	}
	c, err := Load[struct {
		Value int `required:"true"`
	}](WithInitial("value", 0))
	if err != nil || c.Values().Value != 0 {
		t.Fatalf("required zero config = %#v, %v", c.Values(), err)
	}
	noFile, err := Load[struct{ Value string }]()
	if err != nil {
		t.Fatal(err)
	}
	var noTarget *NoConfigFileError
	if !errors.As(noFile.Save(), &noTarget) {
		t.Fatal("fileless Save did not return NoConfigFileError")
	}
}

func TestConcurrentReadAndMutation(t *testing.T) {
	c, err := Load[struct {
		Value int      `default:"1"`
		Tags  []string `default:"[\"x\"]"`
	}]()
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(writer bool) {
			defer group.Done()
			for j := 0; j < 100; j++ {
				_ = c.Values()
				_, _ = c.Source("value")
				if writer {
					if err := c.Set("value", j); err != nil {
						t.Error(err)
					}
				}
			}
		}(i == 0)
	}
	group.Wait()
}

func TestFailedReloadDoesNotPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"value":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load[struct{ Value int }](WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"value":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.Load(); err == nil {
		t.Fatal("invalid reload succeeded")
	}
	if got := c.Values().Value; got != 2 {
		t.Fatalf("failed reload changed value to %d", got)
	}
}
