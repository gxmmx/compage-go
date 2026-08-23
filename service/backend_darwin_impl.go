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

type launchdBackend struct{}

func (launchdBackend) directoryBases(scope Scope, home string) (AppDirectories, error) {
	if scope == User {
		return userDirectoryBases(home)
	}
	if scope != System {
		return AppDirectories{}, &ValidationError{Message: "unknown scope"}
	}
	// /var is a symlink to /private/var on macOS. Use the canonical path because
	// managed directory traversal intentionally rejects symlink components.
	return AppDirectories{Runtime: "/private/var/run", Config: "/Library/Application Support", State: "/Library/Application Support", Logs: "/Library/Logs"}, nil
}

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
	if err := rejectSymlink(o.files, p); err != nil {
		return EnsureResult{}, err
	}
	if o.spec.scope == User {
		if err := ensureDefinitionDirectory(o.files, p); err != nil {
			return EnsureResult{}, err
		}
	}
	w := []byte(plist(o))
	old, e := readDefinition(o.files, p)
	absent := errors.Is(e, fs.ErrNotExist)
	changed := e != nil || string(old) != string(w)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return EnsureResult{}, e
	}
	if changed {
		if e = writeDefinition(o.files, p, w, 0644); e != nil {
			return EnsureResult{}, e
		}
		if o.spec.scope == System {
			if e = setDefinitionOwner(o.files, p, 0, 0, 0o644); e != nil {
				return EnsureResult{}, e
			}
			if e = verifyDefinitionMode(o.files, p, 0o644); e != nil {
				return EnsureResult{}, e
			}
			if e = verifyDefinitionOwner(o.files, p, 0, 0); e != nil {
				return EnsureResult{}, e
			}
		}
	}
	result := EnsureResult{Installed: true, Changed: changed, Reasons: changeReasons(old, o.spec, changed, absent)}
	if changed {
		result.RestartRequired, _ = launchdLoaded(c, o)
	}
	return result, nil
}
func (launchdBackend) start(c context.Context, o *operation) error {
	d := domain(o)
	if _, err := commandOutput(c, o.runner, "launchctl", "enable", d+"/"+o.spec.name); err != nil {
		return err
	}
	loaded, err := launchdLoaded(c, o)
	if err != nil {
		return err
	}
	if !loaded {
		if _, e := commandOutput(c, o.runner, "launchctl", "bootstrap", d, launchdPath(o)); e != nil {
			return e
		}
	}
	_, e := commandOutput(c, o.runner, "launchctl", "kickstart", d+"/"+o.spec.name)
	return e
}
func (launchdBackend) restart(c context.Context, o *operation) error {
	d := domain(o)
	loaded, err := launchdLoaded(c, o)
	if err != nil {
		return err
	}
	if !loaded {
		if _, e := commandOutput(c, o.runner, "launchctl", "bootstrap", d, launchdPath(o)); e != nil {
			return e
		}
	}
	_, err = commandOutput(c, o.runner, "launchctl", "kickstart", "-k", d+"/"+o.spec.name)
	return err
}
func (launchdBackend) stop(c context.Context, o *operation) error {
	if _, err := o.files.stat(launchdPath(o)); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	d := domain(o)
	if _, err := commandOutput(c, o.runner, "launchctl", "disable", d+"/"+o.spec.name); err != nil {
		return err
	}
	_, e := commandOutput(c, o.runner, "launchctl", "bootout", d+"/"+o.spec.name)
	return e
}
func launchdLoaded(c context.Context, o *operation) (bool, error) {
	_, err := o.runner.run(c, "launchctl", "print", domain(o)+"/"+o.spec.name)
	if err == nil {
		return true, nil
	}
	if ctxErr := c.Err(); ctxErr != nil {
		return false, errx.New("service: checking launchd service", errx.WithCause(ctxErr))
	}
	if managerErr := unavailableManager(err, "launchctl"); managerErr != nil {
		return false, managerErr
	}
	return false, nil
}
func (b launchdBackend) uninstall(c context.Context, o *operation) error {
	_ = b.stop(c, o)
	p := launchdPath(o)
	if err := rejectSymlink(o.files, p); err != nil {
		return err
	}
	e := removeDefinition(o.files, p)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return e
	}
	return nil
}
func (launchdBackend) status(c context.Context, o *operation) (Status, error) {
	_, e := o.files.stat(launchdPath(o))
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return Status{}, errx.New("service: inspecting definition "+launchdPath(o), errx.WithCause(e))
	}
	s := Status{Installed: e == nil}
	out, x := o.runner.run(c, "launchctl", "print", domain(o)+"/"+o.spec.name)
	if x != nil {
		if err := unavailableManager(x, "launchctl"); err != nil {
			return Status{}, err
		}
	}
	s.Loaded = x == nil
	s.Detail = strings.TrimSpace(out)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid =") {
			s.PID, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "pid =")))
		}
		if strings.HasPrefix(line, "last exit code =") {
			if code, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "last exit code ="))); err == nil {
				s.ExitCode = &code
			}
		}
	}
	s.Running = s.PID > 0
	if disabled, err := o.runner.run(c, "launchctl", "print-disabled", domain(o)); err == nil {
		s.Enabled = !strings.Contains(disabled, `"`+o.spec.name+`" => true`)
	}
	return s, nil
}
func plist(o *operation) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!-- compage-spec: ")
	b.WriteString(metadataComment(o.spec))
	b.WriteString(" -->\n")
	if o.spec.revision != "" {
		b.WriteString("<!-- compage-revision: ")
		b.WriteString(xml(o.spec.revision))
		b.WriteString(" -->\n")
	}
	b.WriteString("<plist version=\"1.0\"><dict>\n<key>Label</key><string>")
	b.WriteString(xml(o.spec.name))
	b.WriteString("</string>\n<key>ProgramArguments</key><array>\n<string>")
	b.WriteString(xml(o.spec.binary))
	b.WriteString("</string>\n")
	for _, a := range o.spec.args {
		b.WriteString("<string>")
		b.WriteString(xml(a))
		b.WriteString("</string>\n")
	}
	b.WriteString("</array>\n")
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
	if o.spec.restart == RestartAlways {
		b.WriteString("<key>KeepAlive</key><true/>\n")
	} else if o.spec.restart == RestartOnFailure {
		b.WriteString("<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>\n")
	}
	b.WriteString("<key>RunAtLoad</key><true/>\n</dict></plist>\n")
	return b.String()
}
func xml(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(v)
}
