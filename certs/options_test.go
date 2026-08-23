package certs

import (
	"context"
	"testing"
	"time"
)

func TestRotateBeforeRejectsNegative(t *testing.T) {
	store := newTestFileStore(t)
	authority, err := NewAuthorityManager(WithStore(store), WithAuthorityName("Test CA"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = authority.Ensure(context.Background(), WithRootRotateBefore(-time.Second)); err == nil {
		t.Fatal("expected validation error")
	}
}
