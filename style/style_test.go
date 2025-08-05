package style

import "testing"

func TestNewStyle(t *testing.T) {
	// Test creating a new style instance
	s := New()
	if s == nil {
		t.Error("Expected a new Style instance, got nil")
	}
}

func TestNewApplyNoTTY(t *testing.T) {
	// Test applying a style to text
	result := Apply("Hello, World!", "bold")
	expected := "Hello, World!"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestInvalidAttribute(t *testing.T) {
	// Test applying an invalid attribute
	result := New().forceTTY().Attributes("invalid").Apply("Hello, World!")
	expected := "Hello, World!\033[0m"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestInvalidColor(t *testing.T) {
	// Test applying an invalid color
	result := New().forceTTY().Color("invalid").Apply("Hello, World!")
	expected := "\033[0mHello, World!\033[0m"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestNewColor(t *testing.T) {
	// Test applying a color to text
	result := Color("red").forceTTY().Apply("Hello, World!")
	expected := "\033[31mHello, World!\033[0m"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestNewCode(t *testing.T) {

	t.Run("Empty", func(t *testing.T) {
		result := New().forceTTY().Code()
		expected := ""
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
	t.Run("WithAttribute", func(t *testing.T) {
		result := New().forceTTY().Attributes("bold,italic").Code()
		expected := "\033[1m\033[3m"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
	t.Run("WithoutTTY", func(t *testing.T) {
		result := New().Code()
		expected := ""
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
}

func TestConstructors(t *testing.T) {
	// Test the shorthand constructors
	tests := []struct {
		name     string
		f        func() Style
		expected string
	}{
		{"Bold", Bold, "\033[1m"},
		{"Faint", Faint, "\033[2m"},
		{"Italic", Italic, "\033[3m"},
		{"Underline", Underline, "\033[4m"},
		{"Black", Black, "\033[30m"},
		{"Red", Red, "\033[31m"},
		{"Green", Green, "\033[32m"},
		{"Yellow", Yellow, "\033[33m"},
		{"Blue", Blue, "\033[34m"},
		{"Magenta", Magenta, "\033[35m"},
		{"Cyan", Cyan, "\033[36m"},
		{"White", White, "\033[37m"},
		{"BlackB", BlackB, "\033[90m"},
		{"RedB", RedB, "\033[91m"},
		{"GreenB", GreenB, "\033[92m"},
		{"YellowB", YellowB, "\033[93m"},
		{"BlueB", BlueB, "\033[94m"},
		{"MagentaB", MagentaB, "\033[95m"},
		{"CyanB", CyanB, "\033[96m"},
		{"WhiteB", WhiteB, "\033[97m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.f().forceTTY().Apply("Test")
			expected := tt.expected + "Test\033[0m"
			if result != expected {
				t.Errorf("Expected %s, got %s", expected, result)
			}
		})
	}
}
