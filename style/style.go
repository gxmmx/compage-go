package style

import (
	"fmt"
	"io"
	"os"

	"github.com/gxmmx/compage-go/term"
)

// Color represents a foreground text color.
type Color int

const (
	NoColor Color = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Mod represents a text style modifier.
type Mod int

const (
	Bold      Mod = iota + 1
	Dim
	Italic
	Underline
)

const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiItalic    = "\033[3m"
	ansiUnderline = "\033[4m"
	ansiRed       = "\033[31m"
	ansiGreen     = "\033[32m"
	ansiYellow    = "\033[33m"
	ansiBlue      = "\033[34m"
	ansiMagenta   = "\033[35m"
	ansiCyan      = "\033[36m"
	ansiWhite     = "\033[37m"
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
	case White:
		return ansiWhite
	default:
		return ""
	}
}

func modCode(m Mod) string {
	switch m {
	case Bold:
		return ansiBold
	case Dim:
		return ansiDim
	case Italic:
		return ansiItalic
	case Underline:
		return ansiUnderline
	default:
		return ""
	}
}

// Style is a reusable combination of color and modifiers.
type Style struct {
	color Color
	mods  []Mod
}

// Build creates a reusable Style from a color and modifiers.
func Build(c Color, mods ...Mod) Style {
	return Style{color: c, mods: mods}
}

// Apply renders text wrapped with the style's ANSI escape codes.
func (s Style) Apply(text string) string {
	prefix := ""
	for _, m := range s.mods {
		prefix += modCode(m)
	}
	prefix += colorCode(s.color)

	if prefix == "" {
		return text
	}
	return fmt.Sprintf("%s%s%s", prefix, text, ansiReset)
}

// Apply is a one-off convenience: wraps text with the given color and modifiers.
// For repeated use of the same combination, prefer Build().
func Apply(text string, c Color, mods ...Mod) string {
	return Build(c, mods...).Apply(text)
}

// Enabled determines whether styled output should be active for the given writer.
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
