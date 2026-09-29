package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

func contextJSONLogger(t *testing.T) (*slog.Logger, func() []map[string]any, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: removeVolatileSlogAttrs,
	})))
	return l, func() []map[string]any {
		t.Helper()
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var r map[string]any
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				t.Fatalf("decode %q: %v", line, err)
			}
			records = append(records, r)
		}
		return records
	}, &buf
}

func mustJSON(t *testing.T, got any, want string) {
	t.Helper()
	var w any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad expectation: %v", err)
	}
	g, _ := json.Marshal(got)
	wj, _ := json.Marshal(w)
	if string(g) != string(wj) {
		t.Errorf("got %s\nwant %s", g, wj)
	}
}

func TestContextAttrsReachEveryLoggingPath(t *testing.T) {
	l, records, _ := contextJSONLogger(t)
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))
	ctx = logger.WithAttrs(ctx, slog.String("tenant", "acme"))

	l.InfoContext(ctx, "direct", "k", 1)                      // plain slog
	logger.Info(logger.WithContext(ctx, l), "helper", "k", 2) // level helper
	l.With("svc", "api").InfoContext(ctx, "bound", "k", 3)    // logger.With attributes

	mustJSON(t, records(), `[
		{"level":"INFO","msg":"direct","request_id":"r1","tenant":"acme","k":1},
		{"level":"INFO","msg":"helper","request_id":"r1","tenant":"acme","k":2},
		{"level":"INFO","msg":"bound","svc":"api","request_id":"r1","tenant":"acme","k":3}
	]`)
}

func TestContextAttrsKeepFieldOrder(t *testing.T) {
	l, _, buf := contextJSONLogger(t)
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))

	l.InfoContext(ctx, "m", "a", 1)

	if got := strings.TrimSpace(buf.String()); got != `{"level":"INFO","msg":"m","request_id":"r1","a":1}` {
		t.Errorf("context attributes must precede record attributes, got %s", got)
	}
}

func TestSiblingContextsAreIndependent(t *testing.T) {
	l, records, _ := contextJSONLogger(t)
	parent := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))
	a := logger.WithAttrs(parent, slog.String("step", "a"))
	b := logger.WithAttrs(parent, slog.String("step", "b"))

	l.InfoContext(a, "a")
	l.InfoContext(b, "b")
	l.InfoContext(parent, "parent")

	mustJSON(t, records(), `[
		{"level":"INFO","msg":"a","request_id":"r1","step":"a"},
		{"level":"INFO","msg":"b","request_id":"r1","step":"b"},
		{"level":"INFO","msg":"parent","request_id":"r1"}
	]`)
}

func TestContextAttrsStayTopLevelWithGroups(t *testing.T) {
	l, records, _ := contextJSONLogger(t)
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))

	l.WithGroup("db").InfoContext(ctx, "query", "rows", 3)
	l.With("svc", "api").WithGroup("g").With("a", 1).WithGroup("h").InfoContext(ctx, "nested", "k", "v")
	l.WithGroup("db").InfoContext(context.Background(), "no context attrs", "rows", 0)

	mustJSON(t, records(), `[
		{"level":"INFO","msg":"query","request_id":"r1","db":{"rows":3}},
		{"level":"INFO","msg":"nested","svc":"api","request_id":"r1","g":{"a":1,"h":{"k":"v"}}},
		{"level":"INFO","msg":"no context attrs","db":{"rows":0}}
	]`)
}

func TestGroupedSiblingHandlersAreIndependent(t *testing.T) {
	l, records, _ := contextJSONLogger(t)
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))

	parent := l.WithGroup("a").WithGroup("b").WithGroup("c")
	x := parent.WithGroup("x")
	_ = parent.WithGroup("y")
	x.InfoContext(ctx, "m", "k", 1)

	mustJSON(t, records(), `[{"level":"INFO","msg":"m","request_id":"r1","a":{"b":{"c":{"x":{"k":1}}}}}]`)
}

