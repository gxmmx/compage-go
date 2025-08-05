package errors

import (
	"fmt"
	"testing"
)

func TestSlogSimple(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", nil).WithFields(map[string]any{
		"key1": "value1",
		"key2": "value2",
	})
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	msg, _ := err.Slog()
	if msg != "Internal (ComponentFailure): newerror" {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestSlogMaxDepth(t *testing.T) {
	subErr := New(KindCorruptedData, "InvalidXData", "suberror", nil)
	for i := 0; i < maxChainDepth+1; i++ {
		subErr = New(KindInternal, "ComponentFailure", "newerror", subErr)
	}

	msg, fields := subErr.Slog()
	if msg != "Internal (ComponentFailure): newerror" {
		t.Errorf("unexpected message: %s", msg)
	}

	if len(fields) == 0 {
		t.Fatal("expected non-empty fields")
	}
}

func TestSlogNestedErrors(t *testing.T) {
	err0 := fmt.Errorf("some error")
	err1 := fmt.Errorf("another error: %w", err0)

	err3 := New(KindCorruptedData, "InvalidXData", "suberror", err1)
	msg, fields := err3.Slog()

	if msg != "CorruptedData (InvalidXData): suberror" {
		t.Errorf("unexpected message: %s", msg)
	}

	if len(fields) == 0 {
		t.Fatal("expected non-empty fields")
	}
}
