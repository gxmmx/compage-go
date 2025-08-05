package input

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewInput(t *testing.T) {
	input := New()
	if input == nil {
		t.Fatal("Expected New() to return a non-nil Input instance")
	}
	if input.indent != 0 {
		t.Errorf("Expected default indent to be 0, got %d", input.indent)
	}
	if input.pColor != "" {
		t.Errorf("Expected default prompt color to be empty, got %s", input.pColor)
	}
	if input.reader != os.Stdin {
		t.Error("Expected default reader to be os.Stdin")
	}
	if input.writer != os.Stdout {
		t.Error("Expected default writer to be os.Stdout")
	}
	if input.isTTY {
		t.Error("Expected isTTY to be true for terminal input")
	}

	_, err := input.Prompt("Test prompt")
	if err == nil {
		t.Error("Expected Prompt to return an error when not in a TTY")
	}
}

func TestPromptWithDefaultAndColor(t *testing.T) {
	reader := strings.NewReader("\n") // simulates pressing Enter
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		WithDefault("default-val"),
		WithPromptColor("green"),
		WithIndent(2),
		forceTTY(), // simulate TTY even though it's not in test
	)

	val, err := in.Prompt("Enter value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "default-val" {
		t.Errorf("expected 'default-val', got '%s'", val)
	}

	output := writer.String()
	if !strings.Contains(output, "    Enter value") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
	if strings.Contains(output, "\033[32m") {
		t.Errorf("expected prompt to contain color code, got: %s", output)
	}
}

func TestDirectPromptConstructor(t *testing.T) {
	reader := strings.NewReader("tdirinput\n")
	writer := &bytes.Buffer{}

	val, err := Prompt("tdirprompt",
		WithReader(reader),
		WithWriter(writer),
		forceTTY())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "tdirinput" {
		t.Errorf("expected 'tdirinput', got '%s'", val)
	}

	output := writer.String()
	if !strings.Contains(output, "tdirprompt") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
}

func TestPromptMulti(t *testing.T) {
	reader := strings.NewReader("line1\nline2\n\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		forceTTY(),
	)

	lines, err := in.PromptMulti("Enter lines (empty line to finish)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("expected ['line1', 'line2'], got %v", lines)
	}
}

func TestPromptMulti_NotTTY(t *testing.T) {
	reader := strings.NewReader("line1\nline2\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
	)

	_, err := in.PromptMulti("Enter lines:")
	if err == nil {
		t.Errorf("expected TTY error, got: %v", err)
	}
}

func TestPromptUntil(t *testing.T) {
	reader := strings.NewReader("invalid\ninvalid\nvalid\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		WithValidation("input needs to be valid", func(input string) bool {
			return input == "valid"
		}),
		forceTTY(),
	)

	val, err := in.PromptUntil("Enter valid input (type 'exit' to finish)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "valid" {
		t.Errorf("expected 'valid', got '%s'", val)
	}

	output := writer.String()
	if !strings.Contains(output, "Enter valid input") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
}

func TestPromptUntil_NotTTY(t *testing.T) {
	reader := strings.NewReader("invalid\nvalid\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		WithValidation("input needs to be valid", func(input string) bool {
			return input == "valid"
		}),
	)

	_, err := in.PromptUntil("Enter valid input:")
	if err == nil {
		t.Errorf("expected TTY error, got: %v", err)
	}
}

type unexpectedErrorReader struct{}

func (unexpectedErrorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("unexpected read failure")
}

func TestPromptUntil_InvalidReader(t *testing.T) {
	reader := unexpectedErrorReader{}
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		WithValidation("input needs to be valid", func(input string) bool {
			return input == "valid"
		}),
		forceTTY(),
	)

	_, err := in.PromptUntil("Enter valid input:")
	if err == nil {
		t.Errorf("expected error due to invalid reader, got: %v", err)
	}
}

func TestPromptConfirmYes(t *testing.T) {
	reader := strings.NewReader("yes\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		forceTTY(),
	)

	confirmed, err := in.Confirm("Are you sure?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirmed {
		t.Error("expected confirmation to be true")
	}

	output := writer.String()
	if !strings.Contains(output, "Are you sure?") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
}

func TestPromptConfirmNo(t *testing.T) {
	reader := strings.NewReader("no\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		forceTTY(),
	)

	confirmed, err := in.Confirm("Are you sure?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmed {
		t.Error("expected confirmation to be false")
	}

	output := writer.String()
	if !strings.Contains(output, "Are you sure?") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
}

func TestPromptConfirm_InvalidInput(t *testing.T) {
	reader := strings.NewReader("invalid\nyes\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		WithValidation("Please answer with 'yes' or 'no'", func(input string) bool {
			return input == "yes" || input == "no"
		}),
		forceTTY(),
	)

	confirmed, err := in.Confirm("Are you sure?")
	if err == nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmed {
		t.Error("expected confirmation to be true after valid input")
	}

	output := writer.String()
	if !strings.Contains(output, "Are you sure?") {
		t.Errorf("expected prompt to contain message, got: %s", output)
	}
}

func TestPromptConfirm_NotTTY(t *testing.T) {
	reader := strings.NewReader("yes\n")
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
	)

	_, err := in.Confirm("Are you sure?")
	if err == nil {
		t.Errorf("expected TTY error, got: %v", err)
	}
}

func TestPromptConfirm_InvalidReader(t *testing.T) {
	reader := unexpectedErrorReader{}
	writer := &bytes.Buffer{}

	in := New(
		WithReader(reader),
		WithWriter(writer),
		forceTTY(),
	)

	_, err := in.Confirm("Are you sure?")
	if err == nil {
		t.Errorf("expected error due to invalid reader, got: %v", err)
	}
}

func TestPromptSecret(t *testing.T) {
	writer := &bytes.Buffer{}

	// Override readPasswordFn for test
	originalReadPassword := readPasswordFn
	readPasswordFn = func(fd int) ([]byte, error) {
		return []byte("secret-input\n"), nil
	}
	defer func() { readPasswordFn = originalReadPassword }() // restore after test

	in := New(
		WithWriter(writer),
		WithSecret(),
		forceTTY(),
	)

	secret, err := in.Prompt("Enter secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "secret-input" {
		t.Errorf("expected 'secret-input', got '%s'", secret)
	}

	// Optionally check that newline is printed after password entry
	if !strings.Contains(writer.String(), "\n") {
		t.Errorf("expected newline after password input")
	}
}
