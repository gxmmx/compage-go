package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestGetRedacted(t *testing.T) {
	t.Run("GetRedactedValue", func(t *testing.T) {
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-flag1", "visible", "test flag 1")
		testCobraCommand.Flags().String("test-flag2", "hidden", "test flag 2")
		flag1 := testCobraCommand.Flags().Lookup("test-flag1")
		flag2 := testCobraCommand.Flags().Lookup("test-flag2")
		cnf := New(
			WithFlag(flag1, false),
			WithFlag(flag2, true),
		)
		config := cnf.Get()
		if config == nil {
			t.Fatal("expected non-nil config")
		}
		cnfctl := cnf.(*Controller)
		val1, _ := cnfctl.GetRedactedValue("test.flag1")
		val2, _ := cnfctl.GetRedactedValue("test.flag2")
		val3, _ := cnfctl.GetRedactedValue("test.flag3") // Non-existent key

		if val1 != "visible" {
			t.Errorf("expected 'visible', got '%s'", val1)
		}
		if val2 != "<redacted>" {
			t.Errorf("expected '<redacted>', got '%s'", val2)
		}
		if val3 != "" {
			t.Errorf("expected '', got '%s'", val3)
		}
	})
	t.Run("GetRedactedMap", func(t *testing.T) {
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-flag1", "visible", "test flag 1")
		testCobraCommand.Flags().String("test-flag2", "hidden", "test flag 2")
		flag1 := testCobraCommand.Flags().Lookup("test-flag1")
		flag2 := testCobraCommand.Flags().Lookup("test-flag2")
		cnf := New(
			WithFlag(flag1, false),
			WithFlag(flag2, true),
		)
		config := cnf.Get()
		if config == nil {
			t.Fatal("expected non-nil config")
		}
		cnfctl := cnf.(*Controller)
		resultMap := cnfctl.GetRedactedMap()

		expectedMap := map[string]interface{}{
			"test": map[string]interface{}{
				"flag1": "visible",
				"flag2": "<redacted>",
			},
		}

		if !reflect.DeepEqual(resultMap, expectedMap) {
			t.Errorf("expected %v, got %v", expectedMap, resultMap)
		}
	})
}

func TestGetEnvPrefix(t *testing.T) {
	t.Run("GetEnvPrefix", func(t *testing.T) {
		cnf := New(
			WithEnvPrefix("TEST"),
		)
		cnfctl := cnf.(*Controller)
		prefix := cnfctl.GetEnvPrefix()
		if prefix != "TEST" {
			t.Errorf("expected 'TEST', got '%s'", prefix)
		}
	})
}

func TestSaveConfig(t *testing.T) {
	// Create a temporary file for testing config
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
	os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
	defer os.Remove(tmpFile.Name())

	testCobraCommand := &cobra.Command{}
	testCobraCommand.Flags().String("test-flag1", "value1", "test flag 1")
	testCobraCommand.Flags().String("test-flag2", "value2", "test flag 2")
	flag1 := testCobraCommand.Flags().Lookup("test-flag1")
	flag2 := testCobraCommand.Flags().Lookup("test-flag2")

	// Create a config controller
	cnf := New(
		WithFilePath(tmpFile.Name()),
		WithCreateConfig(false),
		WithFlag(flag1, false),
		WithFlag(flag2, false),
	)
	// Try to save a non-existent key
	if err := cnf.Save("test.flag3", "value"); err == nil {
		t.Fatal("expected error when saving non-existent key")
	}
	// Save a valid key
	if err := cnf.Save("test.flag1", "value3"); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Get the config again to verify
	config := cnf.Get()
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.GetString("test.flag1") != "value3" {
		t.Errorf("expected 'value3', got '%s'", config.GetString("test.flag1"))
	}

	// Check if config file exists
	if _, err := os.Stat(tmpFile.Name()); os.IsNotExist(err) {
		t.Fatalf("expected config file to be created at %s", tmpFile.Name())
	}
	// Read the config file to verify the saved value
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if !strings.Contains(string(data), "value3") {
		t.Errorf("expected config file to contain 'value3', got '%s'", string(data))
	}
	if !strings.Contains(string(data), "value2") {
		t.Errorf("expected config file to contain 'value2', got '%s'", string(data))
	}
	// Verify that the value is updated in memory
	if config.GetString("test.flag1") != "value3" {
		t.Errorf("expected in-memory config to be updated to 'value3', got '%s'", config.GetString("test.flag1"))
	}

	// Set invalid permissions to test error handling
	if err := os.Chmod(tmpFile.Name(), 0444); err != nil {
		t.Fatalf("failed to set permissions on config file: %v", err)
	}
	// Try to save again, expecting an error due to permissions
	if err := cnf.Save("test.flag1", "value4"); err == nil {
		t.Fatal("expected error when saving config with invalid permissions")
	}
}
