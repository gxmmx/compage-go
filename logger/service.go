package logger

import (
	"context"
	"log/slog"
)

type LoggerService interface {
	Start(context.Context) error
	SendRecord(slog.Record)
	GetHandler() slog.Handler
}
