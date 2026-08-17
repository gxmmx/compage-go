package service

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gxmmx/compage-go/errx"
)

const minimumSystemdVersion = 260

type systemdBackend struct{}

func (systemdBackend) validate(s specification) error {
	if strings.HasSuffix(s.name, ".service") {
		return &ValidationError{Message: "name must not include .service suffix"}
	}
	return nil
}
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
	if err := checkSystemdVersion(c, o); err != nil {
		return EnsureResult{}, err
	}
	p := systemdPath(o)
	if err := rejectSymlink(o.files, p); err != nil {
		return EnsureResult{}, err
	}
	if o.spec.scope == User {
		if err := ensureDefinitionDirectory(o.files, p); err != nil {
			return EnsureResult{}, err
		}
	}
	want := []byte(renderSystemd(o))
	old, e := readDefinition(o.files, p)
	absent := errors.Is(e, fs.ErrNotExist)
	changed := e != nil || string(old) != string(want)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return EnsureResult{}, e
	}
	if changed {
		if e = writeDefinition(o.files, p, want, 0644); e != nil {
			return EnsureResult{}, e
		}
		if _, e = commandOutput(c, o.runner, "systemctl", systemdArgs(o, "daemon-reload")...); e != nil {
			return EnsureResult{}, e
		}
	}
	return EnsureResult{Installed: true, Changed: changed, Reasons: changeReasons(old, o.spec, changed, absent)}, nil
}
func (systemdBackend) start(c context.Context, o *operation) error {
	if err := checkSystemdVersion(c, o); err != nil {
		return err
	}
	_, e := commandOutput(c, o.runner, "systemctl", systemdArgs(o, "enable", "--now", o.spec.name+".service")...)
	return e
}
func (systemdBackend) stop(c context.Context, o *operation) error {
	if _, err := o.files.stat(systemdPath(o)); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := checkSystemdVersion(c, o); err != nil {
		return err
	}
	_, e := commandOutput(c, o.runner, "systemctl", systemdArgs(o, "disable", "--now", o.spec.name+".service")...)
	return e
}
func (b systemdBackend) uninstall(c context.Context, o *operation) error {
	if err := checkSystemdVersion(c, o); err != nil {
		return err
	}
	_ = b.stop(c, o)
	p := systemdPath(o)
	if err := rejectSymlink(o.files, p); err != nil {
		return err
	}
	e := removeDefinition(o.files, p)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return e
	}
	_, e = commandOutput(c, o.runner, "systemctl", systemdArgs(o, "daemon-reload")...)
	return e
}
func (systemdBackend) status(c context.Context, o *operation) (Status, error) {
	if err := checkSystemdVersion(c, o); err != nil {
		return Status{}, err
	}
	_, e := o.files.stat(systemdPath(o))
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return Status{}, errx.New("service: inspecting definition "+systemdPath(o), errx.WithCause(e))
	}
	s := Status{Installed: e == nil}
	out, x := o.runner.run(c, "systemctl", systemdArgs(o, "show", "--property=LoadState,UnitFileState,ActiveState,MainPID,ExecMainStatus", "--value", o.spec.name+".service")...)
	if x != nil {
		if err := unavailableManager(x, "systemctl"); err != nil {
			return Status{}, err
		}
		return s, nil
	}
	values := strings.Split(strings.TrimSpace(out), "\n")
	if len(values) >= 5 {
		s.Loaded = values[0] == "loaded"
		s.Enabled = values[1] == "enabled"
		s.Running = values[2] == "active"
		s.PID, _ = strconv.Atoi(values[3])
		if code, err := strconv.Atoi(values[4]); err == nil {
			s.ExitCode = &code
		}
	}
	s.Detail = strings.TrimSpace(out)
	return s, nil
}
func checkSystemdVersion(c context.Context, o *operation) error {
	out, err := o.runner.run(c, "systemctl", "--version")
	if err != nil {
		return &UnsupportedError{Capability: "systemctl", Cause: errx.New("service: inspecting systemd", errx.WithCause(err))}
	}
	fields := strings.Fields(out)
	if len(fields) < 2 || fields[0] != "systemd" {
		return &UnsupportedError{Capability: "systemctl version"}
	}
	version, err := strconv.Atoi(fields[1])
	if err != nil || version < minimumSystemdVersion {
		return &UnsupportedError{Capability: "systemd " + strconv.Itoa(minimumSystemdVersion)}
	}
	return nil
}
func quote(v string) string {
	return `"` + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(v) + `"`
}
func args(v []string) string {
	var r strings.Builder
	for _, x := range v {
		r.WriteByte(' ')
		r.WriteString(quote(x))
	}
	return r.String()
}
func renderSystemd(o *operation) string {
	var b strings.Builder
	b.WriteString("# compage-spec: ")
	b.WriteString(metadataComment(o.spec))
	b.WriteByte('\n')
	if o.spec.revision != "" {
		b.WriteString("# compage-revision: ")
		b.WriteString(o.spec.revision)
		b.WriteByte('\n')
	}
	b.WriteString("[Unit]\nDescription=")
	b.WriteString(o.spec.description)
	b.WriteString("\n\n[Service]\nExecStart=")
	b.WriteString(quote(o.spec.binary))
	b.WriteString(args(o.spec.args))
	b.WriteString("\nRestart=")
	switch o.spec.restart {
	case RestartNever:
		b.WriteString("no")
	case RestartAlways:
		b.WriteString("always")
	default:
		b.WriteString("on-failure")
	}
	b.WriteByte('\n')
	if o.spec.scope == System && o.spec.account != nil {
		b.WriteString("User=")
		b.WriteString(o.spec.account.Name)
		b.WriteByte('\n')
	}
	keys := make([]string, 0, len(o.spec.env))
	for k := range o.spec.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("Environment=")
		b.WriteString(quote(k + "=" + o.spec.env[k]))
		b.WriteByte('\n')
	}
	if o.spec.stdout != "" {
		b.WriteString("StandardOutput=append:")
		b.WriteString(o.spec.stdout)
		b.WriteByte('\n')
	}
	if o.spec.stderr != "" {
		b.WriteString("StandardError=append:")
		b.WriteString(o.spec.stderr)
		b.WriteByte('\n')
	}
	b.WriteString("\n[Install]\nWantedBy=")
	if o.spec.scope == User {
		b.WriteString("default.target\n")
	} else {
		b.WriteString("multi-user.target\n")
	}
	return b.String()
}
