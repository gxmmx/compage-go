package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func tf0(cli CliContext) error {
	cli.GetLogger().Info("Successoutput")
	return nil
}

func tf1(cli CliContext) error {
	cli.GetLogger().Info("Successoutput")
	_ = cli.GetArgs()
	_ = cli.GetContext()
	_ = cli.GetConfig()
	_ = cli.SaveConfig("test.conf1", true)
	return nil
}

func tf2(cli CliContext) error {
	cli.GetLogger().Info("Failureoutput")
	return fmt.Errorf("Failureerror")
}

func tf3(cli CliContext) error {
	panic("testpanicinclifunc")
}

func TestNewCli(t *testing.T) {
	// Create a temporary file for testing config
	tmpDir := t.TempDir()
	tmpFile, _ := os.CreateTemp(tmpDir, "testconfig-*.yaml")
	os.Remove(tmpFile.Name()) // Remove before, to simulate non-existence
	defer os.Remove(tmpFile.Name())

	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}

	// Create a new CLI instance
	cli := New(
		SetName("compage-name"),
		SetVersion("1.2.3"),
		SetShort("compage-short"),
		SetLong("compage-long"),
		WithConfigEnvPrefix("test"),
		WithConfigFileName("test-config"),
		WithConfigFileDir(tmpDir),
		WithConfigFileType("yaml"),
		WithConfigFilePath(tmpFile.Name()),
		WithConfigCommands(),
		withOutWriter(ow),
		withErrWriter(ew),
	)
	cli.Option(WithConfigCreate(false))

	cli.AddConf("test.conf1", false, "test1", true)
	cli.AddConf("test.conf2", 2, "test2", false)
	cli.AddFlag("test.flag1", "f", "val3", "test flag 1", false)
	cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", nil)
	sub := cmd1.AddCommand("subcmd1", "sub1", "sub command 1", tf1)
	sub.AddFlag("test.flag2", "t", "val4", "test flag 2", false)
	sub.AddArg("arg1", true, false)
	cmd2 := cli.AddCommand("cmd2", "testcommand2", "test command 2", tf2)
	cmd2.AddArg("arg2", false, true)
	cmd3 := cli.AddCommand("cmd3", "testcommand3", "test command 3", tf0)
	cmd3.AddArg("arg3", true, false)
	cmd3.AddArg("arg4", false, false)
	cmd3.AddFlag("test.invalid", "", []string{}, "test invalid flag", false)
	cmd3.AddFlag("test.boolshort", "m", false, "test bool", false)
	cmd3.AddFlag("test.intshort", "n", 2, "test int", false)

	cli.Option(withArgs([]string{"--config", tmpFile.Name(), "cmd1", "subcmd1", "argval111"}))
	ec := cli.Execute()
	if !strings.Contains(ow.String(), "Successoutput") {
		t.Errorf("Expected 'Successoutput' in output, got: %s", ow.String())
	}
	if ec != 0 {
		t.Errorf("Expected exit code 0, got: %d", ec)
	}
}

func TestLoglevelOnCommandLine(t *testing.T) {
	t.Run("loglevel", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cli.AddCommand("cmd1", "testcommand", "test command", tf0)

		cli.Option(withArgs([]string{"--loglevel", "debug", "cmd1"}))
		cli.Execute()
	})
	t.Run("loglevel", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cli.AddCommand("cmd1", "testcommand", "test command", tf0)

		cli.Option(withArgs([]string{"--quiet", "cmd1"}))
		cli.Execute()
	})
	t.Run("loglevel", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cli.AddCommand("cmd1", "testcommand", "test command", tf0)

		cli.Option(withArgs([]string{"--verbose", "cmd1"}))
		cli.Execute()
	})
}

func TestCliPanic(t *testing.T) {
	t.Run("Panic in command", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		_ = cli.AddCommand("", "testcommand", "test command", tf2)
	})
	t.Run("Panic in flag long", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddFlag("", "f", "val3", "test flag 1", false)
	})
	t.Run("Panic in flag short", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddFlag("test.flag1", "ss", "val3", "test flag 1", false)
	})
	t.Run("Panic in arg name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddArg("", true, false)
	})
	t.Run("Panic in arg same name", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddArg("arg1", true, false)
		cmd1.AddArg("arg1", false, false)
	})
	t.Run("Panic in arg required after optional", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddArg("arg1", false, false)
		cmd1.AddArg("arg2", true, false)
	})
	t.Run("Panic in arg multi after multi", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but did not occur")
			}
		}()

		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		cmd1.AddArg("arg1", false, true)
		cmd1.AddArg("arg2", false, true)
	})
}

func TestParseErrors(t *testing.T) {
	t.Run("Parse error reading config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFilePath(tmpDir),
		)
		cmd1 := cli.AddCommand("cmd1", "testcommand", "test command", tf0)
		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cmd1ctrl, _ := cmd1.(*CommandController)
		cmd1ctrl.cmd.SetOut(ow)
		cmd1ctrl.cmd.SetErr(ew)

		cli.Option(withArgs([]string{"cmd1"}))
		cli.Execute()
		if !strings.Contains(ew.String(), "config-file") {
			t.Errorf("Expected 'config-file' error, got: %s", ew.String())
		}
	})

	t.Run("Parse error writing config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}

		// Create a new CLI instance
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
			WithConfigFileDir(tmpDir+"/nonexistentdir"),
			WithConfigCreate(true),
		)
		cli.AddCommand("cmd1", "testcommand", "test command", tf0)

		cli.Option(withArgs([]string{"cmd1"}))
		cli.Execute()
		if !strings.Contains(ew.String(), "config-file") {
			t.Errorf("Expected 'config-file' error, got: %s", ew.String())
		}
	})
}

func TestPanicRecovery(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}

	// Create a new CLI instance
	cli := New(
		withOutWriter(ow),
		withErrWriter(ew),
	)

	cli.AddCommand("cmd1", "testcommand", "test command", tf3)
	cli.Option(withArgs([]string{"cmd1"}))
	exitCode := cli.Execute()
	if exitCode != panicExitCode {
		t.Errorf("Expected exit code %d, got %d", panicExitCode, exitCode)
	}
}

func TestRootCmdFunc(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}

	// Create a new CLI instance
	cli := New(
		withOutWriter(ow),
		withErrWriter(ew),
	)

	cli.AddCommand("cmd1", "testcommand", "test command", tf0)

	clictrl, _ := cli.(*Controller)
	clictrl.root.cmd.SetOut(ow)
	clictrl.root.cmd.SetErr(ew)

	clictrl.SetRootCommand(tf1)
	cli.Option(withArgs([]string{}))
	exitCode := cli.Execute()
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(ow.String(), "Successoutput") {
		t.Errorf("Expected 'Successoutput' in output, got: %s", ow.String())
	}
}
