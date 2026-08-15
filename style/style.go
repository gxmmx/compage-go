package style

import (
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
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	BrightWhite
)

// Mod represents a text modifier.
type Mod int

const (
	Bold Mod = iota + 1
	Dim
	Italic
	Underline
)

const (
	ansiReset         = "\033[0m"
	ansiBold          = "\033[1m"
	ansiDim           = "\033[2m"
	ansiItalic        = "\033[3m"
	ansiUnderline     = "\033[4m"
	ansiRed           = "\033[31m"
	ansiGreen         = "\033[32m"
	ansiYellow        = "\033[33m"
	ansiBlue          = "\033[34m"
	ansiMagenta       = "\033[35m"
	ansiCyan          = "\033[36m"
	ansiWhite         = "\033[37m"
	ansiBrightRed     = "\033[91m"
	ansiBrightGreen   = "\033[92m"
	ansiBrightYellow  = "\033[93m"
	ansiBrightBlue    = "\033[94m"
	ansiBrightMagenta = "\033[95m"
	ansiBrightCyan    = "\033[96m"
	ansiBrightWhite   = "\033[97m"
)

// Styler gates style application based on an init-time check.
// The zero value is inactive (returns text unchanged).
type Styler struct {
	active bool
}

// New creates a Styler for the given writer.
// Inactive when: NO_COLOR env is set, suppress is true, or w is not a terminal.
// Pass nil as w to force active (useful in tests or when codes are unconditionally wanted).
func New(suppress bool, w io.Writer) Styler {
	if os.Getenv("NO_COLOR") != "" {
		return Styler{}
	}
	if suppress {
		return Styler{}
	}
	if w == nil {
		return Styler{active: true}
	}
	return Styler{active: term.IsTerminal(w)}
}

// Apply wraps text with color and mods if active.
// Returns text unchanged when inactive, or when c is NoColor with no mods.
func (s Styler) Apply(text string, c Color, mods ...Mod) string {
	if !s.active {
		return text
	}
	return render(text, c, mods...)
}

// Build pre-computes a reusable Style.
// An inactive Styler produces a no-op Style.
func (s Styler) Build(c Color, mods ...Mod) Style {
	if !s.active {
		return Style{}
	}
	return Style{prefix: buildPrefix(c, mods...)}
}

// Active reports whether this Styler applies styles.
func (s Styler) Active() bool { return s.active }

// Style is a pre-computed, reusable style. The zero value is a no-op.
type Style struct {
	prefix string
}

// Apply wraps text with the pre-computed ANSI codes.
// Returns text unchanged when the Style is empty.
func (s Style) Apply(text string) string {
	if s.prefix == "" {
		return text
	}
	return s.prefix + text + ansiReset
}

// Raw wraps text with color and mods unconditionally.
// No Styler needed. Use when codes are required regardless of terminal state.
func Raw(text string, c Color, mods ...Mod) string {
	return render(text, c, mods...)
}

func render(text string, c Color, mods ...Mod) string {
	p := buildPrefix(c, mods...)
	if p == "" {
		return text
	}
	return p + text + ansiReset
}

func buildPrefix(c Color, mods ...Mod) string {
	prefix := ""
	for _, m := range mods {
		prefix += modCode(m)
	}
	prefix += colorCode(c)
	return prefix
}

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
	case BrightRed:
		return ansiBrightRed
	case BrightGreen:
		return ansiBrightGreen
	case BrightYellow:
		return ansiBrightYellow
	case BrightBlue:
		return ansiBrightBlue
	case BrightMagenta:
		return ansiBrightMagenta
	case BrightCyan:
		return ansiBrightCyan
	case BrightWhite:
		return ansiBrightWhite
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
