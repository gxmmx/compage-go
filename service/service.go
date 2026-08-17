package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gxmmx/compage-go/account"
	"github.com/gxmmx/compage-go/host"
)

type Scope uint8

const (
	User Scope = iota
	System
)

type RestartPolicy uint8

const (
	RestartNever RestartPolicy = iota
	RestartOnFailure
	RestartAlways
)

type ChangeReason string

const (
	Missing            ChangeReason = "missing"
	BinaryChanged      ChangeReason = "binary_changed"
	ArgumentsChanged   ChangeReason = "arguments_changed"
	EnvironmentChanged ChangeReason = "environment_changed"
	LogsChanged        ChangeReason = "logs_changed"
	RevisionChanged    ChangeReason = "revision_changed"
	DefinitionDrift    ChangeReason = "definition_drift"
	AccountChanged     ChangeReason = "account_changed"
)

type EnsureResult struct {
	Installed       bool
	Changed         bool
	Reasons         []ChangeReason
	RestartRequired bool
	Account         *account.EnsureResult
}

type definitionMetadata struct {
	Description string
	Binary      string
	Args        []string
	Environment map[string]string
	Account     string
	Stdout      string
	Stderr      string
	Revision    string
	Restart     RestartPolicy
}

func metadataFor(s specification) definitionMetadata {
	a := ""
	if s.account != nil {
		a = s.account.Name
	}
	return definitionMetadata{Description: s.description, Binary: s.binary, Args: s.args, Environment: s.env, Account: a, Stdout: s.stdout, Stderr: s.stderr, Revision: s.revision, Restart: s.restart}
}
func metadataComment(s specification) string {
	raw, _ := json.Marshal(metadataFor(s))
	return base64.RawURLEncoding.EncodeToString(raw)
}
func changeReasons(old []byte, s specification, changed, missing bool) []ChangeReason {
	if !changed {
		return nil
	}
	if missing {
		return []ChangeReason{Missing}
	}
	marker := "compage-spec: "
	i := strings.Index(string(old), marker)
	if i < 0 {
		return []ChangeReason{DefinitionDrift}
	}
	value := string(old)[i+len(marker):]
	if j := strings.IndexAny(value, "\r\n "); j >= 0 {
		value = value[:j]
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return []ChangeReason{DefinitionDrift}
	}
	var previous definitionMetadata
	if json.Unmarshal(raw, &previous) != nil {
		return []ChangeReason{DefinitionDrift}
	}
	wanted := metadataFor(s)
	var out []ChangeReason
	if previous.Binary != wanted.Binary {
		out = append(out, BinaryChanged)
	}
	if !slices.Equal(previous.Args, wanted.Args) {
		out = append(out, ArgumentsChanged)
	}
	if !maps.Equal(previous.Environment, wanted.Environment) {
		out = append(out, EnvironmentChanged)
	}
	if previous.Stdout != wanted.Stdout || previous.Stderr != wanted.Stderr {
		out = append(out, LogsChanged)
	}
	if previous.Account != wanted.Account {
		out = append(out, AccountChanged)
	}
	if previous.Revision != wanted.Revision {
		out = append(out, RevisionChanged)
	}
	if len(out) == 0 || previous.Description != wanted.Description {
		out = append(out, DefinitionDrift)
	}
	return out
}

type Status struct {
	Installed bool
	Enabled   bool
	Loaded    bool
	Running   bool
	PID       int
	ExitCode  *int
	Detail    string
}

type specification struct {
	name, description, binary, stdout, stderr, revision string
	args                                                []string
	env                                                 map[string]string
	scope                                               Scope
	restart                                             RestartPolicy
	account                                             *account.Spec
}
type Option func(*specification) error

