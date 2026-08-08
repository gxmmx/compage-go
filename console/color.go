package console

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// Color represents a text color for printer output.
type Color int

const (
	Red Color = iota
	Green
	Yellow
	Blue
	Magenta
	Cyan
	Dim // gray/faint — used internally for Verbose, composable with other colors
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

func colorCode(c Color) string {
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

// colorize wraps text with a color code and reset. If multiple colors are provided
// they are all applied (e.g., dim + magenta).
func colorize(text string, colors ...Color) string {
	if len(colors) == 0 {
		return text
	}
	prefix := ""
	for _, c := range colors {
		prefix += colorCode(c)
	}
	return fmt.Sprintf("%s%s%s", prefix, text, ansiReset)
}

// resolveColor determines whether color output should be enabled.
// Priority: NO_COLOR (always wins) > explicit suppress > TTY auto-detect.
func resolveColor(suppressColor bool, w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if suppressColor {
		return false
	}
	if !isTerminal(w) {
		return false
	}
	return true
}

// isTerminal checks whether the given writer is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
