package logger

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	cmperr "github.com/gxmmx/compage-go/errors"
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

func TestLoggerPreWarnings(t *testing.T) {
	ow := &bytes.Buffer{}
	ew := &bytes.Buffer{}
	loggerInterface := New(
		WithOutWriter(ow),
		WithErrWriter(ew),
		WithPreWarning(fmt.Errorf("Test pre-warning 1")),
		WithPreWarning(cmperr.NewAlreadyExists("Test pre-warning 2", nil)),
	)
	// Get controller from interface
	logger, _ := loggerInterface.(*Controller)

	if len(logger.preWarnings) != 2 {
		t.Fatalf("Expected 2 pre-warnings, got %d", len(logger.preWarnings))
	}

	_ = logger.Get() // Trigger logger initialization

	if !bytes.Contains(ew.Bytes(), []byte("Test pre-warning 1")) {
		t.Error("Expected pre-warning 'Test pre-warning 1' to be logged")
	}
	if !bytes.Contains(ew.Bytes(), []byte("Test pre-warning 2")) {
		t.Error("Expected pre-warning 'Test pre-warning 2' to be logged")
	}
}
