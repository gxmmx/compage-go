package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/gxmmx/compage-go/host"
)

type testDirectoryProvider struct{ bases AppDirectories }

func (p testDirectoryProvider) directoryBases(Scope, string) (AppDirectories, error) {
	return p.bases, nil
}

type directoryTestBackend struct{ testDirectoryProvider }

func (directoryTestBackend) validate(specification) error { return nil }
func (directoryTestBackend) ensure(context.Context, *operation) (EnsureResult, error) {
	return EnsureResult{}, nil
}
func (directoryTestBackend) start(context.Context, *operation) error            { return nil }
func (directoryTestBackend) stop(context.Context, *operation) error             { return nil }
func (directoryTestBackend) uninstall(context.Context, *operation) error        { return nil }
func (directoryTestBackend) status(context.Context, *operation) (Status, error) { return Status{}, nil }

func TestResolveUserDirectoriesUsesExplicitNamesVerbatim(t *testing.T) {
	p := testDirectoryProvider{bases: AppDirectories{Runtime: "/home/alice", Config: "/home/alice", State: "/home/alice", Logs: "/home/alice"}}
	s := specification{name: "worker", scope: User, configDir: strptr("foo"), runtimeDir: strptr(""), stateDir: strptr("data"), logDir: strptr("logs/archive")}
	d, err := resolveDirectories(s, p, host.UserInfo{Home: "/home/alice"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Config != "/home/alice/foo" || d.Runtime != "/home/alice/foo/run" || d.State != "/home/alice/foo/data" || d.Logs != "/home/alice/foo/logs/archive" {
		t.Fatalf("directories=%+v", d)
	}
}

func TestDirectoryOptionsRejectUnsafeNamesAndConflictingConfigPolicies(t *testing.T) {
	for _, name := range []string{"", "/tmp/x", "x/../y", "x//y", "x\\y", "x\n"} {
		if err := WithStateDir(name)(&specification{}); err == nil {
			t.Errorf("unsafe directory name accepted: %q", name)
		}
	}
	s := specification{}
	if err := WithConfigDir()(&s); err != nil {
		t.Fatal(err)
	}
	if err := WithConfigDirWrite()(&s); err == nil {
		t.Fatal("conflicting config declarations accepted")
	}
	for _, name := range []string{"/tmp/x", "a/b", "..", "x\n"} {
		if err := WithStdoutLog(name)(&specification{}); err == nil {
			t.Errorf("unsafe log name accepted: %q", name)
		}
	}
}

func TestResolveUserDirectoriesDefaultsOnlyOmittedConfigName(t *testing.T) {
	p := testDirectoryProvider{bases: AppDirectories{Runtime: "/home/alice", Config: "/home/alice", State: "/home/alice", Logs: "/home/alice"}}
	s := specification{name: "worker", scope: User, logDir: strptr("")}
	d, err := resolveDirectories(s, p, host.UserInfo{Home: "/home/alice"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Config != "/home/alice/.worker" || d.Logs != "/home/alice/.worker/log" {
		t.Fatalf("directories=%+v", d)
	}
}

func TestDirectoryResolutionCombinations(t *testing.T) {
	userBases := testDirectoryProvider{bases: AppDirectories{Runtime: "/home/alice", Config: "/home/alice", State: "/home/alice", Logs: "/home/alice"}}
	systemBases := testDirectoryProvider{bases: AppDirectories{Runtime: "/run", Config: "/etc", State: "/var/lib", Logs: "/var/log"}}
	for _, test := range []struct {
		name     string
		spec     specification
		provider testDirectoryProvider
		want     AppDirectories
		stderr   string
	}{
		{"user stderr only", specification{name: "worker", scope: User, stderrLog: strptr("foo.log")}, userBases, AppDirectories{Config: "/home/alice/.worker", Logs: "/home/alice/.worker/log"}, "/home/alice/.worker/log/foo.log"},
		{"system stderr only", specification{name: "worker", scope: System, stderrLog: strptr("foo.log")}, systemBases, AppDirectories{Logs: "/var/log/worker"}, "/var/log/worker/foo.log"},
		{"user custom config", specification{name: "worker", scope: User, configDir: strptr("foo"), stderrLog: strptr("foo.log")}, userBases, AppDirectories{Config: "/home/alice/foo", Logs: "/home/alice/foo/log"}, "/home/alice/foo/log/foo.log"},
		{"user custom log child", specification{name: "worker", scope: User, configDir: strptr("foo"), logDir: strptr("logs/archive"), stderrLog: strptr("")}, userBases, AppDirectories{Config: "/home/alice/foo", Logs: "/home/alice/foo/logs/archive"}, "/home/alice/foo/logs/archive/err.log"},
		{"system distinct names", specification{name: "worker", scope: System, runtimeDir: strptr("socket"), configDir: strptr("settings"), stateDir: strptr("data"), logDir: strptr("journal"), stderrLog: strptr("foo.log")}, systemBases, AppDirectories{Runtime: "/run/socket", Config: "/etc/settings", State: "/var/lib/data", Logs: "/var/log/journal"}, "/var/log/journal/foo.log"},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := resolveDirectories(test.spec, test.provider, host.UserInfo{Home: "/home/alice"})
			if err != nil {
				t.Fatal(err)
			}
			if d.AppDirectories != test.want {
				t.Fatalf("directories=%+v want=%+v", d.AppDirectories, test.want)
			}
			if got := resolvedLogPath(d.Logs, *test.spec.stderrLog, "err.log"); got != test.stderr {
				t.Fatalf("stderr=%q want=%q", got, test.stderr)
			}
		})
	}
}

func TestPublicDirectoriesMatchesUserServiceResolution(t *testing.T) {
	name := "worker"
	if host.Platform().OS == host.Darwin {
		name = "com.example.worker"
	}
	u, err := host.User()
	if err != nil {
		t.Fatal(err)
	}
	d, err := Directories(name, WithLogDir(), WithStderrLog("foo.log"))
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(u.Home, "."+name)
	if d.Config != config || d.Logs != filepath.Join(config, "log") {
		t.Fatalf("directories=%+v", d)
	}
}

func TestSystemDirectoryNamesAreAppliedByService(t *testing.T) {
	p := testDirectoryProvider{bases: AppDirectories{Runtime: "/run", Config: "/etc", State: "/var/lib", Logs: "/var/log"}}
	s := specification{name: "worker", scope: System, runtimeDir: strptr("socket"), configDir: strptr("settings"), stateDir: strptr("data"), logDir: strptr("journal")}
	d, err := resolveDirectories(s, p, host.UserInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Runtime != "/run/socket" || d.Config != "/etc/settings" || d.State != "/var/lib/data" || d.Logs != "/var/log/journal" {
		t.Fatalf("directories=%+v", d)
	}
}

func TestEnsureManagedDirectoryRejectsSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureManagedDirectory(osFiles{}, filepath.Join(link, "child"), os.Geteuid(), os.Getegid(), 0o700); err == nil {
		t.Fatal("symlink ancestor accepted")
	}
}

func TestManagedLogsUseNamedDirectoryAndDefaultFiles(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	uid, gid := strconv.Itoa(os.Geteuid()), strconv.Itoa(os.Getegid())
	o := operation{spec: specification{scope: User, stdoutLog: strptr(""), stderrLog: strptr("server.err")}, files: osFiles{}}
	d := managedDirectories{AppDirectories: AppDirectories{Logs: filepath.Join(root, "log")}, logs: true}
	changes, err := o.reconcileDirectories(d, uid, gid)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || o.spec.stdout != filepath.Join(d.Logs, "out.log") || o.spec.stderr != filepath.Join(d.Logs, "server.err") {
		t.Fatalf("changes=%v spec=%+v", changes, o.spec)
	}
	for _, name := range []string{"out.log", "server.err"} {
		if info, err := os.Lstat(filepath.Join(d.Logs, name)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("log %s: %v %v", name, info, err)
		}
	}
}

func TestTraversableDirectoryRejectsInaccessibleAncestor(t *testing.T) {
	info := testDirInfo{mode: 0o700, stat: syscall.Stat_t{Uid: 1, Gid: 1}}
	if err := traversableDirectory(info, 2, 2); err == nil {
		t.Fatal("inaccessible ancestor accepted")
	}
}

func TestUninstallRemovesOnlyManagedRuntimeDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := directoryTestBackend{testDirectoryProvider{bases: AppDirectories{Runtime: root, Config: root, State: root, Logs: root}}}
	o := operation{spec: specification{name: "worker", scope: User, runtimeDir: strptr("run"), configDir: strptr("config"), stateDir: strptr("state"), logDir: strptr("log")}, user: host.UserInfo{Home: root}, backend: b, files: osFiles{}}
	d, err := resolveDirectories(o.spec, b, o.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{d.Runtime, d.Config, d.State, d.Logs} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d.Runtime, "socket"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := o.uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(d.Runtime); !os.IsNotExist(err) {
		t.Fatalf("runtime remains: %v", err)
	}
	for _, path := range []string{d.Config, d.State, d.Logs} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("persistent directory missing: %s: %v", path, err)
		}
	}
}

