package certs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type FileStore struct {
	directory, owner, group string
	uid, gid                int
}

func NewFileStore(directory string, opts ...Option) (*FileStore, error) {
	o, err := parseOptions(scopeStore, opts)
	if err != nil {
		return nil, err
	}
	if !o.ownerSet || strings.TrimSpace(o.owner) == "" {
		return nil, invalid("FileStore requires WithOwner", nil)
	}
	if !o.groupSet || strings.TrimSpace(o.group) == "" {
		return nil, invalid("FileStore requires WithGroup", nil)
	}
	u, err := user.Lookup(o.owner)
	if err != nil {
		return nil, invalid("unknown owner", err)
	}
	g, err := user.LookupGroup(o.group)
	if err != nil {
		return nil, invalid("unknown group", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return nil, err
	}
	s := &FileStore{directory: filepath.Clean(directory), owner: o.owner, group: o.group, uid: uid, gid: gid}
	if err := s.validateDirectory(true); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *FileStore) kind() string                { return "file" }
func (s *FileStore) principal() (string, string) { return s.owner, s.group }
func (s *FileStore) authorizeOwner() error {
	if effectiveUID() != s.uid {
		return &ForbiddenError{Message: "authority operations require the configured owner"}
	}
	return nil
}
func (s *FileStore) authorizeIssuer() error {
	if effectiveUID() == s.uid {
		return nil
	}
	groups, err := supplementaryGroups()
	if err != nil {
		return &ForbiddenError{Message: "cannot inspect issuer group membership", Cause: err}
	}
	for _, group := range groups {
		if group == s.gid {
			return nil
		}
	}
	return &ForbiddenError{Message: "issuer operations require configured group membership"}
}
func (s *FileStore) validateDirectory(requireOwner bool) error {
	if err := rejectSymlinkParents(s.directory); err != nil {
		return err
	}
	info, err := os.Lstat(s.directory)
	if err != nil {
		return invalid("authority directory must already exist", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return &ForbiddenError{Message: "authority path must be a non-symlink directory"}
	}
	uid, gid, ok := fileOwnership(info)
	if !ok {
		return &UnavailableError{Message: "filesystem ownership inspection"}
	}
	if requireOwner && (uid != s.uid || gid != s.gid) {
		return &ForbiddenError{Message: "authority directory owner or group does not match"}
	}
	if info.Mode().Perm() != 0750 {
		return &ForbiddenError{Message: "authority directory mode must be 0750"}
	}
	return nil
}
func (s *FileStore) validateFile(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return &ForbiddenError{Message: "store file must be regular and not a symlink"}
	}
	if info.Mode().Perm() != mode {
		return &ForbiddenError{Message: fmt.Sprintf("invalid mode for %s", filepath.Base(path))}
	}
	uid, gid, ok := fileOwnership(info)
	if !ok {
		return &UnavailableError{Message: "filesystem ownership inspection"}
	}
	if uid != s.uid || gid != s.gid {
		return &ForbiddenError{Message: "store file owner or group does not match"}
	}
	return nil
}
func (s *FileStore) statePath() string { return filepath.Join(s.directory, "ca.json") }
func (s *FileStore) lockPath() string  { return filepath.Join(s.directory, "ca.lock") }
func (s *FileStore) load(ctx context.Context, issuer bool) (*authorityState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.validateDirectory(true); err != nil {
		return nil, err
	}
	path := s.statePath()
	if err := s.validateFile(path, 0640); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &NotFoundError{Resource: "authority", Cause: err}
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state authorityState
	if err = json.Unmarshal(data, &state); err != nil {
		return nil, corrupt("malformed ca.json", err)
	}
	if err = validateState(&state); err != nil {
		return nil, err
	}
	return &state, nil
}
func (s *FileStore) withLock(ctx context.Context, fn func() error) error {
	path := s.lockPath()
	f, err := openNoFollow(path, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return err
	}
	defer f.Close()
	_ = f.Chmod(0640)
	_ = f.Chown(s.uid, s.gid)
	if err = acquireFileLock(ctx, f); err != nil {
		return err
	}
	defer releaseFileLock(f)
	return fn()
}
func (s *FileStore) update(ctx context.Context, mutate func(*authorityState) error) error {
	return s.withLock(ctx, func() error {
		var state *authorityState
		existingState := false
		current, err := s.load(ctx, false)
		if err != nil {
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				return err
			}
			state = &authorityState{}
		} else {
			state = current
			existingState = true
		}
		if existingState && state.Spec.IssuanceMode == Ledger {
			if err = s.validateLedgerStorage(); err != nil {
				return err
			}
			records, readErr := s.readLedger(ctx)
			if readErr != nil {
				return readErr
			}
			if err = validateLedgerRecords(state, records); err != nil {
				return err
			}
		}
		before := state.Revision
		if err = mutate(state); errors.Is(err, errNoMutation) {
			return nil
		} else if err != nil {
			return err
		}
		if state.Spec.ID == "" {
			return corrupt("mutation produced empty authority", nil)
		}
		if state.Revision <= before {
			state.Revision = before + 1
		}
		if err = validateState(state); err != nil {
			return err
		}
		if state.Spec.IssuanceMode == Ledger && !existingState {
			if err = s.ensureLedger(); err != nil {
				return err
			}
		}
		return s.writeState(state)
	})
}
func (s *FileStore) writeState(state *authorityState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.directory, ".ca-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if err = tmp.Chmod(0640); err == nil {
		err = tmp.Chown(s.uid, s.gid)
	}
	if err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, s.statePath()); err != nil {
		return err
	}
	ok = true
	d, err := os.Open(s.directory)
	if err == nil {
		err = d.Sync()
		_ = d.Close()
	}
	return err
}
func (s *FileStore) purge(ctx context.Context) error {
	err := s.withLock(ctx, func() error {
		paths := []string{s.statePath(), filepath.Join(s.directory, "localunlock")}
		for _, p := range paths {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		ledger := filepath.Join(s.directory, "ledger")
		if entries, err := os.ReadDir(ledger); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					return &ForbiddenError{Message: "unexpected directory in ledger"}
				}
				if err = os.Remove(filepath.Join(ledger, e.Name())); err != nil {
					return err
				}
			}
			if err = os.Remove(ledger); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = os.Remove(s.lockPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func (s *FileStore) loadUnlocks(ctx context.Context) (map[int][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.directory, "localunlock")
	if err := s.validateFile(path, 0600); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &NotFoundError{Resource: "local unlock", Cause: err}
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[int][]byte
	if err = json.Unmarshal(data, &v); err != nil {
		return nil, corrupt("malformed localunlock", err)
	}
	return v, nil
}
func (s *FileStore) storeUnlocks(ctx context.Context, v map[int][]byte, replace bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(s.directory, "localunlock")
	if _, err := os.Lstat(path); err == nil && !replace {
		return &ConflictError{Message: "local unlock already exists"}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return atomicStoreFile(path, append(data, '\n'), 0600, s.uid, s.gid)
}
func atomicStoreFile(path string, data []byte, mode os.FileMode, uid, gid int) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".store-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(mode); err == nil && uid >= 0 && gid >= 0 {
		err = f.Chown(uid, gid)
	}
	if err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
func (s *FileStore) ledgerPaths() (string, string, string) {
	d := filepath.Join(s.directory, "ledger")
	return d, filepath.Join(d, "ledger.jsonl"), filepath.Join(d, "ledger.lock")
}
func (s *FileStore) ensureLedger() error {
	d, data, lock := s.ledgerPaths()
	directoryCreated := false
	if existing, err := os.Lstat(d); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 || !existing.IsDir() {
			return &ForbiddenError{Message: "invalid ledger directory"}
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err = os.Mkdir(d, 0770); err != nil {
			return err
		}
		directoryCreated = true
	}
	if directoryCreated {
		if err := os.Chmod(d, 0770); err != nil {
			return err
		}
		if err := os.Chown(d, s.uid, s.gid); err != nil {
			return err
		}
	}
	for _, p := range []string{data, lock} {
		created := false
		if existing, statErr := os.Lstat(p); statErr == nil {
			if existing.Mode()&os.ModeSymlink != 0 || !existing.Mode().IsRegular() {
				return &ForbiddenError{Message: "invalid ledger file"}
			}
		} else {
			if !errors.Is(statErr, os.ErrNotExist) {
				return statErr
			}
			created = true
		}
		f, err := openNoFollow(p, os.O_CREATE|os.O_RDWR, 0660)
		if err != nil {
			return err
		}
		if created {
			if err = f.Chmod(0660); err == nil {
				err = f.Chown(s.uid, s.gid)
			}
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if err = s.validateFile(p, 0660); err != nil {
			return err
		}
	}
	return s.validateLedgerStorage()
}
func (s *FileStore) validateLedgerStorage() error {
	d, data, lock := s.ledgerPaths()
	info, err := os.Lstat(d)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0770 {
		return &ForbiddenError{Message: "invalid ledger directory"}
	}
	uid, gid, ok := fileOwnership(info)
	if !ok || uid != s.uid || gid != s.gid {
		return &ForbiddenError{Message: "ledger directory owner or group does not match"}
	}
	for _, path := range []string{data, lock} {
		if err = s.validateFile(path, 0660); err != nil {
			return err
		}
	}
	return nil
}
func (s *FileStore) appendLedger(ctx context.Context, r LedgerRecord) error {
	if err := s.validateLedgerStorage(); err != nil {
		return err
	}
	return s.withAuthorityReadLock(ctx, func() error {
		state, err := s.load(ctx, true)
		if err != nil {
			return err
		}
		if err = validateLedgerAuthority(state, &r); err != nil {
			return err
		}
		return s.appendLedgerRecord(ctx, r)
	})
}
func (s *FileStore) withAuthorityReadLock(ctx context.Context, fn func() error) error {
	f, err := openNoFollow(s.lockPath(), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = acquireFileReadLock(ctx, f); err != nil {
		return err
	}
	defer releaseFileLock(f)
	return fn()
}
func (s *FileStore) appendLedgerRecord(ctx context.Context, r LedgerRecord) error {
	_, data, lock := s.ledgerPaths()
	f, err := openNoFollow(lock, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = acquireFileLock(ctx, f); err != nil {
		return err
	}
	defer releaseFileLock(f)
	records, err := s.readLedger(ctx)
	if err != nil {
		return err
	}
	for _, x := range records {
		if x.Issuer == r.Issuer && x.IssuerVersion == r.IssuerVersion && x.Serial == r.Serial {
			return &ConflictError{Message: "issuer serial already recorded"}
		}
		if x.Fingerprint == r.Fingerprint {
			return &ConflictError{Message: "certificate already recorded"}
		}
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	out, err := openNoFollow(data, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err = out.Write(append(b, '\n')); err == nil {
		err = out.Sync()
	}
	ce := out.Close()
	if err == nil {
		err = ce
	}
	return err
}
func (s *FileStore) readLedger(ctx context.Context) ([]LedgerRecord, error) {
	_, data, _ := s.ledgerPaths()
	f, err := openNoFollow(data, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []LedgerRecord
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				return nil, corrupt("incomplete final ledger record", nil)
			}
			var r LedgerRecord
			if err = json.Unmarshal(line, &r); err != nil {
				return nil, corrupt("malformed ledger record", err)
			}
			if err = validateLedgerRecord(&r); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *FileStore) lookupLedger(ctx context.Context, issuer string, version int, serial string) (*LedgerRecord, error) {
	v, err := s.readLedger(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range v {
		if r.Issuer == issuer && r.IssuerVersion == version && r.Serial == serial {
			x := r
			return &x, nil
		}
	}
	return nil, &NotFoundError{Resource: "ledger certificate"}
}
func (s *FileStore) listLedger(ctx context.Context, cursor string, limit int) ([]LedgerRecord, string, error) {
	var v []LedgerRecord
	err := s.withLedgerReadLock(ctx, func() error { var readErr error; v, readErr = s.readLedger(ctx); return readErr })
	if err != nil {
		return nil, "", err
	}
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(cursor)
	}
	if start < 0 || start > len(v) {
		return nil, "", invalid("invalid ledger cursor", nil)
	}
	if limit <= 0 {
		limit = 100
	}
	end := start + limit
	if end > len(v) {
		end = len(v)
	}
	next := ""
	if end < len(v) {
		next = strconv.Itoa(end)
	}
	return append([]LedgerRecord(nil), v[start:end]...), next, nil
}
func (s *FileStore) withLedgerReadLock(ctx context.Context, fn func() error) error {
	if err := s.validateLedgerStorage(); err != nil {
		return err
	}
	_, _, lock := s.ledgerPaths()
	f, err := openNoFollow(lock, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = acquireFileReadLock(ctx, f); err != nil {
		return err
	}
	defer releaseFileLock(f)
	return fn()
}
