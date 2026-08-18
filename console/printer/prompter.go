package printer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/style"
	"github.com/gxmmx/compage-go/term"
)

// Prompter extends Printer with interactive input capabilities.
type Prompter interface {
	Printer
	Prompt(msg string, fallback string) string
	Continue(msg string) bool
}

// NewPrompter creates a Prompter with the given options. It accepts the same
// options as New, validates that its input is interactive, and returns an
// unavailable error when it is not. A Prompter is a Printer that can also read
// interactive input.
func NewPrompter(opts ...Option) (Prompter, error) {
	cfg := newConfig(opts...)
	input := cfg.inputReader
	if input == nil {
		input = os.Stdin
	}
	check := cfg.terminalCheck
	if check == nil {
		check = isTerminalInput
	}
	if !check(input) {
		return nil, errx.New(
			"printer: interactive input unavailable: stdin is not a terminal; supply the required value with a flag or configuration",
			errx.WithKind(errx.Unavailable),
		)
	}

	return &prompter{
		printer: newPrinter(cfg),
		input:   input,
	}, nil
}

func isTerminalInput(input io.Reader) bool {
	stream, ok := input.(io.Writer)
	return ok && term.IsTerminal(stream)
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