func WithName(v string) Option { return func(s *specification) error { s.name = v; return nil } }
func WithDescription(v string) Option {
	return func(s *specification) error { s.description = v; return nil }
}
func WithBinary(v string) Option { return func(s *specification) error { s.binary = v; return nil } }
func WithArgs(v ...string) Option {
	return func(s *specification) error { s.args = append([]string(nil), v...); return nil }
}
func WithEnvironment(v map[string]string) Option {
	return func(s *specification) error { s.env = mapsClone(v); return nil }
}
func WithScope(v Scope) Option { return func(s *specification) error { s.scope = v; return nil } }
func WithRestartPolicy(v RestartPolicy) Option {
	return func(s *specification) error { s.restart = v; return nil }
}
func WithAccount(v account.Spec) Option {
	return func(s *specification) error { copy := v; s.account = &copy; return nil }
}
func WithStdoutLog(v string) Option { return func(s *specification) error { s.stdout = v; return nil } }
func WithStderrLog(v string) Option { return func(s *specification) error { s.stderr = v; return nil } }
func WithLog(v string) Option {
	return func(s *specification) error { s.stdout, s.stderr = v, v; return nil }
}
func WithRevision(v string) Option {
	return func(s *specification) error { s.revision = v; return nil }
}

type Manager struct{ op operation }

func New(opts ...Option) (*Manager, error) {
	s := specification{scope: User, restart: RestartOnFailure}
	for _, opt := range opts {
		if opt == nil {
			return nil, &ValidationError{Message: "nil option"}
		}
		if err := opt(&s); err != nil {
			return nil, err
		}
	}
	p := host.Platform()
	b, err := platformBackend(p.OS)
	if err != nil {
		return nil, err
	}
	if s.name == "" {
		s.name = filepath.Base(s.binary)
	}
	if err := validate(s); err != nil {
		return nil, err
	}
	if err := b.validate(s); err != nil {
		return nil, err
	}
	if s.scope == System && !host.IsRoot() {
		return nil, &PrivilegeError{Capability: "manage system service"}
	}
	u, err := host.User()
	if err != nil {
		return nil, err
	}
	return &Manager{op: operation{spec: s, platform: p, user: u, root: host.IsRoot(), backend: b, runner: execRunner{}, files: osFiles{}, ensureAccount: account.Ensure}}, nil
}
func (m *Manager) Ensure(c context.Context) (EnsureResult, error) { return m.op.ensure(c) }
func (m *Manager) Start(c context.Context) error                  { return m.op.start(c) }
func (m *Manager) Stop(c context.Context) error                   { return m.op.stop(c) }
func (m *Manager) Uninstall(c context.Context) error              { return m.op.uninstall(c) }
func (m *Manager) Status(c context.Context) (Status, error)       { return m.op.status(c) }
func validate(s specification) error {
	if s.scope != User && s.scope != System {
		return &ValidationError{Message: "unknown scope"}
	}
	if s.restart != RestartNever && s.restart != RestartOnFailure && s.restart != RestartAlways {
		return &ValidationError{Message: "unknown restart policy"}
	}
	if s.binary == "" || !filepath.IsAbs(s.binary) || filepath.Clean(s.binary) != s.binary {
		return &ValidationError{Message: "binary must be a clean absolute path"}
	}
	if strings.HasPrefix(s.binary, "/tmp/") || strings.Contains(s.binary, "/.dist/") {
		return &ValidationError{Message: "binary must not be in a temporary or build-output path"}
	}
	if !validServiceName(s.name) {
		return &ValidationError{Message: "invalid service name"}
	}
	if strings.ContainsAny(s.description, "\x00\r\n") || strings.ContainsAny(s.revision, "\x00\r\n") {
		return &ValidationError{Message: "description and revision must not contain control line breaks"}
	}
	if s.scope == User && s.account != nil {
		return &ValidationError{Message: "account requires system scope"}
	}
	for _, p := range []string{s.stdout, s.stderr} {
		if p != "" && (!filepath.IsAbs(p) || filepath.Clean(p) != p) {
			return &ValidationError{Message: "log path must be a clean absolute path"}
		}
	}
	for k, v := range s.env {
		if !validEnv(k) || strings.ContainsAny(v, "\x00\r\n") {
			return &ValidationError{Message: "invalid environment"}
		}
	}
	for _, arg := range s.args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return &ValidationError{Message: "arguments must not contain control line breaks"}
		}
	}
	return nil
}
func validServiceName(v string) bool {
	if v == "" || v == "." || v == ".." || strings.HasPrefix(v, "-") {
		return false
	}
	for _, r := range v {
		if !(r == '.' || r == '-' || r == '_' || r == ':' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
func validEnv(v string) bool {
	if v == "" {
		return false
	}
	for i, r := range v {
		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
func mapsClone(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	o := make(map[string]string, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}
