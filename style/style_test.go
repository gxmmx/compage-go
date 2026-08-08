package style

import (
	"bytes"
	"testing"
)

func TestApply_ColorOnly(t *testing.T) {
	got := Apply("hello", Red)
	want := ansiRed + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApply_ModOnly(t *testing.T) {
	got := Apply("hello", NoColor, Dim)
	want := ansiDim + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApply_ColorAndMod(t *testing.T) {
	got := Apply("hello", Green, Bold)
	want := ansiBold + ansiGreen + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApply_MultipleMods(t *testing.T) {
	got := Apply("hello", Magenta, Dim, Italic)
	want := ansiDim + ansiItalic + ansiMagenta + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApply_NoColorNoMod(t *testing.T) {
	got := Apply("hello", NoColor)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestBuild_Reusable(t *testing.T) {
	s := Build(Red, Bold)
	a := s.Apply("one")
	b := s.Apply("two")

	want := ansiBold + ansiRed
	if a != want+"one"+ansiReset {
		t.Errorf("first apply: got %q", a)
	}
	if b != want+"two"+ansiReset {
		t.Errorf("second apply: got %q", b)
	}
}

func TestBuild_ModOnlyStyle(t *testing.T) {
	s := Build(NoColor, Underline)
	got := s.Apply("link")
	want := ansiUnderline + "link" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnabled_NonTTY(t *testing.T) {
	var buf bytes.Buffer
	if Enabled(false, &buf) {
		t.Error("expected disabled for non-TTY writer")
	}
}

func TestEnabled_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	if Enabled(true, &buf) {
		t.Error("expected disabled when suppress=true")
	}
}

func TestEnabled_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if Enabled(false, &buf) {
		t.Error("expected disabled when NO_COLOR is set")
	}
}

func TestWhite(t *testing.T) {
	got := Apply("text", White)
	want := ansiWhite + "text" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
