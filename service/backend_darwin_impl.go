package service

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type launchdBackend struct{}

func (launchdBackend) validate(s specification) error {
	if !strings.Contains(s.name, ".") {
		return &ValidationError{Message: "launchd label must contain a namespace dot"}
	}
	return nil
}
func launchdPath(o *operation) string {
	if o.spec.scope == System {
		return filepath.Join("/Library/LaunchDaemons", o.spec.name+".plist")
	}
	return filepath.Join(o.user.Home, "Library/LaunchAgents", o.spec.name+".plist")
}
func domain(o *operation) string {
	if o.spec.scope == System {
		return "system"
	}
	return "gui/" + o.user.UID
}
func (b launchdBackend) ensure(c context.Context, o *operation) (EnsureResult, error) {
	p := launchdPath(o)
	w := []byte(plist(o))
	old, e := o.files.read(p)
	changed := e != nil || string(old) != string(w)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return EnsureResult{}, e
	}
	if changed {
		if e = o.files.write(p, w, 0644); e != nil {
			return EnsureResult{}, e
		}
	}
	return EnsureResult{Installed: true, Changed: changed, Reasons: reason(changed)}, nil
}
func (launchdBackend) start(c context.Context, o *operation) error {
	d := domain(o)
	_, _ = o.runner.run(c, "launchctl", "enable", d+"/"+o.spec.name)
	_, _ = o.runner.run(c, "launchctl", "bootout", d+"/"+o.spec.name)
	if _, e := o.runner.run(c, "launchctl", "bootstrap", d, launchdPath(o)); e != nil {
		return e
	}
	_, e := o.runner.run(c, "launchctl", "kickstart", "-k", d+"/"+o.spec.name)
	return e
}
func (launchdBackend) stop(c context.Context, o *operation) error {
	d := domain(o)
	_, _ = o.runner.run(c, "launchctl", "disable", d+"/"+o.spec.name)
	_, e := o.runner.run(c, "launchctl", "bootout", d+"/"+o.spec.name)
	return e
}
func (b launchdBackend) uninstall(c context.Context, o *operation) error {
	_ = b.stop(c, o)
	e := o.files.remove(launchdPath(o))
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return e
	}
	return nil
}
func (launchdBackend) status(c context.Context, o *operation) (Status, error) {
	_, e := o.files.stat(launchdPath(o))
	s := Status{Installed: e == nil}
	out, x := o.runner.run(c, "launchctl", "print", domain(o)+"/"+o.spec.name)
	s.Loaded = x == nil
	s.Running = s.Loaded
	s.Detail = strings.TrimSpace(out)
	if x != nil {
		return s, nil
	}
	return s, nil
}
func plist(o *operation) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<plist version=\"1.0\"><dict>\n<key>Label</key><string>")
	b.WriteString(xml(o.spec.name))
	b.WriteString("</string>\n<key>ProgramArguments</key><array>\n<string>")
	b.WriteString(xml(o.spec.binary))
	b.WriteString("</string>\n")
	for _, a := range o.spec.args {
		b.WriteString("<string>")
		b.WriteString(xml(a))
		b.WriteString("</string>\n")
	}
	if len(o.spec.env) > 0 {
		keys := make([]string, 0, len(o.spec.env))
		for k := range o.spec.env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("<key>EnvironmentVariables</key><dict>\n")
		for _, k := range keys {
			b.WriteString("<key>")
			b.WriteString(xml(k))
			b.WriteString("</key><string>")
			b.WriteString(xml(o.spec.env[k]))
			b.WriteString("</string>\n")
		}
		b.WriteString("</dict>\n")
	}
	if o.spec.scope == System && o.spec.account != nil {
		b.WriteString("<key>UserName</key><string>")
		b.WriteString(xml(o.spec.account.Name))
		b.WriteString("</string>\n")
	}
	if o.spec.stdout != "" {
		b.WriteString("<key>StandardOutPath</key><string>")
		b.WriteString(xml(o.spec.stdout))
		b.WriteString("</string>\n")
	}
	if o.spec.stderr != "" {
		b.WriteString("<key>StandardErrorPath</key><string>")
		b.WriteString(xml(o.spec.stderr))
		b.WriteString("</string>\n")
	}
	b.WriteString("</array>\n<key>RunAtLoad</key><true/>\n</dict></plist>\n")
	return b.String()
}
func xml(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(v)
}
