package config

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

const maxLogRecords = 256

func newLogRecord(level slog.Level, message string) slog.Record {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	return slog.NewRecord(time.Now(), level, message, pcs[0])
}

// logBuffer retains initialization diagnostics until the caller has a logger.
// It is deliberately not used for runtime operation errors.
type logBuffer struct {
	mu        sync.Mutex
	records   []slog.Record
	discarded int
}

func (b *logBuffer) add(record slog.Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.records) == maxLogRecords {
		b.records = b.records[1:]
		b.discarded++
	}
	b.records = append(b.records, record.Clone())
}

func (b *logBuffer) flush(logger *slog.Logger) {
	if logger == nil {
		return
	}
	b.mu.Lock()
	records, discarded := b.records, b.discarded
	b.records, b.discarded = nil, 0
	b.mu.Unlock()
	if discarded > 0 {
		logger.Warn("config initialization diagnostics discarded", "count", discarded)
	}
	for _, record := range records {
		_ = logger.Handler().Handle(context.Background(), record)
	}
}
