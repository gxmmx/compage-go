package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestConfigCommands(t *testing.T) {
	// Create a temporary file for testing config
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
	os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
	defer os.Remove(tmpFile.Name())

	t.Run("list config", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "list"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
	})
	t.Run("get unknown config key", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "get", "unknown.key"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "not found") {
			t.Errorf("expected 'not found' in output, got: %s", ow.String())
		}
	})
	t.Run("get redacted config key", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "get", "test.secret"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "<redacted>") {
			t.Errorf("expected '<redacted>' in output, got: %s", ow.String())
		}
	})
	t.Run("get valid config key", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "get", "test.flag"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "defaultvalue") {
			t.Errorf("expected 'defaultvalue' in output, got: %s", ow.String())
		}
	})
	t.Run("get unknown config env", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "env", "unknown.key"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "not found") {
			t.Errorf("expected 'not found' in output, got: %s", ow.String())
		}
	})
	t.Run("get valid config env", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "env", "test.flag"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "defaultvalue") {
			t.Errorf("expected 'defaultvalue' in output, got: %s", ow.String())
		}
	})
	t.Run("set config key", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		cli.Option(withArgs([]string{"config", "set", "test.flag", "newvalue"}))
		ec := cli.Execute()
		if ec != 0 {
			t.Fatalf("expected exit code 0, got %d", ec)
		}
		if !strings.Contains(ow.String(), "test.flag: saved") {
			t.Errorf("expected 'test.flag: saved' in output, got: %s", ow.String())
		}

		// verify value was set in config file
		data, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}
		if !strings.Contains(string(data), "newvalue") {
			t.Errorf("expected 'newvalue' in config file, got: %s", string(data))
		}
	})
	t.Run("set config write failure", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpFile.Name()),
			WithConfigCreate(false),
			WithConfigCommands(),
		)
		cli.AddConf("test.secret", "secretvalue", "this is a secret value", true)
		cli.AddFlag("test-flag", "f", "defaultvalue", "this is a test flag", false)

		// Set read only permissions on the config file
		if err := os.Chmod(tmpFile.Name(), 0444); err != nil {
			t.Fatalf("failed to set file permissions: %v", err)
		}

		// Attempt to set a config key with an invalid file path
		cli.Option(withArgs([]string{"config", "set", "test.flag", "newervalue"}))
		ec := cli.Execute()
		if ec == 0 {
			t.Fatalf("expected non-zero exit code, got %d", ec)
		}
		if !strings.Contains(ew.String(), "failed to write config file") {
			t.Errorf("expected 'failed to write config file' in error output, got: %s", ew.String())
		}
	})
}
