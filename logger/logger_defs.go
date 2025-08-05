package logger

import "os"

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

// IndentString is the string used for every level of indentation
var IndentString = "  "

// Default writer for output
var defaultOutWriter = os.Stdout

// Default writer for error output
var defaultErrWriter = os.Stderr
