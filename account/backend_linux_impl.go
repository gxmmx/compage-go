package account

import (
	"context"
	"errors"
	"os/user"
	"strconv"
	"strings"

	"github.com/gxmmx/compage-go/errx"
)

// linuxBackend uses the shadow-utils interface.  Every flag it emits is
// capability-checked before mutation, and state is reread by operation after
// application rather than trusting command exit status alone.
type linuxBackend struct{ backendDeps }

func newLinuxBackend() backend { return linuxBackend{backendDeps: defaultBackendDeps()} }

func (b linuxBackend) lookup(ctx context.Context, key string, byID bool) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, errx.New("account: lookup cancelled", errx.WithCause(err))
	}
	var (
		u   *user.User
		err error
	)
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
	out, err := commandOutput(ctx, b.runner, "getent", "passwd", u.Username)
	if err != nil {
		return Record{}, err
	}
	fields := strings.Split(strings.TrimSpace(out), ":")
	if len(fields) != 7 || fields[0] != u.Username {
		return Record{}, errx.New("account: malformed passwd record for " + u.Username)
	}
	return Record{Name: u.Username, UID: u.Uid, GID: u.Gid, Group: primary.Name, Home: fields[5], Shell: fields[6], Groups: groups}, nil
}

func (b linuxBackend) homeExists(ctx context.Context, path string) (bool, error) {
	return homeExists(ctx, b.fs, path)
}

func (b linuxBackend) preflight(ctx context.Context, s Spec, modifying bool) error {
	return b.checkCapabilities(ctx, s, modifying)
}

func (b linuxBackend) apply(ctx context.Context, s Spec, existing *Record) ([]Change, error) {
	if err := ctx.Err(); err != nil {
		return nil, errx.New("account: reconcile cancelled", errx.WithCause(err))
	}
	if err := b.preflight(ctx, s, existing != nil); err != nil {
		return nil, err
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
		if err := b.modify(ctx, s); err != nil {
			return completed, err
		}
		completed = append(completed, compare(s, *existing)...)
	}
	if s.HomePolicy == EnsureHome {
		wasPresent, err := b.homeExists(ctx, s.Home)
		if err != nil {
			return completed, err
		}
		if err := b.ensureHome(ctx, s); err != nil {
			return completed, err
		}
		if !wasPresent {
			completed = append(completed, Change{Field: "home directory", Before: "absent", After: "present"})
		}
	}
	return completed, nil
}

func (b linuxBackend) ensureGroup(ctx context.Context, s Spec) (bool, error) {
	if s.Group == "" {
		return false, nil
	}
	group, err := b.store.lookupGroup(s.Group)
	if err == nil {
		if s.GID != nil && group.Gid != strconv.Itoa(*s.GID) {
			out, capabilityErr := commandOutput(ctx, b.runner, "groupmod", "--help")
			if capabilityErr != nil {
				return false, errx.New("account: inspecting Linux groupmod capabilities", errx.WithCause(capabilityErr))
			}
			if !strings.Contains(out, "--gid") {
				return false, &UnsupportedError{Capability: "Linux groupmod --gid"}
			}
			if err := runCommand(ctx, b.runner, "groupmod", "--gid", strconv.Itoa(*s.GID), s.Group); err != nil {
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
	args := []string{}
	if s.GID != nil {
		args = append(args, "--gid", strconv.Itoa(*s.GID))
	}
	args = append(args, s.Group)
	if err := runCommand(ctx, b.runner, "groupadd", args...); err != nil {
		return false, err
	}
	group, err = b.store.lookupGroup(s.Group)
	if err != nil {
		return false, errx.New("account: verifying created group "+s.Group, errx.WithCause(err))
	}
	if s.GID != nil && group.Gid != strconv.Itoa(*s.GID) {
		return false, &DriftError{Changes: []Change{{Field: "group gid", Before: group.Gid, After: strconv.Itoa(*s.GID)}}}
	}
	return true, nil
}

func (b linuxBackend) create(ctx context.Context, s Spec) error {
	args := []string{}
	if s.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*s.UID))
	}
	args = append(args, "--gid", s.Group)
	if s.Home != "" {
		args = append(args, "--home-dir", s.Home)
	}
	if s.HomePolicy == EnsureHome {
		args = append(args, "--create-home")
	} else {
		args = append(args, "--no-create-home")
	}
	if s.Shell != "" {
		args = append(args, "--shell", s.Shell)
	}
	if s.Groups != nil {
		args = append(args, "--groups", strings.Join(s.Groups, ","))
	}
	args = append(args, s.Name)
	return runCommand(ctx, b.runner, "useradd", args...)
}

func (b linuxBackend) modify(ctx context.Context, s Spec) error {
	args := []string{}
	if s.UID != nil {
		args = append(args, "--uid", strconv.Itoa(*s.UID))
	}
	if s.Group != "" {
		args = append(args, "--gid", s.Group)
	}
	if s.HomePolicy != LeaveHomeUnchanged {
		args = append(args, "--home", s.Home)
	}
	if s.Shell != "" {
		args = append(args, "--shell", s.Shell)
	}
	if s.Groups != nil {
		args = append(args, "--groups", strings.Join(s.Groups, ","))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, s.Name)
	return runCommand(ctx, b.runner, "usermod", args...)
}

func (b linuxBackend) ensureHome(ctx context.Context, s Spec) error {
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

func (b linuxBackend) checkCapabilities(ctx context.Context, s Spec, modifying bool) error {
	command, needed := "useradd", []string{"--uid", "--gid", "--home-dir", "--shell", "--groups", "--no-create-home"}
	if modifying {
		command, needed = "usermod", []string{"--uid", "--gid", "--home", "--shell", "--groups"}
	}
	if s.UID == nil {
		needed = without(needed, "--uid")
	}
	if s.HomePolicy == LeaveHomeUnchanged {
		needed = without(without(needed, "--home-dir"), "--home")
	} else {
		needed = without(needed, "--no-create-home")
	}
	if s.HomePolicy == EnsureHome && !modifying {
		needed = append(needed, "--create-home")
	}
	if s.Shell == "" {
		needed = without(needed, "--shell")
	}
	if s.Groups == nil {
		needed = without(needed, "--groups")
	}
	out, err := commandOutput(ctx, b.runner, command, "--help")
	if err != nil {
		return errx.New("account: inspecting Linux "+command+" capabilities", errx.WithCause(err))
	}
	for _, flag := range needed {
		if !strings.Contains(out, flag) {
			return &UnsupportedError{Capability: "Linux " + command + " " + flag}
		}
	}
	out, err = commandOutput(ctx, b.runner, "groupadd", "--help")
	if err != nil {
		return errx.New("account: inspecting Linux groupadd capabilities", errx.WithCause(err))
	}
	if s.GID != nil && !strings.Contains(out, "--gid") {
		return &UnsupportedError{Capability: "Linux groupadd --gid"}
	}
	if s.GID != nil && modifying {
		out, err = commandOutput(ctx, b.runner, "groupmod", "--help")
		if err != nil {
			return errx.New("account: inspecting Linux groupmod capabilities", errx.WithCause(err))
		}
		if !strings.Contains(out, "--gid") {
			return &UnsupportedError{Capability: "Linux groupmod --gid"}
		}
	}
	return nil
}
