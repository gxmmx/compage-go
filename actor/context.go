package actor

import (
	"context"
	"time"

	"github.com/gxmmx/compage-go/safemap"
)

type Context struct {
	pid       *PID
	sender    *PID
	engine    *Engine
	receiver  Receiver
	message   any
	parentCtx *Context
	children  *safemap.SafeMap[string, *PID]
	context   context.Context
}

func newContext(ctx context.Context, e *Engine, pid *PID) *Context {
	return &Context{
		context:  ctx,
		engine:   e,
		pid:      pid,
		children: safemap.New[string, *PID](),
	}
}

func (c *Context) Context() context.Context {
	return c.context
}

func (c *Context) Receiver() Receiver {
	return c.receiver
}

func (c *Context) Request(pid *PID, msg any, timeout time.Duration) *Response {
	return c.engine.Request(pid, msg, timeout)
}
