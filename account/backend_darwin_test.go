package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"os/user"
)

func TestDarwinUIDAllocationSkipsUsedIDs(t *testing.T) {
	t.Parallel()
	b := darwinBackend{backendDeps: backendDeps{runner: outputRunner{value: "daemon 500\nworker 501\n"}}}
	got, err := b.allocateUID(context.Background())
	if err != nil || got != 502 {
		t.Fatalf("allocateUID() = %d, %v", got, err)
	}
}
func TestDarwinLookupParsesDirectoryServiceAttributes(t *testing.T) {
	t.Parallel()
	b := darwinBackend{backendDeps: backendDeps{store: lookupStore(), runner: outputRunner{value: "NFSHomeDirectory: /srv/worker\nUserShell: /usr/bin/false\n"}}}
	got, err := b.lookup(context.Background(), "worker", false)
	if err != nil {
		t.Fatalf("lookup() error = %v", err)
	}
	if got.Home != "/srv/worker" || got.Shell != "/usr/bin/false" || got.Group != "worker" || !reflect.DeepEqual(got.Groups, []string{"logs"}) {
		t.Fatalf("lookup() = %#v", got)
	}
}

func TestDarwinLookupRejectsMissingDirectoryServiceAttribute(t *testing.T) {
	t.Parallel()
	b := darwinBackend{backendDeps: backendDeps{store: lookupStore(), runner: outputRunner{value: "NFSHomeDirectory: /srv/worker\n"}}}
	if _, err := b.lookup(context.Background(), "worker", false); err == nil {
		t.Fatal("lookup() succeeded without UserShell")
	}
}
func TestDarwinEnsureGroupAllocatesAndVerifiesGID(t *testing.T) {
	t.Parallel()
	store := &mutableStore{groups: map[string]*user.Group{}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "dseditgroup" && sameCommand(append([]string{name}, args...), []string{"dseditgroup", "-o", "create", "worker"}) {
			store.groups["worker"] = &user.Group{Name: "worker", Gid: "500"}
		}
	}}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	created, err := b.ensureGroup(context.Background(), Spec{Group: "worker"})
	if err != nil || !created {
		t.Fatalf("ensureGroup() = %v, %v", created, err)
	}
	if !runner.contains([]string{"dscl", ".", "-create", "/Groups/worker", "PrimaryGroupID", "500"}) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}

func TestDarwinApplyKeepsCompletedGroupAfterAccountFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("dscl user creation failed")
	store := &mutableStore{groups: map[string]*user.Group{}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "dseditgroup" && len(args) == 3 {
			store.groups["worker"] = &user.Group{Name: "worker", Gid: "500"}
		}
	}, errFor: func(name string, args []string) error {
		if name == "dscl" && sameCommand(append([]string{name}, args...), []string{"dscl", ".", "-create", "/Users/worker"}) {
			return cause
		}
		return nil
	}}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	completed, err := b.apply(context.Background(), Spec{Name: "worker", Group: "worker", Existing: Reconcile}, nil)
	if !errors.Is(err, cause) || len(completed) != 1 || completed[0].Field != "group record" {
		t.Fatalf("apply() = %#v, %v", completed, err)
	}
}
func TestDarwinModifyReconcilesSupplementaryGroupsExactly(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := darwinBackend{backendDeps: backendDeps{runner: runner}}
	if err := b.modify(context.Background(), Spec{Name: "worker", Groups: []string{"metrics", "ops"}}, Record{Name: "worker", Groups: []string{"logs", "metrics"}}); err != nil {
		t.Fatalf("modify() error = %v", err)
	}
	want := [][]string{{"dseditgroup", "-o", "edit", "-a", "worker", "-t", "user", "ops"}, {"dseditgroup", "-o", "edit", "-d", "worker", "-t", "user", "logs"}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}
func TestDarwinModifyDoesNotChangeCreationOnlyKind(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := darwinBackend{backendDeps: backendDeps{runner: runner}}
	if err := b.modify(context.Background(), Spec{Name: "worker", Shell: "/bin/zsh"}, Record{Name: "worker"}); err != nil {
		t.Fatalf("modify() error = %v", err)
	}
	want := []string{"dscl", ".", "-create", "/Users/worker", "UserShell", "/bin/zsh"}
	if len(runner.calls) != 1 || !sameCommand(runner.calls[0], want) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}

func TestDarwinPreflightUsesOnlyFakeCapabilityCalls(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{err: errors.New("dscl unavailable")}
	b := darwinBackend{backendDeps: backendDeps{runner: runner}}
	err := b.preflight(context.Background(), Spec{Name: "worker", Group: "worker"}, false)
	if !errors.Is(err, runner.err) || len(runner.calls) != 1 || !sameCommand(runner.calls[0], []string{"dscl", "-help"}) {
		t.Fatalf("preflight() error = %v, commands = %#v", err, runner.calls)
	}
}
