package input

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Input interface {
	// Prompt creates a prompt with the given message and returns the input string.
	Prompt(msg string) (string, error)
	// PromptMulti creates a prompt for multiple lines of input.
	PromptMulti(msg string) ([]string, error)
	// PromptUntil creates a prompt that continues until valid input is provided.
	PromptUntil(msg string) (string, error)
	// Confirm creates a confirmation prompt with the given message.
	Confirm(msg string) (bool, error)
}
