package utils

import (
	"os"
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

func TestAppNameFromBin(t *testing.T) {
	// Mock os.Args[0]
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	mockArgs := []string{"/path/to/mock-binary"}
	os.Args = mockArgs

	expected := "mock-binary"
	binName := AppNameFromBin()
	if binName != expected {
		t.Errorf("AppNameFromBin() = %q; expected %q", binName, expected)
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
