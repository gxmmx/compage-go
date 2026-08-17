package host

import (
	"errors"
	"os/user"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

func TestPlatform(t *testing.T) {
	t.Parallel()

	got := platform("linux", "arm64")
	want := PlatformInfo{OS: Linux, Arch: "arm64"}
	if got != want {
		t.Fatalf("platform() = %#v, want %#v", got, want)
	}
}

func TestOSIsUnix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		os   OS
		want bool
	}{
		{name: "linux", os: Linux, want: true},
		{name: "darwin", os: Darwin, want: true},
		{name: "windows", os: Windows, want: false},
		{name: "other", os: "plan9", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.os.IsUnix(); got != test.want {
				t.Errorf("IsUnix() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsRoot(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		os   string
		uid  int
		want bool
	}{
		{name: "unix root", os: "linux", uid: 0, want: true},
		{name: "unix user", os: "darwin", uid: 501, want: false},
		{name: "windows", os: "windows", uid: 0, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isRoot(test.os, test.uid); got != test.want {
				t.Errorf("isRoot() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCurrentUser(t *testing.T) {
	t.Parallel()

	query := fakeSystem{uid: 123, user: &user.User{Uid: "123", Gid: "456", Username: "worker"}}
	got, err := currentUser(query, "linux")
	if err != nil {
		t.Fatalf("currentUser() error = %v", err)
	}
	want := UserInfo{UID: "123", GID: "456", Name: "worker"}
	if got != want {
		t.Errorf("currentUser() = %#v, want %#v", got, want)
	}
}

func TestCurrentUserErrorWrapsCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("lookup failed")
	_, err := currentUser(fakeSystem{uid: 123, lookupErr: cause}, "linux")
	if !errors.Is(err, cause) {
		t.Fatalf("currentUser() error = %v, want wrapped %v", err, cause)
	}
	var wrapped *errx.Error
	if !errors.As(err, &wrapped) || wrapped.Kind() != errx.Unknown {
		t.Fatalf("currentUser() error = %T %v, want unclassified errx.Error", err, err)
	}
}

func TestCurrentProcess(t *testing.T) {
	t.Parallel()

	got, err := currentProcess(fakeSystem{executablePath: "/tmp/tool", canonicalPath: "/opt/tool"})
	if err != nil {
		t.Fatalf("currentProcess() error = %v", err)
	}
	if got.PID <= 0 || got.Executable != "/opt/tool" {
		t.Errorf("currentProcess() = %#v, want current PID and /opt/tool", got)
	}
}

func TestCurrentProcessErrorWrapsCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("symlink failed")
	_, err := currentProcess(fakeSystem{executablePath: "/tmp/tool", symlinkErr: cause})
	if !errors.Is(err, cause) {
		t.Fatalf("currentProcess() error = %v, want wrapped %v", err, cause)
	}
}

func TestWorkingDir(t *testing.T) {
	t.Parallel()

	got, err := workingDir(fakeSystem{workingDirectory: "/workspace"})
	if err != nil {
		t.Fatalf("workingDir() error = %v", err)
	}
	if got != "/workspace" {
		t.Errorf("workingDir() = %q, want /workspace", got)
	}
}

type fakeSystem struct {
	uid              int
	user             *user.User
	lookupErr        error
	currentErr       error
	executablePath   string
	executableErr    error
	canonicalPath    string
	symlinkErr       error
	workingDirectory string
	workingDirErr    error
}

func (f fakeSystem) effectiveUID() int                     { return f.uid }
func (f fakeSystem) lookupUser(string) (*user.User, error) { return f.user, f.lookupErr }
func (f fakeSystem) currentUser() (*user.User, error)      { return f.user, f.currentErr }
func (f fakeSystem) executable() (string, error)           { return f.executablePath, f.executableErr }
func (f fakeSystem) evalSymlinks(string) (string, error)   { return f.canonicalPath, f.symlinkErr }
func (f fakeSystem) getwd() (string, error)                { return f.workingDirectory, f.workingDirErr }
