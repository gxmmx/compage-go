package context

import (
	"context"
	"fmt"
)

type contextKey string

const unitKey contextKey = "unit"

// Returns a new context with unit string
func New(name string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, unitKey, name)
	return ctx, cancel
}

// ChildUnitContext returns a new context with unit appended like "parent:child"
func Child(ctx context.Context, unit string) (context.Context, context.CancelFunc) {
	parentUnit := GetUnitName(ctx)
	var newUnit string
	if parentUnit != "" {
		newUnit = fmt.Sprintf("%s:%s", parentUnit, unit)
	} else {
		newUnit = unit
	}
	childCtx, cancel := context.WithCancel(ctx)
	return context.WithValue(childCtx, unitKey, newUnit), cancel
}

// UnitFromContext returns the current unit string (or empty)
func GetUnitName(ctx context.Context) string {
	val, _ := ctx.Value(unitKey).(string)
	return val
}
