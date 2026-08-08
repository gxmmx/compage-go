package term

import (
	"io"
	"os"

	"golang.org/x/term"
)

// IsTerminal reports whether w is connected to a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
