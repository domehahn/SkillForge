package observability

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// NewLogger creates a structured JSON logger
func NewLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext retrieves the request ID from context
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// LoggerWithContext returns a logger with context fields
func LoggerWithContext(logger *slog.Logger, ctx context.Context) *slog.Logger {
	if id := RequestIDFromContext(ctx); id != "" {
		return logger.With("request_id", id)
	}
	return logger
}
