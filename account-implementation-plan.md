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

type Kind uint8

const (
    System Kind = iota
    Regular
)

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
    Kind       Kind
    UID        *int
    Group      string
    GID        *int
    Groups     []string
    Home       string
    HomePolicy HomePolicy
    HomeMode   fs.FileMode
    Shell      string
    Existing   ExistingPolicy
}

type EnsureResult struct {
    Created bool
    Changed []Change
    Account Record
}

func Ensure(context.Context, Spec) (EnsureResult, error)
```

`Lookup` is read-only. `Ensure` creates a missing account or evaluates an
existing one against its explicit specification. Applying an unchanged
specification again performs no mutation.

## Account policy

Account creation cannot safely have one implicit default. The specification
requires callers to choose the parts that change host policy:

- system/service account or regular login account;
- primary group and optional supplementary groups;
- explicit or automatically allocated UID/GID;
- home path and whether it must exist;
- home mode;
- shell, including an explicit no-login value;
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

Provide typed errors that retain their underlying OS/command cause:

```go
type NotFoundError struct { /* account name or ID */ }
type PrivilegeError struct { /* required capability */ }
type DriftError struct { /* expected and observed fields */ }
type UnsupportedError struct { /* platform or account capability */ }
```

All errors wrap their underlying cause so callers can use `errors.Is` and
`errors.As`. Command output belongs in contextual errors for operability.

## Test strategy

- Unit-test specification validation, policy comparison, path validation,
  privilege preflight, and result generation without touching host accounts.
- Use injected account-store, command-runner, filesystem, and host-query seams
  for backend mutation tests.
- Run real integration tests only in disposable Linux/macOS environments with
  dedicated temporary accounts and directories.
- Never run account-mutating tests against a developer workstation by default.
- Cover idempotency, missing accounts, drift, no-home accounts, nonstandard
  homes, group changes, insufficient privilege, and partial command failures.

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
