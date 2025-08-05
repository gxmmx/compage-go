package spinner

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Prepare
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestNewSpinner(t *testing.T) {
	writer := &bytes.Buffer{}
	spinnerInterface := Start(
		"T1 Spinner",
		WithWriter(writer),
		WithDelay(10*time.Millisecond),
		WithMessageColor("blue"),
		WithSpinnerColor("red"),
		WithIndent(2),
		forceTTY(),
	)
	// Get controller from interface
	spinner, _ := spinnerInterface.(*Controller)

	time.Sleep(30 * time.Millisecond)
	spinner.Stop("T1 Done")
	if !spinner.isTTY {
		t.Error("expected spinner to be TTY, but it is not")
	}
	output := writer.String()
	if !strings.Contains(output, "T1 Spinner") {
		t.Errorf("Expected output to contain 'T2 Spinner', got: %q", output)
	}
	if !strings.Contains(output, "\033[34m") { // blue color code
		t.Errorf("Expected output to contain blue color code, got: %q", output)
	}
	if !strings.Contains(output, "\033[31m") { // red color code
		t.Errorf("Expected output to contain red color code, got: %q", output)
	}
	if !strings.Contains(output, "T1 Done") {
		t.Errorf("Expected output to contain 'T1 Done', got: %q", output)
	}
	if !strings.Contains(output, "\u280B") { // first spinner character
		t.Errorf("Expected output to contain cursor show code, got: %q", output)
	}
}

func TestNewSpinnerNoTTY(t *testing.T) {
	writer := &bytes.Buffer{}
	spinnerInterface := Start(
		"T2 Spinner",
		WithWriter(writer),
		WithDelay(10*time.Millisecond),
		WithIndent(2),
		WithMutex(&sync.Mutex{}),
	)

	// Get controller from interface
	spinner, _ := spinnerInterface.(*Controller)

	time.Sleep(30 * time.Millisecond)
	spinner.Stop("T2 Done")
	if spinner.isTTY {
		t.Error("expected spinner to not be TTY, but it is")
	}
	output := writer.String()
	if len(output) > 0 {
		t.Errorf("Expected no output when not running in TTY, but got: %q", output)
	}
}
