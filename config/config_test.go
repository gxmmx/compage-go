package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestNewConfig(t *testing.T) {
	pflag1 := &pflag.Flag{Name: "flag1", Usage: "Test flag 1"}
	pflag2 := &pflag.Flag{Name: "flag2", Usage: "Test flag 2"}
	// Create config controller with various options
	cnf := New(
		WithFileName("test"),
		WithFileDir("/tmp"),
		WithFileType("json"),
		WithFilePath("/tmp/test.json"),
		WithEnvPrefix("TEST"),
		WithCreateConfig(true),
		WithCreateConfig(false),
	)
	// Add options lazily
	cnf.Option(WithFlag(pflag1, false))
	cnf.Option(WithFlag(pflag2, true))

	if cnf == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestConfigPathsParse(t *testing.T) {
	t.Run("ParseConfigPath", func(t *testing.T) {
		// Create a temporary file for testing config
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
		defer os.Remove(tmpFile.Name())

		// Create a config flag to set config value
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-flag", "defaultflagval", "test flag")
		flag := testCobraCommand.Flags().Lookup("test-flag")

		// Using FileDir and FileName
		cnf1 := New(
			WithFileDir(tmpDir),
			WithFileName(filepath.Base(tmpFile.Name())),
		)
		config1 := cnf1.Get()
		if config1 == nil {
			t.Fatal("expected non-nil config")
		}

		// using FilePath
		cnf2 := New(
			WithFilePath(tmpFile.Name()),
		)
		cnf2.Option(WithFlag(flag, false))
		config2 := cnf2.Get()
		if config2 == nil {
			t.Fatal("expected non-nil config")
		}

		// Using env
		cnf3 := New(
			WithEnvPrefix("TEST"),
		)
		os.Setenv("TEST_CONFIG", tmpFile.Name())
		config3 := cnf3.Get()
		if config3 == nil {
			t.Fatal("expected non-nil config")
		}

		// t.Log(fmt.Sprintf("Config: %s", config.AllSettings()))
	})
}

func TestConfigFlags(t *testing.T) {
	t.Run("InvalidConfigFlag", func(t *testing.T) {
		// Expect panic due to nil flag
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic, got none")
			}
		}()

		// Create a config controller
		cnf := New()

		// Attempt to add an invalid nil flag
		var flag *pflag.Flag = nil
		cnf.Option(WithFlag(flag, false))
	})
	t.Run("ValidConfigFlags", func(t *testing.T) {

		// Create a temporary file for testing config
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
		defer os.Remove(tmpFile.Name())

		// Create a config flag to set config value
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-flag1", "defaultflagval1", "test flag 1")
		testCobraCommand.Flags().String("test-flag2", "defaultflagval2", "test flag 2")
		flag1 := testCobraCommand.Flags().Lookup("test-flag1")
		flag2 := testCobraCommand.Flags().Lookup("test-flag2")

		// Want
		want := map[string]interface{}{
			"test": map[string]interface{}{
				"flag1": "defaultflagval1",
				"flag2": "defaultflagval2",
			}}

		// using FilePath
		cnf := New(
			WithFilePath(tmpFile.Name()),
			WithCreateConfig(false),
		)
		cnf.Option(WithFlag(flag1, false))
		cnf.Option(WithFlag(flag2, true))
		config := cnf.Get()
		if config == nil {
			t.Fatal("expected non-nil config")
		}
		if !reflect.DeepEqual(config.AllSettings(), want) {
			t.Errorf("maps are not equal\ngot:  %#v\nwant: %#v", config.AllSettings(), want)
		}

		// Check if config file exists
		if _, err := os.Stat(tmpFile.Name()); os.IsNotExist(err) {
			t.Fatalf("expected config file to be created at %s", tmpFile.Name())
		}
	})
}

func TestReadConfig(t *testing.T) {
	t.Run("CreateConfig", func(t *testing.T) {
		// Create a temporary file for testing config
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
		os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
		defer os.Remove(tmpFile.Name())

		// Create a config controller
		cnf := New(
			WithFilePath(tmpFile.Name()),
			WithCreateConfig(true),
		)

		config := cnf.Get()
		if config == nil {
			t.Fatal("expected non-nil config")
		}

		// Check if config file exists
		if _, err := os.Stat(tmpFile.Name()); os.IsNotExist(err) {
			t.Fatalf("expected config file to be created at %s", tmpFile.Name())
		}
		// Check config file permissions
		info, err := os.Stat(tmpFile.Name())
		if err != nil {
			t.Fatalf("failed to stat config file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected config file permissions to be 0600, got %v", info.Mode().Perm())
		}
		// Parse again to ensure created config is read correctly
		cnf2 := New(
			WithFilePath(tmpFile.Name()),
			WithCreateConfig(true),
		)
		config2 := cnf2.Get()
		if config2 == nil {
			t.Fatal("expected non-nil config after re-reading")
		}
	})
	t.Run("ReadErrors", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a config controller with invalid path
		cnf := New(
			WithFilePath(tmpDir),
		)

		_ = cnf.Get()
		if errs := cnf.GetParseErrors(); len(errs) == 0 {
			t.Fatal("expected parse errors due to non-readable config")
		}
	})
	t.Run("CreateErrors", func(t *testing.T) {
		// Create a temporary file for testing config
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
		os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
		defer os.Remove(tmpFile.Name())

		// Create a config controller with invalid permissions
		cnf := New(
			WithFileDir(tmpDir+"/nonexistent"), // Non-existent directory
			WithCreateConfig(false),
		)

		_ = cnf.Get()
		if errs := cnf.GetParseErrors(); len(errs) == 0 {
			t.Fatal("expected parse errors due to non-readable config")
		}
	})
	t.Run("SetPermissionErrors", func(t *testing.T) {
		// Overwrite the os.Chmod function to simulate permission errors
		osChmodFunc = func(path string, mode os.FileMode) error {
			return &os.PathError{Op: "chmod", Path: path, Err: os.ErrPermission}
		}
		defer func() { osChmodFunc = os.Chmod }()

		// Create a temporary file for testing config
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
		os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
		defer os.Remove(tmpFile.Name())

		// Create a config controller with strict permissions
		cnf := New(
			WithFilePath(tmpFile.Name()),
			WithCreateConfig(false),
		)

		_ = cnf.Get()
		if errs := cnf.GetParseErrors(); len(errs) == 0 {
			t.Fatal("expected parse errors due to non-readable config")
		}
	})
}
