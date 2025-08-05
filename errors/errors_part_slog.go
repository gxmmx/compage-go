package errors

import "log/slog"

// -----------------------------------------------------------------------------
// Controller Slog methods
// -----------------------------------------------------------------------------

func (ctl *Controller) Slog() (string, []any) {
	msg := ctl.Error()

	fieldmap := ctl.Fields()
	fields := make([]slog.Attr, 0, len(fieldmap)+1)
	for k, v := range fieldmap {
		fields = append(fields, slog.Any(k, v))
	}

	errorGroup := slog.Attr{
		Key:   "error",
		Value: ctl.buildErrorGroup(ctl, 0),
	}
	fields = append(fields, errorGroup)
	return msg, slogArgs(fields)
}

// -----------------------------------------------------------------------------
// Controller Slog internal methods
// -----------------------------------------------------------------------------

func (ctl *Controller) buildErrorGroup(err error, depth int) slog.Value {
	if depth >= maxChainDepth {
		return slog.GroupValue(slog.String("msg", "max depth reached"))
	}

	switch e := err.(type) {
	case *Controller:
		attrs := []slog.Attr{
			slog.String("msg", e.msg),
			slog.String("kind", string(e.kind)),
			slog.String("class", e.class),
			slog.String("caller", e.caller),
		}
		if e.err != nil {
			attrs = append(attrs, slog.Attr{
				Key:   "sub",
				Value: ctl.buildErrorGroup(e.err, depth+1),
			})
		}
		return slog.GroupValue(attrs...)
	default:
		var inner error
		if unwrapper, ok := e.(interface{ Unwrap() error }); ok {
			inner = unwrapper.Unwrap()
		}
		attrs := []slog.Attr{slog.String("msg", e.Error())}
		if inner != nil {
			attrs = append(attrs, slog.Attr{
				Key:   "sub",
				Value: ctl.buildErrorGroup(inner, depth+1),
			})
		}
		return slog.GroupValue(attrs...)
	}
}

// -----------------------------------------------------------------------------
// Controller Slog helper methods
// -----------------------------------------------------------------------------

func slogArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}
