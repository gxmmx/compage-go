# OS service manager implementation plan

## Purpose

Add a `service` package to `compage-go` for installing, reconciling, and
controlling a program's own operating-system service.

The package supports these targets:

| Scope | Linux | macOS |
| --- | --- | --- |
| User | systemd user unit | launchd LaunchAgent |
| System | systemd system unit | launchd LaunchDaemon |

The package owns service-definition generation and reconciliation. Applications
own their command-line arguments, account policy, data directories, logging
policy, and health checks.

## Public API

```go
mgr, err := service.New(
    service.WithName("com.example.worker"),
    service.WithDescription("Example worker"),
    service.WithBinary("/opt/example/worker"),
    service.WithArgs("serve", "--port", "9000"),
    service.WithEnvironment(map[string]string{
        "EXAMPLE_CONFIG": "/etc/example/config.toml",
    }),
    service.WithScope(service.System),
    service.WithAccount(account.Spec{
        Name:     "example",
        Group:    "example",
        Home:     "/var/lib/example",
        HomePolicy: account.EnsureHome,
        Shell:    account.NoLoginShell,
        Existing: account.Reconcile,
    }),
    service.WithStdoutLog("/var/log/example/worker.log"),
    service.WithStderrLog("/var/log/example/worker.error.log"),
    service.WithRevision(build.Version),
)
if err != nil {
    // invalid specification, unsupported platform, or unavailable manager
}

result, err := mgr.Ensure(ctx)
err = mgr.Start(ctx)
status, err := mgr.Status(ctx)
err = mgr.Stop(ctx)
err = mgr.Uninstall(ctx)
```

`New` validates the desired specification and resolves the backend using
`host.Platform()`. For `System` scope it must call `host.IsRoot()` and return a
typed privilege error before constructing a manager when the effective UID is
not root. This is an intentionally early failure: a non-root process cannot
install, control, or remove a system service. Every operation accepts a
`context.Context` so command execution can be cancelled or timed out.

The package name is `service`; its package documentation must state that it
manages OS services, rather than application-layer services.

## Specification model

The internal immutable specification contains:

- service name and description;
- binary path and ordered arguments;
- environment variables;
- scope: `User` or `System`;
- optional account specification for a system service;
- stdout and stderr destinations;
- optional application revision;
- restart policy and platform-neutral lifecycle settings supported by all
  initial backends.

`WithBinary` requires an absolute executable path. A later explicit
`WithCurrentExecutable` option may resolve `os.Executable`, canonicalising
symlinks, but a service must never implicitly infer a binary from the process
creating the manager.

When `WithName` is omitted, derive the name from the binary basename after
validating it. Explicit names are recommended, particularly on macOS where a
reverse-DNS launchd label such as `com.example.worker` avoids collisions.

Validate names per backend before any files are changed. A name represents the
systemd unit stem and launchd label; it must not be treated as a path.

`WithAccount(account.Spec)` is permitted only for `System` scope. `Ensure`
must invoke `account.Ensure(ctx, spec)` before writing the service definition;
the account package remains the sole owner of account validation, lookup,
creation, reconciliation, group membership, home-directory ownership, and
platform commands. The service runtime account is `account.Spec.Name`.
`WithRunAsUser(string)` should not be provided as a second, lookup-only account
path: it cannot express the account policy required to safely ensure an absent
account. Callers that require a pre-existing account use an `account.Spec` with
`Existing: account.Verify`. User-scoped services always run as the current
user, and reject `WithAccount`.

## Reconciliation: `Ensure`

`Ensure` makes the installed definition match the complete desired
specification. It does not silently restart a running service.

1. Validate and normalize the specification.
2. Resolve the installation path and launchd/systemd domain for the selected
   platform and scope.
3. For a system service with a runtime account, call `account.Ensure` with the
   supplied `account.Spec`; retain its result in `EnsureResult`. Account
   reconciliation is deliberately not rolled back if a later service-manager
   step fails, matching the account package's non-transactional contract.
4. Render a deterministic desired unit or plist.
5. Compare the managed definition with the desired definition.
6. If it is absent or different, atomically replace it with safe permissions.
7. Reload systemd. For launchd, do not boot out and bootstrap an already-loaded
   job during `Ensure`, because that is a restart. Write the plist and report
   `RestartRequired` when the installed definition changed and the job is
   loaded; `Start` performs the explicit reload/start sequence.
8. Return what changed.

```go
type EnsureResult struct {
    Installed       bool
    Changed         bool
    Reasons         []ChangeReason
    RestartRequired bool
    Account         *account.EnsureResult
}
```

Examples of `ChangeReason` are `Missing`, `BinaryChanged`, `ArgumentsChanged`,
`EnvironmentChanged`, `LogsChanged`, `AccountChanged`, `RevisionChanged`, and
`DefinitionDrift`.

