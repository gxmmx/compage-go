# Host package implementation plan

## Purpose

Add a small, read-only `host` package to `compage-go` that provides a coherent
API for facts about the host and the currently executing process.

The package wraps standard-library and OS-specific details behind stable,
focused functions. It does not mutate the machine, create accounts, manage
services, or make generic authorization claims.

## Public API

```go
platform := host.Platform()
if platform.OS == host.Linux {
    // Linux-specific behaviour
}

current, err := host.User()
if err != nil { /* effective user cannot be resolved */ }

if host.IsRoot() {
    // effective UID is root
}

process, err := host.Process()
if err != nil { /* executable cannot be resolved */ }

dir, err := host.WorkingDir()
if err != nil { /* current directory cannot be resolved */ }
```

```go
type OS string

const (
    Linux   OS = "linux"
    Darwin  OS = "darwin"
    Windows OS = "windows"
)

type PlatformInfo struct {
    OS   OS
    Arch string
}

type UserInfo struct {
    UID    string
    GID    string
    Name   string
    IsRoot bool
}

type ProcessInfo struct {
    PID        int
    Executable string
}

func Platform() PlatformInfo
func User() (UserInfo, error)
func IsRoot() bool
func Process() (ProcessInfo, error)
func WorkingDir() (string, error)
func (o OS) IsUnix() bool
```

`Platform` is a cheap runtime-platform wrapper and cannot fail. `User` reports
the effective identity of the current process; it does not use environment
variables such as `USER`, `HOME`, or `PATH` as authoritative sources.

`IsRoot` is the intentionally narrow, cheap privilege preflight. On the
initially supported Unix platforms it reports whether the effective UID is zero.
It does not assert that a requested operation will succeed: service-manager
authorization, filesystem ACLs, and OS policy are verified by the package that
performs the operation.

`Process` reports the current PID and canonical executable path. It returns an
error when the executable cannot be resolved. `WorkingDir` returns the process
working directory without reading any unrelated host/process details.

## Error handling

`host` has no domain-level failure taxonomy. Its fallible functions return an
unclassified `errx.Error` that wraps the original standard-library/OS cause.
Its `Kind()` is therefore `errx.Unknown`: failing to resolve the current
executable, working directory, or effective user is not enough information to
assign a transport-neutral semantic meaning. Callers that own a higher-level
operation may add a precise `errx` classification at their own boundary.

## Boundaries

`host` answers only facts about the currently executing process:

- OS and architecture;
- effective user identity and root status;
- process ID and executable path;
- current working directory.

It does not own:

- arbitrary account lookup, creation, group membership, or home directories
  (`account` owns local account management);
- service-manager discovery or permission checks (`service` owns those);
- generic `CanAccess(resource)` checks;
- Linux-distribution, kernel, container, CPU-feature, or filesystem-layout
  detection;
- environment policy or application configuration.

A generic access predicate is deliberately excluded. Filesystem ACLs, MAC
systems, mount policy, capabilities, service-manager authorization, and TOCTOU
make it unsafe to report a portable yes/no answer. The package that owns the
actual operation must validate and perform that operation, returning its
contextual result.

For example, `service` combines `host.IsRoot()` with service-manager discovery
before managing a system service. `account` uses the same check before account
or home-directory mutation. A future filesystem package would attempt an
appropriately scoped filesystem operation.

## Testability

Use small internal seams for platform, identity, executable, and
working-directory lookups. The exported API stays thin; consumers that need
cross-platform tests inject an internal adapter or the resulting values into
their own backend selection logic rather than relying on mutable global
overrides.

Tests are table-driven and do not mutate the host. Validate that each public
function invokes only the system query it needs.

## Delivery sequence

1. Add `compage-go/host` with package documentation, platform identity, and
   `IsRoot`.
2. Add current effective-user, process, and working-directory wrappers with
   explicit error contracts.
3. Add unit tests using internal system-query fakes.
4. Make `service` use `host.Platform`, `host.User`, and `host.IsRoot` for
   backend selection, user-scoped paths/domains, and system-scope preflight.
5. Make `account` use `host.Platform` and `host.IsRoot` before platform-specific
   mutations.
6. Add only facts that describe the current execution context; put host
   mutations and feature validation in the packages that own them.
