package console

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestWithRequestID_Roundtrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")
	got := RequestIDFrom(ctx)
	if got != "req-123" {
		t.Errorf("got %q, want %q", got, "req-123")
	}
}

func TestWithTraceID_Roundtrip(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-abc")
	got := TraceIDFrom(ctx)
	if got != "trace-abc" {
		t.Errorf("got %q, want %q", got, "trace-abc")
	}
}

func TestRequestIDFrom_Empty(t *testing.T) {
	got := RequestIDFrom(context.Background())
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestTraceIDFrom_Empty(t *testing.T) {
	got := TraceIDFrom(context.Background())
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestInfoContext_ExtractsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-from-ctx")
	log.InfoContext(ctx, "hello")

	m := parseJSON(t, buf.Bytes())
	if m["request_id"] != "req-from-ctx" {
		t.Errorf("expected request_id=req-from-ctx, got %v", m["request_id"])
	}
}

func TestInfoContext_ExtractsTraceID(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithTraceID(context.Background(), "trace-xyz")
	log.InfoContext(ctx, "hello")

	m := parseJSON(t, buf.Bytes())
	if m["trace_id"] != "trace-xyz" {
		t.Errorf("expected trace_id=trace-xyz, got %v", m["trace_id"])
	}
}

func TestInfoContext_ExtractsBothKeys(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithTraceID(ctx, "trace-1")
	log.InfoContext(ctx, "both")

	m := parseJSON(t, buf.Bytes())
	if m["request_id"] != "req-1" {
		t.Errorf("expected request_id=req-1, got %v", m["request_id"])
	}
	if m["trace_id"] != "trace-1" {
		t.Errorf("expected trace_id=trace-1, got %v", m["trace_id"])
	}
}

func TestInfoContext_NoContextKeys_NoExtraction(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	log.InfoContext(context.Background(), "plain")

	m := parseJSON(t, buf.Bytes())
	if _, ok := m["request_id"]; ok {
		t.Errorf("expected no request_id, got %v", m["request_id"])
	}
	if _, ok := m["trace_id"]; ok {
		t.Errorf("expected no trace_id, got %v", m["trace_id"])
	}
}

func TestWithRequest_Dedup_ContextIgnored(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	child := log.WithRequest("from-with")
	ctx := WithRequestID(context.Background(), "from-ctx")
	child.InfoContext(ctx, "should use from-with")

	raw := buf.String()
	count := strings.Count(raw, "request_id")
	if count != 1 {
		t.Errorf("expected exactly 1 request_id, got %d in: %s", count, raw)
	}
	m := parseJSON(t, buf.Bytes())
	if m["request_id"] != "from-with" {
		t.Errorf("expected request_id=from-with, got %v", m["request_id"])
	}
}

func TestWithTrace_Dedup_ContextIgnored(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	child := log.WithTrace("from-with")
	ctx := WithTraceID(context.Background(), "from-ctx")
	child.InfoContext(ctx, "should use from-with")

	raw := buf.String()
	count := strings.Count(raw, "trace_id")
	if count != 1 {
		t.Errorf("expected exactly 1 trace_id, got %d in: %s", count, raw)
	}
	m := parseJSON(t, buf.Bytes())
	if m["trace_id"] != "from-with" {
		t.Errorf("expected trace_id=from-with, got %v", m["trace_id"])
	}
}

func TestFor_InheritsDedup(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	child := log.WithRequest("req-parent").For("storage")
	ctx := WithRequestID(context.Background(), "req-ctx")
	child.InfoContext(ctx, "nested")

	raw := buf.String()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	last := lines[len(lines)-1]

	count := strings.Count(last, "request_id")
	if count != 1 {
		t.Errorf("expected 1 request_id in child output, got %d in: %s", count, last)
	}
}

func TestDebugContext_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-1")
	log.DebugContext(ctx, "should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output at info level for DebugContext, got: %s", buf.String())
	}
}

func TestWarnContext_ExtractsKeys(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithTraceID(context.Background(), "trace-warn")
	log.WarnContext(ctx, "careful")

	m := parseJSON(t, buf.Bytes())
	if m["trace_id"] != "trace-warn" {
		t.Errorf("expected trace_id=trace-warn, got %v", m["trace_id"])
	}
}

func TestErrorContext_ExtractsKeys(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)

	ctx := WithRequestID(context.Background(), "req-err")
	log.ErrorContext(ctx, "failed")

	m := parseJSON(t, buf.Bytes())
	if m["request_id"] != "req-err" {
		t.Errorf("expected request_id=req-err, got %v", m["request_id"])
	}
}

func TestNonContextMethods_NoExtraction(t *testing.T) {
	var buf bytes.Buffer
	log := newContextTestLogger(&buf, slog.LevelInfo)
	log.Info("no context")

	m := parseJSON(t, buf.Bytes())
	if _, ok := m["request_id"]; ok {
		t.Errorf("non-context method should not extract request_id")
	}
	if _, ok := m["trace_id"]; ok {
		t.Errorf("non-context method should not extract trace_id")
	}
}

// newContextTestLogger creates a logger with contextHandler for testing.
func newContextTestLogger(buf *bytes.Buffer, level slog.Level) Logger {
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	h := slog.Handler(newContextHandler(inner))
	h = h.WithAttrs([]slog.Attr{
		slog.String("program", "test"),
		slog.String("unit", "test"),
		slog.String("run_id", "test-run"),
	})
	return &logger{
		slogger: slog.New(h),
		handler: h,
		unit:    "test",
	}
}

func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, string(data))
	}
	return m
}
