# Service-managed application directories

Status: Proposed

## Purpose

Extend `compage-go/service` so a privileged service installation can provision
the directories needed by the unprivileged program it starts. The service
manager creates and reconciles those directories while it has the required
privileges; the running program only reads and writes within the directories it
has been granted.

This is a pre-adoption API revision. `compage-go/service` is not yet consumed
by an external application, so this work makes the complete desired API change
at once. Backwards compatibility, deprecated aliases, and migration shims are
explicitly out of scope.

## Problem

`service.Manager.Ensure` can create a service account and a service definition,
but it cannot provision application locations such as configuration, durable
state, runtime sockets, or file logs. A system service runs as an unprivileged
account, so its program cannot create a missing system configuration or state
directory itself.

The current `WithStdoutLog` and `WithStderrLog` options require an existing,
writable log parent. This is safe, but leaves applications to create the parent
somewhere else.

## Design principles

- Directory names are semantic, not literal Unix paths. Applications say
  "configuration" or "runtime", not `"/etc"` or `"/var/run"`.
- Platform path resolution is shared with the unprivileged program. It must be
  possible to calculate paths without constructing a privileged
  `service.Manager`.
- `Ensure` reconciles service-owned directories after resolving the runtime
  account and before starting the program.
- A normal uninstall removes transient runtime files only. It preserves
  configuration, state, and logs.
- Purging user data is explicit and controlled by the consuming application.
- No directory is recursively removed unless it was declared as a managed
  application directory and the caller explicitly requested purge.

## Proposed public API

Keep directory resolution in `compage-go/service`. It is part of the service
backend contract rather than a host-wide convention: each backend supplies its
own system directory bases, while user-scope declarations are based on the
resolved account home directory. `host` remains unchanged.

```go
type AppDirectories struct {
    Runtime string
    Config  string
    State   string
    Logs    string
}

type systemDirs struct { Runtime, Config, State, Logs string }

func (backend) systemDirs() systemDirs
```

The backend returns bases only; it does not append an application name. The
service directory declarations append their supplied names, or the final
resolved service name when no name is supplied. This is necessary because a
single service may intentionally use different names for runtime, config,
state, and logs. At user scope, all bases are the effective account home
directory, so `WithConfigDir("foo")` resolves to `~/foo`; only the omitted
config name receives the default `~/.<resolved-service-name>`.

Add semantic directory declarations to `compage-go/service`:

```go
service.WithRuntimeDir("example")
service.WithConfigDir("example")
service.WithStateDir("example")
service.WithLogDir("example")
service.WithConfigDirWrite("example")
```

Each option accepts a clean relative directory hierarchy, not an absolute path.
For example, `"bar/baz"` and `".config/example"` are valid. Empty names,
absolute paths, traversal components, control characters, empty path components,
and non-clean spellings are invalid.

`WithConfigDir()` accepts no name as a convenience. Its default name is
`"." + resolvedServiceName`; the resolved service name is the explicit
`WithName` value, or the binary basename when `WithName` is omitted. Directory
defaults must therefore be resolved only after all options have been applied
and the final service name is known.

`WithConfigDirWrite` is the group-writable variant of `WithConfigDir` for a
system service. It declares the same kind of managed config directory but gives
the runtime account's primary group write access. Supplying both options is an
error. The final API names remain subject to normal package API review.

### Managed log directories and files

File logs must always live inside a named, managed log subdirectory. Compage
must never select a root log location such as `/var/log/out.log`; the resolved
form is always `/var/log/<directory>/<file>` (or its platform equivalent).

Update the existing log options to support default filenames:

```go
service.WithStdoutLog()                 // <log-dir>/out.log
service.WithStderrLog()                 // <log-dir>/err.log
service.WithStdoutLog("server-out.log")
service.WithStderrLog("server-err.log")
service.WithLogDir("example")
```

When either stdout or stderr logging is requested without `WithLogDir`, a
system service implicitly uses the resolved service name as the directory name.
That name is explicit when supplied through `WithName`; otherwise it is the
basename derived from the configured binary, matching the current `WithName`
default. A user service instead uses its config root's default `log` child.

Log filenames are simple file names, not paths: they must reject separators,
absolute paths, traversal components, and control characters. `Ensure` creates
the managed log directory and the selected `out.log` and/or `err.log` files with
the runtime account's ownership and safe file modes. The service definition then
uses those resolved absolute file paths.

The existing `WithLog` shared-destination behaviour needs an explicit
compatibility decision: retain it as a shared file inside `WithLogDir`, or
deprecate it in favour of separate stdout and stderr files. It must not retain
an arbitrary absolute-path escape hatch once managed log directories exist.

## Nested-directory ownership and mode rules

