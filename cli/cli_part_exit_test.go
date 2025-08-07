package cli

import (
	"bytes"
	"fmt"
	"testing"

	cmperr "github.com/gxmmx/compage-go/errors"
)

func exitWithAppError(cli CliContext) error {
	err := cmperr.NewInternal("testapperror", nil).WithExitCode(2)
	return err
}

func exitWithError(cli CliContext) error {
	err := fmt.Errorf("testnormalerror")
	return err
}

func exitWithNoError(cli CliContext) error {
	// No error, just return nil
	return nil
}

func TestCliExit(t *testing.T) {
	t.Run("exit no error", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)
		cli.AddCommand("cmd1", "no error", "Exit with no errir", exitWithNoError)

		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{"cmd1"}))
		exitCode := cli.Execute()
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
	})
	t.Run("exit with app error", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)
		cli.AddCommand("cmd1", "app error", "Exit with application error", exitWithAppError)

		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{"cmd1"}))
		exitCode := cli.Execute()
		if exitCode != 2 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
	})
	t.Run("exit with normal error", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)
		cli.AddCommand("cmd1", "normal error", "Exit with normal error", exitWithError)

		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{"cmd1"}))
		exitCode := cli.Execute()
		if exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
	})
	t.Run("exit with usage root command", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)
		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{}))
		exitCode := cli.Execute()
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
	})
	t.Run("exit with usage unknown command", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)

		cli.AddCommand("cmd1", "testcommand", "test command", exitWithNoError)

		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{"invalidcommmnd"}))
		exitCode := cli.Execute()
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
	})
	t.Run("exit with usage not runnable", func(t *testing.T) {
		ow := &bytes.Buffer{}
		ew := &bytes.Buffer{}
		cli := New(
			withOutWriter(ow),
			withErrWriter(ew),
		)
		cmd1 := cli.AddCommand("cmd1", "normal command", "has subcommand", nil)
		cmd1.AddCommand("subcmd1", "sub command", "Exit with sub command", exitWithNoError)

		clictrl, _ := cli.(*Controller)
		clictrl.root.cmd.SetOut(ow)
		clictrl.root.cmd.SetErr(ew)
		cli.Option(withArgs([]string{"cmd1"}))
		exitCode := cli.Execute()
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
	})
}
