package actor

type Receiver interface {
	Receive(ctx *Context, msg any)
}

type Producer func() Receiver

type Engine struct {
	Registry    *Registry
	address     string
	eventStream *PID
}

type EngineConfig struct {
	cfg
}
