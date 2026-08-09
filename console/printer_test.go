package console

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/gxmmx/compage-go/style"
)

const (
	testAnsiGreen   = "\033[32m"
	testAnsiMagenta = "\033[35m"
	testAnsiCyan    = "\033[36m"
	testAnsiDim     = "\033[2m"
)

func TestPrinter_Info(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Info("hello %s", "world")

	got := buf.String()
	if got != "hello world\n" {
		t.Errorf("got %q, want %q", got, "hello world\n")
	}
}

func TestPrinter_Success(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Success("done %d", 1)

	got := buf.String()
	if !strings.Contains(got, "✓") {
		t.Errorf("expected ✓ marker, got %q", got)
	}
	if !strings.Contains(got, "done 1") {
		t.Errorf("expected message, got %q", got)
	}
}

func TestPrinter_Warn_RoutesToErr(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(WithOutTo(&out), WithErrTo(&errBuf), WithColor(false))
	p.Warn("careful %s", "now")

	if out.Len() != 0 {
		t.Errorf("expected no output to out, got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "!") {
		t.Errorf("expected ! marker in err, got %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "careful now") {
		t.Errorf("expected message in err, got %q", errBuf.String())
	}
}

func TestPrinter_Error_RoutesToErr(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(WithOutTo(&out), WithErrTo(&errBuf), WithColor(false))
	p.Error("bad %s", "thing")

	if out.Len() != 0 {
		t.Errorf("expected no output to out, got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "✗") {
		t.Errorf("expected ✗ marker, got %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "bad thing") {
		t.Errorf("expected message, got %q", errBuf.String())
	}
}

func TestPrinter_Verbose_ShowsAtDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithLevel(slog.LevelDebug), WithColor(false))
	p.Verbose("detail %d", 42)

	if !strings.Contains(buf.String(), "detail 42") {
		t.Errorf("expected verbose output, got %q", buf.String())
	}
}

func TestPrinter_Verbose_HiddenAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithLevel(slog.LevelInfo), WithColor(false))
	p.Verbose("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output at info level, got %q", buf.String())
	}
}

func TestPrinter_LevelFiltering_WarnOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(WithOutTo(&out), WithErrTo(&errBuf), WithLevel(slog.LevelWarn), WithColor(false))

	p.Info("hidden")
	p.Success("hidden")
	p.Verbose("hidden")
	p.Warn("visible")
	p.Error("visible")

	if out.Len() != 0 {
		t.Errorf("expected no stdout output at warn level, got %q", out.String())
	}
	lines := strings.Split(strings.TrimSpace(errBuf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines in err, got %d: %q", len(lines), errBuf.String())
	}
}

func TestPrinter_LevelFiltering_ErrorOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(WithOutTo(&out), WithErrTo(&errBuf), WithLevel(slog.LevelError), WithColor(false))

	p.Info("hidden")
	p.Warn("hidden")
	p.Error("visible one")
	p.Error("visible two")

	if out.Len() != 0 {
		t.Errorf("expected no stdout output, got %q", out.String())
	}
	lines := strings.Split(strings.TrimSpace(errBuf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines in err, got %d: %q", len(lines), errBuf.String())
	}
}

func TestPrinter_Print(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Print("raw %s", "output")

	if buf.String() != "raw output\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestPrinter_Printf(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Printf("no newline %d", 1)

	if buf.String() != "no newline 1" {
		t.Errorf("got %q", buf.String())
	}
}

func TestPrinter_Println(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Println("a", "b")

	if buf.String() != "a b\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestPrinter_WithIndent(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	sub := p.WithIndent(1)
	sub.Info("indented")

	if buf.String() != "  indented\n" {
		t.Errorf("got %q, want %q", buf.String(), "  indented\n")
	}
}

func TestPrinter_WithIndent_Deep(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	deep := p.WithIndent(1).WithIndent(1)
	deep.Info("deep")

	if buf.String() != "    deep\n" {
		t.Errorf("got %q, want %q", buf.String(), "    deep\n")
	}
}

func TestPrinter_WithIndent_Marker(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	sub := p.WithIndent(1)
	sub.Success("step done")

	got := buf.String()
	if !strings.HasPrefix(got, "  ✓") {
		t.Errorf("expected indent before marker, got %q", got)
	}
}

func TestPrinter_WithTextColor_NoColorMode(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	colored := p.WithTextColor(style.Magenta)
	colored.Info("plain")

	// No ANSI codes when color is off
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI codes, got %q", buf.String())
	}
}

