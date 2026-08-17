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
    service.WithRunAsUser("example"),
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

`New` validates the desired specification and resolves the backend. Every
operation accepts a `context.Context` so command execution can be cancelled or
timed out.

The package name is `service`; its package documentation must state that it
manages OS services, rather than application-layer services.

## Specification model

The internal immutable specification contains:

- service name and description;
- binary path and ordered arguments;
- environment variables;
- scope: `User` or `System`;
- optional runtime account for a system service;
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

## Reconciliation: `Ensure`

`Ensure` makes the installed definition match the complete desired
specification. It does not silently restart a running service.

1. Validate and normalize the specification.
2. Resolve the installation path and launchd/systemd domain for the selected
   platform and scope.
3. For a system service with a runtime account, use `account.Lookup` to verify
   that the account exists.
4. Render a deterministic desired unit or plist.
5. Compare the managed definition with the desired definition.
6. If it is absent or different, atomically replace it with safe permissions.
7. Reload systemd or update launchd registration as required.
8. Return what changed.

```go
type EnsureResult struct {
    Installed       bool
    Changed         bool
    Reasons         []ChangeReason
    RestartRequired bool
}
```

Examples of `ChangeReason` are `Missing`, `BinaryChanged`, `ArgumentsChanged`,
`EnvironmentChanged`, `LogsChanged`, `AccountChanged`, `RevisionChanged`, and
`DefinitionDrift`.

Compare canonical desired content or a digest of a canonical desired
specification, not just the application revision and executable path. This
ensures argument, environment, user, log, and renderer changes also converge.

After `Ensure` reports a changed definition, callers choose whether to restart
an active service. Add an explicit combined operation such as
`EnsureAndStart(ctx, RestartIfChanged)` if applications need this common,
disruptive workflow.

## Operations and status

`Start` enables and starts a systemd unit; it bootstraps and kickstarts a
launchd job. It is idempotent.

`Stop` stops and disables/unloads the job. It is idempotent when the service is
already absent or stopped.

`Uninstall` stops the service, removes only the definition owned by this
manager, and reloads/unloads its manager. It must not remove application data,
logs, directories, or user accounts.

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
- system service installation requires appropriate privileges;
- an omitted runtime user uses systemd's default root identity; an explicit
  runtime user renders `User=` and must exist;
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
- target: the current user's `gui/<uid>` launchd domain;
- commands use `launchctl bootstrap`, `kickstart`, `bootout`, and `print`;
- LaunchAgents require a logged-in GUI user and are not the correct mechanism
  for headless boot-time work;
- the initial package scope is the current user's agent. Managing another
  user's GUI domain needs separate session-aware design.

### macOS: LaunchDaemon

- definition: `/Library/LaunchDaemons/<label>.plist`;
- target: launchd's `system` domain;
- system installation requires appropriate privileges;
- an omitted runtime user runs as root; an explicit runtime user renders
  `UserName` and must exist;
- launchd requires suitable ownership and permissions on daemon plists; write
  and validate these deliberately.

## Accounts

An explicit runtime account is verified through `account.Lookup` before writing
a system-service definition. If it does not exist, return a typed
`AccountNotFound` error.

Account creation is a separate, explicit policy feature because Linux
distributions and macOS differ in account tooling and because a safe account
definition requires decisions about UID, group, home directory, shell, and
login permissions. It must never be an unannounced side effect of `Ensure`.

## Logging, directories, and hardening

Keep stdout and stderr separate in the API, with an option to set both to the
same path. The package creates no log rotation policy: applications, journald,
or a platform logging facility own rotation and retention.

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
- Validate absolute binary and log paths, environment variable names, and all
  service identifiers.
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

## Error handling

`service` uses the repository's `errx` package for semantic classification.
Every exported domain error implements `errx.Classified` and wraps its original
filesystem, account-lookup, OS, or command cause where one exists. Callers use
`errors.As` for service-specific context and `errx.IsKind` for broad handling;
the package must not discard command output or causes in formatted strings.

Use these classifications consistently: invalid manager options and invalid
service specifications are `errx.Validation`; missing runtime accounts and
absent managed definitions are `errx.NotFound` when surfaced as errors;
system-scope privilege preflight failures are `errx.Forbidden`; unavailable
systemd/launchd managers and unsupported platforms are `errx.Unavailable`;
and unexpected command, rendering, or filesystem failures are contextual,
unclassified errors unless the operation can establish a more precise kind.
Do not classify lower-level `host` lookup errors inside `host`; `service` may
wrap and classify them only when its operation establishes their meaning.

## Package structure

```text
compage-go/service/
  doc.go             public package contract
  service.go         Manager, options, spec, result and status types
  errors.go          typed errors
  ensure.go          common validation and reconciliation
  command.go         injectable command runner
  filesystem.go      injectable filesystem and atomic writer
  systemd.go          Linux rendering and lifecycle backend
  launchd.go          macOS rendering and lifecycle backend
  *_test.go           renderer, lifecycle and reconciliation tests
```

Use internal interfaces for command execution and filesystem access. Resolve OS
through `host.Platform()`, current user details through `host.User()`, and the
system-scope privilege preflight through `host.IsRoot()`. Resolve arbitrary
account information through the `account` package. Production implementations
use the standard library; tests use fakes and temporary directories. Keep
exported seams small and only expose them when applications genuinely need
custom integration.

## Delivery sequence

1. Depend on platform, current-user, and privilege facts from `host`, plus
   arbitrary account lookup from `account`.
2. Define the public specification, typed errors, status/result contracts, and
   package documentation.
3. Implement shared validation, deterministic canonicalisation, change
   detection, atomic definition writing, and fakeable command/filesystem seams.
4. Implement and test systemd system units.
5. Implement and test systemd user units, including clear diagnostics for a
   missing user manager/session.
6. Implement and test LaunchAgents and LaunchDaemons with correct domains,
   paths, user handling, ownership, and lifecycle commands.
7. Add renderer/lifecycle unit tests and platform-gated integration tests.
   Validate generated units with `systemd-analyze verify` where available and
   generated plists with `plutil -lint` on macOS.
8. Migrate Persephone only after the generic package is stable; application
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
