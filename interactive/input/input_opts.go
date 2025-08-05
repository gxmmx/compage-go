package input

import (
	"bufio"
	"io"
)

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

type Option func(*Controller)

// -----------------------------------------------------------------------------
// Options
// -----------------------------------------------------------------------------

// Sets the indentation level
func WithIndent(i int) Option {
	return func(ctl *Controller) {
		ctl.indent = i
	}
}

// Sets prompt color
func WithPromptColor(c string) Option {
	return func(ctl *Controller) {
		ctl.pColor = c
	}
}

// Sets output writer (stdout by default)
func WithWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.writer = w
	}
}

// Sets input reader (stdin by default)
func WithReader(r io.Reader) Option {
	return func(ctl *Controller) {
		ctl.reader = r
		ctl.bufReader = bufio.NewReader(r)
	}
}

// Sets a validation function and message
func WithValidation(msg string, f Validator) Option {
	return func(ctl *Controller) {
		ctl.validationFunc = f
		ctl.validationMsg = msg
	}
}

// Sets a default value for the prompt
func WithDefault(val string) Option {
	return func(ctl *Controller) {
		ctl.defaultValue = val
	}
}

// Sets the input to be secret (password input)
func WithSecret() Option {
	return func(ctl *Controller) {
		ctl.secret = true
	}
}

// -----------------------------------------------------------------------------
// Testing overrides
// -----------------------------------------------------------------------------

// Forces the input to behave as if it is a TTY (terminal)
func forceTTY() Option {
	return func(ctl *Controller) {
		ctl.isTTY = true
	}
}
