package core

import (
	"fmt"
	"time"

	apperrors "github.com/gxmmx/compage-go/errors"
)

func WaitForHandle[T any](caller Unit, name string) (T, error) {
	var zero T
	if caller == nil || name == "" {
		return zero, apperrors.Internal(nil, "app is not initialized or referenced handle name is empty")
	}
	ctx := caller.GetCtx()
	app := caller.getApp()
	appt, ok := app.(*App)
	if !ok {
		return zero, apperrors.Internal(nil, "app is not initialized")
	}
	u, ok := appt.units.get(name)
	if !ok {
		return zero, apperrors.Internal(nil, fmt.Sprintf("unit %s not found", name))
	}

	for {
		select {
		case <-ctx.Done():
			return zero, nil
		default:
			if handle, ok := u.getHandle(); ok {
				if val, ok := handle.(T); ok {
					return val, nil
				} else {
					return zero, apperrors.Internal(nil, "handle is not of correct type")
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
