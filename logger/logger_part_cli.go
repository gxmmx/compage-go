package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	cmpstl "github.com/gxmmx/compage-go/style"
)

// -----------------------------------------------------------------------------
// Handler
// -----------------------------------------------------------------------------

type cliHandler struct {
	level     *slog.LevelVar
	attrs     []slog.Attr
	outWriter io.Writer
	errWriter io.Writer
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func (ctl *Controller) newCliLogger() {
	ctl.logger = slog.New(ctl.newCliLogHandler())
}

func (c *Controller) newCliLogHandler() slog.Handler {
	return &cliHandler{
		level:     c.level,
		attrs:     []slog.Attr{},
		outWriter: c.outWriter,
		errWriter: c.errWriter,
	}
}

// -----------------------------------------------------------------------------
// Handler methods
// -----------------------------------------------------------------------------

func (h *cliHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *cliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &cliHandler{
		level:     h.level,
		attrs:     newAttrs,
		outWriter: h.outWriter,
		errWriter: h.errWriter,
	}
}

func (h *cliHandler) WithGroup(_ string) slog.Handler {
	return h
}

// -----------------------------------------------------------------------------
// Handle
// -----------------------------------------------------------------------------

func (h *cliHandler) Handle(_ context.Context, r slog.Record) error {
	indent := 0
	out := h.outWriter
	style := ""
	msg := r.Message

	switch r.Level {
	case slog.LevelDebug:
		style = "blackb"
	case slog.LevelInfo:
		style = ""
	case slog.LevelWarn:
		style = "yellow"
		out = h.errWriter
	case slog.LevelError:
		style = "red"
		out = h.errWriter
	}

	// Combine handler and record attrs
	allAttrs := append([]slog.Attr{}, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, a)
		return true
	})

	extraAttrs := []slog.Attr{}

	// Extract known attributes and collect extra
	for _, a := range allAttrs {
		switch a.Key {
		case "indent":
			switch v := a.Value.Any().(type) {
			case int64:
				indent = int(v)
			case float64:
				indent = int(v)
			}
		case "style":
			if v, ok := a.Value.Any().(string); ok {
				style = v
			}
		default:
			extraAttrs = append(extraAttrs, a)
		}
	}

	indentation := strings.Repeat(IndentString, indent)

	fmt.Fprintf(out, "%s\n", cmpstl.Apply(indentation+msg, style))

	// Dump extra attributes
	for _, a := range extraAttrs {
		a.Value = a.Value.Resolve()
		argline := fmt.Sprintf("%s%s: %v", indentation+IndentString, a.Key, a.Value.Any())
		fmt.Fprintf(out, "%s\n", cmpstl.Apply(argline, style))
	}

	return nil
}
