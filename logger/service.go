package logger

import (
	"context"
	"log/slog"
)

type LoggerService interface {
	Start(context.Context) error
	Stop() error
	SendRecord(slog.Record)
	GetHandler() slog.Handler
}
