package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

const minimumSystemdVersion = 260

type systemdBackend struct{}

func (systemdBackend) validate(specification) error { return nil }
func systemdPath(o *operation) string {
	if o.spec.scope == System {
		return filepath.Join("/etc/systemd/system", o.spec.name+".service")
	}
	return filepath.Join(o.user.Home, ".config/systemd/user", o.spec.name+".service")
}
func systemdArgs(o *operation, a ...string) []string {
	if o.spec.scope == User {
		return append([]string{"--user"}, a...)
	}
	return a
}
func (b systemdBackend) ensure(c context.Context, o *operation) (EnsureResult, error) {
	p := systemdPath(o)
	want := []byte("[Unit]\nDescription=" + o.spec.description + "\n\n[Service]\nExecStart=" + quote(o.spec.binary) + args(o.spec.args) + "\nRestart=on-failure\n\n[Install]\nWantedBy=multi-user.target\n")
	old, e := o.files.read(p)
	changed := e != nil || string(old) != string(want)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return EnsureResult{}, e
	}
	if changed {
		if e = o.files.write(p, want, 0644); e != nil {
			return EnsureResult{}, e
		}
		if _, e = o.runner.run(c, "systemctl", systemdArgs(o, "daemon-reload")...); e != nil {
			return EnsureResult{}, fmt.Errorf("systemctl daemon-reload: %w", e)
		}
	}
	return EnsureResult{Installed: true, Changed: changed, Reasons: reason(changed)}, nil
}
func (systemdBackend) start(c context.Context, o *operation) error {
	_, e := o.runner.run(c, "systemctl", systemdArgs(o, "enable", "--now", o.spec.name+".service")...)
	return e
}
func (systemdBackend) stop(c context.Context, o *operation) error {
	_, e := o.runner.run(c, "systemctl", systemdArgs(o, "disable", "--now", o.spec.name+".service")...)
	return e
}
func (b systemdBackend) uninstall(c context.Context, o *operation) error {
	_ = b.stop(c, o)
	e := o.files.remove(systemdPath(o))
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return e
	}
	_, e = o.runner.run(c, "systemctl", systemdArgs(o, "daemon-reload")...)
	return e
}
func (systemdBackend) status(c context.Context, o *operation) (Status, error) {
	_, e := o.files.stat(systemdPath(o))
	s := Status{Installed: e == nil}
	out, x := o.runner.run(c, "systemctl", systemdArgs(o, "is-active", o.spec.name+".service")...)
	s.Detail = strings.TrimSpace(out)
	s.Running = s.Detail == "active"
	return s, x
}
func quote(v string) string {
	return `"` + strings.NewReplacer(`\\`, `\\\\`, `"`, `\\"`).Replace(v) + `"`
}
func args(v []string) string {
	var r strings.Builder
	for _, x := range v {
		r.WriteByte(' ')
		r.WriteString(quote(x))
	}
	return r.String()
}
func reason(v bool) []ChangeReason {
	if v {
		return []ChangeReason{DefinitionDrift}
	}
	return nil
}
