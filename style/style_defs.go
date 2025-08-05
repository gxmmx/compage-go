package style

import (
	"os"

	"golang.org/x/term"
)

var isTTY bool = term.IsTerminal(int(os.Stdin.Fd()))

// -----------------------------------------------------------------------------
// Color constants and enum
// -----------------------------------------------------------------------------

type attr int

const (
	reset attr = iota
	bold
	faint
	italic
	underline
)

const (
	black attr = iota + 30
	red
	green
	yellow
	blue
	magenta
	cyan
	white
)

const (
	blackb attr = iota + 90
	redb
	greenb
	yellowb
	blueb
	magentab
	cyanb
	whiteb
)

var attributeNames = map[string]attr{
	"reset":     reset,
	"bold":      bold,
	"faint":     faint,
	"italic":    italic,
	"underline": underline,
	"black":     black,
	"red":       red,
	"green":     green,
	"yellow":    yellow,
	"blue":      blue,
	"magenta":   magenta,
	"cyan":      cyan,
	"white":     white,
	"blackb":    blackb,
	"redb":      redb,
	"greenb":    greenb,
	"yellowb":   yellowb,
	"blueb":     blueb,
	"magentab":  magentab,
	"cyanb":     cyanb,
	"whiteb":    whiteb,
}
