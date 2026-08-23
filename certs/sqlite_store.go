package certs

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	path, owner, group string
	uid, gid           int
	private            bool
	readOnly           bool
	db                 *sql.DB
	mu                 sync.Mutex
}

func NewSQLiteStore(path string, opts ...Option) (*SQLiteStore, error) {
	o, err := parseOptions(scopeStore, opts)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, invalid("SQLite path is required", nil)
	}
	if o.ownerSet != o.groupSet {
		return nil, invalid("SQLite split-principal mode requires both WithOwner and WithGroup", nil)
	}
	if o.ownerSet && !splitSecuritySupported() {
		return nil, &UnavailableError{Message: "SQLite split owner/group mode is unsupported on this platform"}
	}
	directory := filepath.Dir(path)
	if err := rejectSymlinkParents(directory); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalid("SQLite parent must be an existing non-symlink directory", err)
	}
	s := &SQLiteStore{path: filepath.Clean(path), private: !o.ownerSet}
	if o.ownerSet {
		u, e := user.Lookup(o.owner)
		if e != nil {
			return nil, invalid("unknown owner", e)
		}
		g, e := user.LookupGroup(o.group)
		if e != nil {
			return nil, invalid("unknown group", e)
		}
		s.uid, _ = strconv.Atoi(u.Uid)
		s.gid, _ = strconv.Atoi(g.Gid)
		s.owner = o.owner
		s.group = o.group
		if info.Mode().Perm() != 0750 {
			return nil, &ForbiddenError{Message: "SQLite directory mode must be 0750"}
		}
		uid, gid, ok := fileOwnership(info)
		if !ok || uid != s.uid || gid != s.gid {
			return nil, &ForbiddenError{Message: "SQLite directory owner or group does not match"}
		}
	} else {
		u, e := user.Current()
		if e != nil {
			return nil, e
		}
		s.owner = u.Username
		s.uid, err = strconv.Atoi(u.Uid)
		if err != nil {
			if splitSecuritySupported() {
				return nil, &UnavailableError{Message: "cannot resolve current user ID", Cause: err}
			}
			s.uid = -1
		}
		s.gid = effectiveGID()
		if info.Mode().Perm()&0077 != 0 {
			return nil, &ForbiddenError{Message: "private SQLite directory must not permit group or other access"}
		}
	}
	existed := false
	if existing, e := os.Lstat(s.path); e == nil {
		existed = true
		if existing.Mode()&os.ModeSymlink != 0 || !existing.Mode().IsRegular() {
			return nil, &ForbiddenError{Message: "SQLite authority must be a regular non-symlink file"}
		}
		expectedMode := os.FileMode(0600)
		if !s.private {
			expectedMode = 0640
		}
		if existing.Mode().Perm() != expectedMode {
			return nil, &ForbiddenError{Message: "invalid SQLite authority mode"}
		}
		if !s.private {
			uid, gid, ok := fileOwnership(existing)
			if !ok || uid != s.uid || gid != s.gid {
				return nil, &ForbiddenError{Message: "SQLite authority owner or group does not match"}
			}
		}
	}
	s.readOnly = existed && !s.private && effectiveUID() != s.uid
	if !existed && effectiveUID() != s.uid {
		return nil, &ForbiddenError{Message: "only the configured owner may create the SQLite authority"}
	}
	dsn := s.path
	if s.readOnly {
		dsn = sqliteReadOnlyDSN(s.path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s.db = db
	db.SetMaxOpenConns(1)
	if !s.readOnly {
		err = s.migrate(context.Background())
	} else {
		err = s.verifySchema(context.Background())
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	mode := os.FileMode(0600)
	if !s.private {
		mode = 0640
	}
	if !s.readOnly {
		err = os.Chmod(s.path, mode)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	if !s.private && !s.readOnly {
		if err = os.Chown(s.path, s.uid, s.gid); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *SQLiteStore) kind() string                { return "sqlite" }
func (s *SQLiteStore) principal() (string, string) { return s.owner, s.group }
func (s *SQLiteStore) authorizeOwner() error {
	if s.private {
		return nil
	}
	if effectiveUID() != s.uid {
		return &ForbiddenError{Message: "authority operations require the configured owner"}
	}
	return nil
}
func (s *SQLiteStore) authorizeIssuer() error {
	if s.private || effectiveUID() == s.uid {
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
func (s *SQLiteStore) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode=DELETE`, `PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`,
		`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`,
		`CREATE TABLE IF NOT EXISTS authority_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), revision INTEGER NOT NULL, document BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS authority_records (authority_id TEXT PRIMARY KEY, format_version INTEGER NOT NULL, specification BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS root_generations (generation INTEGER PRIMARY KEY, certificate_der BLOB NOT NULL, fingerprint TEXT NOT NULL, retired_trust_until TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issuer_definitions (slug TEXT PRIMARY KEY, display_name TEXT NOT NULL, definition BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS issuer_versions (issuer_slug TEXT NOT NULL, version INTEGER NOT NULL, root_generation INTEGER NOT NULL, certificate_der BLOB NOT NULL, fingerprint TEXT NOT NULL, PRIMARY KEY(issuer_slug,version))`,
		`CREATE TABLE IF NOT EXISTS topology_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), revision INTEGER NOT NULL, active_root INTEGER NOT NULL, pending BLOB)`,
		`CREATE TABLE IF NOT EXISTS encrypted_keys (key_kind TEXT NOT NULL, key_id TEXT NOT NULL, key_der BLOB NOT NULL, PRIMARY KEY(key_kind,key_id))`,
	}
	for _, q := range statements {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("certs: SQLite migration: %w", err)
		}
	}
	return nil
}
func (s *SQLiteStore) verifySchema(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version != 1 {
		return &UnavailableError{Message: "unsupported SQLite schema version"}
	}
	var mode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		return err
	}
	if strings.ToLower(mode) != "delete" {
		return &CorruptError{Message: "SQLite authority must use DELETE journal mode"}
	}
	return nil
}
func (s *SQLiteStore) load(ctx context.Context, issuer bool) (*authorityState, error) {
	var data []byte
	var err error
	var parityDB *sql.DB
	if issuer {
		dsn := sqliteReadOnlyDSN(s.path)
		db, e := sql.Open("sqlite", dsn)
		if e != nil {
			return nil, e
		}
		defer db.Close()
		parityDB = db
		_, _ = db.ExecContext(ctx, `PRAGMA query_only=ON`)
		err = db.QueryRowContext(ctx, `SELECT document FROM authority_state WHERE singleton=1`).Scan(&data)
	} else {
		parityDB = s.db
		err = s.db.QueryRowContext(ctx, `SELECT document FROM authority_state WHERE singleton=1`).Scan(&data)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &NotFoundError{Resource: "authority"}
	}
	if err != nil {
		return nil, err
	}
	var state authorityState
	if err = json.Unmarshal(data, &state); err != nil {
		return nil, corrupt("malformed SQLite authority state", err)
	}
	if err = validateState(&state); err != nil {
		return nil, err
	}
	if err = validateSQLiteParity(ctx, parityDB, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
func (s *SQLiteStore) update(ctx context.Context, mutate func(*authorityState) error) error {
	if s.readOnly {
		return &ForbiddenError{Message: "SQLite authority was opened read-only"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var data []byte
	state := &authorityState{}
	existingState := false
	err = tx.QueryRowContext(ctx, `SELECT document FROM authority_state WHERE singleton=1`).Scan(&data)
	if err == nil {
		existingState = true
		if err = json.Unmarshal(data, state); err != nil {
			return corrupt("malformed SQLite authority state", err)
		}
		if err = validateState(state); err != nil {
			return err
		}
		if err = validateSQLiteParity(ctx, tx, state); err != nil {
			return err
		}
		if state.Spec.IssuanceMode == Ledger {
			records, _, ledgerErr := s.listLedger(ctx, "", int(^uint(0)>>1))
			if ledgerErr != nil {
				return ledgerErr
			}
			if ledgerErr = validateLedgerRecords(state, records); ledgerErr != nil {
				return ledgerErr
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	before := state.Revision
	if err = mutate(state); errors.Is(err, errNoMutation) {
		return nil
	} else if err != nil {
		return err
	}
	if state.Revision <= before {
		state.Revision = before + 1
	}
	if err = validateState(state); err != nil {
		return err
	}
	data, err = json.Marshal(state)
	if err != nil {
		return err
	}
	if err = syncSQLiteTables(ctx, tx, state); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO authority_state(singleton,revision,document) VALUES(1,?,?) ON CONFLICT(singleton) DO UPDATE SET revision=excluded.revision, document=excluded.document`, state.Revision, data); err != nil {
		return err
	}
	if state.Spec.IssuanceMode == Ledger && !existingState {
		ledger, ledgerErr := s.openLedger()
		if ledgerErr != nil {
			return ledgerErr
		}
		if ledgerErr = ledger.Close(); ledgerErr != nil {
			return ledgerErr
		}
	}
	return tx.Commit()
}

func syncSQLiteTables(ctx context.Context, tx *sql.Tx, state *authorityState) error {
	for _, table := range []string{"authority_records", "root_generations", "issuer_definitions", "issuer_versions", "topology_state", "encrypted_keys"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	spec, err := json.Marshal(state.Spec)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO authority_records(authority_id,format_version,specification) VALUES(?,?,?)`, state.Spec.ID, state.Spec.FormatVersion, spec); err != nil {
		return err
	}
	for generation, root := range state.Roots {
		if _, err = tx.ExecContext(ctx, `INSERT INTO root_generations(generation,certificate_der,fingerprint,retired_trust_until) VALUES(?,?,?,?)`, generation, root.CertificateDER, root.Fingerprint, root.RetiredTrustUntil.UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if len(root.EncryptedKeyDER) > 0 {
			if _, err = tx.ExecContext(ctx, `INSERT INTO encrypted_keys(key_kind,key_id,key_der) VALUES('root',?,?)`, strconv.Itoa(generation), root.EncryptedKeyDER); err != nil {
				return err
			}
		}
	}
	for slug, issuer := range state.Issuers {
		definition, marshalErr := json.Marshal(issuer.Definition)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO issuer_definitions(slug,display_name,definition) VALUES(?,?,?)`, slug, issuer.Definition.Name, definition); err != nil {
			return err
		}
		for version, v := range issuer.Versions {
			if _, err = tx.ExecContext(ctx, `INSERT INTO issuer_versions(issuer_slug,version,root_generation,certificate_der,fingerprint) VALUES(?,?,?,?,?)`, slug, version, v.RootGeneration, v.CertificateDER, v.Fingerprint); err != nil {
				return err
			}
			if len(v.KeyDER) > 0 {
				if _, err = tx.ExecContext(ctx, `INSERT INTO encrypted_keys(key_kind,key_id,key_der) VALUES('issuer',?,?)`, slug+":"+strconv.Itoa(version), v.KeyDER); err != nil {
					return err
				}
			}
		}
	}
	var pending []byte
	if state.Pending != nil {
		pending, err = json.Marshal(state.Pending)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO topology_state(singleton,revision,active_root,pending) VALUES(1,?,?,?)`, state.Revision, state.ActiveRoot, pending)
	return err
}

type sqliteQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateSQLiteParity(ctx context.Context, q sqliteQueryRower, state *authorityState) error {
	var authorityID string
	var format int
	var specification []byte
	if err := q.QueryRowContext(ctx, `SELECT authority_id,format_version,specification FROM authority_records`).Scan(&authorityID, &format, &specification); err != nil {
		return corrupt("missing SQLite authority record", err)
	}
	expectedSpec, _ := json.Marshal(state.Spec)
	if authorityID != state.Spec.ID || format != state.Spec.FormatVersion || !bytes.Equal(specification, expectedSpec) {
		return &DriftError{Field: "SQLite authority record", Expected: state.Spec.ID, Actual: authorityID}
	}
	var revision uint64
	var activeRoot int
	var pending []byte
	if err := q.QueryRowContext(ctx, `SELECT revision,active_root,pending FROM topology_state WHERE singleton=1`).Scan(&revision, &activeRoot, &pending); err != nil {
		return corrupt("missing SQLite topology", err)
	}
	var expectedPending []byte
	if state.Pending != nil {
		expectedPending, _ = json.Marshal(state.Pending)
	}
	if revision != state.Revision || activeRoot != state.ActiveRoot || !bytes.Equal(pending, expectedPending) {
		return &DriftError{Field: "SQLite topology", Expected: state.Revision, Actual: revision}
	}
	var count int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM root_generations`).Scan(&count); err != nil || count != len(state.Roots) {
		return &DriftError{Field: "SQLite root count", Expected: len(state.Roots), Actual: count}
	}
	for generation, root := range state.Roots {
		var cert []byte
		var fingerprint, retired string
		if err := q.QueryRowContext(ctx, `SELECT certificate_der,fingerprint,retired_trust_until FROM root_generations WHERE generation=?`, generation).Scan(&cert, &fingerprint, &retired); err != nil {
			return corrupt("missing SQLite root generation", err)
		}
		if !bytes.Equal(cert, root.CertificateDER) || fingerprint != root.Fingerprint || retired != root.RetiredTrustUntil.UTC().Format(time.RFC3339Nano) {
			return &DriftError{Field: "SQLite root generation", Expected: generation, Actual: fingerprint}
		}
	}
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM issuer_definitions`).Scan(&count); err != nil || count != len(state.Issuers) {
		return &DriftError{Field: "SQLite issuer count", Expected: len(state.Issuers), Actual: count}
	}
	versionCount := 0
	keyCount := 0
	for slug, issuer := range state.Issuers {
		var display string
		var definition []byte
		if err := q.QueryRowContext(ctx, `SELECT display_name,definition FROM issuer_definitions WHERE slug=?`, slug).Scan(&display, &definition); err != nil {
			return corrupt("missing SQLite issuer definition", err)
		}
		expectedDefinition, _ := json.Marshal(issuer.Definition)
		if display != issuer.Definition.Name || !bytes.Equal(definition, expectedDefinition) {
			return &DriftError{Field: "SQLite issuer definition", Expected: issuer.Definition.Name, Actual: display}
		}
		for version, v := range issuer.Versions {
			versionCount++
			var rootGeneration int
			var cert []byte
			var fingerprint string
			if err := q.QueryRowContext(ctx, `SELECT root_generation,certificate_der,fingerprint FROM issuer_versions WHERE issuer_slug=? AND version=?`, slug, version).Scan(&rootGeneration, &cert, &fingerprint); err != nil {
				return corrupt("missing SQLite issuer version", err)
			}
			if rootGeneration != v.RootGeneration || !bytes.Equal(cert, v.CertificateDER) || fingerprint != v.Fingerprint {
				return &DriftError{Field: "SQLite issuer version", Expected: version, Actual: fingerprint}
			}
			if len(v.KeyDER) > 0 {
				keyCount++
				if err := validateSQLiteKey(ctx, q, "issuer", slug+":"+strconv.Itoa(version), v.KeyDER); err != nil {
					return err
				}
			}
		}
	}
	for generation, root := range state.Roots {
		if len(root.EncryptedKeyDER) > 0 {
			keyCount++
			if err := validateSQLiteKey(ctx, q, "root", strconv.Itoa(generation), root.EncryptedKeyDER); err != nil {
				return err
			}
		}
	}
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM issuer_versions`).Scan(&count); err != nil || count != versionCount {
		return &DriftError{Field: "SQLite issuer version count", Expected: versionCount, Actual: count}
	}
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM encrypted_keys`).Scan(&count); err != nil || count != keyCount {
		return &DriftError{Field: "SQLite key count", Expected: keyCount, Actual: count}
	}
	return nil
}
func validateSQLiteKey(ctx context.Context, q sqliteQueryRower, kind, id string, expected []byte) error {
	var actual []byte
	if err := q.QueryRowContext(ctx, `SELECT key_der FROM encrypted_keys WHERE key_kind=? AND key_id=?`, kind, id).Scan(&actual); err != nil {
		return corrupt("missing SQLite key record", err)
	}
	if !bytes.Equal(actual, expected) {
		return &DriftError{Field: "SQLite key record", Expected: id, Actual: "mismatch"}
	}
	return nil
}
func (s *SQLiteStore) unlockPath() string { return s.path + ".localunlock" }
func (s *SQLiteStore) loadUnlocks(ctx context.Context) (map[int][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(s.unlockPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &NotFoundError{Resource: "local unlock"}
		}
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 {
		return nil, &ForbiddenError{Message: "invalid local unlock file"}
	}
	data, err := os.ReadFile(s.unlockPath())
	if err != nil {
		return nil, err
	}
	var out map[int][]byte
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, corrupt("malformed local unlock", err)
	}
	return out, nil
}
func (s *SQLiteStore) storeUnlocks(ctx context.Context, v map[int][]byte, replace bool) error {
	if s.readOnly {
		return &ForbiddenError{Message: "SQLite authority was opened read-only"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(s.unlockPath()); err == nil && !replace {
		return &ConflictError{Message: "local unlock already exists"}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	uid, gid := s.uid, s.gid
	if s.private {
		uid, gid = -1, -1
	}
	return atomicStoreFile(s.unlockPath(), append(data, '\n'), 0600, uid, gid)
}
func (s *SQLiteStore) ledgerDirectory() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + ".ledger"
}
func (s *SQLiteStore) ledgerPath() string { return filepath.Join(s.ledgerDirectory(), "ledger.sqlite") }
func (s *SQLiteStore) openLedger() (*sql.DB, error) {
	directory := s.ledgerDirectory()
	directoryMode := os.FileMode(0700)
	fileMode := os.FileMode(0600)
	if !s.private {
		directoryMode = 0770
		fileMode = 0660
	}
	directoryCreated := false
	if existing, err := os.Lstat(directory); err == nil {
		if err = s.validateSQLiteLedgerDirectory(existing, directoryMode); err != nil {
			return nil, err
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err = os.Mkdir(directory, directoryMode); err != nil {
			return nil, err
		}
		directoryCreated = true
	}
	if directoryCreated {
		if err := os.Chmod(directory, directoryMode); err != nil {
			return nil, err
		}
		if !s.private {
			if chownErr := os.Chown(directory, s.uid, s.gid); chownErr != nil {
				return nil, chownErr
			}
		}
	}
	fileCreated := false
	if existing, err := os.Lstat(s.ledgerPath()); err == nil {
		if err = s.validateSQLiteLedgerFile(existing, fileMode); err != nil {
			return nil, err
		}
	} else {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		fileCreated = true
	}
	db, err := sql.Open("sqlite", s.ledgerPath())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	statements := []string{`PRAGMA journal_mode=DELETE`, `CREATE TABLE IF NOT EXISTS ledger (fingerprint TEXT PRIMARY KEY, issuer TEXT NOT NULL, issuer_version INTEGER NOT NULL, serial TEXT NOT NULL, document BLOB NOT NULL, UNIQUE(issuer,issuer_version,serial))`}
	for _, q := range statements {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	if fileCreated {
		if err = os.Chmod(s.ledgerPath(), fileMode); err != nil {
			db.Close()
			return nil, err
		}
		if !s.private {
			if err = os.Chown(s.ledgerPath(), s.uid, s.gid); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	info, err := os.Lstat(s.ledgerPath())
	if err != nil {
		db.Close()
		return nil, err
	}
	if err = s.validateSQLiteLedgerFile(info, fileMode); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func (s *SQLiteStore) validateSQLiteLedgerDirectory(info os.FileInfo, mode os.FileMode) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != mode {
		return &ForbiddenError{Message: "invalid SQLite ledger directory"}
	}
	if !s.private {
		uid, gid, ok := fileOwnership(info)
		if !ok || uid != s.uid || gid != s.gid {
			return &ForbiddenError{Message: "SQLite ledger directory owner or group does not match"}
		}
	}
	return nil
}
func (s *SQLiteStore) validateSQLiteLedgerFile(info os.FileInfo, mode os.FileMode) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		return &ForbiddenError{Message: "invalid SQLite ledger file"}
	}
	if !s.private {
		uid, gid, ok := fileOwnership(info)
		if !ok || uid != s.uid || gid != s.gid {
			return &ForbiddenError{Message: "SQLite ledger owner or group does not match"}
		}
	}
	return nil
}
func (s *SQLiteStore) appendLedger(ctx context.Context, r LedgerRecord) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stateData []byte
	if err = tx.QueryRowContext(ctx, `SELECT document FROM authority_state WHERE singleton=1`).Scan(&stateData); err != nil {
		return err
	}
	var state authorityState
	if err = json.Unmarshal(stateData, &state); err != nil {
		return corrupt("malformed SQLite authority state", err)
	}
	if err = validateLedgerAuthority(&state, &r); err != nil {
		return err
	}
	db, err := s.openLedgerWriteExisting()
	if err != nil {
		return err
	}
	defer db.Close()
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO ledger(fingerprint,issuer,issuer_version,serial,document) VALUES(?,?,?,?,?)`, r.Fingerprint, r.Issuer, r.IssuerVersion, r.Serial, data)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return &ConflictError{Message: "issuer serial already recorded"}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLiteStore) openLedgerWriteExisting() (*sql.DB, error) {
	directoryMode := os.FileMode(0700)
	fileMode := os.FileMode(0600)
	if !s.private {
		directoryMode = 0770
		fileMode = 0660
	}
	info, err := os.Lstat(s.ledgerDirectory())
	if err != nil {
		return nil, err
	}
	if err = s.validateSQLiteLedgerDirectory(info, directoryMode); err != nil {
		return nil, err
	}
	info, err = os.Lstat(s.ledgerPath())
	if err != nil {
		return nil, err
	}
	if err = s.validateSQLiteLedgerFile(info, fileMode); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", s.ledgerPath())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
func (s *SQLiteStore) lookupLedger(ctx context.Context, issuer string, version int, serial string) (*LedgerRecord, error) {
	db, err := s.openLedgerReadOnly()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data []byte
	err = db.QueryRowContext(ctx, `SELECT document FROM ledger WHERE issuer=? AND issuer_version=? AND serial=?`, issuer, version, serial).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &NotFoundError{Resource: "ledger certificate"}
	}
	if err != nil {
		return nil, err
	}
	var r LedgerRecord
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, corrupt("malformed ledger record", err)
	}
	if err = validateLedgerRecord(&r); err != nil {
		return nil, err
	}
	return &r, nil
}
func (s *SQLiteStore) listLedger(ctx context.Context, cursor string, limit int) ([]LedgerRecord, string, error) {
	db, err := s.openLedgerReadOnly()
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	offset := 0
	if cursor != "" {
		offset, err = strconv.Atoi(cursor)
		if err != nil || offset < 0 {
			return nil, "", invalid("invalid ledger cursor", err)
		}
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `SELECT document FROM ledger ORDER BY rowid LIMIT ? OFFSET ?`, limit+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []LedgerRecord
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, "", err
		}
		var r LedgerRecord
		if err = json.Unmarshal(data, &r); err != nil {
			return nil, "", corrupt("malformed ledger record", err)
		}
		if err = validateLedgerRecord(&r); err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = strconv.Itoa(offset + limit)
	}
	return out, next, rows.Err()
}
func (s *SQLiteStore) openLedgerReadOnly() (*sql.DB, error) {
	directory := s.ledgerDirectory()
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	expectedDirectoryMode := os.FileMode(0700)
	expectedFileMode := os.FileMode(0600)
	if !s.private {
		expectedDirectoryMode = 0770
		expectedFileMode = 0660
	}
	if err = s.validateSQLiteLedgerDirectory(info, expectedDirectoryMode); err != nil {
		return nil, err
	}
	path := s.ledgerPath()
	info, err = os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err = s.validateSQLiteLedgerFile(info, expectedFileMode); err != nil {
		return nil, err
	}
	dsn := sqliteReadOnlyDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
func (s *SQLiteStore) purge(ctx context.Context) error {
	if s.readOnly {
		return &ForbiddenError{Message: "SQLite authority was opened read-only"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	paths := []string{s.path, s.path + "-journal", s.unlockPath(), s.ledgerPath(), s.ledgerPath() + "-journal"}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(s.ledgerDirectory()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}
func sqliteReadOnlyDSN(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String() + "?mode=ro"
}
