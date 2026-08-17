package service

import (
	"context"
	"github.com/gxmmx/compage-go/account"
	"github.com/gxmmx/compage-go/host"
	"os"
	"os/exec"
	"path/filepath"
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
	stat(string) (os.FileInfo, error)
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
func (osFiles) stat(p string) (os.FileInfo, error)     { return os.Stat(p) }

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
