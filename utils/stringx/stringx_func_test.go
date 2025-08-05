package stringx

import (
	"testing"
)

func TestSlugifyString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World!!", "hello-world"},
		{"GoLang !!101", "golang-101"},
		{"Test@123", "test-123"},
		{"  Spaces   and   $omething", "spaces-and-omething"},
	}

	for _, test := range tests {
		result := SlugifyString(test.input)
		if result != test.expected {
			t.Errorf("SlugifyString(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestEnvifyString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World!!", "HELLO_WORLD"},
		{"GoLang !!101", "GOLANG_101"},
		{"Test@123", "TEST_123"},
		{"  Spaces   and   $omething", "SPACES_AND_OMETHING"},
	}

	for _, test := range tests {
		result := EnvifyString(test.input)
		if result != test.expected {
			t.Errorf("EnvifyString(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestKeyifyString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World!!", "hello.world"},
		{"GoLang !!101", "golang.101"},
		{"Test@123", "test.123"},
		{"  Spaces   and   $omething", "spaces.and.omething"},
	}

	for _, test := range tests {
		result := KeyifyString(test.input)
		if result != test.expected {
			t.Errorf("KeyifyString(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		input    string
		prefix   string
		expected string
	}{
		{"prefix_value", "prefix_", "value"},
		{"prefix.value", "prefix", "value"},
		{"value", "prefix_", "value"},
		{"value", "prefix", "value"},
		{"_value", "prefix", "_value"},
		{"-value", "prefix", "-value"},
		{".value", "prefix", ".value"},
		{"prefixvalue", "prefix", "value"},
	}

	for _, test := range tests {
		result := StripPrefix(test.input, test.prefix)
		if result != test.expected {
			t.Errorf("StripPrefix(%q, %q) = %q; expected %q", test.input, test.prefix, result, test.expected)
		}
	}
}

func TestEnsureLeadingSlash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"path/to/file", "/path/to/file"},
		{"/already/has/slash", "/already/has/slash"},
		{"", "/"},
	}

	for _, test := range tests {
		result := EnsureLeadingSlash(test.input)
		if result != test.expected {
			t.Errorf("EnsureLeadingSlash(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}
