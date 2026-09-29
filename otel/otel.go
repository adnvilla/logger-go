// Package otel correlates logs with OpenTelemetry traces.
//
// It is a separate module (github.com/adnvilla/logger-go/otel) so that services
// that do not use OpenTelemetry do not depend on it. It adds the active span's
// trace_id, span_id and trace_flags, as named by the OpenTelemetry log data
// model and docs/schema.md, to every record logged with that context.
package otel

import (
	"context"
	"log/slog"

	logger "github.com/adnvilla/logger-go"
	"go.opentelemetry.io/otel/trace"
)

// SpanContext is a logger.ContextExtractor that returns trace_id, span_id and
// trace_flags for the valid span context carried by ctx, or nil when there is
// none.
func SpanContext(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String(logger.KeyTraceID, sc.TraceID().String()),
		slog.String(logger.KeySpanID, sc.SpanID().String()),
		slog.String(logger.KeyTraceFlags, sc.TraceFlags().String()),
	}
}

// NewHandler wraps next with logger.NewContextHandler and the SpanContext
// extractor: records get the trace fields at the top level, followed by the
// attributes carried with logger.WithAttrs.
func NewHandler(next slog.Handler) slog.Handler {
	return logger.NewContextHandler(next, SpanContext)
}
