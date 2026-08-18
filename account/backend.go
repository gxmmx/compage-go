package account

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/host"
)

func productionDeps() dependencies {
	p := host.Platform()
	return dependencies{platform: p, root: host.IsRoot(), backend: platformBackend()}
}

// backendDeps are host-facing primitives.  They intentionally carry no
// account policy: that lives entirely in the platform-specific backends.
type backendDeps struct {
	store  accountStore
	runner commandRunner
	fs     fileSystem
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
	mkdir(string, os.FileMode) error
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

func (osFS) stat(path string) (os.FileInfo, error)     { return os.Stat(path) }
func (osFS) mkdir(path string, mode os.FileMode) error { return os.Mkdir(path, mode) }
func (osFS) chown(path string, uid, gid int) error     { return os.Chown(path, uid, gid) }
func (osFS) chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }

func defaultBackendDeps() backendDeps {
	return backendDeps{store: osStore{}, runner: execRunner{}, fs: osFS{}}
}

func commandOutput(ctx context.Context, runner commandRunner, name string, args ...string) (string, error) {
	out, err := runner.output(ctx, name, args...)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", errx.New("account: command cancelled: "+name, errx.WithCause(errors.Join(ctxErr, err)))
		}
		return "", errx.New("account: command failed: "+name+" "+strings.Join(args, " ")+": "+strings.TrimSpace(out), errx.WithCause(err))
	}
	return out, nil
}

func runCommand(ctx context.Context, runner commandRunner, name string, args ...string) error {
	_, err := commandOutput(ctx, runner, name, args...)
	return err
}

func homeExists(ctx context.Context, files fileSystem, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errx.New("account: home inspection cancelled", errx.WithCause(err))
	}
	_, err := files.stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, errx.New("account: inspecting home "+path, errx.WithCause(err))
}

func homeMode(s Spec) os.FileMode {
	if s.HomeMode != 0 {
		return s.HomeMode.Perm()
	}
	return 0o755
}

// ensureHomeDirectory creates only the target directory. Parents are never
// created implicitly, and an existing non-directory is never modified.
func ensureHomeDirectory(files fileSystem, path string, mode os.FileMode) error {
	err := files.mkdir(path, mode)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return errx.New("account: creating home "+path, errx.WithCause(err))
	}
	info, statErr := files.stat(path)
	if statErr != nil {
		return errx.New("account: inspecting existing home "+path, errx.WithCause(statErr))
	}
	if info == nil || !info.IsDir() {
		return &DriftError{Changes: []Change{{Field: "home directory", Before: "non-directory", After: "directory"}}}
	}
	return nil
}

func verifyHome(files fileSystem, path string, uid, gid int, mode os.FileMode) error {
	info, err := files.stat(path)
	if err != nil {
		return errx.New("account: verifying home "+path, errx.WithCause(err))
	}
	if info == nil {
		return errx.New("account: verifying home " + path + ": empty file information")
	}
	if !info.IsDir() {
		return &DriftError{Changes: []Change{{Field: "home directory", Before: "non-directory", After: "directory"}}}
	}
	if info.Mode().Perm() != mode.Perm() {
		return &DriftError{Changes: []Change{{Field: "home mode", Before: info.Mode().Perm().String(), After: mode.Perm().String()}}}
	}
	actualUID, actualGID, err := homeOwner(info)
	if err != nil {
		return errx.New("account: verifying home ownership "+path, errx.WithCause(err))
	}
	if actualUID != uid || actualGID != gid {
		return &DriftError{Changes: []Change{{Field: "home ownership", Before: strconv.Itoa(actualUID) + ":" + strconv.Itoa(actualGID), After: strconv.Itoa(uid) + ":" + strconv.Itoa(gid)}}}
	}
	return nil
}

func homeOwner(info fs.FileInfo) (int, int, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, 0, fmt.Errorf("unexpected home filesystem metadata")
	}
	return int(stat.Uid), int(stat.Gid), nil
}

func isUnknownGroup(err error) bool {
	var missing user.UnknownGroupError
	return errors.As(err, &missing)
}

func without(values []string, value string) []string {
	out := values[:0]
	for _, candidate := range values {
		if candidate != value {
			out = append(out, candidate)
		}
	}
	return out
}

// platformBackend returns the native backend compiled for the current target.
func platformBackend() backend {
	switch host.Platform().OS {
	case host.Linux:
		return newLinuxBackend()
	case host.Darwin:
		return newDarwinBackend()
	default:
		return unsupportedBackend{}
	}
}

// unsupportedBackend is selected only on targets other than Linux and macOS.
type unsupportedBackend struct{}

func (unsupportedBackend) lookup(context.Context, string, bool) (Record, error) {
	return Record{}, &UnsupportedError{Capability: "platform"}
}
func (unsupportedBackend) homeExists(context.Context, string) (bool, error) {
	return false, &UnsupportedError{Capability: "platform"}
}
func (unsupportedBackend) preflight(context.Context, Spec, bool) error {
	return &UnsupportedError{Capability: "platform"}
}
func (unsupportedBackend) apply(context.Context, Spec, *Record) ([]Change, error) {
	return nil, &UnsupportedError{Capability: "platform"}
}