func TestPrinter_WithTextColor_ColorMode(t *testing.T) {
	var buf bytes.Buffer
	p := newTestPrinterWithColor(&buf)
	colored := p.WithTextColor(style.Magenta)
	colored.Info("tinted")

	got := buf.String()
	if !strings.Contains(got, testAnsiMagenta) {
		t.Errorf("expected magenta ANSI code, got %q", got)
	}
	if !strings.Contains(got, "tinted") {
		t.Errorf("expected message text, got %q", got)
	}
}

func TestPrinter_WithTextColor_MarkerKeepsSemantic(t *testing.T) {
	var buf bytes.Buffer
	p := newTestPrinterWithColor(&buf)
	colored := p.WithTextColor(style.Cyan)
	colored.Success("deployed")

	got := buf.String()
	if !strings.Contains(got, testAnsiGreen) {
		t.Errorf("expected green marker, got %q", got)
	}
	if !strings.Contains(got, testAnsiCyan) {
		t.Errorf("expected cyan text, got %q", got)
	}
}

func TestPrinter_Verbose_Dimmed(t *testing.T) {
	var buf bytes.Buffer
	p := newTestPrinterWithColor(&buf)
	p = setPrinterLevel(p, slog.LevelDebug)
	p.Verbose("dim text")

	got := buf.String()
	if !strings.Contains(got, testAnsiDim) {
		t.Errorf("expected dim ANSI code, got %q", got)
	}
}

func TestPrinter_Verbose_DimAndTextColor(t *testing.T) {
	var buf bytes.Buffer
	p := newTestPrinterWithColor(&buf)
	p = setPrinterLevel(p, slog.LevelDebug)
	mag := p.WithTextColor(style.Magenta)
	mag.Verbose("dim magenta")

	got := buf.String()
	if !strings.Contains(got, testAnsiDim) {
		t.Errorf("expected dim, got %q", got)
	}
	if !strings.Contains(got, testAnsiMagenta) {
		t.Errorf("expected magenta, got %q", got)
	}
}

func TestPrinter_ErrFallsBackToOut(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	// No WithErrTo set — should fall back to out writer
	p.Warn("goes to out")

	if !strings.Contains(buf.String(), "goes to out") {
		t.Errorf("expected warn in out buffer, got %q", buf.String())
	}
}

func TestPrinter_Table(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Table(
		[]string{"NAME", "AGE"},
		[][]string{
			{"alice", "30"},
			{"bob", "25"},
		},
	)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "NAME") {
		t.Errorf("expected header, got %q", lines[0])
	}
	// Verify alignment — "AGE" and "30" should be at same column
	nameCol := strings.Index(lines[0], "AGE")
	ageCol := strings.Index(lines[1], "30")
	if nameCol != ageCol {
		t.Errorf("column misalignment: header AGE at %d, data 30 at %d", nameCol, ageCol)
	}
}

func TestPrinter_Table_Empty(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	p.Table([]string{"A", "B"}, nil)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty table, got %q", buf.String())
	}
}

func TestPrinter_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false), WithLevel(slog.LevelWarn))
	child := p.WithIndent(2)

	p.Info("before")
	child.Info("before")
	if buf.Len() != 0 {
		t.Fatal("expected no output at Warn level for Info messages")
	}

	p.SetLevel(slog.LevelInfo)

	p.Info("after-root")
	child.Info("after-child")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines after SetLevel, got %d: %s", len(lines), buf.String())
	}
}

// newTestPrinterWithColor creates a printer with color forced on (bypasses TTY check).
func newTestPrinterWithColor(buf *bytes.Buffer) Printer {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	return &printer{
		outW:     buf,
		errW:     buf,
		level:    lvl,
		mu:       &sync.Mutex{},
		outStyle: style.New(false, nil),
		errStyle: style.New(false, nil),
	}
}

// setPrinterLevel changes the level on an existing printer (shared via LevelVar).
func setPrinterLevel(p Printer, lvl slog.Level) Printer {
	p.SetLevel(lvl)
	return p
}

// TestPrinter_ConcurrentWrites verifies that concurrent goroutines writing
// through a printer and its derived printers (which share the write mutex)
// never interleave partial lines. Run with -race to also catch data races on
// the underlying writer.
func TestPrinter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(WithOutTo(&buf), WithColor(false))
	indented := p.WithIndent(1)

	const goroutines = 50
	const perGoroutine = 20

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				p.Info("plain-line-content")
				indented.Info("indented-line-content")
			}
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	wantLines := goroutines * perGoroutine * 2
	if len(lines) != wantLines {
		t.Fatalf("expected %d lines, got %d", wantLines, len(lines))
	}
	for _, line := range lines {
		if line != "plain-line-content" && line != "  indented-line-content" {
			t.Fatalf("torn or unexpected line: %q", line)
		}
	}
}
