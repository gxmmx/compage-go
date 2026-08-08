package console

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

// NewPrompter creates a Prompter with the given options.
func NewPrompter(opts ...PrompterOption) Prompter {
	cfg := &prompterConfig{}
	cfg.level = 0
	for _, opt := range opts {
		opt(cfg)
	}

	printerOpts := []PrinterOption{}
	if cfg.outWriter != nil {
		printerOpts = append(printerOpts, WithOutTo(cfg.outWriter))
	}
	if cfg.errWriter != nil {
		printerOpts = append(printerOpts, WithErrTo(cfg.errWriter))
	}
	if cfg.level != 0 {
		printerOpts = append(printerOpts, WithLevel(cfg.level))
	}
	if cfg.suppressColor {
		printerOpts = append(printerOpts, WithColor(false))
	}

	p := NewPrinter(printerOpts...)

	input := cfg.input
	if input == nil {
		input = os.Stdin
	}

	outW := resolveOutWriter(&cfg.printerConfig)

	return &prompter{
		Printer: p,
		input:   input,
		outW:    outW,
		color:   style.Enabled(cfg.suppressColor, outW),
	}
}

type prompter struct {
	Printer
	input io.Reader
	outW  io.Writer
	color bool
}

// Prompt prints msg and reads a line of input. If the user provides empty input,
// fallback is returned. The fallback value is shown in brackets if non-empty.
func (pr *prompter) Prompt(msg string, fallback string) string {
	if fallback != "" {
		hint := fallback
		if pr.color {
			hint = style.Apply(hint, style.NoColor, style.Dim)
		}
		_, _ = fmt.Fprintf(pr.outW, "%s [%s]: ", msg, hint)
	} else {
		_, _ = fmt.Fprintf(pr.outW, "%s: ", msg)
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
	_, _ = fmt.Fprintf(pr.outW, "%s [y/N]: ", msg)
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

func resolveOutWriter(cfg *printerConfig) io.Writer {
	if cfg.outWriter != nil {
		return cfg.outWriter
	}
	return os.Stdout
}
