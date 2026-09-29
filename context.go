package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

var loggerKey = contextKey{}

// WithContext returns a copy of ctx that carries l. It does not change the
// process-wide default logger.
//
// A nil ctx is treated as context.Background(). A nil l is ignored: ctx is
// returned unchanged, so FromContext keeps resolving to the logger it resolved
// to before.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext returns the logger carried by ctx, or slog.Default() when ctx is
// nil or carries none. It never returns nil.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// With returns a copy of ctx carrying FromContext(ctx).With(args...). A nil ctx
// is treated as context.Background().
func With(ctx context.Context, args ...any) context.Context {
	return WithContext(ctx, FromContext(ctx).With(args...))
}
