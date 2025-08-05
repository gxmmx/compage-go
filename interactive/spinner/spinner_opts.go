package spinner

import (
	"io"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------
// Option
// -----------------------------------------------------------------------------

type Option func(*Controller)

// -----------------------------------------------------------------------------
// Options
// -----------------------------------------------------------------------------

// Sets the delay between spinner updates (shorter is faster, default is 80ms)
func WithDelay(delay time.Duration) Option {
	return func(ctl *Controller) {
		ctl.delay = delay
	}
}

// Sets the indentation level
func WithIndent(i int) Option {
	return func(ctl *Controller) {
		ctl.indent = i
	}
}

// Sets the color for the spinner itself
func WithSpinnerColor(color string) Option {
	return func(ctl *Controller) {
		ctl.sColor = color
	}
}

// Sets the message color
func WithMessageColor(c string) Option {
	return func(ctl *Controller) {
		ctl.mColor = c
	}
}

// Sets the output writer (stdout by default)
func WithWriter(w io.Writer) Option {
	return func(ctl *Controller) {
		ctl.writer = w
	}
}

// Pass an external mutex for synchronization
func WithMutex(m *sync.Mutex) Option {
	return func(ctl *Controller) {
		ctl.mutex = m
	}
}

// -----------------------------------------------------------------------------
// Internal testing overrides
// -----------------------------------------------------------------------------

// Forces the input to behave as if it is a TTY (terminal)
func forceTTY() Option {
	return func(ctl *Controller) {
		ctl.isTTY = true
	}
}
