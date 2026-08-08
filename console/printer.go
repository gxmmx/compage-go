package console

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gxmmx/compage-go/style"
)

// Printer provides structured, level-gated CLI output with semantic markers,
// color support, indent nesting, and text color overrides.
type Printer interface {
	Info(msg string, args ...any)
	Success(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Verbose(msg string, args ...any)

	Print(msg string, args ...any)
	Printf(format string, args ...any)
	Println(args ...any)

	Table(headers []string, rows [][]string)

	WithIndent(n int) Printer
	WithTextColor(c style.Color) Printer

	SetLevel(lvl slog.Level)
	Slog() *slog.Logger
}

// NewPrinter creates a Printer with the given options.
func NewPrinter(opts ...PrinterOption) Printer {
	cfg := &printerConfig{
		level: slog.LevelInfo,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	outW := cfg.outWriter
	if outW == nil {
		outW = os.Stdout
	}

	errW := cfg.errWriter
	if errW == nil {
		errW = outW
	}

	colorOn := style.Enabled(cfg.suppressColor, outW)

	lvl := &slog.LevelVar{}
	lvl.Set(cfg.level)

	return &printer{
		outW:      outW,
		errW:      errW,
		level:     lvl,
		color:     colorOn,
		indent:    0,
		textColor: nil,
	}
}

type printer struct {
	outW      io.Writer
	errW      io.Writer
	level     *slog.LevelVar
	color     bool
	indent    int
	textColor *style.Color
}

func (p *printer) Info(msg string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(msg, args...)
	p.writeLine(p.outW, "", text)
}

func (p *printer) Success(msg string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := "✓"
	if p.color {
		marker = style.Apply(marker, style.Green)
	}
	p.writeLine(p.outW, marker, text)
}

func (p *printer) Warn(msg string, args ...any) {
	if p.level.Level() > slog.LevelWarn {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := "!"
	if p.color {
		marker = style.Apply(marker, style.Yellow)
	}
	p.writeLine(p.errW, marker, text)
}

func (p *printer) Error(msg string, args ...any) {
	if p.level.Level() > slog.LevelError {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := "✗"
	if p.color {
		marker = style.Apply(marker, style.Red)
	}
	p.writeLine(p.errW, marker, text)
}

func (p *printer) Verbose(msg string, args ...any) {
	if p.level.Level() > slog.LevelDebug {
		return
	}
	text := fmt.Sprintf(msg, args...)
	if p.color {
		if p.textColor != nil {
			text = style.Apply(text, *p.textColor, style.Dim)
		} else {
			text = style.Apply(text, style.NoColor, style.Dim)
		}
	}
	p.writeRaw(p.outW, p.indentPrefix()+text+"\n")
}

func (p *printer) Print(msg string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(msg, args...)
	p.writeRaw(p.outW, p.indentPrefix()+text+"\n")
}

func (p *printer) Printf(format string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(format, args...)
	p.writeRaw(p.outW, p.indentPrefix()+text)
}

func (p *printer) Println(args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintln(args...)
	p.writeRaw(p.outW, p.indentPrefix()+text)
}

func (p *printer) Table(headers []string, rows [][]string) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	renderTable(p.outW, headers, rows)
}

func (p *printer) SetLevel(lvl slog.Level) {
	p.level.Set(lvl)
}

func (p *printer) WithIndent(n int) Printer {
	return &printer{
		outW:      p.outW,
		errW:      p.errW,
		level:     p.level,
		color:     p.color,
		indent:    p.indent + n,
		textColor: p.textColor,
	}
}

func (p *printer) WithTextColor(c style.Color) Printer {
	return &printer{
		outW:      p.outW,
		errW:      p.errW,
		level:     p.level,
		color:     p.color,
		indent:    p.indent,
		textColor: &c,
	}
}

func (p *printer) indentPrefix() string {
	if p.indent <= 0 {
		return ""
	}
	prefix := ""
	for i := 0; i < p.indent; i++ {
		prefix += "  "
	}
	return prefix
}

func (p *printer) writeLine(w io.Writer, marker string, text string) {
	if p.color && p.textColor != nil {
		text = style.Apply(text, *p.textColor)
	}

	indent := p.indentPrefix()
	if marker == "" {
		_, _ = fmt.Fprintf(w, "%s%s\n", indent, text)
	} else {
		_, _ = fmt.Fprintf(w, "%s%s %s\n", indent, marker, text)
	}
}

func (p *printer) writeRaw(w io.Writer, s string) {
	_, _ = fmt.Fprint(w, s)
}

func (p *printer) Slog() *slog.Logger {
	return slog.New(&printerHandler{p: p})
}
