package service

import (
	"context"
	"errors"
	"github.com/gxmmx/compage-go/account"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestValidateRejectsUnsafeNamesAndEnvironment(t *testing.T) {
	for _, name := range []string{"", ".", "../x", "x/y", "-x", "x y"} {
		if err := validate(specification{name: name, binary: "/bin/x", scope: User}); err == nil {
			t.Errorf("validate(%q) succeeded", name)
		}
	}
	if err := validate(specification{name: "x", binary: "/bin/x", scope: User, env: map[string]string{"A-B": "x"}}); err == nil {
		t.Fatal("invalid environment accepted")
	}
}

func TestRejectSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "service")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := rejectSymlink(osFiles{}, link); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestCheckLogParentsRequiresExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	uid, gid := strconv.Itoa(os.Geteuid()), strconv.Itoa(os.Getegid())
	if err := checkLogParents(osFiles{}, specification{stdout: filepath.Join(dir, "worker.log")}, uid, gid); err != nil {
		t.Fatal(err)
	}
	if err := checkLogParents(osFiles{}, specification{stdout: filepath.Join(dir, "missing", "worker.log")}, uid, gid); err == nil {
		t.Fatal("missing log parent accepted")
	}
}

func TestEnsureAccountFailurePreventsDefinitionWrite(t *testing.T) {
	cause := errors.New("account failed")
	f := &fakeFiles{values: map[string][]byte{"/bin/x": {}}}
	called := false
	o := operation{spec: specification{name: "x", binary: "/bin/x", scope: System, account: &account.Spec{Name: "worker"}}, root: true, backend: systemdBackend{}, files: f, runner: &fakeRunner{outputs: map[string]string{"--version": "systemd 260\n"}}, ensureAccount: func(context.Context, account.Spec) (account.EnsureResult, error) {
		called = true
		return account.EnsureResult{}, cause
	}}
	_, err := o.ensure(context.Background())
	if !called || !errors.Is(err, cause) {
		t.Fatalf("called=%v err=%v", called, err)
	}
	if _, exists := f.values[systemdPath(&o)]; exists {
		t.Fatal("definition written after account failure")
	}
}
func TestOperationRejectsSystemScopeWithoutRoot(t *testing.T) {
	o := operation{spec: specification{scope: System}}
	if _, err := o.ensure(context.Background()); err == nil {
		t.Fatal("non-root system ensure accepted")
	}
}

func TestOperationRejectsCancelledLifecycleCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := operation{backend: systemdBackend{}, runner: &fakeRunner{}}
	if err := o.start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("start error=%v", err)
	}
}

func TestEnsureRejectsNonExecutableBinary(t *testing.T) {
	o := operation{spec: specification{name: "x", binary: "/opt/x", scope: System}, files: &fakeFiles{values: map[string][]byte{"/opt/x": {}}}, backend: systemdBackend{}}
	if _, err := o.ensure(context.Background()); err == nil {
		t.Fatal("non-executable binary accepted")
	}
}

func TestUniqueReasons(t *testing.T) {
	got := uniqueReasons([]ChangeReason{AccountChanged, AccountChanged, Missing})
	if len(got) != 2 || got[0] != AccountChanged || got[1] != Missing {
		t.Fatalf("reasons=%v", got)
	}
}

type directoryInfo struct {
	mode os.FileMode
	stat syscall.Stat_t
}

func (directoryInfo) Name() string        { return "logs" }
func (directoryInfo) Size() int64         { return 0 }
func (d directoryInfo) Mode() os.FileMode { return d.mode }
func (directoryInfo) ModTime() time.Time  { return time.Time{} }
func (directoryInfo) IsDir() bool         { return true }
func (d directoryInfo) Sys() any          { return &d.stat }
func TestWritableDirectoryChecksRuntimeIdentity(t *testing.T) {
	info := directoryInfo{mode: 0o500, stat: syscall.Stat_t{Uid: 100, Gid: 100}}
	if err := writableDirectory(info, "100", "100"); err == nil {
		t.Fatal("non-writable owner directory accepted")
	}
	info.mode = 0o700
	if err := writableDirectory(info, "100", "100"); err != nil {
		t.Fatal(err)
	}
	if err := writableDirectory(directoryInfo{mode: 0o500, stat: syscall.Stat_t{Uid: 0, Gid: 0}}, "0", "0"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDefinitionOwnerRejectsMismatch(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{"/tmp/unit": {}}}
	if err := verifyDefinitionOwner(f, "/tmp/unit", 1, 0); err == nil {
		t.Fatal("mismatched owner accepted")
	}
}
func TestAtomicWriterNeverLeavesPartialDefinition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service")
	fs := osFiles{}
	values := [][]byte{[]byte("first complete definition"), []byte("second complete definition")}
	var wg sync.WaitGroup
	for _, value := range values {
		wg.Add(1)
		go func(v []byte) {
			defer wg.Done()
			if err := fs.write(path, v, 0o644); err != nil {
				t.Error(err)
			}
		}(value)
	}
	wg.Wait()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(values[0]) && string(got) != string(values[1]) {
		t.Fatalf("partial definition: %q", got)
	}
}

func TestBackendNameValidation(t *testing.T) {
	if err := (systemdBackend{}).validate(specification{name: "x.service"}); err == nil {
		t.Fatal("systemd suffix accepted")
	}
	if err := (launchdBackend{}).validate(specification{name: "worker"}); err == nil {
		t.Fatal("launchd non-label accepted")
	}
}

func TestValidateRejectsTransientBinary(t *testing.T) {
	for _, binary := range []string{"/tmp/worker", "/opt/example/.dist/worker"} {
		if err := validate(specification{name: "worker", binary: binary, scope: User}); err == nil {
			t.Errorf("transient binary accepted: %s", binary)
		}
	}
}

func TestValidateRejectsLineInjection(t *testing.T) {
	for _, spec := range []specification{{name: "x", binary: "/bin/x", scope: User, description: "x\nUser=root"}, {name: "x", binary: "/bin/x", scope: User, args: []string{"x\ny"}}} {
		if err := validate(spec); err == nil {
			t.Fatal("line injection accepted")
		}
	}
}
