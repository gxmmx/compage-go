package core

import (
	"context"
	"log/slog"
	"sync"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Unit interface {
	Name() string
	GetLogger() *slog.Logger
	GetConfig(sub string, unmarshal any) error
	GetConfigString(key string) string
	GetConfigInt(key string) int
	GetConfigBool(key string) bool
	GetCtx() context.Context
	GetEnd() context.CancelFunc
	SetHandle(handle any)
	EndApp(rc AppReturnCode)

	// Internal methods
	setApp(app ApplicationRuntime)
	setCtx(ctx context.Context)
	setEnd(end context.CancelFunc)
	getHandle() (any, bool)
	getApp() Application
	getKind() UnitKind
	run(unit Unit) error
}

type unitSink interface {
	get() (value any, isset bool)
	set(value any)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type AppUnit struct {
	app ApplicationRuntime
	ctx context.Context
	end context.CancelFunc

	name    string
	kind    UnitKind
	runFunc UnitFunc
	handle  unitSink
}

type UnitFunc func(unit Unit) error

type sink struct {
	mu    sync.RWMutex
	data  any
	isSet bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewUnit(name string, kind UnitKind, run UnitFunc) *AppUnit {
	return &AppUnit{
		app: nil,
		ctx: nil,
		end: nil,

		handle: newUnitSink(),

		name:    name,
		kind:    kind,
		runFunc: run,
	}
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (u *AppUnit) Name() string {
	return u.name
}

func (u *AppUnit) GetLogger() *slog.Logger {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.GetLogger()
}

func (u *AppUnit) GetConfig(sub string, m any) error {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.GetConfig(sub, m)
}

func (u *AppUnit) GetConfigString(key string) string {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.getConfigString(key)
}

func (u *AppUnit) GetConfigInt(key string) int {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.getConfigInt(key)
}

func (u *AppUnit) GetConfigBool(key string) bool {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.getConfigBool(key)
}

func (u *AppUnit) GetCtx() context.Context {
	if u.ctx == nil {
		return context.Background()
	}
	return u.ctx
}
func (u *AppUnit) GetEnd() context.CancelFunc {
	if u.end == nil {
		return func() {}
	}
	return u.end
}

func (u *AppUnit) SetHandle(handle any) {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	u.handle.set(handle)
}

func (u *AppUnit) EndApp(rc AppReturnCode) {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	u.app.End(rc)
}

// -----------------------------------------------------------------------------
// Internal Unit methods
// -----------------------------------------------------------------------------

func (u *AppUnit) setApp(app ApplicationRuntime) {
	u.app = app
}
func (u *AppUnit) setCtx(ctx context.Context) {
	u.ctx = ctx
}
func (u *AppUnit) setEnd(end context.CancelFunc) {
	u.end = end
}

func (u *AppUnit) getHandle() (any, bool) {
	return u.handle.get()
}

func (u *AppUnit) getApp() Application {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.(Application)
}

func (u *AppUnit) getKind() UnitKind {
	return u.kind
}

func (u *AppUnit) run(unit Unit) error {
	return u.runFunc(unit)
}

// -----------------------------------------------------------------------------
// Internal Sink methods
// -----------------------------------------------------------------------------

// Set stores a typed value safely
func (s *sink) set(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = v
	s.isSet = true
}

// Get retrieves the typed value safely
func (s *sink) get() (value any, isset bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.isSet
}
