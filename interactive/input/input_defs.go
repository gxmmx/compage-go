package input

import (
	"os"

	"golang.org/x/term"
)

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

// Default writer for output
var defaultWriter = os.Stdout

// Default reader for input
var defaultReader = os.Stdin

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// Validator function
type Validator func(input string) bool

// -----------------------------------------------------------------------------
// Variables
// -----------------------------------------------------------------------------

// Testing override
var readPasswordFn = term.ReadPassword
