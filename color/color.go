package color

import (
	"fmt"
	"io"
	"os"

	"github.com/gxmmx/compage-go/term"
)

// Color represents a text color for terminal output.
type Color int

const (
	None    Color = iota // zero value — produces no color output
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	Dim // gray/faint — composable with other colors
)

const (
	ansiReset   = "\033[0m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiDim     = "\033[2m"
)

func code(c Color) string {
	switch c {
	case Red:
		return ansiRed
	case Green:
		return ansiGreen
	case Yellow:
		return ansiYellow
	case Blue:
		return ansiBlue
	case Magenta:
		return ansiMagenta
	case Cyan:
		return ansiCyan
	case Dim:
		return ansiDim
	default:
		return ""
	}
}

// Apply wraps text with ANSI color codes and a reset suffix. Multiple colors
// compose (e.g., Dim + Magenta). Returns text unchanged if no colors are
// provided or all colors are None.
func Apply(text string, colors ...Color) string {
	if len(colors) == 0 {
		return text
	}
	prefix := ""
	for _, c := range colors {
		prefix += code(c)
	}
	if prefix == "" {
		return text
	}
	return fmt.Sprintf("%s%s%s", prefix, text, ansiReset)
}

// Enabled determines whether color output should be active for the given writer.
// Priority: NO_COLOR env var (always wins) > explicit suppress > TTY auto-detect.
func Enabled(suppress bool, w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if suppress {
		return false
	}
	if !term.IsTerminal(w) {
		return false
	}
	return true
}
