package logger

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

func SetLogger(ctx context.Context, l slog.Handler) context.Context {
	slog.SetDefault(slog.New(l))
	ctx = WithContext(ctx, slog.Default())
	return ctx
}

func Info(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelInfo, msg, attrs...)
}

func Warn(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelWarn, msg, attrs...)
}

func Error(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelError, msg, attrs...)
}

func Debug(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelDebug, msg, attrs...)
}

// log builds the record itself so that its PC points at the code calling the
// level helper, not at this package. This is the "wrapping output methods"
// pattern documented by log/slog.
func log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	l := FromContext(ctx)
	if !l.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip runtime.Callers, log and the level helper
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(attrs...)
	_ = l.Handler().Handle(ctx, r)
}
