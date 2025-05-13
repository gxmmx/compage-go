package fileutils

import (
	"os"
	"testing"
)

func TestFetchStringFromFile(t *testing.T) {
	t.Run("should return empty string for empty path", func(t *testing.T) {
		result, err := FetchStringFromFile("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})
	t.Run("should return error for non-existing file", func(t *testing.T) {
		result, err := FetchStringFromFile("./nonexisting_file.txt")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})
	t.Run("should return content for existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, err := os.CreateTemp(tmpDir, "testfile-*.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		content := "Hello, World!"
		tmpFile.WriteString(content)
		tmpFile.Close()

		result, err := FetchStringFromFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != content {
			t.Errorf("expected '%s', got '%s'", content, result)
		}
	})
	t.Run("should return error for unreadable file", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile, err := os.CreateTemp(tmpDir, "testfile-*.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		content := "Hello, World!"
		tmpFile.WriteString(content)
		tmpFile.Close()

		// Make the file unreadable
		if err := os.Chmod(tmpFile.Name(), 0000); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := FetchStringFromFile(tmpFile.Name())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})
}
