package host

import "runtime"

// OS identifies an operating system.
type OS string

const (
	// Linux identifies Linux hosts.
	Linux OS = "linux"
	// Darwin identifies macOS hosts.
	Darwin OS = "darwin"
	// Windows identifies Windows hosts.
	Windows OS = "windows"
)

// PlatformInfo describes the operating system and processor architecture.
type PlatformInfo struct {
	OS   OS
	Arch string
}

// UserInfo describes the effective identity of the current process.
type UserInfo struct {
	UID    string
	GID    string
	Name   string
	Home   string
	IsRoot bool
}

// ProcessInfo describes the current process.
type ProcessInfo struct {
	PID        int
	Executable string
}

// Platform reports the runtime operating system and architecture.
func Platform() PlatformInfo {
	return platform(runtime.GOOS, runtime.GOARCH)
}

// IsUnix reports whether o is an initially supported Unix platform.
func (o OS) IsUnix() bool {
	return o == Linux || o == Darwin
}

func platform(goos, arch string) PlatformInfo {
	return PlatformInfo{OS: OS(goos), Arch: arch}
}
