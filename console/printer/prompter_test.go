package printer

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

func testPrompter(t *testing.T, out *bytes.Buffer, input string) Prompter {
	t.Helper()
	pr, err := NewPrompter(
		withWriters(out, out),
		WithColor(false),
		withReader(strings.NewReader(input)),
		withTerminalCheck(func(io.Reader) bool { return true }),
	)
	if err != nil {
		t.Fatalf("NewPrompter() error = %v", err)
	}
	return pr
}

func TestPrompter_Prompt_ReturnsInput(t *testing.T) {
	var out bytes.Buffer
	pr := testPrompter(t, &out, "custom value\n")

	result := pr.Prompt("Enter name", "default")

	if result != "custom value" {
		t.Errorf("got %q, want %q", result, "custom value")
	}
	if !strings.Contains(out.String(), "Enter name") {
		t.Errorf("expected prompt in output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[default]") {
		t.Errorf("expected fallback hint, got %q", out.String())
	}
}

func TestPrompter_Prompt_EmptyInput_ReturnsFallback(t *testing.T) {
	var out bytes.Buffer
	pr := testPrompter(t, &out, "\n")

	result := pr.Prompt("Enter name", "bob")

	if result != "bob" {
		t.Errorf("got %q, want %q", result, "bob")
	}
}

func TestPrompter_Prompt_NoFallback(t *testing.T) {
	var out bytes.Buffer
	pr := testPrompter(t, &out, "value\n")

	result := pr.Prompt("Enter", "")

	if result != "value" {
		t.Errorf("got %q, want %q", result, "value")
	}
	if !strings.Contains(out.String(), "Enter: ") {
		t.Errorf("expected plain prompt format, got %q", out.String())
	}
}

func TestPrompter_Continue_Yes(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"maybe\n", false},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		pr := testPrompter(t, &out, tt.input)

		got := pr.Continue("proceed?")
		if got != tt.want {
			t.Errorf("Continue with input %q: got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPrompter_Continue_ShowsPrompt(t *testing.T) {
	var out bytes.Buffer
	pr := testPrompter(t, &out, "n\n")

	pr.Continue("delete everything?")

	if !strings.Contains(out.String(), "delete everything?") {
		t.Errorf("expected prompt text, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[y/N]") {
		t.Errorf("expected [y/N] hint, got %q", out.String())
	}
}

func TestPrompter_HasPrinterMethods(t *testing.T) {
	var out bytes.Buffer
	pr := testPrompter(t, &out, "")

	pr.Info("info message")
	pr.Success("success message")

	got := out.String()
	if !strings.Contains(got, "info message") {
		t.Errorf("expected info, got %q", got)
	}
	if !strings.Contains(got, "✓") {
		t.Errorf("expected success marker, got %q", got)
	}
}

func TestNewPrompter_NonTerminalInput_ReturnsUnavailableWithoutWriting(t *testing.T) {
	var out bytes.Buffer
	pr, err := NewPrompter(
		withWriters(&out, &out),
		withTerminalCheck(func(input io.Reader) bool {
			if input != os.Stdin {
				t.Errorf("terminal check input = %T, want os.Stdin", input)
			}
			return false
		}),
	)
	if pr != nil {
		t.Fatal("NewPrompter() returned a prompter for non-terminal input")
	}
	if !errx.IsKind(err, errx.Unavailable) {
		t.Fatalf("NewPrompter() error kind = %v, want unavailable", err)
	}
	if !strings.Contains(err.Error(), "flag or configuration") {
		t.Errorf("NewPrompter() error = %q, want actionable flag or configuration guidance", err)
	}
	if got := out.String(); got != "" {
		t.Errorf("NewPrompter() wrote %q before returning an error", got)
	}
}

func TestNewPrompter_TerminalCheckAllowsInput(t *testing.T) {
	var out bytes.Buffer
	pr, err := NewPrompter(
		withWriters(&out, &out),
		withReader(strings.NewReader("value\n")),
		withTerminalCheck(func(io.Reader) bool { return true }),
	)
	if err != nil {
		t.Fatalf("NewPrompter() error = %v", err)
	}
	if got := pr.Prompt("Value", ""); got != "value" {
		t.Errorf("Prompt() = %q, want value", got)
	}
}
