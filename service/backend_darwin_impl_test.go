package service

import (
	"context"
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
}

func TestLaunchdStartUsesOnlyRunner(t *testing.T) {
	f := &fakeFiles{values: map[string][]byte{}}
	r := &fakeRunner{}
	o := operation{spec: specification{name: "com.example.x", binary: "/bin/x", scope: User}, user: host.UserInfo{UID: "501", Home: "/tmp/user"}, files: f, runner: r}
	if err := (launchdBackend{}).start(context.Background(), &o); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls[2], " "); got != "launchctl bootstrap gui/501 /tmp/user/Library/LaunchAgents/com.example.x.plist" {
		t.Fatalf("bootstrap=%q", got)
	}
}