Set `AccountChanged` whenever the embedded `Account` result reports creation
or completed changes, and set `EnsureResult.Changed` in that case even if the
rendered service definition is unchanged.
The manager's authority is the single definition path derived from its backend,
scope, and validated name. A caller selecting that name authorizes management
of that path; do not claim that an easily forged comment marker proves external
ownership. `Uninstall` must never follow a symlink at that path or act on any
other definition.

Compare canonical desired content or a digest of a canonical desired
specification, not just the application revision and executable path. This
ensures argument, environment, user, log, and renderer changes also converge.

After `Ensure` reports a changed definition, callers choose whether to restart
an active service. Add an explicit combined operation such as
`EnsureAndStart(ctx, RestartIfChanged)` if applications need this common,
disruptive workflow.

## Operations and status

`Start` enables and starts a systemd unit. For launchd it uses
`launchctl enable <domain>/<label>`, first makes the current plist registration effective
(booting out a loaded job when necessary), then bootstraps and kickstarts it.
It is idempotent, but its launchd form may restart an already-running job;
document that explicitly.

`Stop` stops and disables a systemd unit. For launchd it uses
`launchctl disable <domain>/<label>` and boots the job out of its domain. It is idempotent
when the service is already absent or stopped. For launchd, `Enabled` means the
job is not disabled in launchd's override state; the on-disk plist provides
persistence, while `Loaded` says whether it is registered in the current
domain.

`Uninstall` stops the service, removes only the exact definition path this
manager is authorized to manage, and reloads/unloads its manager. It must not
remove application data, logs, directories, or user accounts.

`Status` must distinguish a definition being present from a job being enabled,
loaded, running, or failed:

```go
type Status struct {
    Installed bool
    Enabled   bool
    Loaded    bool
    Running   bool
    PID       int
    ExitCode  *int
    Detail    string
}
```

Some fields are unavailable on one platform; document their zero-value meaning
and return the best platform-native detail available. A running job is not a
health check; application health remains application-specific.

## Backend mapping

### Linux: systemd system unit

- definition: `/etc/systemd/system/<name>.service`;
- commands: `systemctl daemon-reload`, then `systemctl enable --now`, stop via
  `systemctl disable --now`;
- system service construction requires `host.IsRoot()`; do not defer this
  check to an eventual filesystem or `systemctl` failure;
- an omitted runtime user uses systemd's default root identity; an explicit
  account renders `User=` from `account.Spec.Name` after `account.Ensure` has
  completed;
- default logging should be journald. Direct file logging is opt-in and its
  directory must be writable by the runtime account.

### Linux: systemd user unit

- definition: `~/.config/systemd/user/<name>.service`;
- commands: `systemctl --user daemon-reload` and `systemctl --user ...`;
- the unit always runs as the owning user; it does not use `User=`;
- the user systemd manager and D-Bus session may be unavailable under cron,
  non-interactive SSH, or CI;
- jobs usually stop at logout unless the host enables user lingering. Detect and
  explain this condition; changing lingering is host policy and not an implicit
  package action.

### macOS: LaunchAgent

- definition: `~/Library/LaunchAgents/<label>.plist`;
- target: the current user's `gui/<uid>` launchd domain, derived from
  `host.User()` (including its home directory), not `os.UserHomeDir()`;
- commands use `launchctl bootstrap`, `kickstart`, `bootout`, and `print`;
- LaunchAgents require a logged-in GUI user and are not the correct mechanism
  for headless boot-time work;
- the initial package scope is the current user's agent. Managing another
  user's GUI domain needs separate session-aware design.

### macOS: LaunchDaemon

- definition: `/Library/LaunchDaemons/<label>.plist`;
- target: launchd's `system` domain;
- system service construction requires `host.IsRoot()`;
- an omitted runtime user runs as root; an explicit runtime user renders
  `UserName` from `account.Spec.Name` after `account.Ensure` has completed;
- launchd requires suitable ownership and permissions on daemon plists; write
  and validate these deliberately.

## Accounts

An explicit runtime account is represented by `account.Spec` and is reconciled
through `account.Ensure` before writing a system-service definition. Account
creation is therefore explicit in the caller's supplied specification, never
an unannounced side effect invented by `service`. The caller must choose the
primary group, home policy, shell, and existing-account policy; `service` must
not duplicate account validation or invoke account-management commands itself.
Return the underlying typed `account` errors with service-operation context so
both `errors.As` and `errx.IsKind` continue to work.

## Logging, directories, and hardening

Keep stdout and stderr separate in the API, with an option to set both to the
same path. The package creates no log rotation policy: applications, journald,
or a platform logging facility own rotation and retention.

