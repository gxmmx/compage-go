package console

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestPreLogger_QueueAndCount(t *testing.T) {
	pre := Pre()
	pre.Debug("d1")
	pre.Info("i1")
	pre.Info("i2")
	pre.Warn("w1")
	pre.Error("e1")
	pre.Error("e2")

	tests := []struct {
		level slog.Level
		want  int
	}{
		{slog.LevelDebug, 6},
		{slog.LevelInfo, 5},
		{slog.LevelWarn, 3},
		{slog.LevelError, 2},
	}
	for _, tt := range tests {
		got := pre.Count(tt.level)
		if got != tt.want {
			t.Errorf("Count(%v) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestPreLogger_FlushPreservesTimestamps(t *testing.T) {
	pre := Pre()
	beforeQueue := time.Now()
	pre.Info("queued early", "step", 1)
	time.Sleep(10 * time.Millisecond)
	afterQueue := time.Now()

	time.Sleep(50 * time.Millisecond)

	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}

	ts, err := time.Parse(time.RFC3339Nano, m["time"].(string))
	if err != nil {
		t.Fatalf("cannot parse time: %v", err)
	}

	if ts.Before(beforeQueue) || ts.After(afterQueue) {
		t.Errorf("expected timestamp between %v and %v, got %v", beforeQueue, afterQueue, ts)
	}
}

func TestPreLogger_FlushOutputsAllRecords(t *testing.T) {
	pre := Pre()
	pre.Info("first", "n", 1)
	pre.Warn("second", "n", 2)
	pre.Error("third", "n", 3)

	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), buf.String())
	}

	expected := []struct {
		msg   string
		level string
	}{
		{"first", "INFO"},
		{"second", "WARN"},
		{"third", "ERROR"},
	}

	for i, exp := range expected {
		var m map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &m); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if m["msg"] != exp.msg {
			t.Errorf("line %d: expected msg=%q, got %q", i, exp.msg, m["msg"])
		}
		if m["level"] != exp.level {
			t.Errorf("line %d: expected level=%q, got %q", i, exp.level, m["level"])
		}
	}
}

func TestPreLogger_FlushIncludesAttrs(t *testing.T) {
	pre := Pre()
	pre.Info("with attrs", "key", "value", "count", 42)

	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
	if m["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", m["count"])
	}
}

func TestPreLogger_PanicAfterFlush(t *testing.T) {
	pre := Pre()
	pre.Info("before flush")

	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic after Flush")
		}
	}()
	pre.Info("after flush")
}

func TestPreLogger_DoubleFLushPanics(t *testing.T) {
	pre := Pre()
	pre.Info("msg")

	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on double Flush")
		}
	}()
	pre.Flush(log.Slog())
}

func TestPreLogger_EmptyFlush(t *testing.T) {
	pre := Pre()
	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelDebug, "run1")
	pre.Flush(log.Slog())

	if buf.Len() != 0 {
		t.Errorf("expected no output from empty PreLogger, got: %s", buf.String())
	}
}

func TestPreLogger_CountOnEmpty(t *testing.T) {
	pre := Pre()
	if pre.Count(slog.LevelError) != 0 {
		t.Error("expected 0 count on empty PreLogger")
	}
}
