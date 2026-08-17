package service

import (
	"context"
	"github.com/gxmmx/compage-go/errx"
	"os"
)

func (o *operation) ensure(c context.Context) (EnsureResult, error) {
	if e := c.Err(); e != nil {
		return EnsureResult{}, errx.New("service: ensure cancelled", errx.WithCause(e))
	}
	if _, e := o.files.stat(o.spec.binary); e != nil {
		return EnsureResult{}, errx.New("service: inspecting binary", errx.WithCause(e))
	}
	var r EnsureResult
	if o.spec.account != nil {
		a, e := o.ensureAccount(c, *o.spec.account)
		r.Account = &a
		if a.Created || len(a.Changed) > 0 {
			r.Changed = true
			r.Reasons = append(r.Reasons, AccountChanged)
		}
		if e != nil {
			return r, errx.New("service: ensuring runtime account", errx.WithCause(e))
		}
	}
	x, e := o.backend.ensure(c, o)
	x.Account = r.Account
	if r.Changed {
		x.Changed = true
		x.Reasons = append(x.Reasons, r.Reasons...)
	}
	return x, e
}

func (o *operation) start(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: start cancelled", errx.WithCause(err))
	}
	return o.backend.start(c, o)
}
func (o *operation) stop(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: stop cancelled", errx.WithCause(err))
	}
	return o.backend.stop(c, o)
}
func (o *operation) uninstall(c context.Context) error {
	if err := c.Err(); err != nil {
		return errx.New("service: uninstall cancelled", errx.WithCause(err))
	}
	return o.backend.uninstall(c, o)
}
func (o *operation) status(c context.Context) (Status, error) {
	if err := c.Err(); err != nil {
		return Status{}, errx.New("service: status cancelled", errx.WithCause(err))
	}
	return o.backend.status(c, o)
}

var _ = os.ErrNotExist
