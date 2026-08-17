package account

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/host"
)

func productionDeps() dependencies {
	p := host.Platform()
	return dependencies{platform: p, root: host.IsRoot(), backend: newSystemBackend(p.OS)}
}

type accountStore interface {
	lookup(string) (*user.User, error)
	lookupID(string) (*user.User, error)
	groupIDs(*user.User) ([]string, error)
	lookupGroup(string) (*user.Group, error)
	lookupGroupID(string) (*user.Group, error)
}
type commandRunner interface {
	output(context.Context, string, ...string) (string, error)
}
type fileSystem interface {
	stat(string) (os.FileInfo, error)
	mkdirAll(string, os.FileMode) error
	chown(string, int, int) error
	chmod(string, os.FileMode) error
}
type osStore struct{}

func (osStore) lookup(name string) (*user.User, error)       { return user.Lookup(name) }
func (osStore) lookupID(id string) (*user.User, error)       { return user.LookupId(id) }
func (osStore) groupIDs(u *user.User) ([]string, error)      { return u.GroupIds() }
func (osStore) lookupGroup(name string) (*user.Group, error) { return user.LookupGroup(name) }
func (osStore) lookupGroupID(id string) (*user.Group, error) { return user.LookupGroupId(id) }

type execRunner struct{}

func (execRunner) output(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

type osFS struct{}

func (osFS) stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (osFS) mkdirAll(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }
func (osFS) chown(path string, uid, gid int) error        { return os.Chown(path, uid, gid) }
func (osFS) chmod(path string, mode os.FileMode) error    { return os.Chmod(path, mode) }

type systemBackend struct {
	os     host.OS
	store  accountStore
	runner commandRunner
	fs     fileSystem
}

func newSystemBackend(osName host.OS) systemBackend {
	return systemBackend{os: osName, store: osStore{}, runner: execRunner{}, fs: osFS{}}
}

func (b systemBackend) lookup(ctx context.Context, key string, byID bool) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, errx.New("account: lookup cancelled", errx.WithCause(err))
	}
	var u *user.User
	var err error
	if byID {
		u, err = b.store.lookupID(key)
	} else {
		u, err = b.store.lookup(key)
	}
	if err != nil {
		return Record{}, &NotFoundError{Key: key, Cause: err}
	}
	groups, err := b.store.groupIDs(u)
	if err != nil {
		return Record{}, errx.New("account: resolving groups for "+u.Username, errx.WithCause(err))
	}
	var names []string
	primary := u.Gid
	for _, gid := range groups {
		g, e := b.store.lookupGroupID(gid)
		if e != nil {
			return Record{}, errx.New("account: resolving group "+gid, errx.WithCause(e))
		}
		if gid == u.Gid {
			primary = g.Name
		} else {
			names = append(names, g.Name)
		}
	}
	home, shell, err := b.accountPaths(ctx, u.Username, u.HomeDir)
	if err != nil {
		return Record{}, err
	}
	return Record{Name: u.Username, UID: u.Uid, GID: u.Gid, Group: primary, Home: home, Shell: shell, Groups: names}, nil
}

func (b systemBackend) accountPaths(ctx context.Context, name, fallbackHome string) (string, string, error) {
	if b.os == host.Darwin {
		out, err := b.commandOutput(ctx, "dscl", ".", "-read", "/Users/"+name, "NFSHomeDirectory", "UserShell")
		if err != nil {
			return "", "", err
		}
		values := map[string]string{}
		for _, line := range strings.Split(out, "\n") {
			if parts := strings.SplitN(strings.TrimSpace(line), ":", 2); len(parts) == 2 {
				values[parts[0]] = strings.TrimSpace(parts[1])
			}
		}
		return values["NFSHomeDirectory"], values["UserShell"], nil
	}
	out, err := b.commandOutput(ctx, "getent", "passwd", name)
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(out), ":")
	if len(parts) != 7 {
		return "", "", errx.New("account: malformed passwd record for " + name)
	}
	return parts[5], parts[6], nil
}

func (b systemBackend) apply(ctx context.Context, s Spec, existing *Record) error {
	if err := ctx.Err(); err != nil {
		return errx.New("account: reconcile cancelled", errx.WithCause(err))
	}
	if s.Group != "" {
		if err := b.ensureGroup(ctx, s); err != nil {
			return err
		}
	}
	if existing == nil {
		if err := b.create(ctx, s); err != nil {
			return err
		}
	} else if err := b.modify(ctx, s); err != nil {
		return err
	}
	if s.HomePolicy == EnsureHome {
		return b.ensureHome(ctx, s)
	}
	return nil
}

