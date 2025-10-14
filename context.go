package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

var loggerKey = contextKey{}

func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(loggerKey).(*slog.Logger)

	if !ok {
		return slog.Default()
	}

	return l
}

func With(ctx context.Context, args ...any) context.Context {
	l := FromContext(ctx).With(args...)
	return WithContext(ctx, l)
}
