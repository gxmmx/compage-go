# Account package implementation plan

## Purpose

Add an `account` package to `compage-go` for looking up, creating, and
idempotently reconciling arbitrary local operating-system accounts, their
groups, and their home directories.

It is a system-management package. Its primary responsibility is not the
identity of the process invoking it; that belongs to `host`. Before every
mutation, `account` checks `host.IsRoot()` and returns a typed privilege error
when the effective process identity lacks the necessary authority.

The package never elevates privileges, invokes `sudo`, or silently changes an
existing account beyond the explicit policy supplied by the caller.

## Implementation status

The package has its public API, `Check`/`Ensure` planning flow, typed `errx`
errors, cancellation handling, and a fakeable architecture in place. Account
store, command-runner, and filesystem adapters are injected, so unit tests do
not create, modify, or delete real accounts, groups, home directories, or other
account-owned filesystem objects. Current tests cover core planning, drift,
privilege, cancellation, command rendering, command failures, and primary-group
GID drift. Repository verification passes through `task check`.

This is not yet a declaration that the package is safe for real account
mutation. The following work remains before that claim is justified:

- split the current conditional implementation into fully specified Linux and
  macOS backends;
- add fake-backed tests for lookup parsing, group creation/reconciliation, home
  filesystem failures, partial mutations, and exact supplementary-group
  reconciliation;
- validate Linux tool capabilities rather than assuming `useradd`/`usermod`
  flags are portable;
- implement safe macOS UID allocation and complete directory-service semantics;
- verify every postcondition, including home ownership/mode and group membership.

Until these items are complete, do not use `Ensure` against a real host.

## Public API

```go
existing, err := account.Lookup(ctx, "example")
if err != nil {
    // a typed NotFoundError means no local account exists
}

result, err := account.Ensure(ctx, account.Spec{
    Name:       "example",
    Kind:       account.System,
    Group:      "example",
    Home:       "/var/lib/example",
    HomePolicy: account.EnsureHome,
    Shell:      account.NoLoginShell,
    Existing:   account.Verify,
})
```

```go
type Record struct {
    Name    string
    UID     string
    GID     string
    Group   string
    Home    string
    Shell   string
    Groups  []string
}

func Lookup(context.Context, name string) (Record, error)
func LookupID(context.Context, uid string) (Record, error)
func Check(context.Context, Spec) (EnsureResult, error)

type HomePolicy uint8

const (
    LeaveHomeUnchanged HomePolicy = iota
    RequireHome
    EnsureHome
)

type ExistingPolicy uint8

const (
    Verify ExistingPolicy = iota
    Reconcile
)

type Spec struct {
    Name       string
    UID        *int
    Group      string
    GID        *int
    Groups     []string
    Home       string
    HomePolicy HomePolicy
    HomeMode   fs.FileMode
    Shell      string
    Hidden     *bool
    Existing   ExistingPolicy
}

type EnsureResult struct {
    Created bool
    Changed []Change
    Account Record
}

func Ensure(context.Context, Spec) (EnsureResult, error)
```

`Lookup` and `Check` are read-only. `Check` evaluates precisely what `Ensure`
would do, including specification validation, account/group/home inspection,
policy comparison, and capability/privilege preflight, but never invokes a
mutation backend. Its result is a plan: `Created` means the account would be
created, `Changed` contains the exact changes that would be applied, and
`Account` is the observed account when present (or its zero value when absent).
No `DryRun` option or result field is needed because the method itself makes
the no-mutation guarantee explicit.

`Ensure` creates a missing account or evaluates an existing one against its
explicit specification. Applying an unchanged specification again performs no
mutation. With `Existing: Verify`, both `Check` and `Ensure` return the same
typed `DriftError` when a present account does not meet the specification.

## Context, cancellation, and partial application

Every operation accepts a caller-owned context. `Check` and `Ensure` must
return promptly when `ctx` is already cancelled and must check `ctx.Err()`
before each backend operation. Command-backed backends use the same context for
their child process so a parent cancellation or deadline can stop it where the
operating system permits. The package never creates, replaces, or cancels a
caller context; applications own deadline policy through a derived context.

