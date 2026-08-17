package service

import (
	"context"
	"errors"
	"github.com/gxmmx/compage-go/account"
	"github.com/gxmmx/compage-go/host"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gxmmx/compage-go/errx"
)

type runner interface {
	run(context.Context, string, ...string) (string, error)
}
type execRunner struct{}

func (execRunner) run(c context.Context, n string, a ...string) (string, error) {
	o, e := exec.CommandContext(c, n, a...).CombinedOutput()
	return string(o), e
}

type files interface {
	read(string) ([]byte, error)
	write(string, []byte, os.FileMode) error
	mkdirAll(string, os.FileMode) error
	remove(string) error
	chmod(string, os.FileMode) error
	chown(string, int, int) error
	stat(string) (os.FileInfo, error)
	lstat(string) (os.FileInfo, error)
}
type osFiles struct{}

func (osFiles) read(p string) ([]byte, error) { return os.ReadFile(p) }
func (osFiles) write(p string, b []byte, m os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".service-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(m); err == nil {
		_, err = tmp.Write(b)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, p); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(p))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (osFiles) mkdirAll(p string, m os.FileMode) error { return os.MkdirAll(p, m) }
func (osFiles) remove(p string) error                  { return os.Remove(p) }
func (osFiles) chmod(p string, m os.FileMode) error    { return os.Chmod(p, m) }
func (osFiles) chown(p string, u, g int) error         { return os.Chown(p, u, g) }
func (osFiles) stat(p string) (os.FileInfo, error)     { return os.Stat(p) }
func (osFiles) lstat(p string) (os.FileInfo, error)    { return os.Lstat(p) }

func rejectSymlink(files files, path string) error {
	info, err := files.lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info == nil {
		return errx.New("service: inspecting definition path: empty file information")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return &ValidationError{Message: "service definition path must not be a symlink"}
	}
	return nil
}

func commandOutput(ctx context.Context, r runner, name string, args ...string) (string, error) {
	out, err := r.run(ctx, name, args...)
	if err == nil {
		return out, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", errx.New("service: command cancelled: "+name, errx.WithCause(errors.Join(ctxErr, err)))
	}
	message := "service: command failed: " + name
	if len(args) > 0 {
		message += " " + strings.Join(args, " ")
	}
	if detail := strings.TrimSpace(out); detail != "" {
		message += ": " + detail
	}
	wrapped := errx.New(message, errx.WithCause(err))
	if errors.Is(err, exec.ErrNotFound) {
		return "", &UnsupportedError{Capability: name, Cause: wrapped}
	}
	return "", wrapped
}

func readDefinition(files files, path string) ([]byte, error) {
	data, err := files.read(path)
	if err != nil {
		return nil, errx.New("service: reading definition "+path, errx.WithCause(err))
	}
	return data, nil
}
func writeDefinition(files files, path string, data []byte, mode os.FileMode) error {
	if err := files.write(path, data, mode); err != nil {
		return errx.New("service: writing definition "+path, errx.WithCause(err))
	}
	return nil
}
func removeDefinition(files files, path string) error {
	if err := files.remove(path); err != nil {
		return errx.New("service: removing definition "+path, errx.WithCause(err))
	}
	return nil
}
func setDefinitionOwner(files files, path string, uid, gid int, mode os.FileMode) error {
	if err := files.chown(path, uid, gid); err != nil {
		return errx.New("service: owning definition "+path, errx.WithCause(err))
	}
	if err := files.chmod(path, mode); err != nil {
		return errx.New("service: setting definition mode "+path, errx.WithCause(err))
	}
	return nil
}
func verifyDefinitionMode(files files, path string, mode os.FileMode) error {
	info, err := files.stat(path)
	if err != nil {
		return errx.New("service: verifying definition "+path, errx.WithCause(err))
	}
	if info == nil {
		return errx.New("service: verifying definition " + path + ": empty file information")
	}
	if info.Mode().Perm() != mode.Perm() {
		return &ValidationError{Message: "definition has unexpected mode"}
	}
	return nil
}
func verifyDefinitionOwner(files files, path string, uid, gid int) error {
	info, err := files.stat(path)
	if err != nil {
		return errx.New("service: verifying definition owner "+path, errx.WithCause(err))
	}
	if info == nil {
		return errx.New("service: verifying definition owner " + path + ": empty file information")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return errx.New("service: unsupported definition ownership metadata")
	}
	if int(stat.Uid) != uid || int(stat.Gid) != gid {
		return &ValidationError{Message: "definition has unexpected ownership"}
	}
	return nil
}

type backend interface {
	validate(specification) error
	ensure(context.Context, *operation) (EnsureResult, error)
	start(context.Context, *operation) error
	stop(context.Context, *operation) error
	uninstall(context.Context, *operation) error
	status(context.Context, *operation) (Status, error)
}
type operation struct {
	spec          specification
	platform      host.PlatformInfo
	user          host.UserInfo
	root          bool
	backend       backend
	runner        runner
	files         files
	ensureAccount func(context.Context, account.Spec) (account.EnsureResult, error)
}

func platformBackend(o host.OS) (backend, error) {
	switch o {
	case host.Linux:
		return systemdBackend{}, nil
	case host.Darwin:
		return launchdBackend{}, nil
	default:
		return nil, &UnsupportedError{Capability: string(o)}
	}
}
