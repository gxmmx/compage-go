package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/gxmmx/compage-go/host"
)

type fakeFiles struct{ values map[string][]byte }

func (f *fakeFiles) read(p string) ([]byte, error) {
	v, ok := f.values[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return v, nil
}
func (f *fakeFiles) write(p string, v []byte, _ os.FileMode) error {
	f.values[p] = append([]byte(nil), v...)
	return nil
}
func (f *fakeFiles) remove(p string) error { delete(f.values, p); return nil }
func (f *fakeFiles) stat(p string) (os.FileInfo, error) {
	if _, ok := f.values[p]; !ok {
		return nil, fs.ErrNotExist
	}
	return nil, nil
}

type fakeRunner struct {
	calls [][]string
	out   string
	err   error
}

func (f *fakeRunner) run(_ context.Context, n string, a ...string) (string, error) {
	f.calls = append(f.calls, append([]string{n}, a...))
	return f.out, f.err
}
func TestSystemdEnsureRendersDeterministically(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{}
	o := operation{spec: specification{name: "example", description: "Example", binary: "/opt/a b", args: []string{"serve", "x y"}, scope: System, env: map[string]string{"Z": "1", "A": "2"}, stdout: "/var/log/out", stderr: "/var/log/err"}, user: host.UserInfo{Home: "/tmp/u"}, files: f, runner: r}
	got, e := systemdBackend{}.ensure(context.Background(), &o)
	if e != nil || !got.Changed {
		t.Fatalf("ensure=%+v,%v", got, e)
	}
	s := string(f.values[systemdPath(&o)])
	for _, want := range []string{`ExecStart="/opt/a b" "serve" "x y"`, `Environment="A=2"`, `Environment="Z=1"`, "StandardOutput=append:/var/log/out"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if len(r.calls) != 1 || !strings.Contains(strings.Join(r.calls[0], " "), "daemon-reload") {
		t.Fatalf("calls=%v", r.calls)
	}
}
func TestSystemdEnsureLeavesDefinitionWhenEqual(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{}
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System}, files: f, runner: r}
	f.values[systemdPath(&o)] = []byte(renderSystemd(&o))
	got, e := systemdBackend{}.ensure(context.Background(), &o)
	if e != nil || got.Changed || len(r.calls) != 0 {
		t.Fatalf("ensure=%+v,%v calls=%v", got, e, r.calls)
	}
}

var _ = errors.New
