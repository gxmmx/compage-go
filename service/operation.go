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

var _ = os.ErrNotExist
