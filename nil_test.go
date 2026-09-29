package logger

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

// useDefault installs a JSON default logger writing to the returned buffer and
// restores the previous default when the test ends.
func useDefault(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf
}

func TestFromContextNeverReturnsNil(t *testing.T) {
	useDefault(t)
	var nilCtx context.Context

	tests := map[string]context.Context{
		"nil context":         nilCtx,
		"empty context":       context.Background(),
		"typed nil logger":    context.WithValue(context.Background(), loggerKey, (*slog.Logger)(nil)),
		"wrong type in ctx":   context.WithValue(context.Background(), loggerKey, "not a logger"),
		"nil via WithContext": WithContext(context.Background(), nil),
	}
	for name, ctx := range tests {
		t.Run(name, func(t *testing.T) {
			if got := FromContext(ctx); got != slog.Default() {
				t.Errorf("FromContext = %p, want slog.Default() %p", got, slog.Default())
			}
		})
	}
}

func TestWithContextNilArguments(t *testing.T) {
	l := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	t.Run("nil context is treated as Background", func(t *testing.T) {
		var nilCtx context.Context
		ctx := WithContext(nilCtx, l)
		if ctx == nil || FromContext(ctx) != l {
			t.Fatalf("WithContext(nil, l) did not carry l")
		}
	})

	t.Run("nil logger keeps the parent logger", func(t *testing.T) {
		parent := WithContext(context.Background(), l)
		ctx := WithContext(parent, nil)
		if ctx != parent {
			t.Error("WithContext(ctx, nil) must return ctx unchanged")
		}
		if FromContext(ctx) != l {
			t.Error("the parent logger must still be resolved")
		}
	})
}

func TestNilContextInHelpers(t *testing.T) {
	buf := useDefault(t)
	var nilCtx context.Context

	ctx := With(nilCtx, "request_id", "r1")
	Info(ctx, "with")
	Info(nilCtx, "helper")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d records, want 2:\n%s", len(lines), buf)
	}
	first := decodeJSONRecord(t, bytes.NewBuffer(lines[0]))
	assertJSONField(t, first, "msg", "with")
	assertJSONField(t, first, "request_id", "r1")
	assertJSONField(t, decodeJSONRecord(t, bytes.NewBuffer(lines[1])), "msg", "helper")
}

func TestSetLoggerNilArguments(t *testing.T) {
	useDefault(t)

	t.Run("nil context", func(t *testing.T) {
		var nilCtx context.Context
		h := slog.NewJSONHandler(&bytes.Buffer{}, nil)
		ctx := SetLogger(nilCtx, h)
		if ctx == nil || FromContext(ctx).Handler() != h || slog.Default().Handler() != h {
			t.Fatal("SetLogger(nil, h) must set the default and return a context carrying it")
		}
	})

	t.Run("nil handler panics like slog.New", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("SetLogger(ctx, nil) did not panic")
			}
		}()
		SetLogger(context.Background(), nil)
	})
}
