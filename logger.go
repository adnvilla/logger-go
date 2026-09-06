package logger

import (
	"context"
	"log/slog"
)

func SetLogger(ctx context.Context, l slog.Handler) context.Context {
	slog.SetDefault(slog.New(l))
	ctx = WithContext(ctx, slog.Default())
	return ctx
}

func Info(ctx context.Context, msg string, attrs ...any) {
	FromContext(ctx).InfoContext(ctx, msg, attrs...)
}

func Warn(ctx context.Context, msg string, attrs ...any) {
	FromContext(ctx).WarnContext(ctx, msg, attrs...)
}

func Error(ctx context.Context, msg string, attrs ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, attrs...)
}

func Debug(ctx context.Context, msg string, attrs ...any) {
	FromContext(ctx).DebugContext(ctx, msg, attrs...)
}
