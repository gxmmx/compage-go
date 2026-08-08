package console

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestFanHandler_WritesToAllTargets(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h := newFanHandler([]slog.Handler{
		slog.NewJSONHandler(&buf1, nil),
		slog.NewJSONHandler(&buf2, nil),
	})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	_ = h.Handle(context.Background(), r)

	if !strings.Contains(buf1.String(), `"msg":"hello"`) {
		t.Errorf("buf1 missing message: %s", buf1.String())
	}
	if !strings.Contains(buf2.String(), `"msg":"hello"`) {
		t.Errorf("buf2 missing message: %s", buf2.String())
	}
}

func TestFanHandler_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	h := newFanHandler([]slog.Handler{
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}),
	})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "should be dropped", 0)
	_ = h.Handle(context.Background(), r)

	if buf.Len() != 0 {
		t.Errorf("expected no output for info record with warn-level handler, got: %s", buf.String())
	}
}

func TestFanHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	var h slog.Handler = newFanHandler([]slog.Handler{
		slog.NewJSONHandler(&buf, nil),
	})
	h = h.WithAttrs([]slog.Attr{slog.String("k", "v")})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	_ = h.Handle(context.Background(), r)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["k"] != "v" {
		t.Errorf("expected attr k=v, got: %v", m)
	}
}

func TestFanHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	h := newFanHandler([]slog.Handler{
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}),
	})

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("should not be enabled for debug")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("should be enabled for error")
	}
}

func TestContextHandler_DropsHintAttrs_InRecord(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	h := newContextHandler(inner)
	l := slog.New(h)

	l.Info("test", "real", "data", "console.indent", 1, "console.success", true)

	got := buf.String()
	if strings.Contains(got, "console.indent") {
		t.Errorf("hint console.indent should not appear in JSON, got %s", got)
	}
	if strings.Contains(got, "console.success") {
		t.Errorf("hint console.success should not appear in JSON, got %s", got)
	}
	if !strings.Contains(got, `"real":"data"`) {
		t.Errorf("expected real attr in JSON, got %s", got)
	}
}

func TestContextHandler_DropsHintAttrs_InWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	h := newContextHandler(inner)
	l := slog.New(h).With("console.indent", 2, "component", "config")

	l.Info("test")

	got := buf.String()
	if strings.Contains(got, "console.indent") {
		t.Errorf("hint console.indent should not appear in JSON, got %s", got)
	}
	if !strings.Contains(got, `"component":"config"`) {
		t.Errorf("expected component attr in JSON, got %s", got)
	}
}

func TestContextHandler_NoHints_PassesThrough(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	h := newContextHandler(inner)
	l := slog.New(h)

	l.Info("normal", "key", "val")

	got := buf.String()
	if !strings.Contains(got, `"key":"val"`) {
		t.Errorf("expected key attr, got %s", got)
	}
}
