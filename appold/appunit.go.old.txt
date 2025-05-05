package app

import (
	"context"
	"log/slog"
	"time"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ApplicationUnit interface {
	Name() string
	GetLogger() *slog.Logger
	GetConfig(sub string, m any) error
	GetApp() *App
	GetCtx() context.Context
	GetEnd() context.CancelFunc
	GetSink(ctx context.Context, unit string) (any, bool)
	SetSink(value any)
	setApp(app *App)
	setCtx(ctx context.Context)
	setEnd(end context.CancelFunc)
	run(ApplicationUnit)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type UnitFunc func(unit ApplicationUnit)

type AppUnit struct {
	app *App
	ctx context.Context
	end context.CancelFunc

	name    string
	runFunc UnitFunc
	sink    Sink
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewUnit(name string, run UnitFunc) *AppUnit {
	return &AppUnit{
		app: nil,
		ctx: nil,
		end: nil,

		sink: *NewSink(),

		name:    name,
		runFunc: run,
	}
}

// -----------------------------------------------------------------------------
// Methods
// -----------------------------------------------------------------------------

func (u *AppUnit) run(unit ApplicationUnit) {
	u.runFunc(unit)
}

func (u *AppUnit) Name() string {
	return u.name
}

func (u *AppUnit) GetLogger() *slog.Logger {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.logger.GetLogger()
}

func (u *AppUnit) GetConfig(sub string, m any) error {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app.config.GetConfig(sub, m)
}

func (u *AppUnit) GetApp() *App {
	if u.app == nil {
		panic("Unit is not initialized with an app definition")
	}
	return u.app
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

func (u *AppUnit) setApp(app *App) {
	u.app = app
}
func (u *AppUnit) setCtx(ctx context.Context) {
	u.ctx = ctx
}
func (u *AppUnit) setEnd(end context.CancelFunc) {
	u.end = end
}

func (u *AppUnit) GetSink(ctx context.Context, unit string) (any, bool) {
	// TODO: - wait for sink to be set
	if unit == u.name {
		for {
			select {
			case <-ctx.Done():
				return nil, false
			default:
				snk, ok := u.sink.get()
				if ok {
					return snk, ok
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
	if ou, ok := u.app.units[unit]; ok {
		return ou.GetSink(ctx, ou.Name())
	}
	return nil, false
}

func (u *AppUnit) SetSink(value any) {
	u.sink.set(value)
}
