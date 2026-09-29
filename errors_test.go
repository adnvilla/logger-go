package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

func errorJSONLogger(t *testing.T, opts ...logger.ErrorOption) (*slog.Logger, func() map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	l := slog.New(logger.NewErrorHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: removeVolatileSlogAttrs,
	}), opts...))
	return l, func() map[string]any {
		t.Helper()
		var r map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &r); err != nil {
			t.Fatalf("decode %q: %v", buf.String(), err)
		}
		buf.Reset()
		return r
	}
}

type codedError struct{ code int }

func (e *codedError) Error() string { return "code " + strconv.Itoa(e.code) }

type redactedError struct{}

func (redactedError) LogValue() slog.Value { return slog.AnyValue(errors.New("resolved")) }

func TestErrorSerialization(t *testing.T) {
	base := errors.New("connection refused")
	tests := []struct {
		name string
		log  func(*slog.Logger)
		want string
	}{
		{
			name: "error key",
			log:  func(l *slog.Logger) { l.Error("failed", "error", base) },
			want: `{"level":"ERROR","msg":"failed","error.type":"*errors.errorString","error.message":"connection refused"}`,
		},
		{
			name: "err key and custom type",
			log:  func(l *slog.Logger) { l.Warn("failed", "err", &codedError{503}, "k", 1) },
			want: `{"level":"WARN","msg":"failed","error.type":"*logger_test.codedError","error.message":"code 503","k":1}`,
		},
		{
			name: "wrapped error keeps the full chain",
			log:  func(l *slog.Logger) { l.Error("failed", "error", fmt.Errorf("dial: %w", base)) },
			want: `{"level":"ERROR","msg":"failed","error.type":"*fmt.wrapError","error.message":"dial: connection refused"}`,
		},
		{
			name: "joined errors are deterministic",
			log:  func(l *slog.Logger) { l.Error("failed", "error", errors.Join(base, &codedError{1})) },
			want: `{"level":"ERROR","msg":"failed","error.type":"*errors.joinError","error.message":"connection refused\ncode 1"}`,
		},
		{
			name: "bound with Logger.With",
			log:  func(l *slog.Logger) { l.With("error", base).Info("degraded") },
			want: `{"level":"INFO","msg":"degraded","error.type":"*errors.errorString","error.message":"connection refused"}`,
		},
		{
			name: "LogValuer resolving to an error",
			log:  func(l *slog.Logger) { l.Error("failed", "error", redactedError{}) },
			want: `{"level":"ERROR","msg":"failed","error.type":"*errors.errorString","error.message":"resolved"}`,
		},
		{
			name: "other keys and non-error values are untouched",
			log:  func(l *slog.Logger) { l.Info("m", "cause", base, "error", "plain string", "err", nil) },
			want: `{"level":"INFO","msg":"m","cause":"connection refused","error":"plain string","err":null}`,
		},
		{
			name: "error fields follow open groups",
			log:  func(l *slog.Logger) { l.WithGroup("db").Error("failed", "error", base) },
			want: `{"level":"ERROR","msg":"failed","db":{"error.type":"*errors.errorString","error.message":"connection refused"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, record := errorJSONLogger(t)
			tt.log(l)
			mustJSON(t, record(), tt.want)
		})
	}
}

func TestErrorStacktrace(t *testing.T) {
	l, record := errorJSONLogger(t, logger.WithErrorStacktrace(slog.LevelError))

	_, file, line, _ := runtime.Caller(0)
	l.Error("failed", "error", errors.New("boom")) // the stack must start here

	r := record()
	stack, _ := r[logger.KeyExceptionStacktrace].(string)
	first := strings.SplitN(stack, "\n", 3)
	if len(first) < 2 || !strings.HasSuffix(first[0], "TestErrorStacktrace") ||
		strings.TrimSpace(first[1]) != file+":"+strconv.Itoa(line+1) {
		t.Fatalf("stack does not start at the log call:\n%s", stack)
	}
	if frames := strings.Count(stack, "\n\t"); frames > 32 {
		t.Errorf("stack has %d frames, want at most 32", frames)
	}

	l.Warn("below threshold", "error", errors.New("boom"))
	if _, ok := record()[logger.KeyExceptionStacktrace]; ok {
		t.Error("no stack expected below the configured level")
	}
	l.Error("no error attribute")
	if _, ok := record()[logger.KeyExceptionStacktrace]; ok {
		t.Error("no stack expected without an error attribute")
	}
}

func TestErrorHandlerDelegates(t *testing.T) {
	h := logger.NewErrorHandler(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if h.Enabled(context.Background(), slog.LevelInfo) || !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("Enabled must follow the wrapped handler")
	}
	if h.WithGroup("") != h {
		t.Error("WithGroup(\"\") must return the handler")
	}
	defer func() {
		if recover() == nil {
			t.Error("NewErrorHandler(nil) did not panic")
		}
	}()
	logger.NewErrorHandler(nil)
}

func BenchmarkErrorHandlerNoError(b *testing.B) {
	l := slog.New(logger.NewErrorHandler(slog.NewJSONHandler(io.Discard, nil)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Info("msg", "a", 1, "b", "x")
	}
}

func BenchmarkErrorHandlerWithError(b *testing.B) {
	l := slog.New(logger.NewErrorHandler(slog.NewJSONHandler(io.Discard, nil)))
	err := errors.New("boom")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Error("msg", "error", err, "a", 1)
	}
}
