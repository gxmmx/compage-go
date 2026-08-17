package account

import (
	"context"
	"errors"
	"os/user"
	"reflect"
	"testing"
)

func TestLinuxCreateRendersDeclaredSpec(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := linuxBackend{backendDeps: backendDeps{runner: runner}}
	uid := 42
	err := b.create(context.Background(), Spec{Name: "worker", Kind: System, UID: &uid, Group: "worker", Home: "/var/lib/worker", HomePolicy: EnsureHome, Shell: NoLoginShell, Groups: []string{"logs", "metrics"}})
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	want := []string{"useradd", "--system", "--uid", "42", "--gid", "worker", "--home-dir", "/var/lib/worker", "--create-home", "--shell", NoLoginShell, "--groups", "logs,metrics", "worker"}
	if !sameCommand(runner.calls[0], want) {
		t.Errorf("command = %#v, want %#v", runner.calls[0], want)
	}
}
func TestLinuxCreateDisablesImplicitHomeCreation(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := linuxBackend{backendDeps: backendDeps{runner: runner}}
	if err := b.create(context.Background(), Spec{Name: "worker", Kind: Regular, Group: "worker"}); err != nil {
		t.Fatalf("create() error = %v", err)
	}
	if !sameCommand(runner.calls[0], []string{"useradd", "--gid", "worker", "--no-create-home", "worker"}) {
		t.Fatalf("command = %#v", runner.calls[0])
	}
}
func TestLinuxLookupParsesPasswdAndGroups(t *testing.T) {
	t.Parallel()
	b := linuxBackend{backendDeps: backendDeps{store: lookupStore(), runner: outputRunner{value: "worker:x:101:201:Worker:/srv/worker:/usr/sbin/nologin\n"}}}
	got, err := b.lookup(context.Background(), "worker", false)
	if err != nil {
		t.Fatalf("lookup() error = %v", err)
	}
	want := Record{Name: "worker", UID: "101", GID: "201", Group: "worker", Home: "/srv/worker", Shell: "/usr/sbin/nologin", Groups: []string{"logs"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lookup() = %#v, want %#v", got, want)
	}
}
func TestLinuxLookupRejectsMalformedPasswd(t *testing.T) {
	t.Parallel()
	b := linuxBackend{backendDeps: backendDeps{store: lookupStore(), runner: outputRunner{value: "not-a-passwd-record"}}}
	if _, err := b.lookup(context.Background(), "worker", false); err == nil {
		t.Fatal("lookup() succeeded for malformed passwd record")
	}
}
func TestLinuxApplyCreatesAndVerifiesPrimaryGroup(t *testing.T) {
	t.Parallel()
	store := &mutableStore{groups: map[string]*user.Group{}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "groupadd" && len(args) == 1 && args[0] == "worker" {
			store.groups["worker"] = &user.Group{Name: "worker", Gid: "900"}
		}
	}}
	b := linuxBackend{backendDeps: backendDeps{store: store, runner: runner}}
	completed, err := b.apply(context.Background(), Spec{Name: "worker", Kind: Regular, Group: "worker", Existing: Reconcile}, nil)
	if err != nil {
		t.Fatalf("apply() error = %v", err)
	}
	if _, ok := store.groups["worker"]; !ok {
		t.Fatal("group was not verified")
	}
	if len(completed) == 0 || completed[0].Field != "group record" {
		t.Fatalf("completed = %#v", completed)
	}
	if !runner.contains([]string{"groupadd", "worker"}) || !runner.contains([]string{"useradd", "--gid", "worker", "--no-create-home", "worker"}) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}

func TestLinuxApplyKeepsCompletedGroupAfterAccountFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("useradd failed")
	store := &mutableStore{groups: map[string]*user.Group{}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "groupadd" {
			store.groups[args[len(args)-1]] = &user.Group{Name: args[len(args)-1], Gid: "900"}
		}
	}, errFor: func(name string, _ []string) error {
		if name == "useradd" {
			return cause
		}
		return nil
	}}
	b := linuxBackend{backendDeps: backendDeps{store: store, runner: runner}}
	completed, err := b.apply(context.Background(), Spec{Name: "worker", Kind: Regular, Group: "worker", Existing: Reconcile}, nil)
	if !errors.Is(err, cause) || len(completed) != 1 || completed[0].Field != "group record" {
		t.Fatalf("apply() = %#v, %v", completed, err)
	}
}
func TestLinuxModifyReplacesSupplementaryGroupsExactly(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := linuxBackend{backendDeps: backendDeps{runner: runner}}
	if err := b.modify(context.Background(), Spec{Name: "worker", Groups: []string{}}); err != nil {
		t.Fatalf("modify() error = %v", err)
	}
	want := []string{"usermod", "--groups", "", "worker"}
	if len(runner.calls) != 1 || !sameCommand(runner.calls[0], want) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}
func TestLinuxGroupGIDVerificationReturnsDrift(t *testing.T) {
	t.Parallel()
	gid := 200
	b := linuxBackend{backendDeps: backendDeps{store: fakeStore{group: &user.Group{Name: "worker", Gid: "100"}}, runner: outputRunner{value: "--gid"}}}
	_, err := b.ensureGroup(context.Background(), Spec{Group: "worker", GID: &gid})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("ensureGroup() error = %v", err)
	}
}
func TestLinuxRejectsUnavailableCommandCapability(t *testing.T) {
	t.Parallel()
	b := linuxBackend{backendDeps: backendDeps{runner: &recordingRunner{}}}
	_, err := b.apply(context.Background(), Spec{Name: "worker", Kind: System, Existing: Reconcile}, nil)
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("apply() error = %v", err)
	}
}

func TestLinuxPreflightChecksRequiredCommandsWithoutMutation(t *testing.T) {
	t.Parallel()
	runner := &scriptRunner{}
	b := linuxBackend{backendDeps: backendDeps{runner: runner}}
	uid, gid := 101, 201
	spec := Spec{Name: "worker", Kind: System, UID: &uid, Group: "worker", GID: &gid, Groups: []string{"logs"}, Home: "/srv/worker", HomePolicy: EnsureHome, Shell: NoLoginShell}
	if err := b.preflight(context.Background(), spec, false); err != nil {
		t.Fatalf("preflight() error = %v", err)
	}
	if !runner.contains([]string{"useradd", "--help"}) || !runner.contains([]string{"groupadd", "--help"}) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}