func TestContextAttrsEdgeCases(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		var nilCtx context.Context
		ctx := logger.WithAttrs(nilCtx, slog.Int("a", 1))
		if got := logger.AttrsFromContext(ctx); len(got) != 1 {
			t.Fatalf("AttrsFromContext = %v, want one attribute", got)
		}
		if logger.AttrsFromContext(nilCtx) != nil {
			t.Error("AttrsFromContext(nil) must be nil")
		}
	})

	t.Run("no attributes returns ctx unchanged", func(t *testing.T) {
		ctx := context.Background()
		if logger.WithAttrs(ctx) != ctx {
			t.Error("WithAttrs(ctx) without attributes must return ctx")
		}
	})

	t.Run("wrapping twice is a no-op", func(t *testing.T) {
		h := logger.NewContextHandler(slog.NewJSONHandler(io.Discard, nil))
		if logger.NewContextHandler(h) != h {
			t.Error("NewContextHandler must not wrap a context handler twice")
		}
	})

	t.Run("nil handler panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("NewContextHandler(nil) did not panic")
			}
		}()
		logger.NewContextHandler(nil)
	})

	t.Run("Enabled delegates", func(t *testing.T) {
		h := logger.NewContextHandler(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
		if h.Enabled(context.Background(), slog.LevelInfo) || !h.Enabled(context.Background(), slog.LevelWarn) {
			t.Error("Enabled must follow the wrapped handler")
		}
	})
}

func TestContextHandlerConcurrentUse(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})
	l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(w, nil))).WithGroup("g")
	base := logger.WithAttrs(context.Background(), slog.String("svc", "api"))

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := logger.WithAttrs(base, slog.Int("worker", i))
			for j := 0; j < 50; j++ {
				l.With("j", j).InfoContext(ctx, "m")
			}
		}(i)
	}
	wg.Wait()

	if n := strings.Count(buf.String(), `"svc":"api","worker":`); n != 16*50 {
		t.Errorf("got %d records with context attributes at the top level, want %d", n, 16*50)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func benchmarkContextHandler(b *testing.B, l *slog.Logger, ctx context.Context) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "msg", "a", 1, "b", "x")
	}
}

func BenchmarkContextHandlerNoAttrs(b *testing.B) {
	l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(io.Discard, nil)))
	benchmarkContextHandler(b, l, context.Background())
}

func BenchmarkContextHandlerWithAttrs(b *testing.B) {
	l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(io.Discard, nil)))
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"), slog.String("tenant", "acme"))
	benchmarkContextHandler(b, l, ctx)
}

func BenchmarkContextHandlerWithAttrsAndGroup(b *testing.B) {
	l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(io.Discard, nil))).WithGroup("db")
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"), slog.String("tenant", "acme"))
	benchmarkContextHandler(b, l, ctx)
}

func BenchmarkJSONHandlerBaseline(b *testing.B) {
	benchmarkContextHandler(b, slog.New(slog.NewJSONHandler(io.Discard, nil)), context.Background())
}

func TestContextExtractors(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: removeVolatileSlogAttrs})
	type key struct{}
	fromKey := func(ctx context.Context) []slog.Attr {
		if v, ok := ctx.Value(key{}).(string); ok {
			return []slog.Attr{slog.String("trace_id", v)}
		}
		return nil
	}
	static := func(context.Context) []slog.Attr { return []slog.Attr{slog.String("zone", "z1")} }

	h := logger.NewContextHandler(logger.NewContextHandler(base, fromKey), static) // extractors accumulate
	l := slog.New(h)
	ctx := logger.WithAttrs(context.WithValue(context.Background(), key{}, "t1"), slog.String("request_id", "r1"))

	l.WithGroup("db").InfoContext(ctx, "q", "rows", 1)
	l.InfoContext(context.Background(), "no trace", "k", 1)

	var got []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	mustJSON(t, got, `[
		{"level":"INFO","msg":"q","trace_id":"t1","zone":"z1","request_id":"r1","db":{"rows":1}},
		{"level":"INFO","msg":"no trace","zone":"z1","k":1}
	]`)
}
