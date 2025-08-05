package errors

import (
	"fmt"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestNewError(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", nil)
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if err.Error() != "Internal (ComponentFailure): newerror" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestNewErrorRegularSub(t *testing.T) {
	subErr := fmt.Errorf("suberror")
	err := New(KindInternal, "ComponentFailure", "newerror", subErr)
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if err.Error() != "Internal (ComponentFailure): newerror" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if err.Chain() != "Internal (ComponentFailure): newerror -> suberror" {
		t.Errorf("unexpected error chain: %s", err.Chain())
	}
}

func TestNewErrorAppErrorSub(t *testing.T) {
	subErr := New(KindCorruptedData, "InvalidXData", "suberror", nil).WithFields(map[string]any{
		"f1": "v1", "f2": "v2",
	})
	err := New(KindInternal, "ComponentFailure", "newerror", subErr).WithField("f3", "v3").WithClass("Component2Failure")

	if err.Error() != "Internal (Component2Failure): newerror" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if err.Chain() != "Internal (Component2Failure): newerror -> CorruptedData (InvalidXData): suberror" {
		t.Errorf("unexpected error chain: %s", err.Chain())
	}

	if len(err.Fields()) != 3 {
		t.Errorf("expected 3 fields, got %d", len(err.Fields()))
	}

	if !strings.Contains(err.Caller(), "errors_test.go:") {
		t.Errorf("expected caller to contain 'errors_test.go:', got %s", err.Caller())
	}
}

func TestErrorIs(t *testing.T) {
	subErr := New(KindCorruptedData, "InvalidXData", "suberror", nil)
	err := New(KindInternal, "ComponentFailure", "newerror", subErr)

	if !err.IsKind(KindInternal) {
		t.Error("expected error to be of kind Internal")
	}

	errtomatch := New(KindInternal, "ComponentFailure", "newerror", nil)
	errnottomatch := New(KindInternal, "OtherClass", "newerror", nil)

	if !err.Is(errtomatch) {
		t.Error("expected error to match passed error")
	}

	if err.Is(errnottomatch) {
		t.Error("expected error not to match different class error")
	}

	if err.IsKind(KindCorruptedData) {
		t.Error("expected error not to be of kind CorruptedData")
	}

	if !err.IsClass("ComponentFailure") {
		t.Error("expected error to be of class ComponentFailure")
	}
}

func TestErrorMaxDepth(t *testing.T) {
	subErr := New(KindCorruptedData, "InvalidXData", "suberror", nil)
	err := New(KindInternal, "ComponentFailure", "newerror", subErr)

	// Create a chain that exceeds max depth
	for i := 0; i < maxChainDepth+1; i++ {
		err = New(KindInternal, "ComponentFailure", fmt.Sprintf("nested error %d", i), err)
	}

	if !strings.Contains(err.Chain(), "max depth reached") {
		t.Error("expected error chain to indicate max depth reached")
	}
}
