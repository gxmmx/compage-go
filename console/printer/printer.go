package printer

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

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

// New creates a Printer with the given options.
func New(opts ...Option) Printer {
	return newPrinter(newConfig(opts...))
}

// newConfig resolves construction options once. It is shared by NewPrompter so
// both its printer and input use the same configuration.
func newConfig(opts ...Option) *config {
	cfg := &config{
		level: slog.LevelInfo,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// newPrinter builds the concrete *printer from resolved configuration. It
// exists so callers within the package (e.g. NewPrompter) can share the
// printer's writer and mutex without an interface type assertion.
func newPrinter(cfg *config) *printer {
	outW, errW := resolveStreams(cfg)

	lvl := &slog.LevelVar{}
	lvl.Set(cfg.level)

	return &printer{
		outW:     outW,
		errW:     errW,
		level:    lvl,
		mu:       &sync.Mutex{},
		outStyle: style.New(cfg.suppressColor, outW),
		errStyle: style.New(cfg.suppressColor, errW),
	}
}

// resolveStreams determines the out/err targets. A printer is a human
// interface, so it writes to the standard streams: normal output to stdout and
// warn/error output to stderr. WithOutToErr/WithErrToOut redirect between the
// two; explicit writers (test-only) take precedence over everything.
func resolveStreams(cfg *config) (out, err io.Writer) {
	out = os.Stdout
	if cfg.outToErr {
		out = os.Stderr
	}
	if cfg.outWriter != nil {
		out = cfg.outWriter
	}

	err = os.Stderr
	if cfg.errToOut {
		err = os.Stdout
	}
	if cfg.errWriter != nil {
		err = cfg.errWriter
	}
	return out, err
}

type printer struct {
	outW  io.Writer
	errW  io.Writer
	level *slog.LevelVar
	// mu serializes writes to outW/errW. It is shared by pointer across all
	// printers derived via WithIndent/WithTextColor so concurrent goroutines
	// writing through any of them cannot interleave partial lines.
	mu        *sync.Mutex
	outStyle  style.Styler
	errStyle  style.Styler
	indent    int
	textColor style.Color
}

func (p *printer) Info(msg string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(msg, args...)
	p.writeLine(p.outW, p.outStyle, "", text)
}

func (p *printer) Success(msg string, args ...any) {
	if p.level.Level() > slog.LevelInfo {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := p.outStyle.Apply("✓", style.Green)
	p.writeLine(p.outW, p.outStyle, marker, text)
}

func (p *printer) Warn(msg string, args ...any) {
	if p.level.Level() > slog.LevelWarn {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := p.errStyle.Apply("!", style.Yellow)
	p.writeLine(p.errW, p.errStyle, marker, text)
}

func (p *printer) Error(msg string, args ...any) {
	if p.level.Level() > slog.LevelError {
		return
	}
	text := fmt.Sprintf(msg, args...)
	marker := p.errStyle.Apply("✗", style.Red)
	p.writeLine(p.errW, p.errStyle, marker, text)
}

func (p *printer) Verbose(msg string, args ...any) {
	if p.level.Level() > slog.LevelDebug {
		return
	}
	text := fmt.Sprintf(msg, args...)
	text = p.outStyle.Apply(text, p.textColor, style.Dim)
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
	p.mu.Lock()
	defer p.mu.Unlock()
	renderTable(p.outW, headers, rows)
}

func (p *printer) SetLevel(lvl slog.Level) {
	p.level.Set(lvl)
}

func (p *printer) WithIndent(n int) Printer {
	return p.withIndent(n)
}

// withIndent returns a derived *printer with indent increased by n, sharing the
// same level and write mutex. Used internally where the concrete type is needed
// (e.g. the slog handler) to avoid an interface type assertion.
func (p *printer) withIndent(n int) *printer {
	return &printer{
		outW:      p.outW,
		errW:      p.errW,
		level:     p.level,
		mu:        p.mu,
		outStyle:  p.outStyle,
		errStyle:  p.errStyle,
		indent:    p.indent + n,
		textColor: p.textColor,
	}
}

func (p *printer) WithTextColor(c style.Color) Printer {
	return &printer{
		outW:      p.outW,
		errW:      p.errW,
		level:     p.level,
		mu:        p.mu,
		outStyle:  p.outStyle,
		errStyle:  p.errStyle,
		indent:    p.indent,
		textColor: c,
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

func (p *printer) writeLine(w io.Writer, s style.Styler, marker string, text string) {
	text = s.Apply(text, p.textColor)

	indent := p.indentPrefix()
	p.mu.Lock()
	defer p.mu.Unlock()
	if marker == "" {
		_, _ = fmt.Fprintf(w, "%s%s\n", indent, text)
	} else {
		_, _ = fmt.Fprintf(w, "%s%s %s\n", indent, marker, text)
	}
}

func (p *printer) writeRaw(w io.Writer, s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = fmt.Fprint(w, s)
}

func (p *printer) Slog() *slog.Logger {
	return slog.New(&printerHandler{p: p})
}
