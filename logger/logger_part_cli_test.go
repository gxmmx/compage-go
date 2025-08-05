package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestCliLogger(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}
	logger := New(
		WithOutWriter(ow),
		WithErrWriter(ew),
	)

	log := logger.Get()

	// Test adding attributes and group to logger directly
	log2 := log.With(
		slog.String("baz", "qux"),
	).WithGroup("testgroup")

	if log == nil {
		t.Fatal("Expected Get() to return a non-nil logger instance")
	}
	log.Info("Info1", slog.String("style", "bold"), slog.Int("indent", 2), slog.String("foo", "bar"))

	// Test manual indent
	manualindent := map[string]any{
		"indent": float64(2),
	}
	var attrs []any
	for k, v := range manualindent {
		attrs = append(attrs, slog.Any(k, v))
	}

	log.Warn("Warning1", attrs...)
	log.Error("Error1")
	log.Debug("Debug1")
	logger.SetLevel("debug")
	log2.Debug("Debug2")
	output := ow.String()
	if !strings.Contains(output, IndentString+IndentString+"Info1") {
		t.Errorf("Expected output to contain indent and 'Info1', got: %s", output)
	}
	if !strings.Contains(output, "foo") {
		t.Errorf("Expected output to contain 'foo', got: %s", output)
	}
	if !strings.Contains(output, "bar") {
		t.Errorf("Expected output to contain 'bar', got: %s", output)
	}
	if strings.Contains(output, "Debug1") {
		t.Errorf("Expected output to not contain 'Debug message', got: %s", output)
	}
	if !strings.Contains(output, "Debug2") {
		t.Errorf("Expected output to contain 'Debug2', got: %s", output)
	}
	if !strings.Contains(ew.String(), "Error1") {
		t.Errorf("Expected error output to contain 'Error1', got: %s", ew.String())
	}
	if !strings.Contains(ew.String(), IndentString+IndentString+"Warning1") {
		t.Errorf("Expected error output to contain 'Warning1', got: %s", ew.String())
	}
	if !strings.Contains(ow.String(), "baz") {
		t.Errorf("Expected output to contain 'baz', got: %s", ow.String())
	}
	if strings.Contains(ow.String(), "testgroup") {
		t.Errorf("Expected output to not contain 'testgroup', got: %s", ow.String())
	}
}
