package style

import (
	"strings"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

type Controller struct {
	isTTY bool
	attrs []attr
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New() Style {
	return &Controller{
		isTTY: isTTY,
		attrs: []attr{},
	}
}

func Apply(text string, style string) string {
	ctl := New().Attributes(style)
	return ctl.Apply(text)
}

func Color(c string) Style { return New().Color(c) }

func Bold() Style { return New().Bold() }

func Faint() Style { return New().Faint() }

func Italic() Style { return New().Italic() }

func Underline() Style { return New().Underline() }

func Black() Style { return New().Color("black") }

func Red() Style { return New().Color("red") }

func Green() Style { return New().Color("green") }

func Yellow() Style { return New().Color("yellow") }

func Blue() Style { return New().Color("blue") }

func Magenta() Style { return New().Color("magenta") }

func Cyan() Style { return New().Color("cyan") }

func White() Style { return New().Color("white") }

func BlackB() Style { return New().Color("blackb") }

func RedB() Style { return New().Color("redb") }

func GreenB() Style { return New().Color("greenb") }

func YellowB() Style { return New().Color("yellowb") }

func BlueB() Style { return New().Color("blueb") }

func MagentaB() Style { return New().Color("magentab") }

func CyanB() Style { return New().Color("cyanb") }

func WhiteB() Style { return New().Color("whiteb") }

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

func (ctl *Controller) Color(c string) Style {
	ctl.attrs = append(ctl.attrs, colorFromString(c))
	return ctl
}

func (ctl *Controller) Attributes(a string) Style {
	ctl.attrs = append(ctl.attrs, attrsFromString(a)...)
	return ctl
}

func (ctl *Controller) Bold() Style {
	ctl.attrs = append(ctl.attrs, bold)
	return ctl
}

func (ctl *Controller) Faint() Style {
	ctl.attrs = append(ctl.attrs, faint)
	return ctl
}

func (ctl *Controller) Italic() Style {
	ctl.attrs = append(ctl.attrs, italic)
	return ctl
}

func (ctl *Controller) Underline() Style {
	ctl.attrs = append(ctl.attrs, underline)
	return ctl
}

func (ctl *Controller) Apply(text string) string {
	if !ctl.isTTY {
		return text
	}

	var codes []string
	for _, a := range ctl.attrs {
		codes = append(codes, genCode(a))
	}
	codes = append(codes, text, genCode(reset))

	return strings.Join(codes, "")
}

func (ctl *Controller) Code() string {
	if !ctl.isTTY {
		return ""
	}

	var codes []string
	for _, a := range ctl.attrs {
		codes = append(codes, genCode(a))
	}
	return strings.Join(codes, "")
}

// -----------------------------------------------------------------------------
// Testing overrides
// -----------------------------------------------------------------------------

func (ctl *Controller) forceTTY() Style {
	ctl.isTTY = true
	return ctl
}
