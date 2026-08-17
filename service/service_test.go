package service

import (
	"context"
	"errors"
	"testing"
)

func TestValidateRejectsUnsafeNamesAndEnvironment(t *testing.T) {
	for _, name := range []string{"", ".", "../x", "x/y", "-x", "x y"} {
		if err := validate(specification{name: name, binary: "/bin/x", scope: User}); err == nil {
			t.Errorf("validate(%q) succeeded", name)
		}
	}
	if err := validate(specification{name: "x", binary: "/bin/x", scope: User, env: map[string]string{"A-B": "x"}}); err == nil {
		t.Fatal("invalid environment accepted")
	}
}

func TestOperationRejectsCancelledLifecycleCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := operation{backend: systemdBackend{}, runner: &fakeRunner{}}
	if err := o.start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("start error=%v", err)
	}
}

func TestBackendNameValidation(t *testing.T) {
	if err := (systemdBackend{}).validate(specification{name: "x.service"}); err == nil {
		t.Fatal("systemd suffix accepted")
	}
	if err := (launchdBackend{}).validate(specification{name: "worker"}); err == nil {
		t.Fatal("launchd non-label accepted")
	}
}
