package printer

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/gxmmx/compage-go/style"
)

func TestPrinterHandler_LevelRouting(t *testing.T) {
	tests := []struct {
		name   string
		level  slog.Level
		msg    string
		wantIn string // "out" or "err"
	}{
		{"debug routes to verbose", slog.LevelDebug, "debug msg", "out"},
		{"info routes to info", slog.LevelInfo, "info msg", "out"},
		{"warn routes to warn", slog.LevelWarn, "warn msg", "err"},
		{"error routes to error", slog.LevelError, "error msg", "err"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			p := New(withWriters(&out, &errBuf), WithLevel(slog.LevelDebug), WithColor(false))
			l := p.Slog()

			l.Log(context.TODO(), tt.level, tt.msg)

			var got string
			if tt.wantIn == "out" {
				got = out.String()
			} else {
				got = errBuf.String()
			}
			if !strings.Contains(got, tt.msg) {
				t.Errorf("expected %q in %s buffer, got out=%q err=%q", tt.msg, tt.wantIn, out.String(), errBuf.String())
			}
		})
	}
}

func TestPrinterHandler_DebugSuppressedAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithLevel(slog.LevelInfo), WithColor(false))
	l := p.Slog()

	l.Debug("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output at info level, got %q", buf.String())
	}
}

func TestPrinterHandler_WarnMarker(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := New(withWriters(&out, &errBuf), WithColor(false))
	l := p.Slog()

	l.Warn("careful")

	if !strings.Contains(errBuf.String(), "!") {
		t.Errorf("expected ! marker, got %q", errBuf.String())
	}
}

func TestPrinterHandler_ErrorMarker(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := New(withWriters(&out, &errBuf), WithColor(false))
	l := p.Slog()

	l.Error("failed")

	if !strings.Contains(errBuf.String(), "✗") {
		t.Errorf("expected ✗ marker, got %q", errBuf.String())
	}
}

func TestPrinterHandler_AttrsRendered(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog()

	l.Info("loaded", "path", "/etc/app.toml", "format", "toml")

	got := buf.String()
	if !strings.Contains(got, "path=/etc/app.toml") {
		t.Errorf("expected path attr, got %q", got)
	}
	if !strings.Contains(got, "format=toml") {
		t.Errorf("expected format attr, got %q", got)
	}
}

func TestPrinterHandler_HintSuccess(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog()

	l.Info("enrolled", "console.success", true)

	got := buf.String()
	if !strings.Contains(got, "✓") {
		t.Errorf("expected ✓ marker for success hint, got %q", got)
	}
	if strings.Contains(got, "console.success") {
		t.Errorf("hint should not appear in output, got %q", got)
	}
}

func TestPrinterHandler_HintIndent(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog()

	l.Info("indented", "console.indent", 1)

	got := buf.String()
	if !strings.HasPrefix(got, "  ") {
		t.Errorf("expected 2-space indent, got %q", got)
	}
	if strings.Contains(got, "console.indent") {
		t.Errorf("hint should not appear in output, got %q", got)
	}
}

func TestPrinterHandler_HintIndentComposesWithBase(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	sub := p.WithIndent(2)
	l := sub.Slog()

	l.Info("deep", "console.indent", 1)

	got := buf.String()
	// base indent 2 + hint indent 1 = 3 levels = 6 spaces
	if !strings.HasPrefix(got, "      ") {
		t.Errorf("expected 6-space indent (base 2 + hint 1), got %q", got)
	}
}

func TestPrinterHandler_WithAttrs_Persist(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog().With("component", "config")

	l.Info("first")
	l.Info("second")

	got := buf.String()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	for i, line := range lines {
		if !strings.Contains(line, "component=config") {
			t.Errorf("line %d missing stamped attr: %q", i, line)
		}
	}
}

func TestPrinterHandler_WithAttrs_HintPersists(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog().With("console.indent", 1)

	l.Info("one")
	l.Info("two")

	got := buf.String()
	lines := strings.Split(got, "\n")
	// last element is empty after trailing newline
	for i, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("line %d expected indent, got %q", i, line)
		}
	}
}

func TestPrinterHandler_WithAttrs_HintIndentAccumulates(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog().With("console.indent", 1).With("console.indent", 1)

	l.Info("double")

	got := buf.String()
	// 2 accumulated indent levels = 4 spaces
	if !strings.HasPrefix(got, "    ") {
		t.Errorf("expected 4-space indent, got %q", got)
	}
}

func TestPrinterHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog().WithGroup("db")

	l.Info("query", "rows", 42)

	got := buf.String()
	if !strings.Contains(got, "db.rows=42") {
		t.Errorf("expected grouped key, got %q", got)
	}
}

func TestPrinterHandler_WithGroup_Nested(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog().WithGroup("app").WithGroup("db")

	l.Info("query", "rows", 42)

	got := buf.String()
	if !strings.Contains(got, "app.db.rows=42") {
		t.Errorf("expected nested group key, got %q", got)
	}
}

func TestPrinterHandler_WithTextColorCarries(t *testing.T) {
	var buf bytes.Buffer
	p := newTestPrinterWithColor(&buf)
	colored := p.WithTextColor(style.Cyan)
	l := colored.Slog()

	l.Info("tinted")

	got := buf.String()
	if !strings.Contains(got, "\033[36m") {
		t.Errorf("expected cyan ANSI code from WithTextColor, got %q", got)
	}
}

func TestPrinterHandler_HintNotInOutput(t *testing.T) {
	var buf bytes.Buffer
	p := New(withWriters(&buf, &buf), WithColor(false))
	l := p.Slog()

	l.Info("msg", "console.indent", 1, "console.success", true, "real", "data")

	got := buf.String()
	if strings.Contains(got, "console.indent") {
		t.Errorf("hint console.indent should not appear in output, got %q", got)
	}
	if strings.Contains(got, "console.success") {
		t.Errorf("hint console.success should not appear in output, got %q", got)
	}
	if !strings.Contains(got, "real=data") {
		t.Errorf("expected real attr in output, got %q", got)
	}
}
