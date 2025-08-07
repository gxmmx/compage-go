package cli

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

// Set the default exit code for a panic
const panicExitCode = 70

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type CliCommandFunc func(cli CliContext) error

type cliArg struct {
	name     string
	required bool
	multi    bool
}
