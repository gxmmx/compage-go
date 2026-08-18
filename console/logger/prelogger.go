package logger

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// PreLogger captures structured log records before the Logger is initialized.
// It is not safe for concurrent use — startup is expected to be single-goroutine.
type PreLogger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Count(level slog.Level) int
	Flush(target *slog.Logger)
}

// Pre creates a new PreLogger ready to queue records.
func Pre() PreLogger {
	return &preLogger{}
}

type preLogger struct {
	records []slog.Record
	flushed bool
}

func (p *preLogger) Debug(msg string, args ...any) { p.record(slog.LevelDebug, msg, args) }
func (p *preLogger) Info(msg string, args ...any)  { p.record(slog.LevelInfo, msg, args) }
func (p *preLogger) Warn(msg string, args ...any)  { p.record(slog.LevelWarn, msg, args) }
func (p *preLogger) Error(msg string, args ...any) { p.record(slog.LevelError, msg, args) }

func (p *preLogger) record(level slog.Level, msg string, args []any) {
	if p.flushed {
		panic("logger.PreLogger: used after Flush")
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	p.records = append(p.records, r)
}

// Count returns the number of queued records at the given level or above.
func (p *preLogger) Count(level slog.Level) int {
	n := 0
	for _, r := range p.records {
		if r.Level >= level {
			n++
		}
	}
	return n
}

// Flush replays all queued records through the target's handler, preserving
// their original timestamps. After Flush, the PreLogger is spent and any
// subsequent method call will panic.
func (p *preLogger) Flush(target *slog.Logger) {
	if p.flushed {
		panic("logger.PreLogger: Flush called twice")
	}
	p.flushed = true

	if target == nil {
		p.records = nil
		return
	}

	h := target.Handler()
	for _, r := range p.records {
		_ = h.Handle(context.Background(), r)
	}
	p.records = nil
}
