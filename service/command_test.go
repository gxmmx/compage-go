package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

type failingFiles struct {
	fakeFiles
	writeErr error
}

func TestCommandOutputClassifiesMissingTool(t *testing.T) {
	_, err := commandOutput(context.Background(), &fakeRunner{err: exec.ErrNotFound}, "missing-tool")
	if !errx.IsKind(err, errx.Unavailable) || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func (f failingFiles) write(string, []byte, os.FileMode) error { return f.writeErr }

func TestCommandOutputPreservesCauseAndOutput(t *testing.T) {
	cause := errors.New("exit status 1")
	_, err := commandOutput(context.Background(), &fakeRunner{out: "manager detail", err: cause}, "tool", "arg")
	if !errors.Is(err, cause) {
		t.Fatalf("cause not preserved: %v", err)
	}
	if !strings.Contains(err.Error(), "manager detail") {
		t.Fatalf("output omitted: %v", err)
	}
}

func TestWriteDefinitionPreservesCause(t *testing.T) {
	cause := errors.New("disk full")
	err := writeDefinition(&failingFiles{writeErr: cause}, "/tmp/unit", nil, 0o644)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "/tmp/unit") {
		t.Fatalf("write error=%v", err)
	}
}

func TestCommandOutputPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := commandOutput(ctx, &fakeRunner{err: errors.New("killed")}, "tool")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not preserved: %v", err)
	}
}
