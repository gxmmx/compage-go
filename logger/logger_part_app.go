package logger

import (
	"context"
	"log/slog"
)

// -----------------------------------------------------------------------------
// handler
// -----------------------------------------------------------------------------

type appHandler struct {
	app        string
	outHandler slog.Handler
	errHandler slog.Handler
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func (ctl *Controller) newAppLogger() {
	ctl.logger = slog.New(ctl.newAppLogHandler())
}

func (c *Controller) newAppLogHandler() slog.Handler {
	return &appHandler{
		app:        c.app,
		outHandler: slog.NewJSONHandler(c.outWriter, appLoggerHandlerOptions(c.level)),
		errHandler: slog.NewJSONHandler(c.errWriter, appLoggerHandlerOptions(c.level)),
	}
}

func appLoggerHandlerOptions(level *slog.LevelVar) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// Format unix timestamp
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.Int64Value(a.Value.Time().UnixMilli()),
				}
			}
			return a
		},
	}
}

// -----------------------------------------------------------------------------
// Handler methods
// -----------------------------------------------------------------------------

func (h *appHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

func (h *appHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &appHandler{
		app:        h.app,
		outHandler: h.outHandler.WithAttrs(attrs),
		errHandler: h.errHandler.WithAttrs(attrs),
	}
}

func (h *appHandler) WithGroup(name string) slog.Handler {
	return h
}

// -----------------------------------------------------------------------------
// Handle
// -----------------------------------------------------------------------------

func (h *appHandler) Handle(ctx context.Context, r slog.Record) error {
	var recordAttrs []any
	var class = "app"
	var unit = h.app
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "class":
			class = a.Value.String()
		case "unit":
			unit = a.Value.String()
		default:
			recordAttrs = append(recordAttrs, a)
		}
		return true
	})

	r = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	// unit := appcontext.GetUnitName(ctx)
	// if unit != "" {
	// 	r.AddAttrs(slog.String("unit", unit))
	// }

	r.AddAttrs(slog.String("class", class), slog.String("unit", unit), slog.Group("data", recordAttrs...))

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
