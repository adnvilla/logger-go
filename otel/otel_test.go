package otel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
	loggerotel "github.com/adnvilla/logger-go/otel"
	"go.opentelemetry.io/otel/trace"
)

func spanContext(t *testing.T, sampled bool) trace.SpanContext {
	t.Helper()
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	cfg := trace.SpanContextConfig{TraceID: tid, SpanID: sid}
	if sampled {
		cfg.TraceFlags = trace.FlagsSampled
	}
	return trace.NewSpanContext(cfg)
}

func records(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		delete(r, "time")
		out = append(out, r)
	}
	return out
}

func mustJSON(t *testing.T, got any, want string) {
	t.Helper()
	var w any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	g, _ := json.Marshal(got)
	wj, _ := json.Marshal(w)
	if string(g) != string(wj) {
		t.Errorf("got %s\nwant %s", g, wj)
	}
}

func TestTraceCorrelation(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(loggerotel.NewHandler(slog.NewJSONHandler(&buf, nil)))

	traced := trace.ContextWithSpanContext(context.Background(), spanContext(t, true))
	traced = logger.WithAttrs(traced, slog.String(logger.KeyRequestID, "r1"))
	remote := trace.ContextWithRemoteSpanContext(context.Background(), spanContext(t, false))

	l.WithGroup("db").InfoContext(traced, "query", "rows", 3)
	logger.Info(logger.WithContext(remote, l), "remote parent")
	l.InfoContext(context.Background(), "no span")
	l.InfoContext(trace.ContextWithSpanContext(context.Background(), trace.SpanContext{}), "invalid span")

	mustJSON(t, records(t, &buf), `[
		{"level":"INFO","msg":"query","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7","trace_flags":"01","request_id":"r1","db":{"rows":3}},
		{"level":"INFO","msg":"remote parent","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7","trace_flags":"00"},
		{"level":"INFO","msg":"no span"},
		{"level":"INFO","msg":"invalid span"}
	]`)
}

func TestSpanContextWithoutSpan(t *testing.T) {
	if got := loggerotel.SpanContext(context.Background()); got != nil {
		t.Errorf("SpanContext without a span = %v, want nil", got)
	}
}

func BenchmarkTraceHandler(b *testing.B) {
	l := slog.New(loggerotel.NewHandler(slog.NewJSONHandler(io.Discard, nil)))
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid}))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "msg", "a", 1)
	}
}

func TestProductionTracedRequest(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewProduction(logger.Config{
		ServiceName: "navi-api",
		Output:      &buf,
		Extractors:  []logger.ContextExtractor{loggerotel.SpanContext},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext(t, true))
	ctx = logger.WithAttrs(ctx, slog.String(logger.KeyRequestID, "r1"))

	l.InfoContext(ctx, "http request completed", slog.Int(logger.KeyHTTPResponseStatusCode, 200))

	mustJSON(t, records(t, &buf), `[{"level":"INFO","msg":"http request completed","service.name":"navi-api",
		"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7","trace_flags":"01",
		"request_id":"r1","http.response.status_code":200}]`)
}
