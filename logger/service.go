package logger

import (
	"context"
	"fmt"
	"log/slog"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type LoggerService interface {
	// Add Receive method to satisfy Reeiver interface

	// Satisfy Handler interface
	Enabled(context.Context, slog.Level) bool
	Handle(context.Context, slog.Record) error
	WithAttrs(attrs []slog.Attr) slog.Handler
	WithGroup(group string) slog.Handler
}

// -----------------------------------------------------------------------------
// Examples
// -----------------------------------------------------------------------------

type ExampleLoggerService struct {
	msgPrefix string
}

func NewExampleLoggerService() ExampleLoggerService {
	return ExampleLoggerService{
		msgPrefix: "Message from ExampleLoggerService: ",
	}
}

func (e ExampleLoggerService) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (e ExampleLoggerService) Handle(ctx context.Context, r slog.Record) error {
	// Handle the log record here
	// For example, you can print it to the console
	fmt.Println(e.msgPrefix, r.Message)
	return nil
}

func (e ExampleLoggerService) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Create a new handler with the provided attributes
	return ExampleLoggerService{
		msgPrefix: e.msgPrefix,
	}
}

func (e ExampleLoggerService) WithGroup(group string) slog.Handler {
	// Create a new handler with the provided group
	return ExampleLoggerService{
		msgPrefix: e.msgPrefix,
	}
}