func TestPurgeRemovesOnlySelectedDeclaredDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := directoryTestBackend{testDirectoryProvider{bases: AppDirectories{Runtime: root, Config: root, State: root, Logs: root}}}
	o := operation{spec: specification{name: "worker", scope: User, configDir: strptr("config"), stateDir: strptr("state"), logDir: strptr("log")}, user: host.UserInfo{Home: root}, backend: b, files: osFiles{}}
	d, err := resolveDirectories(o.spec, b, o.user)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{d.Config, d.State, d.Logs} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.purge(context.Background(), PurgeOptions{State: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(d.State); !os.IsNotExist(err) {
		t.Fatalf("state remains: %v", err)
	}
	for _, path := range []string{d.Config, d.Logs} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("unselected directory missing: %s: %v", path, err)
		}
	}
	if err := o.purge(context.Background(), PurgeOptions{Logs: true}); err != nil {
		t.Fatal(err)
	}
	if err := o.purge(context.Background(), PurgeOptions{Config: true}); err == nil {
		t.Fatal("config purge accepted without selecting nested state")
	}
}

type testDirInfo struct {
	mode os.FileMode
	stat syscall.Stat_t
}

func (i testDirInfo) Name() string       { return "directory" }
func (i testDirInfo) Size() int64        { return 0 }
func (i testDirInfo) Mode() os.FileMode  { return i.mode }
func (i testDirInfo) ModTime() time.Time { return time.Time{} }
func (i testDirInfo) IsDir() bool        { return true }
func (i testDirInfo) Sys() any           { return &i.stat }
func strptr(v string) *string            { return &v }
