package account

import (
	"context"
	"errors"
	"os/user"
	"strconv"
	"strings"

	"github.com/gxmmx/compage-go/errx"
)

// darwinBackend manages local Directory Service records only.  It does not
// translate shadow-utils semantics or use Linux account-management commands.
type darwinBackend struct{ backendDeps }

func newDarwinBackend() backend { return darwinBackend{backendDeps: defaultBackendDeps()} }

func (b darwinBackend) lookup(ctx context.Context, key string, byID bool) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, errx.New("account: lookup cancelled", errx.WithCause(err))
	}
	var u *user.User
	var err error
	if byID {
		u, err = b.store.lookupID(key)
	} else {
		u, err = b.store.lookup(key)
	}
	if err != nil {
		return Record{}, &NotFoundError{Key: key, Cause: err}
	}
	primary, err := b.store.lookupGroupID(u.Gid)
	if err != nil {
		return Record{}, errx.New("account: resolving primary group "+u.Gid, errx.WithCause(err))
	}
	ids, err := b.store.groupIDs(u)
	if err != nil {
		return Record{}, errx.New("account: resolving groups for "+u.Username, errx.WithCause(err))
	}
	groups := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == u.Gid {
			continue
		}
		group, lookupErr := b.store.lookupGroupID(id)
		if lookupErr != nil {
			return Record{}, errx.New("account: resolving group "+id, errx.WithCause(lookupErr))
		}
		groups = append(groups, group.Name)
	}
	out, err := commandOutput(ctx, b.runner, "dscl", ".", "-read", "/Users/"+u.Username, "NFSHomeDirectory", "UserShell", "IsHidden")
	if err != nil {
		return Record{}, err
	}
	values := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 {
			values[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	if values["NFSHomeDirectory"] == "" || values["UserShell"] == "" {
		return Record{}, errx.New("account: malformed Directory Service record for " + u.Username)
	}
	return Record{Name: u.Username, UID: u.Uid, GID: u.Gid, Group: primary.Name, Home: values["NFSHomeDirectory"], Shell: values["UserShell"], Groups: groups, Hidden: values["IsHidden"] == "1" || values["IsHidden"] == "true"}, nil
}

func (b darwinBackend) homeExists(ctx context.Context, path string) (bool, error) {
	return homeExists(ctx, b.fs, path)
}

func (b darwinBackend) preflight(ctx context.Context, s Spec, _ bool) error {
	if _, err := commandOutput(ctx, b.runner, "dscl", "-help"); err != nil {
		return errx.New("account: inspecting macOS Directory Service capabilities", errx.WithCause(err))
	}
	if s.Group != "" || s.Groups != nil {
		if _, err := commandOutput(ctx, b.runner, "dseditgroup", "-help"); err != nil {
			return errx.New("account: inspecting macOS group capabilities", errx.WithCause(err))
		}
	}
	return nil
}

func (b darwinBackend) apply(ctx context.Context, s Spec, existing *Record) ([]Change, error) {
	if err := ctx.Err(); err != nil {
		return nil, errx.New("account: reconcile cancelled", errx.WithCause(err))
	}
	if existing == nil && s.UID == nil {
		uid, err := b.allocateUID(ctx)
		if err != nil {
			return nil, err
		}
		s.UID = &uid
	}
	var completed []Change
	createdGroup, err := b.ensureGroup(ctx, s)
	if err != nil {
		return completed, err
	}
	if createdGroup {
		completed = append(completed, Change{Field: "group record", Before: "absent", After: s.Group})
		if s.GID != nil {
			completed = append(completed, Change{Field: "group record gid", Before: "absent", After: strconv.Itoa(*s.GID)})
		}
	}
	if existing == nil {
		if err := b.create(ctx, s); err != nil {
			return completed, err
		}
		completed = append(completed, creationChanges(s)...)
	} else {
		if err := b.modify(ctx, s, *existing); err != nil {
			return completed, err
		}
		completed = append(completed, compare(s, *existing)...)
	}
	if s.HomePolicy == EnsureHome {
		present, err := b.homeExists(ctx, s.Home)
		if err != nil {
			return completed, err
		}
		if err := b.ensureHome(ctx, s); err != nil {
			return completed, err
		}
		if !present {
			completed = append(completed, Change{Field: "home directory", Before: "absent", After: "present"})
		}
	}
	return completed, nil
}

func (b darwinBackend) ensureGroup(ctx context.Context, s Spec) (bool, error) {
	if s.Group == "" {
		return false, nil
	}
	group, err := b.store.lookupGroup(s.Group)
	if err == nil {
		if s.GID != nil && group.Gid != strconv.Itoa(*s.GID) {
			if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Groups/"+s.Group, "PrimaryGroupID", strconv.Itoa(*s.GID)); err != nil {
				return false, err
			}
			verified, verifyErr := b.store.lookupGroup(s.Group)
			if verifyErr != nil {
				return false, errx.New("account: verifying reconciled group "+s.Group, errx.WithCause(verifyErr))
			}
			if verified.Gid != strconv.Itoa(*s.GID) {
				return false, &DriftError{Changes: []Change{{Field: "group gid", Before: verified.Gid, After: strconv.Itoa(*s.GID)}}}
			}
			return false, nil
		}
		return false, nil
	}
	if !isUnknownGroup(err) {
		return false, errx.New("account: resolving group "+s.Group, errx.WithCause(err))
	}
	if err := runCommand(ctx, b.runner, "dseditgroup", "-o", "create", s.Group); err != nil {
		return false, err
	}
	gid := s.GID
	if gid == nil {
		allocated, allocationErr := b.allocateGID(ctx)
		if allocationErr != nil {
			return false, allocationErr
		}
		gid = &allocated
	}
	if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Groups/"+s.Group, "PrimaryGroupID", strconv.Itoa(*gid)); err != nil {
		return false, err
	}
	group, err = b.store.lookupGroup(s.Group)
	if err != nil {
		return false, errx.New("account: verifying created group "+s.Group, errx.WithCause(err))
	}
	if group.Gid != strconv.Itoa(*gid) {
		return false, &DriftError{Changes: []Change{{Field: "group gid", Before: group.Gid, After: strconv.Itoa(*gid)}}}
	}
	return true, nil
}

