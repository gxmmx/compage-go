package console

import (
	"context"
	"log/slog"
	"strings"
)

// contextHandler wraps a slog.Handler and extracts known context keys
// (request_id, trace_id) into record attributes before forwarding.
type contextHandler struct {
	inner        slog.Handler
	hasRequestID bool
	hasTraceID   bool
}

func newContextHandler(inner slog.Handler) *contextHandler {
	return &contextHandler{inner: inner}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.hasRequestID {
		if id := RequestIDFrom(ctx); id != "" {
			r.AddAttrs(slog.String("request_id", id))
		}
	}
	if !h.hasTraceID {
		if id := TraceIDFrom(ctx); id != "" {
			r.AddAttrs(slog.String("trace_id", id))
		}
	}

	var filtered slog.Record
	hasHints := false
	r.Attrs(func(a slog.Attr) bool {
		if strings.HasPrefix(a.Key, hintPrefix) {
			hasHints = true
			return false
		}
		return true
	})
	if hasHints {
		filtered = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		r.Attrs(func(a slog.Attr) bool {
			if !strings.HasPrefix(a.Key, hintPrefix) {
				filtered.AddAttrs(a)
			}
			return true
		})
		return h.inner.Handle(ctx, filtered)
	}

	return h.inner.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	filtered := filterHintAttrs(attrs)
	return &contextHandler{
		inner:        h.inner.WithAttrs(filtered),
		hasRequestID: h.hasRequestID,
		hasTraceID:   h.hasTraceID,
	}
}

func filterHintAttrs(attrs []slog.Attr) []slog.Attr {
	n := 0
	for _, a := range attrs {
		if !strings.HasPrefix(a.Key, hintPrefix) {
			n++
		}
	}
	if n == len(attrs) {
		return attrs
	}
	filtered := make([]slog.Attr, 0, n)
	for _, a := range attrs {
		if !strings.HasPrefix(a.Key, hintPrefix) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{
		inner:        h.inner.WithGroup(name),
		hasRequestID: h.hasRequestID,
		hasTraceID:   h.hasTraceID,
	}
}

func (h *contextHandler) withHasRequestID() *contextHandler {
	return &contextHandler{
		inner:        h.inner,
		hasRequestID: true,
		hasTraceID:   h.hasTraceID,
	}
}

func (h *contextHandler) withHasTraceID() *contextHandler {
	return &contextHandler{
		inner:        h.inner,
		hasRequestID: h.hasRequestID,
		hasTraceID:   true,
	}
}

// fanHandler distributes log records to multiple slog.Handlers.
// Each child handler independently formats and writes records.
type fanHandler struct {
	handlers []slog.Handler
}

func newFanHandler(handlers []slog.Handler) *fanHandler {
	return &fanHandler{handlers: handlers}
}

func (h *fanHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, ch := range h.handlers {
		if ch.Enabled(context.Background(), level) {
			return true
		}
	}
	return false
}

func (h *fanHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, ch := range h.handlers {
		if ch.Enabled(ctx, r.Level) {
			_ = ch.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (h *fanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	children := make([]slog.Handler, len(h.handlers))
	for i, ch := range h.handlers {
		children[i] = ch.WithAttrs(attrs)
	}
	return &fanHandler{handlers: children}
}

func (h *fanHandler) WithGroup(name string) slog.Handler {
	children := make([]slog.Handler, len(h.handlers))
	for i, ch := range h.handlers {
		children[i] = ch.WithGroup(name)
	}
	return &fanHandler{handlers: children}
}