func (b systemBackend) homeExists(ctx context.Context, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errx.New("account: home inspection cancelled", errx.WithCause(err))
	}
	_, err := b.fs.stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, errx.New("account: inspecting home "+path, errx.WithCause(err))
}
func (b systemBackend) ensureGroup(ctx context.Context, s Spec) error {
	if group, err := b.store.lookupGroup(s.Group); err == nil {
		if s.GID != nil && group.Gid != strconv.Itoa(*s.GID) {
			return &DriftError{Changes: []Change{{Field: "group gid", Before: group.Gid, After: strconv.Itoa(*s.GID)}}}
		}
		return nil
	}
	args := []string{"groupadd"}
	if s.GID != nil {
		args = append(args, "--gid", strconv.Itoa(*s.GID))
	}
	args = append(args, s.Group)
	if b.os == host.Darwin {
		args = []string{"dseditgroup", "-o", "create"}
		if s.GID != nil {
			args = append(args, "-i", strconv.Itoa(*s.GID))
		}
		args = append(args, s.Group)
	}
	return b.runArgs(ctx, args)
}
func (b systemBackend) create(ctx context.Context, s Spec) error {
	if b.os == host.Darwin {
		return b.createDarwin(ctx, s)
	}
	args := []string{"useradd"}
	if s.Kind == System {
		args = append(args, "--system")
	}
	if s.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*s.UID))
	}
	if s.Group != "" {
		args = append(args, "--gid", s.Group)
	}
	if s.Home != "" {
		args = append(args, "--home-dir", s.Home)
	}
	if s.HomePolicy == EnsureHome {
		args = append(args, "--create-home")
	}
	if s.Shell != "" {
		args = append(args, "--shell", s.Shell)
	}
	if len(s.Groups) > 0 {
		args = append(args, "--groups", strings.Join(s.Groups, ","))
	}
	args = append(args, s.Name)
	return b.runArgs(ctx, args)
}
func (b systemBackend) modify(ctx context.Context, s Spec) error {
	if b.os == host.Darwin {
		return b.modifyDarwin(ctx, s)
	}
	args := []string{"usermod"}
	if s.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*s.UID))
	}
	if s.Group != "" {
		args = append(args, "--gid", s.Group)
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		args = append(args, "--home", s.Home)
	}
	if s.Shell != "" {
		args = append(args, "--shell", s.Shell)
	}
	if s.Groups != nil {
		args = append(args, "--groups", strings.Join(s.Groups, ","))
	}
	if len(args) == 1 {
		return nil
	}
	return b.runArgs(ctx, append(args, s.Name))
}
func (b systemBackend) createDarwin(ctx context.Context, s Spec) error {
	if err := b.run(ctx, "dscl", ".", "-create", "/Users/"+s.Name); err != nil {
		return err
	}
	return b.modifyDarwin(ctx, s)
}
func (b systemBackend) modifyDarwin(ctx context.Context, s Spec) error {
	if s.UID != nil {
		if err := b.run(ctx, "dscl", ".", "-create", "/Users/"+s.Name, "UniqueID", strconv.Itoa(*s.UID)); err != nil {
			return err
		}
	}
	if s.Group != "" {
		g, err := b.store.lookupGroup(s.Group)
		if err != nil {
			return errx.New("account: resolving primary group "+s.Group, errx.WithCause(err))
		}
		if err := b.run(ctx, "dscl", ".", "-create", "/Users/"+s.Name, "PrimaryGroupID", g.Gid); err != nil {
			return err
		}
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		if err := b.run(ctx, "dscl", ".", "-create", "/Users/"+s.Name, "NFSHomeDirectory", s.Home); err != nil {
			return err
		}
	}
	if s.Shell != "" {
		if err := b.run(ctx, "dscl", ".", "-create", "/Users/"+s.Name, "UserShell", s.Shell); err != nil {
			return err
		}
	}
	for _, g := range s.Groups {
		if err := b.run(ctx, "dseditgroup", "-o", "edit", "-a", s.Name, "-t", "user", g); err != nil {
			return err
		}
	}
	return nil
}
func (b systemBackend) ensureHome(ctx context.Context, s Spec) error {
	if err := ctx.Err(); err != nil {
		return errx.New("account: home reconciliation cancelled", errx.WithCause(err))
	}
	if err := b.fs.mkdirAll(s.Home, homeMode(s)); err != nil {
		return errx.New("account: creating home "+s.Home, errx.WithCause(err))
	}
	r, err := b.lookup(ctx, s.Name, false)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(r.UID)
	gid, _ := strconv.Atoi(r.GID)
	if err := b.fs.chown(s.Home, uid, gid); err != nil {
		return errx.New("account: owning home "+s.Home, errx.WithCause(err))
	}
	if err := b.fs.chmod(s.Home, homeMode(s)); err != nil {
		return errx.New("account: chmod home "+s.Home, errx.WithCause(err))
	}
	return nil
}
func homeMode(s Spec) os.FileMode {
	if s.HomeMode != 0 {
		return s.HomeMode.Perm()
	}
	return 0o755
}
func (b systemBackend) run(ctx context.Context, name string, args ...string) error {
	_, err := b.commandOutput(ctx, name, args...)
	if err != nil {
		return err
	}
	return nil
}

func (b systemBackend) commandOutput(ctx context.Context, name string, args ...string) (string, error) {
	out, err := b.runner.output(ctx, name, args...)
	if err != nil {
		return "", errx.New("account: command failed: "+name+" "+strings.Join(args, " ")+": "+strings.TrimSpace(string(out)), errx.WithCause(err))
	}
	return string(out), nil
}

func (b systemBackend) runArgs(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return b.run(ctx, args[0], args[1:]...)
}
