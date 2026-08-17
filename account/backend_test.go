package account

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"reflect"
	"testing"
	"time"
)

func TestHomeMode(t *testing.T) {
	t.Parallel()
	if got := homeMode(Spec{}); got != 0o755 {
		t.Errorf("homeMode() = %o, want 755", got)
	}
	if got := homeMode(Spec{HomeMode: fs.FileMode(0o700)}); got != 0o700 {
		t.Errorf("homeMode() = %o, want 700", got)
	}
}

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
		t.Fatal("apply() did not verify the fake-created group")
	}
	if len(completed) == 0 || completed[0].Field != "group record" {
		t.Fatalf("completed = %#v, want group record creation", completed)
	}
	if !runner.contains([]string{"groupadd", "worker"}) || !runner.contains([]string{"useradd", "--gid", "worker", "--no-create-home", "worker"}) {
		t.Fatalf("commands = %#v", runner.calls)
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
		t.Fatalf("commands = %#v, want %#v", runner.calls, want)
	}
}

func TestCommandFailureRetainsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("command failed")
	err := runCommand(context.Background(), &recordingRunner{err: cause}, "useradd", "worker")
	if !errors.Is(err, cause) {
		t.Fatalf("runCommand() error = %v, want wrapped %v", err, cause)
	}
}

func TestEnsureGroupRejectsUnverifiedGIDReconciliation(t *testing.T) {
	t.Parallel()
	gid := 200
	b := linuxBackend{backendDeps: backendDeps{store: fakeStore{group: &user.Group{Name: "worker", Gid: "100"}}, runner: outputRunner{value: "--gid"}}}
	_, err := b.ensureGroup(context.Background(), Spec{Group: "worker", GID: &gid})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("ensureGroup() error = %v, want DriftError", err)
	}
}

func TestHomeExistsUsesInjectedFilesystem(t *testing.T) {
	t.Parallel()
	b := linuxBackend{backendDeps: backendDeps{fs: fakeFS{err: fs.ErrNotExist}}}
	exists, err := b.homeExists(context.Background(), "/var/lib/worker")
	if err != nil || exists {
		t.Fatalf("homeExists() = %v, %v; want false, nil", exists, err)
	}
}

func TestEnsureHomeDirectoryRejectsExistingRegularFile(t *testing.T) {
	t.Parallel()
	err := ensureHomeDirectory(fakeFS{mkdirErr: fs.ErrExist, info: testFileInfo{mode: 0o644}}, "/var/lib/worker", 0o755)
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("ensureHomeDirectory() error = %v, want DriftError", err)
	}
}

func TestLinuxRejectsUnavailableCommandCapability(t *testing.T) {
	t.Parallel()
	b := linuxBackend{backendDeps: backendDeps{runner: &recordingRunner{}}}
	_, err := b.apply(context.Background(), Spec{Name: "worker", Kind: System, Existing: Reconcile}, nil)
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("apply() error = %v, want UnsupportedError", err)
	}
}

func TestDarwinUIDAllocationSkipsUsedIDs(t *testing.T) {
	t.Parallel()
	b := darwinBackend{backendDeps: backendDeps{runner: outputRunner{value: "daemon 500\nworker 501\n"}}}
	got, err := b.allocateUID(context.Background())
	if err != nil || got != 502 {
		t.Fatalf("allocateUID() = %d, %v; want 502, nil", got, err)
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
		t.Fatalf("ensureGroup() = %v, %v; want true, nil", created, err)
	}
	if !runner.contains([]string{"dscl", ".", "-create", "/Groups/worker", "PrimaryGroupID", "500"}) {
		t.Fatalf("commands = %#v", runner.calls)
	}
}

func TestDarwinModifyReconcilesSupplementaryGroupsExactly(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	b := darwinBackend{backendDeps: backendDeps{runner: runner}}
	spec := Spec{Name: "worker", Groups: []string{"metrics", "ops"}}
	if err := b.modify(context.Background(), spec, Record{Name: "worker", Groups: []string{"logs", "metrics"}}); err != nil {
		t.Fatalf("modify() error = %v", err)
	}
	want := [][]string{
		{"dseditgroup", "-o", "edit", "-a", "worker", "-t", "user", "ops"},
		{"dseditgroup", "-o", "edit", "-d", "worker", "-t", "user", "logs"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, want)
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
		t.Fatalf("commands = %#v, want %#v", runner.calls, want)
	}
}

