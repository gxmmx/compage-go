package errors

import (
	"fmt"
	"testing"
)

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
