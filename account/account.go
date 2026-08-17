package account

import (
	"context"
	"io/fs"
)

// Record is a local account and its observed group membership.
type Record struct {
	Name, UID, GID, Group, Home, Shell string
	Groups                             []string
}

type Kind uint8

const (
	System Kind = iota
	Regular
)

// NoLoginShell is the Linux no-login shell. Callers targeting macOS should
// provide its platform-appropriate no-login shell explicitly.
const NoLoginShell = "/usr/sbin/nologin"

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

// Spec declares the complete account policy managed by Ensure.
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

// Change describes one account property that was or would be changed.
type Change struct{ Field, Before, After string }

// EnsureResult reports completed changes from Ensure, or planned changes from Check.
type EnsureResult struct {
	Created bool
	Changed []Change
	Account Record
}

// Lookup finds a local account by name.
func Lookup(ctx context.Context, name string) (Record, error) {
	return operation{deps: productionDeps()}.lookup(ctx, name, false)
}

// LookupID finds a local account by UID.
func LookupID(ctx context.Context, uid string) (Record, error) {
	return operation{deps: productionDeps()}.lookup(ctx, uid, true)
}

// Check reports what Ensure would do without changing the host.
func Check(ctx context.Context, spec Spec) (EnsureResult, error) {
	return operation{deps: productionDeps()}.run(ctx, spec, false)
}

// Ensure reconciles an account to spec. It may return completed changes with an error.
func Ensure(ctx context.Context, spec Spec) (EnsureResult, error) {
	return operation{deps: productionDeps()}.run(ctx, spec, true)
}
