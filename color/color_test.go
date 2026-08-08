package color

import (
	"bytes"
	"testing"
)

func TestEnabled_DefaultOffForNonTTY(t *testing.T) {
	var buf bytes.Buffer
	if Enabled(false, &buf) {
		t.Error("expected color off for non-TTY writer")
	}
}

func TestEnabled_SuppressExplicit(t *testing.T) {
	var buf bytes.Buffer
	if Enabled(true, &buf) {
		t.Error("expected color off when suppress=true")
	}
}

func TestEnabled_NoColorEnvWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if Enabled(false, &buf) {
		t.Error("expected color off when NO_COLOR is set")
	}
}

func TestEnabled_NoColorEmptyIgnored(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var buf bytes.Buffer
	if Enabled(false, &buf) {
		t.Error("expected color off for non-TTY even with empty NO_COLOR")
	}
}

func TestApply_Single(t *testing.T) {
	result := Apply("hello", Green)
	expected := ansiGreen + "hello" + ansiReset
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApply_Multiple(t *testing.T) {
	result := Apply("hello", Dim, Magenta)
	expected := ansiDim + ansiMagenta + "hello" + ansiReset
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApply_NoColors(t *testing.T) {
	result := Apply("hello")
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestApply_NoneColor(t *testing.T) {
	result := Apply("hello", None)
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}
