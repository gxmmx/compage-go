package printer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gxmmx/compage-go/style"
)

// Prompter extends Printer with interactive input capabilities.
type Prompter interface {
	Printer
	Prompt(msg string, fallback string) string
	Continue(msg string) bool
}

// NewPrompter creates a Prompter with the given options. It accepts the same
// options as New; a Prompter is a Printer that can also read interactive input.
//
// Input always comes from os.Stdin. Prompting is only appropriate when the
// process is attached to a terminal — callers should verify that (e.g. a TTY
// check) before using a Prompter, and otherwise fall back to a non-interactive
// path rather than prompting into a pipe.
func NewPrompter(opts ...Option) Prompter {
	p := newPrinter(opts...)

	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	input := cfg.input
	if input == nil {
		input = os.Stdin
	}

	return &prompter{
		printer: p,
		input:   input,
	}
}

type prompter struct {
	*printer
	input io.Reader
}

// Prompt prints msg and reads a line of input. If the user provides empty input,
// fallback is returned. The fallback value is shown in brackets if non-empty.
func (pr *prompter) Prompt(msg string, fallback string) string {
	if fallback != "" {
		hint := pr.outStyle.Apply(fallback, style.NoColor, style.Dim)
		pr.writeRaw(pr.outW, fmt.Sprintf("%s [%s]: ", msg, hint))
	} else {
		pr.writeRaw(pr.outW, fmt.Sprintf("%s: ", msg))
	}

	line := pr.readLine()
	if line == "" {
		return fallback
	}
	return line
}

// Continue prints msg with a [y/N] prompt and returns true if the user enters
// "y" or "yes" (case-insensitive). Default is No (empty input returns false).
func (pr *prompter) Continue(msg string) bool {
	pr.writeRaw(pr.outW, fmt.Sprintf("%s [y/N]: ", msg))
	line := strings.ToLower(pr.readLine())
	return line == "y" || line == "yes"
}

func (pr *prompter) readLine() string {
	scanner := bufio.NewScanner(pr.input)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
