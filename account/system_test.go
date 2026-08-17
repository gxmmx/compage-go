package account

import (
	"context"
	"errors"
	"io/fs"
	"os/user"
	"testing"

	"github.com/gxmmx/compage-go/host"
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
	b := systemBackend{os: host.Linux, runner: runner}
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

func TestCommandFailureRetainsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("command failed")
	b := systemBackend{runner: &recordingRunner{err: cause}}
	err := b.run(context.Background(), "useradd", "worker")
	if !errors.Is(err, cause) {
		t.Fatalf("run() error = %v, want wrapped %v", err, cause)
	}
}

func TestEnsureGroupRejectsExistingGIDDrift(t *testing.T) {
	t.Parallel()
	gid := 200
	b := systemBackend{os: host.Linux, store: fakeStore{group: &user.Group{Name: "worker", Gid: "100"}}, runner: &recordingRunner{}}
	err := b.ensureGroup(context.Background(), Spec{Group: "worker", GID: &gid})
	var drift *DriftError
	if !errors.As(err, &drift) {
		t.Fatalf("ensureGroup() error = %v, want DriftError", err)
	}
}

type recordingRunner struct {
	calls [][]string
	err   error
}

func (r *recordingRunner) output(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return "output", r.err
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
