package console

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrompter_Prompt_WithInput(t *testing.T) {
	var out bytes.Buffer
	input := strings.NewReader("custom value\n")
	pr := NewPrompter(
		PrinterOptFor(WithOutTo(&out)),
		PrinterOptFor(WithColor(false)),
		WithInput(input),
	)

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
	input := strings.NewReader("\n")
	pr := NewPrompter(
		PrinterOptFor(WithOutTo(&out)),
		PrinterOptFor(WithColor(false)),
		WithInput(input),
	)

	result := pr.Prompt("Enter name", "bob")

	if result != "bob" {
		t.Errorf("got %q, want %q", result, "bob")
	}
}

func TestPrompter_Prompt_NoFallback(t *testing.T) {
	var out bytes.Buffer
	input := strings.NewReader("value\n")
	pr := NewPrompter(
		PrinterOptFor(WithOutTo(&out)),
		PrinterOptFor(WithColor(false)),
		WithInput(input),
	)

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
		pr := NewPrompter(
			PrinterOptFor(WithOutTo(&out)),
			PrinterOptFor(WithColor(false)),
			WithInput(strings.NewReader(tt.input)),
		)

		got := pr.Continue("proceed?")
		if got != tt.want {
			t.Errorf("Continue with input %q: got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPrompter_Continue_ShowsPrompt(t *testing.T) {
	var out bytes.Buffer
	pr := NewPrompter(
		PrinterOptFor(WithOutTo(&out)),
		PrinterOptFor(WithColor(false)),
		WithInput(strings.NewReader("n\n")),
	)

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
	pr := NewPrompter(
		PrinterOptFor(WithOutTo(&out)),
		PrinterOptFor(WithColor(false)),
		WithInput(strings.NewReader("")),
	)

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
