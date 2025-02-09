package zap

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ slog.Handler = (*ZapHandler)(nil)

type ZapHandler struct {
	logger *zap.Logger
}

func NewHandler(logger *zap.Logger) slog.Handler {
	l := logger.WithOptions(zap.AddCallerSkip(4))
	return &ZapHandler{logger: l}
}

func (h *ZapHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.logger.Core().Enabled(convertSlogLevel(level))
}

func (h *ZapHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := make([]zap.Field, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		fields = append(fields, zap.Any(a.Key, a.Value.Any()))
		return true
	})

	switch r.Level {
	case slog.LevelDebug:
		h.logger.Debug(r.Message, fields...)
	case slog.LevelInfo:
		h.logger.Info(r.Message, fields...)
	case slog.LevelWarn:
		h.logger.Warn(r.Message, fields...)
	case slog.LevelError:
		h.logger.Error(r.Message, fields...)
	default:
		h.logger.Info(r.Message, fields...)
	}

	return nil
}

func (h *ZapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make([]zap.Field, len(attrs))
	for i, attr := range attrs {
		fields[i] = zap.Any(attr.Key, attr.Value.Any())
	}
	newLogger := h.logger.With(fields...)
	return &ZapHandler{logger: newLogger}
}

func (h *ZapHandler) WithGroup(name string) slog.Handler {
	newLogger := h.logger.Named(name)
	return &ZapHandler{logger: newLogger}
}

func convertSlogLevel(l slog.Level) zapcore.Level {
	switch {
	case l >= slog.LevelError:
		return zapcore.ErrorLevel
	case l >= slog.LevelWarn:
		return zapcore.WarnLevel
	case l >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}
