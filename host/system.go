package host

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/gxmmx/compage-go/errx"
)

type systemQuery interface {
	effectiveUID() int
	lookupUser(id string) (*user.User, error)
	currentUser() (*user.User, error)
	executable() (string, error)
	evalSymlinks(path string) (string, error)
	getwd() (string, error)
}

type system struct{}

func (system) effectiveUID() int                        { return os.Geteuid() }
func (system) lookupUser(id string) (*user.User, error) { return user.LookupId(id) }
func (system) currentUser() (*user.User, error)         { return user.Current() }
func (system) executable() (string, error)              { return os.Executable() }
func (system) evalSymlinks(path string) (string, error) { return filepath.EvalSymlinks(path) }
func (system) getwd() (string, error)                   { return os.Getwd() }

// User reports the effective identity of the current process. On supported
// Unix systems it resolves the effective UID; elsewhere it uses the current
// operating-system user record.
func User() (UserInfo, error) {
	return currentUser(system{}, runtime.GOOS)
}

func currentUser(query systemQuery, goos string) (UserInfo, error) {
	var (
		record *user.User
		err    error
		uid    string
	)

	if OS(goos).IsUnix() {
		uid = strconv.Itoa(query.effectiveUID())
		record, err = query.lookupUser(uid)
	} else {
		record, err = query.currentUser()
	}
	if err != nil {
		return UserInfo{}, errx.New("host: resolving current user", errx.WithCause(err))
	}
	if record == nil {
		return UserInfo{}, errx.New("host: resolving current user: empty user record")
	}
	if uid == "" {
		uid = record.Uid
	}

	return UserInfo{
		UID:    uid,
		GID:    record.Gid,
		Name:   record.Username,
		Home:   record.HomeDir,
		IsRoot: isRoot(goos, query.effectiveUID()),
	}, nil
}

// IsRoot reports whether the current process has effective UID zero on a
// supported Unix platform. It is a preflight only, not a guarantee that a
// requested operation is authorized.
func IsRoot() bool {
	return isRoot(runtime.GOOS, system{}.effectiveUID())
}

func isRoot(goos string, effectiveUID int) bool {
	return OS(goos).IsUnix() && effectiveUID == 0
}

// Process reports the current process ID and canonical executable path.
func Process() (ProcessInfo, error) {
	return currentProcess(system{})
}

func currentProcess(query systemQuery) (ProcessInfo, error) {
	executable, err := query.executable()
	if err != nil {
		return ProcessInfo{}, errx.New("host: resolving executable", errx.WithCause(err))
	}
	canonical, err := query.evalSymlinks(executable)
	if err != nil {
		return ProcessInfo{}, errx.New(
			"host: canonicalising executable "+executable,
			errx.WithCause(err),
		)
	}
	return ProcessInfo{PID: os.Getpid(), Executable: canonical}, nil
}

// WorkingDir reports the current process working directory.
func WorkingDir() (string, error) {
	return workingDir(system{})
}

func workingDir(query systemQuery) (string, error) {
	dir, err := query.getwd()
	if err != nil {
		return "", errx.New("host: resolving working directory", errx.WithCause(err))
	}
	return dir, nil
}
