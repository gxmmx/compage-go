package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/host"
)

// AppDirectories are the resolved locations managed for a service. Empty
// fields were not declared (except Config, which is implicit with any other
// managed user directory).
type AppDirectories struct {
	Runtime string
	Config  string
	State   string
	Logs    string
}

// DirectoryChange describes a directory created or reconciled by Ensure.
type DirectoryChange struct {
	Path    string
	Created bool
}

// PurgeOptions controls which persistent managed locations are removed.
// Runtime data is removed by Uninstall and is therefore intentionally absent.
type PurgeOptions struct {
	Config bool
	State  bool
	Logs   bool
}

type managedDirectories struct {
	AppDirectories
	runtime, config, state, logs bool
	configWritable               bool
}

func userDirectoryBases(home string) (AppDirectories, error) {
	if home == "" || !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return AppDirectories{}, &ValidationError{Message: "invalid user home directory"}
	}
	return AppDirectories{Runtime: home, Config: home, State: home, Logs: home}, nil
}

func resolveDirectories(s specification, provider directoryProvider, u host.UserInfo) (managedDirectories, error) {
	any := s.runtimeDir != nil || s.configDir != nil || s.stateDir != nil || s.logDir != nil || s.stdoutLog != nil || s.stderrLog != nil
	d := managedDirectories{AppDirectories: AppDirectories{}, configWritable: s.configWritable}
	if !any {
		return d, nil
	}
	bases, err := provider.directoryBases(s.scope, u.Home)
	if err != nil {
		return managedDirectories{}, err
	}
	if s.scope == User || s.configDir != nil {
		configName := "." + s.name
		if s.configDir != nil && *s.configDir != "" {
			configName = *s.configDir
		}
		if s.scope == System && s.configDir == nil {
			configName = s.name
		}
		d.Config = filepath.Join(bases.Config, configName)
		d.config = true
	}
	if s.runtimeDir != nil {
		d.runtime = true
		d.Runtime = directoryPath(s, bases.Runtime, *s.runtimeDir, "run")
	}
	if s.stateDir != nil {
		d.state = true
		d.State = directoryPath(s, bases.State, *s.stateDir, "lib")
	}
	if s.logDir != nil || s.stdoutLog != nil || s.stderrLog != nil {
		d.logs = true
		d.Logs = directoryPath(s, bases.Logs, value(s.logDir), "log")
	}
	if s.scope == User {
		// User runtime/state/logs are children of the chosen config root.
		if d.runtime {
			d.Runtime = filepath.Join(d.Config, userChild(value(s.runtimeDir), "run"))
		}
		if d.state {
			d.State = filepath.Join(d.Config, userChild(value(s.stateDir), "lib"))
		}
		if d.logs {
			d.Logs = filepath.Join(d.Config, userChild(value(s.logDir), "log"))
		}
	}
	return d, nil
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func directoryPath(s specification, base, name, fallback string) string {
	if name == "" {
		name = s.name
	}
	return filepath.Join(base, name)
}
func userChild(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}

func (o *operation) reconcileDirectories(d managedDirectories, uid, gid string) ([]DirectoryChange, error) {
	uidNum, err := strconv.Atoi(uid)
	if err != nil {
		return nil, errx.New("service: invalid runtime UID", errx.WithCause(err))
	}
	gidNum, err := strconv.Atoi(gid)
	if err != nil {
		return nil, errx.New("service: invalid runtime GID", errx.WithCause(err))
	}
	var changes []DirectoryChange
	for _, item := range []struct {
		path   string
		config bool
	}{{d.Config, true}, {d.Runtime, false}, {d.State, false}, {d.Logs, false}} {
		if item.path == "" {
			continue
		}
		mode := os.FileMode(0o770)
		if o.spec.scope == User {
			mode = 0o700
		} else if item.config && !d.configWritable {
			mode = 0o750
		}
		owner := uidNum
		group := gidNum
		if o.spec.scope == System {
			owner = 0
		}
		created, err := ensureManagedDirectory(o.files, item.path, owner, group, mode)
		if err != nil {
			return changes, err
		}
		if created {
			changes = append(changes, DirectoryChange{Path: item.path, Created: true})
		}
	}
	if d.logs {
		for _, log := range []struct {
			request           *string
			path, defaultName string
		}{{o.spec.stdoutLog, "", "out.log"}, {o.spec.stderrLog, "", "err.log"}} {
			if log.request == nil {
				continue
			}
			path := resolvedLogPath(d.Logs, *log.request, log.defaultName)
			if err := ensureManagedLog(o.files, path, uidNum, gidNum, o.spec.scope == System); err != nil {
				return changes, err
			}
			if log.defaultName == "out.log" {
				o.spec.stdout = path
			} else {
				o.spec.stderr = path
			}
		}
	}
	return changes, nil
}

func resolvedLogPath(dir, name, fallback string) string {
	if name == "" {
		name = fallback
	}
	return filepath.Join(dir, name)
}

func ensureManagedDirectory(f files, path string, uid, gid int, mode os.FileMode) (bool, error) {
	parts := splitAbsolute(path)
	for i, part := range parts {
		info, err := f.lstat(part)
		final := i == len(parts)-1
		if errors.Is(err, fs.ErrNotExist) {
			if err := f.mkdir(part, mode); err != nil {
				return false, errx.New("service: creating directory "+part, errx.WithCause(err))
			}
			if err := f.chown(part, uid, gid); err != nil {
				return false, errx.New("service: owning directory "+part, errx.WithCause(err))
			}
			if err := f.chmod(part, mode); err != nil {
				return false, errx.New("service: setting directory mode "+part, errx.WithCause(err))
			}
			if final {
				return true, nil
			}
			continue
		}
		if err != nil {
			return false, errx.New("service: inspecting directory "+part, errx.WithCause(err))
		}
		if info == nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false, &ValidationError{Message: "managed directory path contains a non-directory or symlink: " + part}
		}
		if final {
			if err := f.chown(part, uid, gid); err != nil {
				return false, errx.New("service: owning directory "+part, errx.WithCause(err))
			}
			if err := f.chmod(part, mode); err != nil {
				return false, errx.New("service: setting directory mode "+part, errx.WithCause(err))
			}
		} else if err := traversableDirectory(info, uid, gid); err != nil {
			return false, errx.New("service: checking managed directory ancestor "+part, errx.WithCause(err))
		}
	}
	return false, nil
}

