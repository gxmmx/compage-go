package console

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

const hintPrefix = "console."

const (
	hintIndent  = "console.indent"
	hintSuccess = "console.success"
)

// printerHandler implements slog.Handler by routing records through a printer's
// existing methods. It is the backend for Printer.Slog().
type printerHandler struct {
	p     *printer
	attrs []slog.Attr
	hints hintSet
	group string
}

type hintSet struct {
	indent  int
	success bool
}

var _ slog.Handler = (*printerHandler)(nil)

func (h *printerHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.p.level.Level()
}

func (h *printerHandler) Handle(_ context.Context, r slog.Record) error {
	hints := h.hints
	var dataAttrs []string

	for _, a := range h.attrs {
		dataAttrs = append(dataAttrs, h.formatAttr(a))
	}

	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		if strings.HasPrefix(key, hintPrefix) {
			switch key {
			case hintIndent:
				hints.indent += int(a.Value.Int64())
			case hintSuccess:
				hints.success = a.Value.Bool()
			}
		} else {
			dataAttrs = append(dataAttrs, fmt.Sprintf("%s=%s", key, a.Value.String()))
		}
		return true
	})

	msg := r.Message
	if len(dataAttrs) > 0 {
		msg += " " + strings.Join(dataAttrs, " ")
	}

	target := h.p
	if hints.indent > 0 {
		target = target.withIndent(hints.indent)
	}

	switch {
	case r.Level >= slog.LevelError:
		target.Error("%s", msg)
	case r.Level >= slog.LevelWarn:
		target.Warn("%s", msg)
	case hints.success && r.Level >= slog.LevelInfo:
		target.Success("%s", msg)
	case r.Level >= slog.LevelInfo:
		target.Info("%s", msg)
	default:
		target.Verbose("%s", msg)
	}
	return nil
}

func (h *printerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var dataAttrs []slog.Attr
	hints := h.hints

	for _, a := range attrs {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		if strings.HasPrefix(key, hintPrefix) {
			switch key {
			case hintIndent:
				hints.indent += int(a.Value.Int64())
			case hintSuccess:
				hints.success = true
			}
		} else {
			if h.group != "" {
				a.Key = h.group + "." + a.Key
			}
			dataAttrs = append(dataAttrs, a)
		}
	}

	combined := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(dataAttrs))
	copy(combined, h.attrs)
	combined = append(combined, dataAttrs...)

	return &printerHandler{
		p:     h.p,
		attrs: combined,
		hints: hints,
		group: h.group,
	}
}

func (h *printerHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &printerHandler{
		p:     h.p,
		attrs: h.attrs,
		hints: h.hints,
		group: newGroup,
	}
}

func (h *printerHandler) formatAttr(a slog.Attr) string {
	return fmt.Sprintf("%s=%s", a.Key, a.Value.String())
}
