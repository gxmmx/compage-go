package service

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gxmmx/compage-go/account"
	"github.com/gxmmx/compage-go/host"
)

type Scope uint8

const (
	User Scope = iota
	System
)

type ChangeReason string

const (
	Missing         ChangeReason = "missing"
	DefinitionDrift ChangeReason = "definition_drift"
	AccountChanged  ChangeReason = "account_changed"
)

type EnsureResult struct {
	Installed       bool
	Changed         bool
	Reasons         []ChangeReason
	RestartRequired bool
	Account         *account.EnsureResult
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
func WithAccount(v account.Spec) Option {
	return func(s *specification) error { copy := v; s.account = &copy; return nil }
}
func WithStdoutLog(v string) Option { return func(s *specification) error { s.stdout = v; return nil } }
func WithStderrLog(v string) Option { return func(s *specification) error { s.stderr = v; return nil } }
func WithRevision(v string) Option {
	return func(s *specification) error { s.revision = v; return nil }
}

type Manager struct{ op operation }

func New(opts ...Option) (*Manager, error) {
	s := specification{scope: User}
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
	if s.binary == "" || !filepath.IsAbs(s.binary) || filepath.Clean(s.binary) != s.binary {
		return &ValidationError{Message: "binary must be a clean absolute path"}
	}
	if !validServiceName(s.name) {
		return &ValidationError{Message: "invalid service name"}
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
		if !validEnv(k) || strings.ContainsRune(v, 0) {
			return &ValidationError{Message: "invalid environment"}
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
