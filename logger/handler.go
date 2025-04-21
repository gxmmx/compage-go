package logger

import (
	"context"
	"log/slog"
	"os"

	appcontext "github.com/gxmmx/compage-go/ctx"
)

func logHandlerOptions(level *slog.LevelVar) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// Format the time as Unix timestamp in milliseconds (similar to zap)
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.Int64Value(a.Value.Time().UnixMilli()),
				}
			}
			return a
		},
	}
}

type logHandler struct {
	defaultClass string
	outHandler   slog.Handler
	errHandler   slog.Handler
	extHandler   slog.Handler
}

func newLogHandler(level *slog.LevelVar, class string, logsvc LoggerService) slog.Handler {
	var extHandler slog.Handler
	if logsvc != nil {
		extHandler = logsvc.GetHandler()
	}

	stdoutHandler := slog.NewJSONHandler(os.Stdout, logHandlerOptions(level))
	stderrHandler := slog.NewJSONHandler(os.Stderr, logHandlerOptions(level))
	return &logHandler{
		defaultClass: class,
		outHandler:   stdoutHandler,
		errHandler:   stderrHandler,
		extHandler:   extHandler,
	}
}

func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true // Allow all logs
}

func (h *logHandler) Handle(ctx context.Context, r slog.Record) error {

	var recordAttrs []any
	var class = h.defaultClass
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "class" {
			class = a.Value.String()
		} else {
			recordAttrs = append(recordAttrs, a)
		}
		return true
	})

	r = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	unit := appcontext.GetUnitName(ctx)
	if unit != "" {
		r.AddAttrs(slog.String("unit", unit))
	}

	r.AddAttrs(slog.String("class", class), slog.Group("data", recordAttrs...))

	// Send to external handler if provided
	if h.extHandler != nil && h.extHandler.Enabled(ctx, r.Level) {
		h.extHandler.Handle(ctx, r)
	}
	// Send to stdout or stderr based on log level
	if r.Level >= slog.LevelWarn {
		if h.errHandler.Enabled(ctx, r.Level) {
			return h.errHandler.Handle(ctx, r)
		}
	} else {
		if h.outHandler.Enabled(ctx, r.Level) {
			return h.outHandler.Handle(ctx, r)
		}
	}
	return nil
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h.extHandler == nil {
		return &logHandler{
			defaultClass: h.defaultClass,
			outHandler:   h.outHandler.WithAttrs(attrs),
			errHandler:   h.errHandler.WithAttrs(attrs),
			extHandler:   nil,
		}
	}
	return &logHandler{
		defaultClass: h.defaultClass,
		outHandler:   h.outHandler.WithAttrs(attrs),
		errHandler:   h.errHandler.WithAttrs(attrs),
		extHandler:   h.extHandler.WithAttrs(attrs),
	}
}

func (h *logHandler) WithGroup(name string) slog.Handler {
	if h.extHandler == nil {
		return &logHandler{
			defaultClass: h.defaultClass,
			outHandler:   h.outHandler.WithGroup(name),
			errHandler:   h.errHandler.WithGroup(name),
			extHandler:   nil,
		}
	}
	return &logHandler{
		defaultClass: h.defaultClass,
		outHandler:   h.outHandler.WithGroup(name),
		errHandler:   h.errHandler.WithGroup(name),
		extHandler:   h.extHandler.WithGroup(name),
	}
}
