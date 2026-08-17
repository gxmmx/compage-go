package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/gxmmx/compage-go/errx"
)

func (o *operation) ensure(c context.Context) (EnsureResult, error) {
	if e := c.Err(); e != nil {
		return EnsureResult{}, errx.New("service: ensure cancelled", errx.WithCause(e))
	}
	if err := o.systemPreflight(); err != nil {
		return EnsureResult{}, err
	}
	info, e := o.files.stat(o.spec.binary)
	if e != nil {
		return EnsureResult{}, errx.New("service: inspecting binary", errx.WithCause(e))
	}
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return EnsureResult{}, &ValidationError{Message: "binary must be a regular executable file"}
	}
	var r EnsureResult
	if o.spec.account != nil {
		a, e := o.ensureAccount(c, *o.spec.account)
		r.Account = &a
		if a.Created || len(a.Changed) > 0 {
			r.Changed = true
			r.Reasons = append(r.Reasons, AccountChanged)
		}
		if e != nil {
			return r, errx.New("service: ensuring runtime account", errx.WithCause(e))
		}
	}
	uid, gid := o.user.UID, o.user.GID
	if r.Account != nil {
		uid, gid = r.Account.Account.UID, r.Account.Account.GID
	}
	d, err := resolveDirectories(o.spec, o.backend, o.user)
	if err != nil {
		return r, errx.New("service: resolving directories", errx.WithCause(err))
	}
	r.Directories = d.AppDirectories
	changes, err := o.reconcileDirectories(d, uid, gid)
	r.DirectoryChanges = changes
	if len(changes) > 0 {
		r.Changed = true
		r.Reasons = append(r.Reasons, DirectoriesChanged)
	}
	if err != nil {
		return r, err
	}
	x, e := o.backend.ensure(c, o)
	x.Account = r.Account
	x.Directories, x.DirectoryChanges = r.Directories, r.DirectoryChanges
	if r.Changed {
		x.Changed = true
		x.Reasons = append(x.Reasons, r.Reasons...)
	}
	x.Reasons = uniqueReasons(x.Reasons)
	return x, e
}

func (o *operation) systemPreflight() error {
	if o.spec.scope == System && !o.root {
		return &PrivilegeError{Capability: "manage system service"}
	}
	return nil
}

func uniqueReasons(in []ChangeReason) []ChangeReason {
	seen := make(map[ChangeReason]struct{}, len(in))
	out := make([]ChangeReason, 0, len(in))
	for _, reason := range in {
		if _, ok := seen[reason]; !ok {
			seen[reason] = struct{}{}
			out = append(out, reason)
		}
	}
	return out
}

func checkLogParents(files files, spec specification, uid, gid string) error {
	for _, path := range []string{spec.stdout, spec.stderr} {
		if path == "" {
			continue
		}
		parent := filepath.Dir(path)
		info, err := files.stat(parent)
		if err != nil {
			return errx.New("service: inspecting log directory "+parent, errx.WithCause(err))
		}
		if info == nil || !info.IsDir() {
			return &ValidationError{Message: "log parent must be an existing directory: " + parent}
		}
		if err := writableDirectory(info, uid, gid); err != nil {
			return errx.New("service: checking log directory access "+parent, errx.WithCause(err))
		}
	}
	return nil
}

func writableDirectory(info os.FileInfo, uid, gid string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return errx.New("service: unsupported log directory metadata")
	}
	wantUID, err := strconv.Atoi(uid)
	if err != nil {
		return errx.New("service: invalid runtime UID", errx.WithCause(err))
	}
	wantGID, err := strconv.Atoi(gid)
	if err != nil {
		return errx.New("service: invalid runtime GID", errx.WithCause(err))
	}
	if wantUID == 0 {
		return nil
	}
	mode := info.Mode().Perm()
	allowed := mode&0o003 == 0o003
	if int(stat.Uid) == wantUID {
		allowed = mode&0o300 == 0o300
	} else if int(stat.Gid) == wantGID {
		allowed = mode&0o030 == 0o030
	}
	if !allowed {
		return &PrivilegeError{Capability: "write log directory"}
	}
	return nil
}

func (o *operation) start(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: start cancelled", errx.WithCause(err))
	}
	if err := o.systemPreflight(); err != nil {
		return err
	}
	return o.backend.start(c, o)
}
func (o *operation) stop(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: stop cancelled", errx.WithCause(err))
	}
	if err := o.systemPreflight(); err != nil {
		return err
	}
	return o.backend.stop(c, o)
}
func (o *operation) uninstall(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: uninstall cancelled", errx.WithCause(err))
	}
	if err := o.systemPreflight(); err != nil {
		return err
	}
	if err := o.backend.uninstall(c, o); err != nil {
		return err
	}
	d, err := resolveDirectories(o.spec, o.backend, o.user)
	if err != nil {
		return err
	}
	if d.runtime {
		return o.cleanupManaged(d.Runtime)
	}
	return nil
}

func (o *operation) purge(c context.Context, options PurgeOptions) error {
	if err := c.Err(); err != nil {
		return errx.New("service: purge cancelled", errx.WithCause(err))
	}
	if err := o.systemPreflight(); err != nil {
		return err
	}
	d, err := resolveDirectories(o.spec, o.backend, o.user)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		selected, declared bool
		path               string
	}{{options.Config, d.config, d.Config}, {options.State, d.state, d.State}, {options.Logs, d.logs, d.Logs}} {
		if item.selected {
			if !item.declared {
				return &ValidationError{Message: "cannot purge an undeclared managed directory"}
			}
			if err := o.cleanupManaged(item.path); err != nil {
				return err
			}
		}
	}
	return nil
}
func (o *operation) status(c context.Context) (Status, error) {
	if err := c.Err(); err != nil {
		return Status{}, errx.New("service: status cancelled", errx.WithCause(err))
	}
	if err := o.systemPreflight(); err != nil {
		return Status{}, err
	}
	return o.backend.status(c, o)
}

var _ = os.ErrNotExist
