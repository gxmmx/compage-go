package platform

import (
	"os"
	"testing"
)

func TestBinaryName(t *testing.T) {
	// Mock os.Args[0]
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	mockArgs := []string{"/path/to/mock-binary"}
	os.Args = mockArgs

	expected := "mock-binary"
	binName := BinaryName()
	if binName != expected {
		t.Errorf("AppNameFromBin() = %q; expected %q", binName, expected)
	}
}