Map explicit log paths to `StandardOutput=append:<path>` and
`StandardError=append:<path>` on supported systemd versions, and to
`StandardOutPath` and `StandardErrorPath` in launchd plists. The initial
support floor is the current stable systemd major release:

```go
const minimumSystemdVersion = 260
```

Before a Linux operation, the systemd backend runs `systemctl --version` via
its injected command runner, extracts the leading systemd major version, and
returns a typed unavailable error below this floor. Keep the constant private,
centralized, and covered by a version-parser unit test so a later compatibility
decision is a single deliberate change. This is preferable to rendering a
quietly degraded fallback for old managers: file logging must retain append
semantics. Check only the file's parent directory and declared runtime-account
access before installation; do not create application log directories as a side
effect.

Do not include systemd-only directory controls such as `RuntimeDirectory`,
`StateDirectory`, and `ConfigurationDirectory` in the initial portable API.
Applications create and own their data directories.

Do not impose aggressive systemd hardening defaults in v1. Settings such as
`ProtectSystem`, `ProtectHome`, and `PrivateTmp` can break valid generic
programs unless their required filesystem and network access is fully modeled.
Introduce audited, opt-in hardening profiles later.

## Security and correctness requirements

- Never call `sudo`, prompt for elevation, or create a privileged helper.
- Return typed privilege errors before mutating system-level paths where
  possible.
- Use atomic same-directory replacement to avoid partially written definitions.
- Create directories and files with restrictive, platform-appropriate modes;
  verify launchd ownership requirements for LaunchDaemons.
- Atomic replacement means: create a non-symlink temporary file in the target
  directory with restrictive mode, write and fsync it, set final ownership and
  mode, rename it over the validated non-symlink target, then fsync the parent
  directory where the platform supports it. Never recursively create a
  system-level definition directory.
- Validate absolute binary and log paths, environment variable names, and all
  service identifiers.
- Reject NUL-containing values everywhere, `=` in environment names, and
  environment names that are not portable POSIX identifiers. Preserve empty
  environment values. Check that the binary is a regular executable at
  `Ensure` time (not merely that its path is absolute), so a removed deployment
  artifact cannot be installed.
- Render all dynamic content safely: systemd command and environment escaping,
  and plist XML escaping.
- Treat a malformed existing definition as drift and replace only the managed
  definition after successful rendering of the desired one.
- Include command output in wrapped errors and preserve errors for
  `errors.Is`/`errors.As`.
- Warn or reject binaries in temporary or known build-output paths, since they
  are likely to disappear after installation.
- Support `linux` and `darwin` initially; return a typed unsupported-platform
  error elsewhere.
- Execute service-manager commands directly with an argument vector (never a
  shell), check `context` before each mutation, and distinguish expected
  inactive/not-loaded command outcomes from invocation failures. Build status
  from stable, machine-readable systemd properties where available rather than
  localized `systemctl status` text.

## Error handling

`service` uses the repository's `errx` package for semantic classification.
Every exported domain error implements `errx.Classified` and wraps its original
filesystem, account-lookup, OS, or command cause where one exists. Callers use
`errors.As` for service-specific context and `errx.IsKind` for broad handling;
the package must not discard command output or causes in formatted strings.

Use these classifications consistently: invalid manager options and invalid
service specifications are `errx.Validation`; absent managed definitions are
`errx.NotFound` when surfaced as errors; system-scope privilege preflight
failures are `errx.Forbidden`; unavailable systemd/launchd managers, missing
user systemd D-Bus managers, and unsupported platforms are `errx.Unavailable`;
and unexpected command, rendering, or filesystem failures are contextual,
unclassified errors unless the operation can establish a more precise kind.
Do not classify lower-level `host` lookup errors inside `host`; `service` may
wrap and classify them only when its operation establishes their meaning.

## Backend procedure and platform selection

Model the internal control flow on `account.operation`, rather than placing
platform conditionals throughout `Manager` methods. `Manager` is a thin public
wrapper over an unexported `operation` with a dependency set containing the
immutable specification, `host.PlatformInfo`, current-user and root facts,
the account ensure function, a command runner, filesystem operations, and one
selected `backend`.

```go
type backend interface {
    validate(specification) error
    ensure(context.Context, operation) (EnsureResult, error)
    start(context.Context, operation) error
    stop(context.Context, operation) error
    uninstall(context.Context, operation) error
    status(context.Context, operation) (Status, error)
}
```

