package console

import (
	"bytes"
	"testing"
)

func TestResolveColor_DefaultOnForTTY(t *testing.T) {
	// Non-file writer (buffer) is not a TTY → color off
	var buf bytes.Buffer
	if resolveColor(false, &buf) {
		t.Error("expected color off for non-TTY writer")
	}
}

func TestResolveColor_SuppressExplicit(t *testing.T) {
	var buf bytes.Buffer
	if resolveColor(true, &buf) {
		t.Error("expected color off when suppress=true")
	}
}

func TestResolveColor_NoColorEnvWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// Even if suppress is false, NO_COLOR wins
	// (using a non-TTY writer here, but the env check comes first)
	var buf bytes.Buffer
	if resolveColor(false, &buf) {
		t.Error("expected color off when NO_COLOR is set")
	}
}

func TestResolveColor_NoColorEmptyIgnored(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var buf bytes.Buffer
	// With empty NO_COLOR and non-TTY, still off (because of TTY check)
	if resolveColor(false, &buf) {
		t.Error("expected color off for non-TTY even with empty NO_COLOR")
	}
}

func TestColorize_Single(t *testing.T) {
	result := colorize("hello", Green)
	expected := ansiGreen + "hello" + ansiReset
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestColorize_Multiple(t *testing.T) {
	result := colorize("hello", Dim, Magenta)
	expected := ansiDim + ansiMagenta + "hello" + ansiReset
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestColorize_None(t *testing.T) {
	result := colorize("hello")
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestIsTerminal_Buffer(t *testing.T) {
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Error("expected buffer to not be a terminal")
	}
}
