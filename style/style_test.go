package style

import (
	"bytes"
	"testing"
)

func TestNew_NilWriter_Active(t *testing.T) {
	s := New(false, nil)
	if !s.Active() {
		t.Error("expected active with nil writer and no suppress")
	}
}

func TestNew_Suppress_Inactive(t *testing.T) {
	s := New(true, nil)
	if s.Active() {
		t.Error("expected inactive when suppress=true")
	}
}

func TestNew_NonTTY_Inactive(t *testing.T) {
	var buf bytes.Buffer
	s := New(false, &buf)
	if s.Active() {
		t.Error("expected inactive for non-TTY writer")
	}
}

func TestNew_NoColorEnv_Inactive(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	s := New(false, nil)
	if s.Active() {
		t.Error("expected inactive when NO_COLOR is set")
	}
}

func TestNew_NoColorEnv_OverridesTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	s := New(false, &buf)
	if s.Active() {
		t.Error("expected inactive when NO_COLOR is set regardless of writer")
	}
}

func TestStyler_Apply_Active_ColorOnly(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("hello", Red)
	want := ansiRed + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStyler_Apply_Active_ModOnly(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("hello", NoColor, Dim)
	want := ansiDim + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStyler_Apply_Active_ColorAndMod(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("hello", Green, Bold)
	want := ansiBold + ansiGreen + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStyler_Apply_Active_MultipleMods(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("hello", Magenta, Dim, Italic)
	want := ansiDim + ansiItalic + ansiMagenta + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStyler_Apply_Active_NoColorNoMod(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("hello", NoColor)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStyler_Apply_Inactive(t *testing.T) {
	s := New(true, nil)
	got := s.Apply("hello", Red, Bold)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStyler_Build_Active(t *testing.T) {
	s := New(false, nil)
	st := s.Build(Red, Bold)
	got := st.Apply("text")
	want := ansiBold + ansiRed + "text" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStyler_Build_Active_Reuse(t *testing.T) {
	s := New(false, nil)
	st := s.Build(Green)
	a := st.Apply("one")
	b := st.Apply("two")
	wantA := ansiGreen + "one" + ansiReset
	wantB := ansiGreen + "two" + ansiReset
	if a != wantA {
		t.Errorf("first: got %q, want %q", a, wantA)
	}
	if b != wantB {
		t.Errorf("second: got %q, want %q", b, wantB)
	}
}

func TestStyler_Build_Inactive(t *testing.T) {
	s := New(true, nil)
	st := s.Build(Red, Bold)
	got := st.Apply("text")
	if got != "text" {
		t.Errorf("got %q, want %q", got, "text")
	}
}

func TestStyler_Build_NoColorNoMod(t *testing.T) {
	s := New(false, nil)
	st := s.Build(NoColor)
	got := st.Apply("text")
	if got != "text" {
		t.Errorf("got %q, want %q", got, "text")
	}
}

func TestStyle_ZeroValue_Noop(t *testing.T) {
	var st Style
	got := st.Apply("text")
	if got != "text" {
		t.Errorf("got %q, want %q", got, "text")
	}
}

func TestStyler_ZeroValue_Inactive(t *testing.T) {
	var s Styler
	got := s.Apply("text", Red)
	if got != "text" {
		t.Errorf("got %q, want %q", got, "text")
	}
}

func TestRaw_AlwaysApplies(t *testing.T) {
	got := Raw("hello", Green, Bold)
	want := ansiBold + ansiGreen + "hello" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRaw_NoColorNoMod(t *testing.T) {
	got := Raw("hello", NoColor)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestColors(t *testing.T) {
	tests := []struct {
		color Color
		code  string
	}{
		{Red, ansiRed},
		{Green, ansiGreen},
		{Yellow, ansiYellow},
		{Blue, ansiBlue},
		{Magenta, ansiMagenta},
		{Cyan, ansiCyan},
		{White, ansiWhite},
		{BrightRed, ansiBrightRed},
		{BrightGreen, ansiBrightGreen},
		{BrightYellow, ansiBrightYellow},
		{BrightBlue, ansiBrightBlue},
		{BrightMagenta, ansiBrightMagenta},
		{BrightCyan, ansiBrightCyan},
		{BrightWhite, ansiBrightWhite},
	}

	s := New(false, nil)
	for _, tt := range tests {
		got := s.Apply("x", tt.color)
		want := tt.code + "x" + ansiReset
		if got != want {
			t.Errorf("color %d: got %q, want %q", tt.color, got, want)
		}
	}
}

func TestMods(t *testing.T) {
	tests := []struct {
		mod  Mod
		code string
	}{
		{Bold, ansiBold},
		{Dim, ansiDim},
		{Italic, ansiItalic},
		{Underline, ansiUnderline},
	}

	s := New(false, nil)
	for _, tt := range tests {
		got := s.Apply("x", NoColor, tt.mod)
		want := tt.code + "x" + ansiReset
		if got != want {
			t.Errorf("mod %d: got %q, want %q", tt.mod, got, want)
		}
	}
}

func TestModsBeforeColor(t *testing.T) {
	s := New(false, nil)
	got := s.Apply("x", Cyan, Bold, Underline)
	want := ansiBold + ansiUnderline + ansiCyan + "x" + ansiReset
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
