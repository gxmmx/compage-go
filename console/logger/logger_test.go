package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_DefaultsToStderr(t *testing.T) {
	var buf bytes.Buffer
	log := New("test-app", withWriter(&buf))
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_EmptyProgram_UsesArgv0(t *testing.T) {
	var buf bytes.Buffer
	log := newLoggerWithWriter("", &buf, slog.LevelInfo, "")
	log.Info("hello")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Base(os.Args[0])
	if m["program"] != expected {
		t.Errorf("expected program=%q, got %q", expected, m["program"])
	}
}

func TestLogger_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	log := newLoggerWithWriter("myapp", &buf, slog.LevelDebug, "abc123")
	log.Info("started", "port", 8080)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}

	tests := []struct {
		key  string
		want any
	}{
		{"msg", "started"},
		{"level", "INFO"},
		{"program", "myapp"},
		{"unit", "myapp"},
		{"run_id", "abc123"},
		{"port", float64(8080)},
	}
	for _, tt := range tests {
		if m[tt.key] != tt.want {
			t.Errorf("key %q: want %v, got %v", tt.key, tt.want, m[tt.key])
		}
	}
	if _, ok := m["time"]; !ok {
		t.Error("expected time field in output")
	}
}

func TestLogger_For_NestsUnit(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	child := root.For("storage")
	child.Info("connected")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["unit"] != "app.storage" {
		t.Errorf("expected unit=app.storage, got %v", m["unit"])
	}
	if m["program"] != "app" {
		t.Errorf("expected program=app, got %v", m["program"])
	}
}

func TestLogger_For_DeepNesting(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	deep := root.For("storage").For("sqlite")
	deep.Info("query")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	last := lines[len(lines)-1]

	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		t.Fatal(err)
	}
	if m["unit"] != "app.storage.sqlite" {
		t.Errorf("expected unit=app.storage.sqlite, got %v", m["unit"])
	}
}

func TestLogger_WithRequest(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	req := root.WithRequest("req-xyz")
	req.Info("handling")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["request_id"] != "req-xyz" {
		t.Errorf("expected request_id=req-xyz, got %v", m["request_id"])
	}
}

func TestLogger_WithRequest_InheritedByChild(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	child := root.WithRequest("req-1").For("db")
	child.Info("query")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	last := lines[len(lines)-1]

	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		t.Fatal(err)
	}
	if m["request_id"] != "req-1" {
		t.Errorf("expected request_id=req-1, got %v", m["request_id"])
	}
	if m["unit"] != "app.db" {
		t.Errorf("expected unit=app.db, got %v", m["unit"])
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	tagged := root.With("version", "1.2.3")
	tagged.Info("boot")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["version"] != "1.2.3" {
		t.Errorf("expected version=1.2.3, got %v", m["version"])
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelWarn, "run1")

	log.Debug("no")
	log.Info("no")
	log.Warn("yes")
	log.Error("yes")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %s", len(lines), buf.String())
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelWarn, "run1")
	child := root.For("sub")

	root.Info("before")
	child.Info("before")
	if buf.Len() != 0 {
		t.Fatal("expected no output at Warn level for Info messages")
	}

	root.SetLevel(slog.LevelInfo)

	root.Info("after-root")
	child.Info("after-child")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines after SetLevel, got %d: %s", len(lines), buf.String())
	}
}

func TestLogger_Slog(t *testing.T) {
	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	s := log.Slog()
	s.Info("via slog")

	if !strings.Contains(buf.String(), "via slog") {
		t.Error("expected output from Slog() logger")
	}
}

func TestLogger_WithFile_Fallback(t *testing.T) {
	var buf bytes.Buffer
	log := New("app", WithFile("/nonexistent/path/test.log"), withWriter(&buf))
	if log == nil {
		t.Fatal("expected non-nil logger even with bad file path")
	}
	if !strings.Contains(buf.String(), "logger degraded") {
		t.Errorf("expected degradation warning, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "/nonexistent/path/test.log") {
		t.Errorf("expected file path in warning, got: %s", buf.String())
	}
}

func TestLogger_WithFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	log := New("app", WithFile(path))
	log.Info("to file")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
	if !strings.Contains(string(data), "to file") {
		t.Errorf("expected message in file, got: %s", string(data))
	}
}

func TestLogger_WithTrace(t *testing.T) {
	var buf bytes.Buffer
	root := newLoggerWithWriter("app", &buf, slog.LevelInfo, "run1")

	tr := root.WithTrace("trace-abc")
	tr.Info("tracing")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["trace_id"] != "trace-abc" {
		t.Errorf("expected trace_id=trace-abc, got %v", m["trace_id"])
	}
}

func TestLogger_RunID_AutoGenerated(t *testing.T) {
	var buf bytes.Buffer
	log := newLoggerWithWriter("app", &buf, slog.LevelInfo, "")
	log.Info("test")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	runID, ok := m["run_id"].(string)
	if !ok || len(runID) != 16 {
		t.Errorf("expected 16-char run_id, got %q", runID)
	}
}

// newLoggerWithWriter is a test helper that creates a logger writing to a buffer.
func newLoggerWithWriter(program string, buf *bytes.Buffer, level slog.Level, runID string) Logger {
	if program == "" {
		program = filepath.Base(os.Args[0])
	}
	if runID == "" {
		runID = generateRunID()
	}

	lvl := &slog.LevelVar{}
	lvl.Set(level)

	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: lvl})
	h2 := h.WithAttrs([]slog.Attr{
		slog.String("program", program),
		slog.String("unit", program),
		slog.String("run_id", runID),
	})

	return &logger{
		slogger: slog.New(h2),
		handler: h2,
		level:   lvl,
		unit:    program,
	}
}
