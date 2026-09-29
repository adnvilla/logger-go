package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

type credentials struct{ user, pass string }

// LogValue returns a group that still contains a sensitive key, as careless
// LogValuer implementations do.
func (c credentials) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", c.user), slog.String("password", c.pass))
}

func redactJSONLogger(t *testing.T, opts ...logger.RedactOption) (*slog.Logger, func() map[string]any, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l := slog.New(logger.NewRedactHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
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
	}, &buf
}

func TestRedaction(t *testing.T) {
	tests := []struct {
		name string
		log  func(*slog.Logger)
		want string
	}{
		{
			name: "default keys, any case, dashes",
			log: func(l *slog.Logger) {
				l.Info("m", "password", "hunter2", "Authorization", "Bearer x", "X-Api-Key", "k", "user_password", "p", "token_count", 3, "user", "u1")
			},
			want: `{"level":"INFO","msg":"m","password":"[REDACTED]","Authorization":"[REDACTED]","X-Api-Key":"[REDACTED]","user_password":"[REDACTED]","token_count":3,"user":"u1"}`,
		},
		{
			name: "last dotted segment",
			log:  func(l *slog.Logger) { l.Info("m", "http.request.header.authorization", "Bearer x") },
			want: `{"level":"INFO","msg":"m","http.request.header.authorization":"[REDACTED]"}`,
		},
		{
			name: "nested groups",
			log: func(l *slog.Logger) {
				l.Info("m", slog.Group("req", slog.Group("headers", slog.String("cookie", "c"), slog.String("accept", "json"))))
			},
			want: `{"level":"INFO","msg":"m","req":{"headers":{"cookie":"[REDACTED]","accept":"json"}}}`,
		},
		{
			name: "LogValuer resolving to a group",
			log:  func(l *slog.Logger) { l.Info("login", "creds", credentials{"u1", "hunter2"}) },
			want: `{"level":"INFO","msg":"login","creds":{"user":"u1","password":"[REDACTED]"}}`,
		},
		{
			name: "bound with Logger.With and under WithGroup",
			log:  func(l *slog.Logger) { l.With("token", "t0k3n").WithGroup("db").Info("m", "secret", "s", "rows", 1) },
			want: `{"level":"INFO","msg":"m","token":"[REDACTED]","db":{"secret":"[REDACTED]","rows":1}}`,
		},
		{
			name: "whole value is replaced, whatever its type",
			log:  func(l *slog.Logger) { l.Info("m", "credentials", map[string]string{"a": "b"}, "token", 12345) },
			want: `{"level":"INFO","msg":"m","credentials":"[REDACTED]","token":"[REDACTED]"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, record, _ := redactJSONLogger(t)
			tt.log(l)
			mustJSON(t, record(), tt.want)
		})
	}
}

func TestRedactKeysAddsToTheDefaults(t *testing.T) {
	l, record, buf := redactJSONLogger(t, logger.RedactKeys("SSN"))
	l.Info("m", "ssn", "123", "X-Api-Key", "k", "password", "p")
	mustJSON(t, record(), `{"level":"INFO","msg":"m","ssn":"[REDACTED]","X-Api-Key":"[REDACTED]","password":"[REDACTED]"}`)
	for _, leaked := range []string{"123", `"k"`, `"p"`} {
		if strings.Contains(buf.String(), leaked) {
			t.Errorf("leaked %s", leaked)
		}
	}
}

func TestRedactHandlerDelegates(t *testing.T) {
	h := logger.NewRedactHandler(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if h.Enabled(context.Background(), slog.LevelInfo) || !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("Enabled must follow the wrapped handler")
	}
	if h.WithGroup("") != h {
		t.Error("WithGroup(\"\") must return the handler")
	}
	defer func() {
		if recover() == nil {
			t.Error("NewRedactHandler(nil) did not panic")
		}
	}()
	logger.NewRedactHandler(nil)
}

func BenchmarkRedactHandlerNothingSensitive(b *testing.B) {
	l := slog.New(logger.NewRedactHandler(slog.NewJSONHandler(io.Discard, nil)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Info("msg", "a", 1, "b", "x", "request_id", "r1")
	}
}

func BenchmarkRedactHandlerSensitive(b *testing.B) {
	l := slog.New(logger.NewRedactHandler(slog.NewJSONHandler(io.Discard, nil)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.Info("msg", "a", 1, "password", "x")
	}
}
