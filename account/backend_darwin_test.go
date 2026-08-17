package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"io/fs"
	"os/user"
	"syscall"

	"github.com/gxmmx/compage-go/host"
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

func TestDarwinReconcilesAndVerifiesExistingGroupGID(t *testing.T) {
	t.Parallel()
	store := &mutableStore{groups: map[string]*user.Group{"worker": {Name: "worker", Gid: "100"}}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "dscl" && sameCommand(append([]string{name}, args...), []string{"dscl", ".", "-create", "/Groups/worker", "PrimaryGroupID", "200"}) {
			store.groups["worker"].Gid = "200"
		}
	}}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	gid := 200
	created, err := b.ensureGroup(context.Background(), Spec{Group: "worker", GID: &gid})
	if err != nil || created || store.groups["worker"].Gid != "200" {
		t.Fatalf("ensureGroup() = %v, %v; group = %#v", created, err, store.groups["worker"])
	}
}

func TestDarwinCreateRendersDeclaredAttributesInOrder(t *testing.T) {
	t.Parallel()
	store := &mutableStore{groups: map[string]*user.Group{"worker": {Name: "worker", Gid: "201"}}}
	runner := &recordingRunner{}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	uid := 101
	spec := Spec{Name: "worker", Kind: System, UID: &uid, Group: "worker", Home: "/srv/worker", HomePolicy: EnsureHome, Shell: "/usr/bin/false", Groups: []string{"logs"}}
	if err := b.create(context.Background(), spec); err != nil {
		t.Fatalf("create() error = %v", err)
	}
	want := [][]string{
		{"dscl", ".", "-create", "/Users/worker"},
		{"dscl", ".", "-create", "/Users/worker", "IsHidden", "1"},
		{"dscl", ".", "-create", "/Users/worker", "UniqueID", "101"},
		{"dscl", ".", "-create", "/Users/worker", "PrimaryGroupID", "201"},
		{"dscl", ".", "-create", "/Users/worker", "NFSHomeDirectory", "/srv/worker"},
		{"dscl", ".", "-create", "/Users/worker", "UserShell", "/usr/bin/false"},
		{"dseditgroup", "-o", "edit", "-a", "worker", "-t", "user", "logs"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, want)
	}
}

func TestDarwinEnsureCreatesThenRechecksObservedRecord(t *testing.T) {
	t.Parallel()
	store := &mutableStore{groups: map[string]*user.Group{"worker": {Name: "worker", Gid: "201"}}, ids: []string{"201"}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "dscl" && sameCommand(append([]string{name}, args...), []string{"dscl", ".", "-create", "/Users/worker"}) {
			store.account = &user.User{Username: "worker", Uid: "101", Gid: "201"}
		}
	}, outFor: func(name string, args []string) string {
		if name == "dscl" && len(args) > 2 && args[1] == "-read" {
			return "NFSHomeDirectory: /srv/worker\nUserShell: /usr/bin/false\n"
		}
		return ""
	}}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	op := operation{deps: dependencies{platform: host.PlatformInfo{OS: host.Darwin}, root: true, backend: b}}
	uid := 101
	got, err := op.run(context.Background(), Spec{Name: "worker", Kind: Regular, UID: &uid, Group: "worker", Shell: "/usr/bin/false", Groups: []string{}}, true)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !got.Created || got.Account.Name != "worker" || got.Account.UID != "101" {
		t.Fatalf("Ensure() = %#v", got)
	}
}

func TestDarwinEnsureRechecksExactSupplementaryGroups(t *testing.T) {
	t.Parallel()
	store := &mutableStore{account: &user.User{Username: "worker", Uid: "101", Gid: "201"}, groups: map[string]*user.Group{"worker": {Name: "worker", Gid: "201"}, "logs": {Name: "logs", Gid: "301"}, "metrics": {Name: "metrics", Gid: "302"}}, ids: []string{"201", "301"}}
	runner := &scriptRunner{onCall: func(name string, args []string) {
		if name == "dseditgroup" && len(args) > 3 {
			store.ids = []string{"201", "302"}
		}
	}, outFor: func(name string, args []string) string {
		if name == "dscl" && len(args) > 2 && args[1] == "-read" {
			return "NFSHomeDirectory: /srv/worker\nUserShell: /usr/bin/false\n"
		}
		return ""
	}}
	b := darwinBackend{backendDeps: backendDeps{store: store, runner: runner}}
	op := operation{deps: dependencies{platform: host.PlatformInfo{OS: host.Darwin}, root: true, backend: b}}
	got, err := op.run(context.Background(), Spec{Name: "worker", Groups: []string{"metrics"}, Existing: Reconcile}, true)
	if err != nil || !reflect.DeepEqual(got.Account.Groups, []string{"metrics"}) {
		t.Fatalf("Ensure() = %#v, %v", got, err)
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

func TestDarwinEnsureHomeCreatesOwnsChmodsAndVerifies(t *testing.T) {
	t.Parallel()
	files := &trackingFS{info: testFileInfo{mode: fs.ModeDir | 0o700, sys: &syscall.Stat_t{Uid: 101, Gid: 201}}}
	b := darwinBackend{backendDeps: backendDeps{store: lookupStore(), runner: outputRunner{value: "NFSHomeDirectory: /srv/worker\nUserShell: /usr/bin/false\n"}, fs: files}}
	if err := b.ensureHome(context.Background(), Spec{Name: "worker", Home: "/srv/worker", HomeMode: 0o700}); err != nil {
		t.Fatalf("ensureHome() error = %v", err)
	}
	if files.mkdirs != 1 || files.chowns != 1 || files.chmods != 1 || files.lastMode != 0o700 {
		t.Fatalf("filesystem calls = %#v", files)
	}
}
