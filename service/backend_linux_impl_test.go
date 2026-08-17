package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gxmmx/compage-go/host"
)

type fakeFiles struct {
	values     map[string][]byte
	chownCalls int
	chmodCalls int
}

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
func (f *fakeFiles) chmod(string, os.FileMode) error    { f.chmodCalls++; return nil }
func (f *fakeFiles) chown(string, int, int) error       { f.chownCalls++; return nil }
func (f *fakeFiles) stat(p string) (os.FileInfo, error) {
	if _, ok := f.values[p]; !ok {
		return nil, fs.ErrNotExist
	}
	if strings.HasPrefix(p, "/bin/") {
		return fakeInfo{mode: 0o755}, nil
	}
	return fakeInfo{}, nil
}
func (f *fakeFiles) lstat(p string) (os.FileInfo, error) {
	if _, ok := f.values[p]; !ok {
		return nil, fs.ErrNotExist
	}
	return fakeInfo{}, nil
}

type fakeInfo struct{ mode os.FileMode }

func (fakeInfo) Name() string { return "service" }
func (fakeInfo) Size() int64  { return 0 }
func (f fakeInfo) Mode() os.FileMode {
	if f.mode != 0 {
		return f.mode
	}
	return 0o644
}
func (fakeInfo) ModTime() time.Time { return time.Time{} }
func (fakeInfo) IsDir() bool        { return false }
func (fakeInfo) Sys() any           { return &syscall.Stat_t{} }

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
	if len(got.Reasons) != 1 || got.Reasons[0] != Missing {
		t.Fatalf("reasons=%v", got.Reasons)
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
func TestSystemdEnsureReportsDefinitionDrift(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{"/etc/systemd/system/x.service": []byte("old")}}
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System}, files: f, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}}
	r, err := (systemdBackend{}).ensure(context.Background(), &o)
	if err != nil || len(r.Reasons) != 1 || r.Reasons[0] != DefinitionDrift {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}
func TestSystemdEnsureReportsSpecificChangeReasons(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}
	old := operation{spec: specification{name: "x", binary: "/bin/x", args: []string{"a"}, env: map[string]string{"A": "1"}, revision: "v1", scope: System}, files: f, runner: r}
	f.values[systemdPath(&old)] = []byte(renderSystemd(&old))
	next := old
	next.spec.binary = "/bin/y"
	next.spec.args = []string{"b"}
	next.spec.env = map[string]string{"A": "2"}
	next.spec.revision = "v2"
	result, err := (systemdBackend{}).ensure(context.Background(), &next)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []ChangeReason{BinaryChanged, ArgumentsChanged, EnvironmentChanged, RevisionChanged} {
		found := false
		for _, got := range result.Reasons {
			found = found || got == want
		}
		if !found {
			t.Errorf("reasons %v missing %s", result.Reasons, want)
		}
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

func TestSystemdStatusTreatsInactiveAsStatus(t *testing.T) {
	o := operation{spec: specification{name: "x", scope: System}, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}, err: errors.New("inactive")}, files: &fakeFiles{values: map[string][]byte{}}}
	s, err := (systemdBackend{}).status(context.Background(), &o)
	if err != nil || s.Installed || s.Running {
		t.Fatalf("status=%+v err=%v", s, err)
	}
}

func TestQuoteSystemdArgument(t *testing.T) {
	if got, want := quote("a\"b\\c"), `"a\"b\\c"`; got != want {
		t.Fatalf("quote=%q want=%q", got, want)
	}
}

func TestRevisionParticipatesInRenderedDefinition(t *testing.T) {
	o := operation{spec: specification{name: "x", binary: "/bin/x", revision: "v1", scope: System}}
	if !strings.Contains(renderSystemd(&o), "compage-revision: v1") {
		t.Fatal("systemd revision missing")
	}
}
func TestSystemdRendersRestartPolicy(t *testing.T) {
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System, restart: RestartAlways}}
	if !strings.Contains(renderSystemd(&o), "Restart=always") {
		t.Fatal("restart policy missing")
	}
}

func TestSystemdStopAbsentIsIdempotent(t *testing.T) {
	r := &fakeRunner{}
	o := operation{spec: specification{name: "x", scope: System}, files: &fakeFiles{values: map[string][]byte{}}, runner: r}
	if err := (systemdBackend{}).stop(context.Background(), &o); err != nil || len(r.calls) != 0 {
		t.Fatalf("err=%v calls=%v", err, r.calls)
	}
}
func TestSystemdStartPreservesCommandFailure(t *testing.T) {
	cause := errors.New("systemctl failed")
	o := operation{spec: specification{name: "x", scope: System}, files: &fakeFiles{values: map[string][]byte{"/etc/systemd/system/x.service": {}}}, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}, err: cause}}
	if err := (systemdBackend{}).start(context.Background(), &o); !errors.Is(err, cause) {
		t.Fatalf("start err=%v", err)
	}
}
