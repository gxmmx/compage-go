package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestParse(t *testing.T) {
	t.Run("should parse with defaults passed", func(t *testing.T) {
		ctrl, err := Parse()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ctrl.GetLogLevel() != "info" {
			t.Errorf("expected log level 'info', got '%s'", ctrl.GetLogLevel())
		}
	})
	t.Run("should parse with log level set", func(t *testing.T) {
		ctrl, err := Parse(WithLogLevel("debug"), WithDir("./testdir"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ctrl.GetLogLevel() != "debug" {
			t.Errorf("expected log level 'debug', got '%s'", ctrl.GetLogLevel())
		}
	})
}

func TestConfigPaths(t *testing.T) {
	t.Run("should use custom config path", func(t *testing.T) {
		ctrl, err := Parse(WithPath("./nonexisting_config.yaml"))
		if ctrl.config.ConfigFileUsed() != "./nonexisting_config.yaml" {
			t.Errorf("expected config file './nonexisting_config.yaml', got '%s'", ctrl.config.ConfigFileUsed())
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("should use env config path", func(t *testing.T) {
		os.Setenv("TEST_CONFIG", "./nonexisting_config.yaml")
		ctrl, err := Parse(WithEnvPrefix("TEST"))
		if ctrl.config.ConfigFileUsed() != "./nonexisting_config.yaml" {
			t.Errorf("expected config file './nonexisting_config.yaml', got '%s'", ctrl.config.ConfigFileUsed())
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		os.Unsetenv("TEST_CONFIG")
	})
	t.Run("should fail invalid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "badconfig.yaml")
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(":\ninvalid: yaml:::")
		tmpFile.Close()
		ctrl, err := Parse(WithPath(tmpFile.Name()))
		if err == nil {
			t.Fatalf("expected error reading file, got nil")
		}
		if ctrl.config.ConfigFileUsed() != tmpFile.Name() {
			t.Errorf("expected config file '%s', got '%s'", tmpFile.Name(), ctrl.config.ConfigFileUsed())
		}
	})
}

func TestConfigWrite(t *testing.T) {
	t.Run("should write config to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testwriteconfig-*.yaml")
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString("---")
		tmpFile.Close()
		ctrl, err := Parse(WithPath(tmpFile.Name()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = ctrl.WriteConfig()
		if err != nil {
			t.Fatalf("unexpected error writing config: %v", err)
		}

		data, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(string(data), "log:") {
			t.Errorf("expected file to contain log line but it did not")
		}
	})
	t.Run("should fail writing config to invalid file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, _ := os.CreateTemp(tmpDir, "testwriteconfig-*.yaml")
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString("---")
		tmpFile.Close()

		// Make the file read-only
		if err := os.Chmod(tmpFile.Name(), 0444); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ctrl, err := Parse(WithPath(tmpFile.Name()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = ctrl.WriteConfig()
		if err == nil {
			t.Fatal("expected write error")
		}
	})
	t.Run("should use parsed config path to write config", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctrl, err := Parse(WithName("test_config"), WithDir(tmpDir), WithType("yaml"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = ctrl.WriteConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(tmpDir + "/test_config.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(string(data), "log:") {
			t.Errorf("expected file to contain log line but it did not")
		}
	})
}

func TestConfigBindFlags(t *testing.T) {
	t.Run("should bind flags correctly", func(t *testing.T) {
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-flag", "defaultflagval", "test flag")
		flags := map[string]*pflag.Flag{
			"test-flag": testCobraCommand.Flags().Lookup("test-flag"),
		}
		_, err := Parse(WithFlags(flags))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("should fail invalid bind flag", func(t *testing.T) {
		// testCobraCommand := &cobra.Command{}
		// testCobraCommand.Flags().String("test-flag", "defaultflagval", "test flag")
		flags := map[string]*pflag.Flag{
			"test-flag": nil,
		}
		_, err := Parse(WithFlags(flags))
		if err == nil {
			t.Fatal("Expected error binding nil flag")
		}
	})
}

func TestConfigValues(t *testing.T) {
	t.Run("should get correct values", func(t *testing.T) {
		testCobraCommand := &cobra.Command{}
		testCobraCommand.Flags().String("test-string", "ninenine", "test string")
		testCobraCommand.Flags().Int("test-int", 99, "test int")
		testCobraCommand.Flags().Bool("test-bool", true, "test bool")
		flags := map[string]*pflag.Flag{
			"test-string": testCobraCommand.Flags().Lookup("test-string"),
			"test-int":    testCobraCommand.Flags().Lookup("test-int"),
			"test-bool":   testCobraCommand.Flags().Lookup("test-bool"),
		}
		ctrl, err := Parse(WithFlags(flags))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ctrl.GetString("test-string") != "ninenine" {
			t.Errorf("expected string value 'ninenine', got '%s'", ctrl.GetString("test-string"))
		}
		if ctrl.GetInt("test-int") != 99 {
			t.Errorf("expected int value '99', got '%d'", ctrl.GetInt("test-int"))
		}
		if ctrl.GetBool("test-bool") != true {
			t.Errorf("expected bool value 'true', got '%t'", ctrl.GetBool("test-bool"))
		}
		raw := ctrl.GetRaw()
		if raw["test-string"] != "ninenine" {
			t.Errorf("expected raw value 'ninenine', got '%s'", raw["test-string"])
		}
		var logSub struct {
			Level string `mapstructure:"level"`
		}
		var testBase struct {
			TestString string `mapstructure:"test-string"`
		}

		err = ctrl.GetConfigStruct("log", &logSub)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logSub.Level != "info" {
			t.Errorf("expected log level 'info', got '%s'", logSub.Level)
		}
		err = ctrl.GetConfigStruct("", &testBase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if testBase.TestString != "ninenine" {
			t.Errorf("expected test string 'ninenine', got '%s'", testBase.TestString)
		}
		err = ctrl.GetConfigStruct("nonexistingsub", &testBase)
		if err == nil {
			t.Fatalf("expected error getting config struct")
		}
		err = ctrl.GetConfigStruct("log", nil)
		if err == nil {
			t.Fatalf("expected error unmashalling config")
		}
	})
	t.Run("should set correct value", func(t *testing.T) {
		ctrl, err := Parse()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ctrl.GetLogLevel() != "info" {
			t.Errorf("expected log level 'info', got '%s'", ctrl.GetLogLevel())
		}
		ctrl.Set("log.level", "debug")
		if ctrl.GetLogLevel() != "debug" {
			t.Errorf("expected log level 'debug', got '%s'", ctrl.GetLogLevel())
		}
	})
}
