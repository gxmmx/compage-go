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
func (f *fakeFiles) mkdirAll(string, os.FileMode) error { return nil }
func (f *fakeFiles) remove(p string) error              { delete(f.values, p); return nil }
func (f *fakeFiles) stat(p string) (os.FileInfo, error) {
	if _, ok := f.values[p]; !ok {
		return nil, fs.ErrNotExist
	}
	return nil, nil
}

type fakeRunner struct {
	calls   [][]string
	out     string
	err     error
	outputs map[string]string
}

func (f *fakeRunner) run(_ context.Context, n string, a ...string) (string, error) {
	f.calls = append(f.calls, append([]string{n}, a...))
	if len(a) > 0 && f.outputs != nil {
		if out, ok := f.outputs[a[0]]; ok {
			return out, nil
		}
	}
	return f.out, f.err
}
func TestSystemdEnsureRendersDeterministically(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}
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
	if len(r.calls) != 2 || !strings.Contains(strings.Join(r.calls[1], " "), "daemon-reload") {
		t.Fatalf("calls=%v", r.calls)
	}
}
func TestSystemdEnsureLeavesDefinitionWhenEqual(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System}, files: f, runner: r}
	f.values[systemdPath(&o)] = []byte(renderSystemd(&o))
	got, e := systemdBackend{}.ensure(context.Background(), &o)
	if e != nil || got.Changed || len(r.calls) != 1 {
		t.Fatalf("ensure=%+v,%v calls=%v", got, e, r.calls)
	}
}

func TestSystemdVersionAndStatus(t *testing.T) {
	o := operation{spec: specification{name: "x", scope: System}, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n", "show": "loaded\nenabled\nactive\n42\n0\n"}}, files: &fakeFiles{values: map[string][]byte{"/etc/systemd/system/x.service": {}}}}
	s, e := systemdBackend{}.status(context.Background(), &o)
	if e != nil || !s.Installed || !s.Enabled || !s.Running || s.PID != 42 || s.ExitCode == nil || *s.ExitCode != 0 {
		t.Fatalf("status=%+v err=%v", s, e)
	}
}
func TestSystemdRejectsOldVersion(t *testing.T) {
	o := operation{runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 259\n"}}}
	if err := checkSystemdVersion(context.Background(), &o); err == nil {
		t.Fatal("old systemd accepted")
	}
}

var _ = errors.New
