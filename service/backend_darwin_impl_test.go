package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gxmmx/compage-go/account"
	"github.com/gxmmx/compage-go/host"
)

func TestLaunchdPlistRendersEscapedDeterministicValues(t *testing.T) {
	o := operation{spec: specification{name: "com.example.x", binary: "/opt/a&b", args: []string{"serve", "<x>"}, scope: System, env: map[string]string{"Z": "1", "A": "<&"}, stdout: "/var/log/out", stderr: "/var/log/err", account: &account.Spec{Name: "worker"}}}
	s := plist(&o)
	for _, want := range []string{"/opt/a&amp;b", "&lt;x&gt;", "<key>A</key><string>&lt;&amp;", "<key>Z</key><string>1", "<key>UserName</key><string>worker", "StandardOutPath", "StandardErrorPath"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Index(s, "</array>") > strings.Index(s, "<key>EnvironmentVariables</key>") {
		t.Fatalf("environment is inside ProgramArguments: %s", s)
	}
}

func TestLaunchdStartUsesOnlyRunner(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{errors: map[string]error{"print": errors.New("not loaded")}}
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	if err := (launchdBackend{}).start(context.Background(), &o); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls[2], " "); got != "launchctl bootstrap gui/501 /tmp/user/Library/LaunchAgents/com.example.x.plist" {
		t.Fatalf("bootstrap=%q", got)
	}
}

func TestLaunchdStartDoesNotForceRestartLoadedJob(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{outputs: map[string]string{"print": "loaded"}}
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	if err := (launchdBackend{}).start(context.Background(), &o); err != nil {
		t.Fatal(err)
	}
	for _, call := range r.calls {
		if len(call) > 1 && call[1] == "bootout" {
			t.Fatalf("Start unloaded loaded job: %v", r.calls)
		}
	}
	if got := strings.Join(r.calls[len(r.calls)-1], " "); got != "launchctl kickstart gui/501/com.example.x" {
		t.Fatalf("kickstart=%q", got)
	}
}

func TestLaunchdRestartUsesForceKickstart(t *testing.T) {
	r := &fakeRunner{outputs: map[string]string{"print": "loaded"}}
	o := operation{spec: specification{name: "com.example.x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, runner: r}
	if err := (launchdBackend{}).restart(context.Background(), &o); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls[len(r.calls)-1], " "); got != "launchctl kickstart -k gui/501/com.example.x" {
		t.Fatalf("restart=%q", got)
	}
}

func TestLaunchdEnsureReportsLoadedChangeWithoutRestart(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{}
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	result, err := (launchdBackend{}).ensure(context.Background(), &o)
	if err != nil || !result.RestartRequired {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, call := range r.calls {
		if len(call) > 1 && call[1] == "bootout" {
			t.Fatalf("Ensure restarted job: %v", r.calls)
		}
	}
}

func TestLaunchDaemonEnsureSetsRootOwnershipAndMode(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: System}, files: f, runner: &fakeRunner{}}
	if _, err := (launchdBackend{}).ensure(context.Background(), &o); err != nil {
		t.Fatal(err)
	}
	if f.chownCalls != 1 || f.chmodCalls != 1 {
		t.Fatalf("chown=%d chmod=%d", f.chownCalls, f.chmodCalls)
	}
}

func TestLaunchdStatusParsesAvailableFields(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{"/tmp/user/Library/LaunchAgents/com.example.x.plist": {}}}
	r := &fakeRunner{outputs: map[string]string{"print": "pid = 41\nlast exit code = 2\n", "print-disabled": "{\n}\n"}}
	o := operation{spec: specification{name: "com.example.x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	s, err := (launchdBackend{}).status(context.Background(), &o)
	if err != nil || !s.Installed || !s.Loaded || !s.Enabled || s.PID != 41 || s.ExitCode == nil || *s.ExitCode != 2 {
		t.Fatalf("status=%+v err=%v", s, err)
	}
}

func TestLaunchdStatusHandlesUnloadedAndDisabledJob(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{"/tmp/user/Library/LaunchAgents/com.example.x.plist": {}}}
	r := &fakeRunner{outputs: map[string]string{"print-disabled": "\"com.example.x\" => true\n"}, err: errors.New("not loaded")}
	o := operation{spec: specification{name: "com.example.x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	s, err := (launchdBackend{}).status(context.Background(), &o)
	if err != nil || !s.Installed || s.Loaded || s.Running || s.Enabled {
		t.Fatalf("status=%+v err=%v", s, err)
	}
}

func TestLaunchdRevisionParticipatesInRenderedDefinition(t *testing.T) {
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", revision: "v1", scope: System}}
	if !strings.Contains(plist(&o), "compage-revision: v1") {
		t.Fatal("plist revision missing")
	}
}
func TestLaunchdRendersRestartPolicy(t *testing.T) {
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: System, restart: RestartOnFailure}}
	if !strings.Contains(plist(&o), "<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>") {
		t.Fatal("restart policy missing")
	}
}

func TestLaunchdStopAbsentIsIdempotent(t *testing.T) {
	r := &fakeRunner{}
	o := operation{spec: specification{name: "com.example.x", scope: User}, user: host.UserInfo{Home: "/tmp/user"}, files: &fakeFiles{values: map[string][]byte{}}, runner: r}
	if err := (launchdBackend{}).stop(context.Background(), &o); err != nil || len(r.calls) != 0 {
		t.Fatalf("err=%v calls=%v", err, r.calls)
	}
}
func TestLaunchdStartAndStopPreserveCommandFailures(t *testing.T) {
	cause := errors.New("launchctl failed")
	o := operation{spec: specification{name: "com.example.x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: &fakeFiles{values: map[string][]byte{"/tmp/user/Library/LaunchAgents/com.example.x.plist": {}}}, runner: &fakeRunner{err: cause}}
	if err := (launchdBackend{}).start(context.Background(), &o); !errors.Is(err, cause) {
		t.Fatalf("start err=%v", err)
	}
	if err := (launchdBackend{}).stop(context.Background(), &o); !errors.Is(err, cause) {
		t.Fatalf("stop err=%v", err)
	}
}
