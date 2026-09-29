// Package zap bridges log/slog to go.uber.org/zap.
//
// The bridge delegates to go.uber.org/zap/exp/zapslog, which implements the
// full slog.Handler contract: groups are nested, empty attributes are dropped,
// slog.LogValuer values are resolved, and the record time and source are
// preserved. See docs/adr/0001-zap-backend-and-library-direction.md.
package zap

import (
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

// ZapHandler is the slog.Handler returned by NewHandler.
//
// Deprecated: use slog.Handler. ZapHandler is kept as an alias so existing
// references keep compiling.
type ZapHandler = zapslog.Handler

var _ slog.Handler = (*ZapHandler)(nil)

// noStacktrace disables stack traces: no slog level reaches it.
const noStacktrace = slog.Level(1<<31 - 1)

type options struct {
	caller       bool
	stacktraceAt slog.Level
}

// Option configures NewHandler.
type Option func(*options)

// WithCaller controls whether records carry the caller (file and line) taken
// from slog.Record.PC. It defaults to true; the caller is only printed when the
// Zap encoder configures a CallerKey.
func WithCaller(enabled bool) Option {
	return func(o *options) { o.caller = enabled }
}

// WithStacktraceAt records a stack trace for records at or above level. It
// defaults to slog.LevelError, matching zap.NewProduction.
func WithStacktraceAt(level slog.Level) Option {
	return func(o *options) { o.stacktraceAt = level }
}

// WithoutStacktrace disables stack traces.
func WithoutStacktrace() Option {
	return WithStacktraceAt(noStacktrace)
}

// NewHandler returns a slog.Handler that writes to logger.
//
// It writes through logger.Core(), so the core's level, encoder, outputs,
// hooks and pre-bound fields (logger.With) apply, and the logger name is kept.
// Caller and stack trace settings of the zap.Logger itself (zap.AddCaller,
// zap.AddStacktrace) are not visible through the core; configure them with
// WithCaller and WithStacktraceAt instead.
//
// Levels map by range, the same way for Enabled and Handle:
//
//	slog level < Info          -> zap Debug
//	Info  <= slog level < Warn  -> zap Info
//	Warn  <= slog level < Error -> zap Warn
//	slog level >= Error         -> zap Error
func NewHandler(logger *zap.Logger, opts ...Option) slog.Handler {
	o := options{caller: true, stacktraceAt: slog.LevelError}
	for _, opt := range opts {
		opt(&o)
	}

	return zapslog.NewHandler(
		logger.Core(),
		zapslog.WithName(logger.Name()),
		zapslog.WithCaller(o.caller),
		zapslog.AddStacktraceAt(o.stacktraceAt),
	)
}
