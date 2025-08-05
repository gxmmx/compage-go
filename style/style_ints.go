package style

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Style interface {
	Color(c string) Style
	Attributes(a string) Style
	Bold() Style
	Faint() Style
	Italic() Style
	Underline() Style

	Apply(text string) string
	Code() string

	forceTTY() Style
}