Account reconciliation is not transactional. Cancellation or failure can occur
after an account, group, supplementary-group membership, or home directory has
already changed. `Ensure` must not begin a later mutation after cancellation,
must preserve `context.Canceled` or `context.DeadlineExceeded` in its returned
error, and returns an `EnsureResult` containing changes completed before the
failure. It must not attempt automatic rollback: rollback may be destructive or
incorrect under concurrent host policy. A later `Ensure` is the explicit,
idempotent recovery mechanism.

## Account policy

Account creation cannot safely have one implicit default. The specification
requires callers to choose the parts that change host policy:

- primary group and optional supplementary groups;
- explicit or automatically allocated UID/GID;
- home path and whether it must exist;
- home mode;
- shell, including an explicit no-login value;
- macOS user-picker visibility when explicitly managed;
- whether an existing account is only verified or may be reconciled.

Examples supported by this model include:

| Need | Account specification |
| --- | --- |
| Non-login daemon without a home | system account, `LeaveHomeUnchanged`, no-login shell |
| Daemon state in `/var/lib/example` | system account, explicit home, `EnsureHome` |
| Application-owned nonstandard home | explicit home such as `/etc/example`, `EnsureHome` |
| Existing production account | expected fields, `Verify` |
| Installer-owned account | desired fields, `Reconcile` |

`Verify` is the safe default. It returns a typed drift error when a present
account does not meet declared requirements; it does not change the account.
`Reconcile` changes only fields represented in the explicit specification and
reports every change.

Never infer an account's desired home, shell, UID, groups, or login access from
its name.

`account` does not model a portable system-versus-regular account kind. Those
semantics are operating-system-specific and are expressed explicitly through
the caller's UID, GID, shell, home, and group choices. `Hidden` is an optional
macOS-specific policy: `nil` leaves visibility unmanaged, `true` hides the
local account from the user picker, and `false` makes it visible. Linux treats
`Hidden` as a no-op and never reports drift for it.

## Home directories

Home-directory handling is independent of account creation because an existing
account may have a missing home, intentionally have no home, or have a home
managed by another deployment mechanism.

For `EnsureHome`, the package must:

1. validate an absolute, clean target path;
2. create missing parents only when permitted by policy;
3. create the target with the declared mode;
4. verify or set ownership to the target account and primary group;
5. preserve contents and avoid recursive ownership changes unless a future,
   explicit policy authorizes it.

`Ensure` never removes a home directory or recursively mutates its contents.
Deletion, retention, and migration require a separate, explicitly destructive
lifecycle API.

## Privilege and platform model

`account` checks `host.IsRoot()` before account, group, ownership, or
system-owned directory mutations. The effective identity is a preflight check;
the OS remains authoritative and all native command/system errors are retained.

Read-only account lookups should work without root. Account creation,
modification, supplementary-group changes, ownership changes, and system-owned
directory creation generally require root or equivalent OS authority.

Use `host.Platform()` to select a separate internal backend:

### Linux

Linux distributions vary in account tooling and defaults. Do not assume that a
particular distribution's `useradd` flags are portable. Choose and document a
supported strategy, validate postconditions after every mutation, and return a
capability error if the policy cannot be implemented safely on the host.

System-account semantics, no-login shell locations, UID ranges, and home
creation defaults vary by distribution. The package requires explicit fields
rather than relying on distribution defaults.

### macOS

macOS local-directory-service operations have different account/group semantics
from Linux. Implement this backend separately and validate state through the
native directory service. Do not translate Linux command flags or assume UID
allocation works the same way.

### Unsupported platforms

Return a typed unsupported-platform error. Do not approximate account management
through environment variables or filesystem-only changes.

## Error model

`account` uses the repository's `errx` package for semantic classification.
Every exported domain error implements `errx.Classified`, returns the stated
`errx.Kind`, and retains its underlying OS, filesystem, directory-service, or
command cause through `Unwrap`. Callers can use `errors.As` for account-specific
context and `errx.IsKind` for broad, transport-neutral handling. Do not replace
these errors with unclassified formatted strings, and do not classify errors in
the `host` package on its behalf.