Managed directory names are hierarchies. Compage walks each component from its
platform base directory and handles it independently:

1. If a component already exists, it must be a real directory and is left
   unchanged when it is an intermediate ancestor: Compage does not alter its
   ownership or mode.
2. If a component is missing, Compage creates it with the ownership and mode
   requested by the declaring directory kind.
3. The final declared component is the managed directory. Whether newly
   created or already present, Compage reconciles its ownership and mode to the
   declaring directory policy.
4. A symlink at any component is rejected. Compage must not follow a symlink to
   reach a managed descendant.
5. Existing ancestor components are checked for enough traversal/read/write
   access for the intended runtime account. A wrong ancestor mode is reported
   as an actionable error rather than silently changed.

This supports a shared parent such as `WithConfigDir(".config/persephone")`:
an existing `~/.config` is not touched, while the missing `persephone` child is
created and managed. If `~/.config/persephone` already exists, it is still the
explicitly managed final component and its ownership/mode is reconciled. The
same rule applies to system paths. For example, `WithConfigDir("bar/baz")` on
Linux resolves below `/etc`: `/etc/bar` is an untouched ancestor when it
already exists, while `/etc/bar/baz` is always reconciled as the managed final
directory. With deeper nesting, only the final component receives that
reconciliation treatment.

Declaring a final component therefore claims exclusive management of it. An
application that needs to coexist inside an already shared final directory must
declare a new child directory instead.

For a system service, newly created configuration directories are owned by
`root:<runtime-primary-group>`. `WithConfigDir` uses mode `0750`: root has full
access and the runtime group has read/traverse (`r-x`) access. This lets the
program read configuration but not create or replace files. `WithConfigDirWrite`
uses `0770`: the runtime group receives `rwx`, allowing the unprivileged
program to create and update files in its configuration directory. Root remains
the owner in both cases.

## User-scope directory layout

User services are rooted in their effective user's home directory. A user-scope
config declaration is always present whenever any managed directory is requested:

```text
config root  ~/.<resolved-service-name>     (default)
runtime      <config root>/run
state        <config root>/lib
logs         <config root>/log
```

`WithConfigDir(".config/example")` changes the config root to
`~/.config/example`; runtime, state, and logs then resolve under that root as
`run`, `lib`, and `log`. A user-scope `WithRuntimeDir`, `WithStateDir`, or
`WithLogDir` without an explicit `WithConfigDir` implicitly enables the default
dot-prefixed config root first.

At user scope, `WithConfigDir` establishes the root and the other directory
options select children beneath it. An omitted or empty child name defaults to
`run`, `lib`, or `log` respectively:

```go
service.WithRuntimeDir()        // <config root>/run
service.WithStateDir()          // <config root>/lib
service.WithLogDir()            // <config root>/log
service.WithLogDir("baz")       // <config root>/baz
```

Explicit user-scope child names remain clean relative hierarchies and cannot
escape the config root. For example, `WithLogDir("logs/archive")` resolves to
`<config root>/logs/archive`, with the same ancestor/final-directory ownership
rules as every other managed hierarchy.

The runtime account owns newly created user-scope directories. Existing
ancestors—including shared directories such as `~/.config`—are never modified.
User-scope directory modes should default to a private owner-writable policy
such as `0700`; the exact mode remains a design decision.

`EnsureResult` should expose the resolved locations and changes:

```go
type EnsureResult struct {
    // Existing service/account fields.
    Directories      AppDirectories
    DirectoryChanges []DirectoryChange
}
```

The final shape may instead expose only requested paths, so an absent directory
cannot be mistaken for a provisioned one.

## Platform mapping

The resolver owns these conventions; backends must not duplicate them.

| Scope | Linux runtime | Linux config | Linux state | Linux logs |
|---|---|---|---|---|
| System | `/run/<name>` | `/etc/<name>` | `/var/lib/<name>` | `/var/log/<name>` |
| User | `<config root>/run` | `<config root>` | `<config root>/lib` | `<config root>/log` |

For macOS, system paths are expected to use the appropriate canonical
`/private/var/run` (rather than the `/var` symlink),
`/Library/Application Support`, and `/Library/Logs` conventions. User paths
must be based on the resolved account home directory rather than the invoking
root user's home directory. Exact macOS paths are a design decision to confirm
against the service scope and launchd domain before implementation.

Windows is out of scope until `compage-go/service` gains a Windows backend.

## Ensure behaviour

For a system service with a managed account, `Ensure` performs this order:

1. Validate the directory declarations and resolve all platform paths.
2. Reconcile the account and obtain its final UID/GID.
3. Create each requested directory hierarchy without following symlinks.
4. Apply ownership and mode to newly created ancestor components and to the
   final declared component; validate, but never alter, existing ancestors.