func traversableDirectory(info os.FileInfo, uid, gid int) error {
	if uid == 0 {
		return nil
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return errx.New("service: unsupported directory ownership metadata")
	}
	mode := info.Mode().Perm()
	allowed := mode&0o001 != 0
	if int(stat.Uid) == uid {
		allowed = mode&0o100 != 0
	} else if int(stat.Gid) == gid {
		allowed = mode&0o010 != 0
	}
	if !allowed {
		return &PrivilegeError{Capability: "traverse managed directory ancestor"}
	}
	return nil
}

func splitAbsolute(path string) []string {
	clean := filepath.Clean(path)
	var out []string
	current := string(filepath.Separator)
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part != "" {
			current = filepath.Join(current, part)
			out = append(out, current)
		}
	}
	return out
}

func ensureManagedLog(f files, path string, uid, gid int, system bool) error {
	info, err := f.lstat(path)
	if err == nil && (info == nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return &ValidationError{Message: "managed log file must be a regular file: " + path}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errx.New("service: inspecting log file "+path, errx.WithCause(err))
	}
	mode := os.FileMode(0o600)
	if system {
		mode = 0o660
	}
	if errors.Is(err, fs.ErrNotExist) {
		if err := f.write(path, nil, mode); err != nil {
			return errx.New("service: creating log file "+path, errx.WithCause(err))
		}
	}
	if err := f.chown(path, uid, gid); err != nil {
		return errx.New("service: owning log file "+path, errx.WithCause(err))
	}
	if err := f.chmod(path, mode); err != nil {
		return errx.New("service: setting log file mode "+path, errx.WithCause(err))
	}
	return nil
}

func (o *operation) cleanupManaged(path string) error {
	if path == "" {
		return nil
	}
	for _, component := range splitAbsolute(path) {
		info, err := o.files.lstat(component)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info == nil || info.Mode()&os.ModeSymlink != 0 {
			return &ValidationError{Message: "refusing to remove symlinked managed directory"}
		}
	}
	if err := o.files.removeAll(path); err != nil {
		return fmt.Errorf("service: removing managed directory %s: %w", path, err)
	}
	return nil
}