Provide typed errors that retain their underlying OS/command cause:

```go
type NotFoundError struct { /* account name or ID */ }
type PrivilegeError struct { /* required capability */ }
type DriftError struct { /* expected and observed fields */ }
type UnsupportedError struct { /* platform or account capability */ }
```

Their classifications are: `NotFoundError` → `errx.NotFound`,
`PrivilegeError` → `errx.Forbidden`, `DriftError` → `errx.Conflict`, and
`UnsupportedError` → `errx.Unavailable`. Invalid account specifications,
paths, and policy combinations use a typed validation error classified as
`errx.Validation`. Command output belongs in contextual errors for operability,
but sensitive values must not be included.

All errors wrap their underlying cause so callers can use `errors.Is` and
`errors.As`.

## Test strategy

- Unit-test specification validation, policy comparison, path validation,
  privilege preflight, typed-error classification, `Check` planning, and result
  generation without touching host accounts.
- Use injected account-store, command-runner, filesystem, and host-query seams
  for every lookup and backend-mutation test.
- Tests must never instantiate production dependencies (`productionDeps`,
  `execRunner`, `osStore`, or `osFS`) and must never call a real account,
  directory-service, group-management, ownership, or filesystem mutation API.
  In particular, tests must not execute `useradd`, `usermod`, `groupadd`,
  `groupmod`, `getent`, `dscl`, `dseditgroup`, `stat`, `chown`, or `chmod`.
  Command tests use a fake runner that records command arguments and returns
  caller-supplied output or failure. Filesystem tests use a fake filesystem.
- The Linux and macOS backends are tested through their fake seams on any host.
  Cross-compilation is not a behavioral test and is not part of this package's
  verification strategy. Native live-mutation tests are prohibited.
- The repository test suite must never create, modify, or delete a real account,
  group, home directory, or account-owned filesystem object. Do not add live
  integration tests, even behind build tags, unless this policy is explicitly
  changed in a future plan revision.
- Cover idempotency, missing accounts, drift, no-home accounts, nonstandard
  homes, group changes, insufficient privilege, and partial command failures.

### Backend acceptance matrix

Before declaring either mutation backend production-ready, add fake-backed
tests for every row below. A passing command exit status is insufficient: tests
must also model and assert the required postcondition lookup or filesystem
inspection.

| Area | Linux backend | macOS backend |
| --- | --- | --- |
| Lookup | Parse a valid `getent passwd` record; reject malformed records; resolve primary and supplementary groups through the store. | Parse `dscl -read` attributes; reject missing or malformed attributes; resolve primary and supplementary groups through the store. |
| Capability preflight | Record and validate required `useradd`, `usermod`, `groupadd`, and `groupmod` capabilities without mutation. | Record `dscl` and `dseditgroup` preflight calls without mutation. |
| Account creation | Render every declared field, disable implicit home creation, and verify the created record. | Render local Directory Service attributes, allocate requested/automatic IDs safely, and verify the created record. |
| Primary group | Create a missing group, reconcile an explicit GID, preserve completed changes on a later failure, and verify the group. | Create/reconcile a local group and GID through Directory Service, preserve completed changes on a later failure, and verify the group. |
| Supplementary groups | Reconcile the exact declared set and verify membership after mutation. | Add missing and remove obsolete memberships in deterministic order, then verify membership. |
| Home directory | Require an existing parent; reject a non-directory target before ownership/mode mutation; create only the target; verify mode and ownership. | Apply the identical filesystem safety policy and verify mode and ownership. |
| Failure and cancellation | At every command/filesystem boundary, return the cause, preserve cancellation, stop later mutations, and report only completed changes. | The same guarantees for every Directory Service and filesystem boundary. |

## Delivery sequence

1. Add read-only arbitrary-account lookup, typed errors, and tests.
2. Define account, group, and home reconciliation policies with exhaustive unit
   tests before adding mutations.
3. Implement Linux account reconciliation behind fakeable seams and verify
   postconditions after each mutation.
4. Add macOS reconciliation as a separate backend with platform-gated
   integration tests.
5. Add any account-removal API only as a separately designed, explicit,
   destructive operation.
