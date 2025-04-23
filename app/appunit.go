package app

import (
	"context"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ApplicationUnit interface {
	Name() string
	GetCtx() context.Context
	GetEnd() context.CancelFunc
	setApp(app *App)
	setCtx(ctx context.Context)
	setEnd(end context.CancelFunc)
	run(*App)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type UnitFunc func(app *App)

type AppUnit[T any] struct {
	app *App
	ctx context.Context
	end context.CancelFunc

	name    string
	runFunc UnitFunc
	sink    Sink[T]
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewUnit[T any](name string, run UnitFunc) *AppUnit[T] {
	return &AppUnit[T]{
		app: nil,
		ctx: nil,
		end: nil,

		sink: *NewSink[T](),

		name:    name,
		runFunc: run,
	}
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (u *AppUnit[T]) run(app *App) {
	u.runFunc(app)
}

func (u *AppUnit[T]) Name() string {
	return u.name
}

func (u *AppUnit[T]) GetCtx() context.Context {
	if u.ctx == nil {
		return context.Background()
	}
	return u.ctx
}
func (u *AppUnit[T]) GetEnd() context.CancelFunc {
	if u.end == nil {
		return func() {}
	}
	return u.end
}

func (u *AppUnit[T]) setApp(app *App) {
	u.app = app
}
func (u *AppUnit[T]) setCtx(ctx context.Context) {
	u.ctx = ctx
}
func (u *AppUnit[T]) setEnd(end context.CancelFunc) {
	u.end = end
}

func (u *AppUnit[T]) GetSink() {
	u.sink.get()
}

func (u *AppUnit[T]) SetSink(v T) {
	u.sink.set(v)
}
