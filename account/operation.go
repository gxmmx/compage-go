package account

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/host"
)

type backend interface {
	lookup(context.Context, string, bool) (Record, error)
	homeExists(context.Context, string) (bool, error)
	apply(context.Context, Spec, *Record) error
}
type dependencies struct {
	platform host.PlatformInfo
	root     bool
	backend  backend
}
type operation struct{ deps dependencies }

func (o operation) lookup(ctx context.Context, key string, byID bool) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, errx.New("account: lookup cancelled", errx.WithCause(err))
	}
	if key == "" {
		return Record{}, &ValidationError{Message: "lookup key is required"}
	}
	if !o.deps.platform.OS.IsUnix() {
		return Record{}, &UnsupportedError{Capability: string(o.deps.platform.OS)}
	}
	return o.deps.backend.lookup(ctx, key, byID)
}
func (o operation) run(ctx context.Context, spec Spec, apply bool) (EnsureResult, error) {
	if err := ctx.Err(); err != nil {
		return EnsureResult{}, errx.New("account: operation cancelled", errx.WithCause(err))
	}
	if err := validate(spec); err != nil {
		return EnsureResult{}, err
	}
	if !o.deps.platform.OS.IsUnix() {
		return EnsureResult{}, &UnsupportedError{Capability: string(o.deps.platform.OS)}
	}
	record, err := o.deps.backend.lookup(ctx, spec.Name, false)
	if err != nil {
		var missing *NotFoundError
		if !errors.As(err, &missing) {
			return EnsureResult{}, err
		}
		result := EnsureResult{Created: true, Changed: creationChanges(spec)}
		if !o.deps.root {
			return result, &PrivilegeError{Capability: "create local account"}
		}
		if !apply {
			return result, nil
		}
		if err := ctx.Err(); err != nil {
			return result, errx.New("account: create cancelled", errx.WithCause(err))
		}
		if err := o.deps.backend.apply(ctx, spec, nil); err != nil {
			return result, err
		}
		created, err := o.deps.backend.lookup(ctx, spec.Name, false)
		if err != nil {
			return result, err
		}
		result.Account = created
		if remaining := compare(spec, created); len(remaining) > 0 {
			return result, &DriftError{Changes: remaining}
		}
		return result, nil
	}
	changes := compare(spec, record)
	if spec.HomePolicy != LeaveHomeUnchanged {
		exists, homeErr := o.deps.backend.homeExists(ctx, spec.Home)
		if homeErr != nil {
			return EnsureResult{Changed: changes, Account: record}, homeErr
		}
		if !exists {
			changes = append(changes, Change{Field: "home directory", Before: "absent", After: "present"})
		}
	}
	result := EnsureResult{Changed: changes, Account: record}
	if len(changes) == 0 {
		return result, nil
	}
	if spec.Existing == Verify || spec.HomePolicy == RequireHome {
		return result, &DriftError{Changes: changes}
	}
	if !o.deps.root {
		return result, &PrivilegeError{Capability: "reconcile local account"}
	}
	if !apply {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, errx.New("account: reconcile cancelled", errx.WithCause(err))
	}
	if err := o.deps.backend.apply(ctx, spec, &record); err != nil {
		return result, err
	}
	updated, err := o.deps.backend.lookup(ctx, spec.Name, false)
	if err != nil {
		return result, err
	}
	result.Account = updated
	if remaining := compare(spec, updated); len(remaining) > 0 {
		return result, &DriftError{Changes: remaining}
	}
	return result, nil
}

func creationChanges(s Spec) []Change {
	changes := []Change{{Field: "account", Before: "absent", After: "present"}}
	if s.UID != nil {
		changes = append(changes, Change{Field: "uid", Before: "absent", After: strconv.Itoa(*s.UID)})
	}
	if s.Group != "" {
		changes = append(changes, Change{Field: "group", Before: "absent", After: s.Group})
	}
	if s.GID != nil {
		changes = append(changes, Change{Field: "gid", Before: "absent", After: strconv.Itoa(*s.GID)})
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		changes = append(changes, Change{Field: "home", Before: "absent", After: s.Home})
	}
	if s.Shell != "" {
		changes = append(changes, Change{Field: "shell", Before: "absent", After: s.Shell})
	}
	if s.Groups != nil {
		changes = append(changes, Change{Field: "groups", Before: "absent", After: strings.Join(s.Groups, ",")})
	}
	return changes
}
func validate(s Spec) error {
	if s.Name == "" || strings.ContainsAny(s.Name, ":/\\\x00 \t\n") {
		return &ValidationError{Message: "name must be a simple local account name"}
	}
	if s.Kind != System && s.Kind != Regular {
		return &ValidationError{Message: "unknown account kind"}
	}
	if s.Existing != Verify && s.Existing != Reconcile {
		return &ValidationError{Message: "unknown existing policy"}
	}
	if s.HomePolicy != LeaveHomeUnchanged && s.HomePolicy != RequireHome && s.HomePolicy != EnsureHome {
		return &ValidationError{Message: "unknown home policy"}
	}
	if s.UID != nil && *s.UID < 0 {
		return &ValidationError{Message: "UID must not be negative"}
	}
	if s.GID != nil && *s.GID < 0 {
		return &ValidationError{Message: "GID must not be negative"}
	}
	if s.GID != nil && s.Group == "" {
		return &ValidationError{Message: "GID requires a primary group"}
	}
	seenGroups := make(map[string]struct{}, len(s.Groups))
	for _, group := range s.Groups {
		if group == "" || strings.ContainsAny(group, ":/\\\x00 \t\n") {
			return &ValidationError{Message: "supplementary group must be a simple local group name"}
		}
		if _, duplicate := seenGroups[group]; duplicate {
			return &ValidationError{Message: "supplementary groups must be unique"}
		}
		seenGroups[group] = struct{}{}
	}
	if s.HomePolicy != LeaveHomeUnchanged && (s.Home == "" || !filepath.IsAbs(s.Home) || filepath.Clean(s.Home) != s.Home) {
		return &ValidationError{Message: "home must be a clean absolute path"}
	}
	if s.HomeMode != 0 && s.HomeMode.Perm() != s.HomeMode {
		return &ValidationError{Message: "home mode may contain permissions only"}
	}
	return nil
}
func compare(s Spec, r Record) []Change {
	var out []Change
	add := func(f, a, b string) {
		if a != b {
			out = append(out, Change{f, a, b})
		}
	}
	if s.UID != nil {
		add("uid", r.UID, strconv.Itoa(*s.UID))
	}
	if s.Group != "" {
		add("group", r.Group, s.Group)
	}
	if s.GID != nil {
		add("gid", r.GID, strconv.Itoa(*s.GID))
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		add("home", r.Home, s.Home)
	}
	if s.Shell != "" {
		add("shell", r.Shell, s.Shell)
	}
	if s.Groups != nil && !sameStrings(r.Groups, s.Groups) {
		out = append(out, Change{"groups", strings.Join(r.Groups, ","), strings.Join(s.Groups, ",")})
	}
	return out
}
func sameStrings(a, b []string) bool {
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
