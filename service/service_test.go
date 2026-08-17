package service

import (
	"context"
	"errors"
	"github.com/gxmmx/compage-go/account"
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

func TestEnsureAccountFailurePreventsDefinitionWrite(t *testing.T) {
	cause := errors.New("account failed")
	f := &fakeFiles{values: map[string][]byte{"/bin/x": {}}}
	called := false
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System, account: &account.Spec{Name: "worker"}}, backend: systemdBackend{}, files: f, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}, ensureAccount: func(context.Context, account.Spec) (account.EnsureResult, error) {
		called = true
		return account.EnsureResult{}, cause
	}}
	_, err := o.ensure(context.Background())
	if !called || !errors.Is(err, cause) {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if _, exists := f.values[systemdPath(&o)]; exists {
		t.Fatal("definition written after account failure")
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