type recordingRunner struct {
	calls [][]string
	err   error
}
type outputRunner struct{ value string }

type scriptRunner struct {
	calls   [][]string
	onCall  func(string, []string)
	helpOut string
}

func (r outputRunner) output(context.Context, string, ...string) (string, error) { return r.value, nil }
func (r *recordingRunner) output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return "output", r.err
}
func (r *scriptRunner) output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.onCall != nil {
		r.onCall(name, args)
	}
	if len(args) == 1 && args[0] == "--help" {
		return "--system --uid --gid --home-dir --home --shell --groups --no-create-home --create-home", nil
	}
	return "", nil
}
func (r *scriptRunner) contains(want []string) bool {
	for _, call := range r.calls {
		if sameCommand(call, want) {
			return true
		}
	}
	return false
}
func sameCommand(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

type fakeStore struct{ group *user.Group }

func (f fakeStore) lookup(string) (*user.User, error)     { return nil, errors.New("not implemented") }
func (f fakeStore) lookupID(string) (*user.User, error)   { return nil, errors.New("not implemented") }
func (f fakeStore) groupIDs(*user.User) ([]string, error) { return nil, nil }
func (f fakeStore) lookupGroup(string) (*user.Group, error) {
	if f.group == nil {
		return nil, errors.New("missing")
	}
	return f.group, nil
}
func (f fakeStore) lookupGroupID(string) (*user.Group, error) { return nil, errors.New("missing") }

type memoryStore struct {
	account *user.User
	groups  map[string]*user.Group
	ids     []string
}

type mutableStore struct{ groups map[string]*user.Group }

func (s *mutableStore) lookup(string) (*user.User, error) {
	return nil, user.UnknownUserError("worker")
}
func (s *mutableStore) lookupID(string) (*user.User, error) {
	return nil, errors.New("unknown user ID")
}
func (s *mutableStore) groupIDs(*user.User) ([]string, error) { return nil, nil }
func (s *mutableStore) lookupGroup(name string) (*user.Group, error) {
	group, ok := s.groups[name]
	if !ok {
		return nil, user.UnknownGroupError(name)
	}
	return group, nil
}
func (s *mutableStore) lookupGroupID(id string) (*user.Group, error) {
	for _, group := range s.groups {
		if group.Gid == id {
			return group, nil
		}
	}
	return nil, user.UnknownGroupIdError(id)
}

func lookupStore() memoryStore {
	return memoryStore{
		account: &user.User{Username: "worker", Uid: "101", Gid: "201"},
		groups: map[string]*user.Group{
			"201": {Name: "worker", Gid: "201"},
			"301": {Name: "logs", Gid: "301"},
		},
		ids: []string{"201", "301"},
	}
}
func (s memoryStore) lookup(name string) (*user.User, error) {
	if name != s.account.Username {
		return nil, user.UnknownUserError(name)
	}
	return s.account, nil
}
func (s memoryStore) lookupID(id string) (*user.User, error) {
	if id != s.account.Uid {
		return nil, errors.New("unknown user ID: " + id)
	}
	return s.account, nil
}
func (s memoryStore) groupIDs(*user.User) ([]string, error) { return s.ids, nil }
func (s memoryStore) lookupGroup(name string) (*user.Group, error) {
	for _, group := range s.groups {
		if group.Name == name {
			return group, nil
		}
	}
	return nil, user.UnknownGroupError(name)
}
func (s memoryStore) lookupGroupID(id string) (*user.Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, user.UnknownGroupIdError(id)
	}
	return group, nil
}

type fakeFS struct {
	err, mkdirErr error
	info          os.FileInfo
}

func (f fakeFS) stat(string) (os.FileInfo, error) { return f.info, f.err }
func (f fakeFS) mkdir(string, os.FileMode) error  { return f.mkdirErr }
func (fakeFS) chown(string, int, int) error       { return nil }
func (fakeFS) chmod(string, os.FileMode) error    { return nil }

type testFileInfo struct{ mode os.FileMode }

func (i testFileInfo) Name() string       { return "test" }
func (i testFileInfo) Size() int64        { return 0 }
func (i testFileInfo) Mode() os.FileMode  { return i.mode }
func (i testFileInfo) ModTime() time.Time { return time.Time{} }
func (i testFileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i testFileInfo) Sys() any           { return nil }
