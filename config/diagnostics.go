package config

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

type diagBuffer struct {
	records []slog.Record
}

func (d *diagBuffer) log(msg string, args ...any) {
	r := slog.NewRecord(time.Now(), slog.LevelDebug, msg, callerPC())
	r.Add(args...)
	d.records = append(d.records, r)
}

func (d *diagBuffer) flush(target *slog.Logger) {
	h := target.Handler()
	for _, r := range d.records {
		_ = h.Handle(context.Background(), r)
	}
	d.records = nil
}

func callerPC() uintptr {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	return pcs[0]
}
