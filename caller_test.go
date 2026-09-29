package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"runtime"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

func TestHelpersReportTheCallSite(t *testing.T) {
	helpers := []struct {
		name string
		log  func(context.Context, string, ...any)
	}{
		{"Info", logger.Info},
		{"Warn", logger.Warn},
		{"Error", logger.Error},
		{"Debug", logger.Debug},
	}
	for _, helper := range helpers {
		t.Run(helper.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug})
			ctx := logger.With(logger.WithContext(context.Background(), slog.New(h)), "request_id", "r1")

			_, file, line, _ := runtime.Caller(0)
			helper.log(ctx, "msg") // must be reported as this line

			var record struct {
				Source    slog.Source `json:"source"`
				RequestID string      `json:"request_id"`
			}
			if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
				t.Fatalf("decode %q: %v", buf.String(), err)
			}
			if record.Source.File != file || record.Source.Line != line+1 {
				t.Errorf("source = %s:%d, want %s:%d", record.Source.File, record.Source.Line, file, line+1)
			}
			if record.RequestID != "r1" {
				t.Errorf("request_id = %q, want attributes from logger.With", record.RequestID)
			}
		})
	}
}

func BenchmarkHelperEnabled(b *testing.B) {
	ctx := logger.WithContext(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Info(ctx, "msg", "a", 1, "b", "x")
	}
}

func BenchmarkHelperDisabled(b *testing.B) {
	ctx := logger.WithContext(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.Debug(ctx, "msg", "a", 1, "b", "x")
	}
}

func BenchmarkSlogDirectEnabled(b *testing.B) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "msg", "a", 1, "b", "x")
	}
}
