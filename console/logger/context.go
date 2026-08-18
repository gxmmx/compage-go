package logger

import "context"

type ctxKey struct{ name string }

var (
	requestIDKey = ctxKey{"request_id"}
	traceIDKey   = ctxKey{"trace_id"}
)

// WithRequestID returns a child context carrying the given request ID.
// The Logger extracts this automatically when *Context methods are used.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom extracts the request ID from the context, or "" if not set.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithTraceID returns a child context carrying the given trace ID.
// The Logger extracts this automatically when *Context methods are used.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// TraceIDFrom extracts the trace ID from the context, or "" if not set.
func TraceIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey).(string)
	return id
}
