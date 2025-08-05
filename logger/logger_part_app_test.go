package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestAppLogger(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}
	logger := New(
		WithOutWriter(ow),
		WithErrWriter(ew),
		WithApp("testApp"),
	)

	log := logger.Get()

	log.WithGroup("baz").With("foo", "bar").Info("info1")
	log.Debug("debug1")
	logger.SetLevel("debug")
	log.Debug("debug2")
	log.Warn("warn1")
	log.Error("error1")

	outstring := ow.String()
	errstring := ew.String()

	if !strings.Contains(outstring, "\"unit\":\"testApp\"") {
		t.Errorf("Expected output to contain app name 'testApp', got: %s", outstring)
	}
	if !strings.Contains(outstring, "\"class\":\"app\"") {
		t.Errorf("Expected output to contain class 'app', got: %s", outstring)
	}
	if strings.Contains(outstring, "debug1") {
		t.Errorf("Expected output to not contain 'debug1', got: %s", outstring)
	}
	if !strings.Contains(outstring, "debug2") {
		t.Errorf("Expected output to contain 'debug2', got: %s", outstring)
	}
	if !strings.Contains(errstring, "warn1") {
		t.Errorf("Expected error output to contain 'warn1', got: %s", errstring)
	}
	if !strings.Contains(errstring, "error1") {
		t.Errorf("Expected error output to contain 'error1', got: %s", errstring)
	}
}

func TestAppLoggerWithClassAndUnit(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}
	logger := New(
		WithOutWriter(ow),
		WithErrWriter(ew),
		WithApp("testApp"),
	)

	log := logger.Get()

	log.Info("info", slog.String("class", "testClass"), slog.String("unit", "testUnit"), slog.String("foo", "bar"))

	outstring := ow.String()
	if !strings.Contains(outstring, "\"unit\":\"testUnit\"") {
		t.Errorf("Expected output to contain unit name 'testUnit', got: %s", outstring)
	}
	if !strings.Contains(outstring, "\"class\":\"testClass\"") {
		t.Errorf("Expected output to contain class 'testClass', got: %s", outstring)
	}
	if !strings.Contains(outstring, "\"foo\":\"bar\"") {
		t.Errorf("Expected output to contain 'foo: bar', got: %s", outstring)
	}
	if !strings.Contains(outstring, "data") {
		t.Errorf("Expected output to contain 'data' group, got: %s", outstring)
	}
}
