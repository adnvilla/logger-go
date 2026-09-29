package logger

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// SetLogger builds a logger from h, makes it the process-wide default with
// slog.SetDefault, and returns a copy of ctx that carries it.
//
// Deprecated: SetLogger mixes process configuration with context storage.
// Configure the default once at startup and attach loggers to contexts
// explicitly:
//
//	l := slog.New(h)
//	slog.SetDefault(l)                   // process-wide, once
//	ctx = logger.WithContext(ctx, l)     // request or operation scope
//
// SetLogger panics if h is nil, like slog.New.
func SetLogger(ctx context.Context, h slog.Handler) context.Context {
	l := slog.New(h)
	slog.SetDefault(l)
	return WithContext(ctx, l)
}

// Info logs at slog.LevelInfo with the logger carried by ctx (see FromContext).
// A nil ctx is treated as context.Background().
func Info(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelInfo, msg, attrs...)
}

// Warn logs at slog.LevelWarn with the logger carried by ctx (see FromContext).
// A nil ctx is treated as context.Background().
func Warn(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelWarn, msg, attrs...)
}

// Error logs at slog.LevelError with the logger carried by ctx (see FromContext).
// A nil ctx is treated as context.Background().
func Error(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelError, msg, attrs...)
}

// Debug logs at slog.LevelDebug with the logger carried by ctx (see FromContext).
// A nil ctx is treated as context.Background().
func Debug(ctx context.Context, msg string, attrs ...any) {
	log(ctx, slog.LevelDebug, msg, attrs...)
}

// log builds the record itself so that its PC points at the code calling the
// level helper, not at this package. This is the "wrapping output methods"
// pattern documented by log/slog.
func log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
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
