package account

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"syscall"
	"testing"
	"time"
)

func TestHomeMode(t *testing.T) {
	t.Parallel()
	if got := homeMode(Spec{}); got != 0o755 {
		t.Errorf("homeMode() = %o", got)
	}
	if got := homeMode(Spec{HomeMode: fs.FileMode(0o700)}); got != 0o700 {
		t.Errorf("homeMode() = %o", got)
	}
}
func TestCommandFailureRetainsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("command failed")
	err := runCommand(context.Background(), &recordingRunner{err: cause}, "useradd", "worker")
	if !errors.Is(err, cause) {
		t.Fatalf("runCommand() error = %v", err)
	}
}
func TestHomeExistsUsesInjectedFilesystem(t *testing.T) {
	t.Parallel()
	exists, err := homeExists(context.Background(), fakeFS{err: fs.ErrNotExist}, "/var/lib/worker")
	if err != nil || exists {
		t.Fatalf("homeExists() = %v, %v", exists, err)
	}
}
func TestEnsureHomeDirectoryRejectsExistingRegularFile(t *testing.T) {
	t.Parallel()
	err := ensureHomeDirectory(fakeFS{mkdirErr: fs.ErrExist, info: testFileInfo{mode: 0o644}}, "/var/lib/worker", 0o755)
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("ensureHomeDirectory() error = %v", err)
	}
}

func TestEnsureHomeDirectoryRetainsFilesystemFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("parent missing")
	err := ensureHomeDirectory(fakeFS{mkdirErr: cause}, "/var/lib/worker", 0o755)
	if !errors.Is(err, cause) {
		t.Fatalf("ensureHomeDirectory() error = %v, want wrapped %v", err, cause)
	}
}

func TestVerifyHomeChecksDirectoryModeAndOwnership(t *testing.T) {
	t.Parallel()
	info := testFileInfo{mode: fs.ModeDir | 0o700, sys: &syscall.Stat_t{Uid: 101, Gid: 201}}
	if err := verifyHome(fakeFS{info: info}, "/srv/worker", 101, 201, 0o700); err != nil {
		t.Fatalf("verifyHome() error = %v", err)
	}
	err := verifyHome(fakeFS{info: info}, "/srv/worker", 101, 202, 0o700)
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("verifyHome() error = %v, want DriftError", err)
	}
}

func TestCommandCancellationRetainsContextCause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runCommand(ctx, &recordingRunner{err: errors.New("process terminated")}, "useradd", "worker")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runCommand() error = %v, want context.Canceled", err)
	}
}

type recordingRunner struct {
	calls [][]string
	err   error
}
type outputRunner struct{ value string }
type scriptRunner struct {
	calls  [][]string
	onCall func(string, []string)
	errFor func(string, []string) error
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
	if r.errFor != nil {
		if err := r.errFor(name, args); err != nil {
			return "", err
		}
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

type memoryStore struct {
	account *user.User
	groups  map[string]*user.Group
	ids     []string
}

func lookupStore() memoryStore {
	return memoryStore{account: &user.User{Username: "worker", Uid: "101", Gid: "201"}, groups: map[string]*user.Group{"201": {Name: "worker", Gid: "201"}, "301": {Name: "logs", Gid: "301"}}, ids: []string{"201", "301"}}
}
func (s memoryStore) lookup(name string) (*user.User, error) {
	if name != s.account.Username {
		return nil, user.UnknownUserError(name)
	}
	return s.account, nil
}
func (s memoryStore) lookupID(id string) (*user.User, error) {
	if id != s.account.Uid {
		return nil, errors.New("unknown user ID")
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

type testFileInfo struct {
	mode os.FileMode
	sys  any
}

func (i testFileInfo) Name() string       { return "test" }
func (i testFileInfo) Size() int64        { return 0 }
func (i testFileInfo) Mode() os.FileMode  { return i.mode }
func (i testFileInfo) ModTime() time.Time { return time.Time{} }
func (i testFileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i testFileInfo) Sys() any           { return i.sys }