`productionDependencies` obtains `host.Platform()` and selects the backend in
one isolated `platformBackend(host.OS)` switch: `host.Linux` returns the
systemd backend, `host.Darwin` returns the launchd backend, and every other OS
returns the typed unsupported-platform error. That selection is the only
OS-type dispatch in the package. Shared `operation` code owns context checks,
root preflight, account reconciliation, error wrapping, canonical comparison,
and atomic writes; it delegates platform paths, rendering, manager commands,
status parsing, and platform validation to the selected backend.

Implement the two backends in separate files. The Linux implementation owns
both systemd system-unit and user-unit scope details; the Darwin implementation
owns both LaunchDaemon and LaunchAgent scope details. Scope selection is local
to the relevant backend, never a cross-platform chain of `if runtime.GOOS`
branches. Tests construct `operation` with fake dependencies and either backend
directly, mirroring the account package's backend tests.

## Package structure

```text
compage-go/service/
  doc.go             public package contract
  service.go         Manager, options, spec, result and status types
  errors.go          typed errors
  operation.go       common procedure and reconciliation coordination
  backend.go          backend contract, dependencies, platformBackend selector
  backend_linux_impl.go  systemd backend and Linux-only rendering/lifecycle
  backend_darwin_impl.go launchd backend and Darwin-only rendering/lifecycle
  command.go         injectable command runner
  filesystem.go      injectable filesystem and atomic writer
  *_test.go           renderer, lifecycle and reconciliation tests
```

Use internal interfaces for command execution and filesystem access. Resolve OS
through `host.Platform()`, current user details through `host.User()`, and the
system-scope privilege preflight through `host.IsRoot()`. Reconcile the runtime
account through `account.Ensure`; do not call `os/user`, `useradd`, `dscl`, or
other account tooling from `service`. Production implementations use the
standard library; tests use fakes and temporary directories. The unexported
dependency set must include platform, current-user, root, account ensure,
command execution, and filesystem operations, so all branches are unit-testable
without the host service manager. Keep exported seams small and only expose
them when applications genuinely need custom integration.

## Delivery sequence

1. Depend on platform, current-user, and privilege facts from `host`, plus
   runtime-account reconciliation through `account.Ensure`.
2. Define the public specification, typed errors, status/result contracts, and
   package documentation.
3. Implement shared validation, deterministic canonicalisation, change
   detection, atomic definition writing, fakeable command/filesystem seams, and
   the isolated `platformBackend` selector.
4. Implement and test systemd system units.
5. Implement and test systemd user units, including clear diagnostics for a
   missing user manager/session.
6. Implement and test LaunchAgents and LaunchDaemons with correct domains,
   paths, user handling, ownership, and lifecycle commands.
7. Add renderer, lifecycle, and reconciliation tests that are completely
   isolated from the host service manager. Normal unit and CI tests must never
   create a real unit, plist, account, service-manager registration, or call a
   live `systemctl`, `launchctl`, `systemd-analyze`, or `plutil`. Cover root
   preflight before every system-scope mutation, account-enforcement ordering
   and error wrapping, user-scope account rejection, concurrent/atomic writes,
   unchanged ensures, drift, malformed existing files, cancellation, expected
   inactive status, the no-silent-launchd-restart guarantee, backend selection
   for every OS, and the systemd-version floor/parser.
8. Use a recording fake command runner for all manager interactions. It must
   assert the executable and exact argument vector, expose canned stdout,
   stderr, and exit errors, and support ordered call assertions. Feed it
   representative `systemctl show` and `launchctl print` output to test status
   parsing and expected non-zero inactive/not-loaded outcomes. Use a fake
   account enforcer to assert that account reconciliation occurs before any
   definition write.
9. Test rendering with golden fixtures and parser-oriented edge cases (quotes,
   escapes, Unicode, XML-hostile values, sorted environments, and every scope).
   Exercise atomic writes against temporary directories with an injected
   filesystem fault seam; redirect every backend definition root to `t.TempDir`
   and never use `/etc`, `/Library`, or a real user service directory.
10. Native command validation is optional developer tooling only. If retained,
    run `systemd-analyze verify` or `plutil -lint` solely over temporary rendered
    fixtures behind an explicit opt-in build tag or environment variable; it is
    not part of the default test suite and must not register, start, stop, or
    alter a host service. A future end-to-end suite, if needed, runs only in a
    disposable VM/container image with its own init system, never on the test
    runner's host.
11. Migrate Persephone only after the generic package is stable; application
   data cleanup and app-specific status output remain in Persephone.

## Existing implementation to reuse as reference

`persephone/shared/installer` supplies useful reference implementations for:

- systemd per-argument command quoting;
- deterministic environment ordering;
- XML escaping of plist values;
- injected command runners and filesystem paths for unit tests;
- canonical executable resolution and transient-path detection.

The new package should reimplement these under its own generic contract rather
than importing Persephone or preserving its service names, labels, metadata,
or CLI-level install checks.