func (b darwinBackend) create(ctx context.Context, s Spec) error {
	if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name); err != nil {
		return err
	}
	if err := b.setAttributes(ctx, s); err != nil {
		return err
	}
	for _, group := range s.Groups {
		if err := runCommand(ctx, b.runner, "dseditgroup", "-o", "edit", "-a", s.Name, "-t", "user", group); err != nil {
			return err
		}
	}
	return nil
}

func (b darwinBackend) modify(ctx context.Context, s Spec, existing Record) error {
	if err := b.setAttributes(ctx, s); err != nil {
		return err
	}
	if s.Groups == nil {
		return nil
	}
	for _, group := range s.Groups {
		if contains(existing.Groups, group) {
			continue
		}
		if err := runCommand(ctx, b.runner, "dseditgroup", "-o", "edit", "-a", s.Name, "-t", "user", group); err != nil {
			return err
		}
	}
	for _, group := range existing.Groups {
		if contains(s.Groups, group) {
			continue
		}
		if err := runCommand(ctx, b.runner, "dseditgroup", "-o", "edit", "-d", s.Name, "-t", "user", group); err != nil {
			return err
		}
	}
	return nil
}

func (b darwinBackend) setAttributes(ctx context.Context, s Spec) error {
	if s.Hidden != nil {
		hidden := "0"
		if *s.Hidden {
			hidden = "1"
		}
		if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name, "IsHidden", hidden); err != nil {
			return err
		}
	}
	if s.UID != nil {
		if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name, "UniqueID", strconv.Itoa(*s.UID)); err != nil {
			return err
		}
	}
	if s.Group != "" {
		group, err := b.store.lookupGroup(s.Group)
		if err != nil {
			return errx.New("account: resolving primary group "+s.Group, errx.WithCause(err))
		}
		if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name, "PrimaryGroupID", group.Gid); err != nil {
			return err
		}
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name, "NFSHomeDirectory", s.Home); err != nil {
			return err
		}
	}
	if s.Shell != "" {
		if err := runCommand(ctx, b.runner, "dscl", ".", "-create", "/Users/"+s.Name, "UserShell", s.Shell); err != nil {
			return err
		}
	}
	return nil
}

func (b darwinBackend) ensureHome(ctx context.Context, s Spec) error {
	if err := ensureHomeDirectory(b.fs, s.Home, homeMode(s)); err != nil {
		return err
	}
	record, err := b.lookup(ctx, s.Name, false)
	if err != nil {
		return err
	}
	uid, uidErr := strconv.Atoi(record.UID)
	gid, gidErr := strconv.Atoi(record.GID)
	if uidErr != nil || gidErr != nil {
		return errx.New("account: malformed account identifiers for "+s.Name, errx.WithCause(errors.Join(uidErr, gidErr)))
	}
	if err := b.fs.chown(s.Home, uid, gid); err != nil {
		return errx.New("account: owning home "+s.Home, errx.WithCause(err))
	}
	if err := b.fs.chmod(s.Home, homeMode(s)); err != nil {
		return errx.New("account: chmod home "+s.Home, errx.WithCause(err))
	}
	return verifyHome(b.fs, s.Home, uid, gid, homeMode(s))
}

func (b darwinBackend) allocateUID(ctx context.Context) (int, error) {
	return b.allocateID(ctx, "/Users", "UniqueID", "macOS local UID allocation")
}

func (b darwinBackend) allocateGID(ctx context.Context) (int, error) {
	return b.allocateID(ctx, "/Groups", "PrimaryGroupID", "macOS local GID allocation")
}

func (b darwinBackend) allocateID(ctx context.Context, node, attribute, capability string) (int, error) {
	out, err := commandOutput(ctx, b.runner, "dscl", ".", "-list", node, attribute)
	if err != nil {
		return 0, errx.New("account: listing macOS local UIDs", errx.WithCause(err))
	}
	used := make(map[int]struct{})
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if id, parseErr := strconv.Atoi(fields[len(fields)-1]); parseErr == nil && id >= 500 {
			used[id] = struct{}{}
		}
	}
	for id := 500; id < 1<<31-1; id++ {
		if _, exists := used[id]; !exists {
			return id, nil
		}
	}
	return 0, &UnsupportedError{Capability: capability}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
