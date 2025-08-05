package errors

import (
	"fmt"
	"testing"
)

func TestFuncIsAppError(t *testing.T) {
	err0 := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	err := New(KindCorruptedData, "InvalidXData", "suberror", err0)
	err2 := fmt.Errorf("regular error: %w", err)

	if !IsAppError(err) {
		t.Error("expected IsAppError to return true for ApplicationError")
	}

	if !IsAppError(err2) {
		t.Error("expected IsAppError to return true for wrapped ApplicationError")
	}

	if IsAppError(fmt.Errorf("regular error")) {
		t.Error("expected IsAppError to return false for non-ApplicationError")
	}
}

func TestFuncIsOfKindClass(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	if !IsOfKindClass(err, KindInternal, "ComponentFailure") {
		t.Error("expected IsOfKindClass to return true for matching kind and class")
	}

	if IsOfKindClass(err, KindNotFound, "ComponentFailure") {
		t.Error("expected IsOfKindClass to return false for non-matching kind")
	}

	if IsOfKindClass(err, KindInternal, "OtherClass") {
		t.Error("expected IsOfKindClass to return false for non-matching class")
	}
}

func TestFuncIsOfKind(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	if !IsOfKind(err, KindInternal) {
		t.Error("expected IsOfKind to return true for matching kind")
	}

	if IsOfKind(err, KindNotFound) {
		t.Error("expected IsOfKind to return false for non-matching kind")
	}
}

func TestFuncIsOfKinds(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	if !IsOfKinds(err, KindInternal, KindNotFound) {
		t.Error("expected IsOfKinds to return true for matching kind")
	}

	if IsOfKinds(err, KindNotFound, KindInvalidInput) {
		t.Error("expected IsOfKinds to return false for non-matching kinds")
	}
}

func TestFuncIsOfClass(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	if !IsOfClass(err, "ComponentFailure") {
		t.Error("expected IsOfClass to return true for matching class")
	}

	if IsOfClass(err, "OtherClass") {
		t.Error("expected IsOfClass to return false for non-matching class")
	}
}

func TestFuncIsOfClasses(t *testing.T) {
	err := New(KindInternal, "ComponentFailure", "newerror", fmt.Errorf("suberror"))
	if !IsOfClasses(err, "OtherClass", "ComponentFailure") {
		t.Error("expected IsOfClasses to return true for matching class")
	}

	if IsOfClasses(err, "OtherClass", "AnotherClass") {
		t.Error("expected IsOfClasses to return false for non-matching classes")
	}
}
