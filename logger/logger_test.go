package logger

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestNewLogger(t *testing.T) {
	loggerInterface := New()
	// Get controller from interface
	logger, _ := loggerInterface.(*Controller)

	if logger == nil {
		t.Fatal("Expected New() to return a non-nil Controller instance")
	}
	if logger.level == nil {
		t.Fatal("Expected default level to be set")
	}
	if logger.outWriter == nil {
		t.Fatal("Expected default outWriter to be set")
	}
	if logger.errWriter == nil {
		t.Fatal("Expected default errWriter to be set")
	}
	if logger.app != "" {
		t.Errorf("Expected default app name to be empty, got %s", logger.app)
	}
}

func TestSetOptions(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}
	loggerInterface := New(
		WithApp("testApp"),
		WithLevel("info"),
		WithLevel("warn"),
		WithLevel("error"),
		WithLevel("invalid"),
		WithOutWriter(ow),
		WithErrWriter(ew),
	)
	// Lazily set an option
	loggerInterface.Option(WithLevel("debug"))
	// Dynamically change the level
	loggerInterface.SetLevel("debug")

	// Get controller from interface
	logger, _ := loggerInterface.(*Controller)

	if logger.app != "testApp" {
		t.Errorf("Expected app name to be 'testApp', got %s", logger.app)
	}
	if logger.level.Level() != slog.LevelDebug {
		t.Errorf("Expected level to be 'debug', got %s", logger.level.String())
	}
}
