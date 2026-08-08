package console

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Logger emits structured JSON log records. It is safe for concurrent use.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	For(unit string) Logger
	WithRequest(id string) Logger
	WithTrace(id string) Logger
	With(args ...any) Logger
	SetLevel(lvl slog.Level)
	Slog() *slog.Logger
}

// logger is the concrete Logger implementation.
type logger struct {
	slogger      *slog.Logger
	handler      slog.Handler
	level        *slog.LevelVar
	unit         string
	hasRequestID bool
	hasTraceID   bool
}

// New creates a Logger. The program argument identifies the application and is
// stamped on every record. If empty, it defaults to the binary name.
//
// New never fails. If a configured file cannot be opened, the logger falls back
// to stderr and emits a warning about the degradation.
func New(program string, opts ...Option) Logger {
	if program == "" {
		program = filepath.Base(os.Args[0])
	}

	cfg := &loggerConfig{
		level: slog.LevelInfo,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	runID := cfg.runID
	if runID == "" {
		runID = generateRunID()
	}

	lvl := &slog.LevelVar{}
	lvl.Set(cfg.level)

	writers, fallbackMsg := resolveWriters(cfg)
	handlers := make([]slog.Handler, 0, len(writers))
	for _, w := range writers {
		handlers = append(handlers, slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: lvl,
		}))
	}

	var inner slog.Handler
	if len(handlers) == 1 {
		inner = handlers[0]
	} else {
		inner = newFanHandler(handlers)
	}

	h := slog.Handler(newContextHandler(inner))
	h = h.WithAttrs([]slog.Attr{
		slog.String("program", program),
		slog.String("unit", program),
		slog.String("run_id", runID),
	})

	l := &logger{
		slogger: slog.New(h),
		handler: h,
		level:   lvl,
		unit:    program,
	}

	if fallbackMsg != "" {
		l.Warn("logger degraded", "reason", fallbackMsg)
	}

	return l
}

func (l *logger) Debug(msg string, args ...any) { l.slogger.Debug(msg, args...) }
func (l *logger) Info(msg string, args ...any)  { l.slogger.Info(msg, args...) }
func (l *logger) Warn(msg string, args ...any)  { l.slogger.Warn(msg, args...) }
func (l *logger) Error(msg string, args ...any) { l.slogger.Error(msg, args...) }

func (l *logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.slogger.DebugContext(ctx, msg, args...)
}
func (l *logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.slogger.InfoContext(ctx, msg, args...)
}
func (l *logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.slogger.WarnContext(ctx, msg, args...)
}
func (l *logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.slogger.ErrorContext(ctx, msg, args...)
}

func (l *logger) SetLevel(lvl slog.Level) {
	l.level.Set(lvl)
}

func (l *logger) For(unit string) Logger {
	nested := l.unit + "." + unit
	h := l.handler.WithAttrs([]slog.Attr{
		slog.String("unit", nested),
	})
	return &logger{
		slogger:      slog.New(h),
		handler:      h,
		level:        l.level,
		unit:         nested,
		hasRequestID: l.hasRequestID,
		hasTraceID:   l.hasTraceID,
	}
}

func (l *logger) WithRequest(id string) Logger {
	h := l.handler.WithAttrs([]slog.Attr{
		slog.String("request_id", id),
	})
	if ch, ok := h.(*contextHandler); ok {
		h = ch.withHasRequestID()
	}
	return &logger{
		slogger:      slog.New(h),
		handler:      h,
		level:        l.level,
		unit:         l.unit,
		hasRequestID: true,
		hasTraceID:   l.hasTraceID,
	}
}

func (l *logger) WithTrace(id string) Logger {
	h := l.handler.WithAttrs([]slog.Attr{
		slog.String("trace_id", id),
	})
	if ch, ok := h.(*contextHandler); ok {
		h = ch.withHasTraceID()
	}
	return &logger{
		slogger:      slog.New(h),
		handler:      h,
		level:        l.level,
		unit:         l.unit,
		hasRequestID: l.hasRequestID,
		hasTraceID:   true,
	}
}

func (l *logger) With(args ...any) Logger {
	s := l.slogger.With(args...)
	return &logger{
		slogger:      s,
		handler:      s.Handler(),
		level:        l.level,
		unit:         l.unit,
		hasRequestID: l.hasRequestID,
		hasTraceID:   l.hasTraceID,
	}
}

func (l *logger) Slog() *slog.Logger {
	return l.slogger
}

func resolveWriters(cfg *loggerConfig) ([]io.Writer, string) {
	var writers []io.Writer
	var fallbackMsg string

	stderrTarget := stderrWriter(cfg)

	if cfg.stderr {
		writers = append(writers, stderrTarget)
	}

	if cfg.filePath != "" {
		f, err := os.OpenFile(cfg.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			fallbackMsg = fmt.Sprintf("cannot open log file %q: %v", cfg.filePath, err)
			if len(writers) == 0 {
				writers = append(writers, stderrTarget)
			}
		} else {
			writers = append(writers, f)
		}
	}

	if len(writers) == 0 {
		writers = append(writers, stderrTarget)
	}

	return writers, fallbackMsg
}

func stderrWriter(cfg *loggerConfig) io.Writer {
	if cfg.fallbackWriter != nil {
		return cfg.fallbackWriter
	}
	return os.Stderr
}

func generateRunID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}