5. Create and reconcile selected managed log files without following symlinks.
6. Verify each resulting directory and log file has the expected type,
   ownership, mode, and runtime-account access where required.
7. Reconcile the service definition and return both service and directory
   changes.

The default modes should be conservative and category-specific. The starting
system-scope policy is runtime/state/logs `0770` and config `0750`, with files
inside those locations receiving their own application-defined modes. Runtime,
state, and log directories are always group-writable because the runtime
account must create sockets, state, and log files there. A configuration
directory remains read/traverse-only for the runtime group by default; only
`WithConfigDirWrite` changes it to `0770`. A
configuration file can be root-owned and group-readable by the runtime account,
or owned by the runtime account when the application needs to save it at
runtime. The consuming application must choose this policy explicitly.

On a partial failure, return completed account/directory changes in the result,
matching the current account reconciliation model. Do not claim transactionality
that the filesystem cannot provide.

## Removal behaviour

`Manager.Uninstall`:

1. Stops and removes the service definition.
2. Removes the managed runtime directory, including stale sockets and runtime
   files.
3. Preserves config, state, and logs.

Purging config/state/logs must be a distinct, explicit API rather than an
implicit side effect of `Uninstall`. Candidate shape:

```go
type PurgeOptions struct {
    Config bool
    State  bool
    Logs   bool
}

func (m *Manager) Purge(ctx context.Context, options PurgeOptions) error
```

Before deleting, `Purge` must resolve exact paths, reject symlinks, and refuse
unexpected paths. It must never delete the account, executable, or a broad
parent directory such as `/etc`, `/var/lib`, `/var/log`, or a user home.

## Implementation approach

Implement this as one complete service-package revision, rather than exposing a
partial directory API. The implementation includes the resolver, declarations,
validation, reconciliation, result reporting, runtime cleanup, purge support,
and backend integration together.

1. Define and test the read-only cross-platform directory resolver.
2. Define directory declaration options and validation in `service`.
3. Implement secure directory reconciliation and reporting.
4. Implement runtime-directory cleanup on uninstall.
5. Implement explicit, safe purge support and its tests.
6. Update systemd and launchd backends to use the resolved locations where
   native manager support is appropriate, without relying solely on native
   directives.
7. Exercise the full lifecycle manually on Linux, then macOS.

## Test approach

Follow the existing `service` package test practices:

- All filesystem and command execution uses injected seams. Unit tests never
  invoke a real service manager or mutate production paths.
- Directory base locations are injectable and resolve to temporary test paths;
  tests never create `/etc`, `/run`, `/var/lib`, `/var/log`, or user-home
  locations.
- Filesystem behaviour, ownership metadata, symlink conditions, and command
  results are represented by fakes. Tests assert the exact reconciliation and
  cleanup calls.
- Platform-specific mapping tests use injected platform/user information rather
  than the host running the test.
- A separate manually run acceptance check may exercise Linux and macOS service
  managers after the mocked test suite passes.

## Required tests

- Every directory option rejects traversal, absolute paths, empty components,
  and invalid names while accepting clean relative nesting.
- System and user scopes resolve correct locations on supported platforms.
- User scope defaults config to `~/.<resolved-service-name>` and resolves
  runtime/state/logs below it; any non-config directory declaration enables
  that default config root.
- Account creation occurs before directory ownership is set.
- Ensure creates missing nested components with the requested ownership/mode,
  never modifies an existing ancestor, and always reconciles the final managed
  directory.
- Existing directories that do not grant required runtime access fail with an
  actionable error rather than being modified.
- Symlinks at any declared path component are rejected.
- System `WithConfigDir` creates root:runtime-group `0750` directories, while
  `WithConfigDirWrite` creates root:runtime-group `0770` directories.
- Logging without `WithLogDir` resolves a named directory from the final
  service name.
- Default stdout/stderr log options create `out.log`/`err.log` inside that
  directory; custom filenames cannot escape it.
- Managed log files are reconciled for ownership/mode and reject symlinks.
- A failed directory operation reports completed work and leaves no unsafe
  definition state.
- Uninstall removes runtime only and preserves config/state/logs.
- Purge removes only explicitly selected, declared directories and rejects
  unexpected or symlinked targets.
- An unprivileged program can read and write its declared locations after a
  privileged `Ensure` completes.

## Decisions still needed

- Exact user-scope directory conventions on Linux and macOS.
- Whether configuration files are root-owned/group-readable or
  runtime-account-owned.
- Default directory modes and whether applications should be able to request a
  dedicated access group.
- The final neutral package and types for the read-only directory resolver.
- Whether purge is a general manager API or is left to application-specific
  lifecycle commands after Compage exposes verified managed paths.
