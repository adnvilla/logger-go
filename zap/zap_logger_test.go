package zap

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
	"time"

	logger "github.com/adnvilla/logger-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// jsonLogger returns a zap.Logger that writes JSON to a buffer, and a func that
// decodes every record written so far.
func jsonLogger(t *testing.T, level zapcore.Level) (*zap.Logger, func() []map[string]any) {
	t.Helper()

	var buf bytes.Buffer
	cfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(cfg), zapcore.AddSync(&buf), level)

	return zap.New(core), func() []map[string]any {
		t.Helper()
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("decode %q: %v", line, err)
			}
			records = append(records, record)
		}
		return records
	}
}

func onlyRecord(t *testing.T, records []map[string]any) map[string]any {
	t.Helper()
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %v", len(records), records)
	}
	return records[0]
}

func assertJSON(t *testing.T, got any, want string) {
	t.Helper()
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("bad expectation %q: %v", want, err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(wantValue)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("got %s, want %s", gotJSON, wantJSON)
	}
}

func TestNewHandlerImplementsSlogHandler(t *testing.T) {
	var _ slog.Handler = NewHandler(zap.NewNop())
	var _ *ZapHandler = NewHandler(zap.NewNop()).(*ZapHandler)
}

func TestLevelsMapByRangeForEnabledAndHandle(t *testing.T) {
	// Levels below Debug, between standard levels and above Error.
	levels := []slog.Level{-8, slog.LevelDebug, -2, slog.LevelInfo, 2, slog.LevelWarn, 6, slog.LevelError, 12, 50}
	mapped := func(l slog.Level) zapcore.Level {
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

	for _, threshold := range []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel} {
		for _, level := range levels {
			t.Run(threshold.String()+"/"+level.String(), func(t *testing.T) {
				logger, records := jsonLogger(t, threshold)
				h := NewHandler(logger, WithoutStacktrace())

				want := mapped(level) >= threshold
				if got := h.Enabled(context.Background(), level); got != want {
					t.Fatalf("Enabled(%v) = %v, want %v", level, got, want)
				}

				// A record accepted by Enabled must be emitted, at the same level.
				slog.New(h).Log(context.Background(), level, "msg")
				got := records()
				if !want {
					if len(got) != 0 {
						t.Fatalf("disabled level emitted %v", got)
					}
					return
				}
				if lvl := onlyRecord(t, got)["level"]; lvl != mapped(level).String() {
					t.Errorf("level = %v, want %v", lvl, mapped(level))
				}
			})
		}
	}
}

func TestAttributeKinds(t *testing.T) {
	logger, records := jsonLogger(t, zapcore.DebugLevel)
	at := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)

	slog.New(NewHandler(logger)).Info("kinds",
		slog.String("s", "v"),
		slog.Int("i", -1),
		slog.Uint64("u", 2),
		slog.Float64("f", 1.5),
		slog.Bool("b", true),
		slog.Duration("d", time.Second),
		slog.Time("t", at),
		slog.Any("m", map[string]int{"k": 1}),
	)

	r := onlyRecord(t, records())
	assertJSON(t, []any{r["s"], r["i"], r["u"], r["f"], r["b"], r["d"], r["t"], r["m"]},
		`["v", -1, 2, 1.5, true, "1s", "2001-02-03T04:05:06Z", {"k": 1}]`)
}

func TestLoggerNameAndPreboundFieldsAreKept(t *testing.T) {
	logger, records := jsonLogger(t, zapcore.DebugLevel)
	logger = logger.Named("checkout").With(zap.String("service", "api"))

	slog.New(NewHandler(logger)).With("version", "1.0").Info("ready")

	r := onlyRecord(t, records())
	assertJSON(t, []any{r["logger"], r["service"], r["version"]}, `["checkout", "api", "1.0"]`)
}

func TestGroupsAndAttributes(t *testing.T) {
	tests := []struct {
		name string
		log  func(*slog.Logger)
		key  string
		want string
	}{
		{
			name: "WithGroup nests later attributes",
			log:  func(l *slog.Logger) { l.WithGroup("request").Info("m", "id", 7, "path", "/x") },
			key:  "request",
			want: `{"id": 7, "path": "/x"}`,
		},
		{
			name: "WithGroup then With nests bound attributes",
			log:  func(l *slog.Logger) { l.WithGroup("request").With("id", 7).Info("m", "path", "/x") },
			key:  "request",
			want: `{"id": 7, "path": "/x"}`,
		},
		{
			name: "nested slog.Group",
			log: func(l *slog.Logger) {
				l.Info("m", slog.Group("http", slog.Int("status", 200), slog.Group("req", slog.String("method", "GET"))))
			},
			key:  "http",
			want: `{"status": 200, "req": {"method": "GET"}}`,
		},
		{
			name: "sibling groups from one parent stay independent",
			log: func(l *slog.Logger) {
				parent := l.WithGroup("a").WithGroup("b").WithGroup("c")
				x := parent.WithGroup("x")
				_ = parent.WithGroup("y")
				x.Info("m", "k", 1)
			},
			key:  "a",
			want: `{"b": {"c": {"x": {"k": 1}}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, records := jsonLogger(t, zapcore.DebugLevel)
			tt.log(slog.New(NewHandler(logger)))

			r := onlyRecord(t, records())
			assertJSON(t, r[tt.key], tt.want)
			if name, ok := r["logger"]; ok {
				t.Errorf("groups must not set the logger name, got %v", name)
			}
		})
	}
}

func TestEmptyGroupsAndAttributesFollowSlogRules(t *testing.T) {
	tests := []struct {
		name string
		log  func(*slog.Logger)
		want string
	}{
		{"WithGroup(\"\") is a no-op", func(l *slog.Logger) { l.WithGroup("").Info("m", "k", "v") }, `{"k": "v"}`},
		{"empty-key group is inlined", func(l *slog.Logger) { l.Info("m", slog.Group("", slog.Int("a", 1))) }, `{"a": 1}`},
		{"named empty group is omitted", func(l *slog.Logger) { l.Info("m", slog.Group("empty"), "k", "v") }, `{"k": "v"}`},
		{"empty Attr is ignored", func(l *slog.Logger) { l.Info("m", slog.Attr{}, "k", "v") }, `{"k": "v"}`},
		{"WithGroup without attributes adds nothing", func(l *slog.Logger) { l.WithGroup("g").Info("m") }, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, records := jsonLogger(t, zapcore.DebugLevel)
			tt.log(slog.New(NewHandler(logger, WithCaller(false))))

			r := onlyRecord(t, records())
			for _, key := range []string{"level", "ts", "msg"} {
				delete(r, key)
			}
			assertJSON(t, r, tt.want)
		})
	}
}

type secret string

func (secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

type user struct{ id, token string }

func (u user) LogValue() slog.Value {
	return slog.GroupValue(slog.String("id", u.id), slog.Any("token", secret(u.token)))
}

func TestLogValuerIsResolved(t *testing.T) {
	logger, records := jsonLogger(t, zapcore.DebugLevel)

	slog.New(NewHandler(logger)).With("password", secret("hunter2")).Info("login", "user", user{"u1", "t0k3n"})

	r := onlyRecord(t, records())
	assertJSON(t, []any{r["password"], r["user"]}, `["[REDACTED]", {"id": "u1", "token": "[REDACTED]"}]`)
	if raw, _ := json.Marshal(r); strings.Contains(string(raw), "hunter2") || strings.Contains(string(raw), "t0k3n") {
		t.Errorf("secret leaked: %s", raw)
	}
}

func TestRecordTimeIsPreserved(t *testing.T) {
	logger, records := jsonLogger(t, zapcore.DebugLevel)
	h := NewHandler(logger)
	at := time.Date(2001, 2, 3, 4, 5, 6, 7, time.UTC)

	if err := h.Handle(context.Background(), slog.NewRecord(at, slog.LevelInfo, "old", 0)); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(context.Background(), slog.NewRecord(time.Time{}, slog.LevelInfo, "zero", 0)); err != nil {
		t.Fatal(err)
	}

	got := records()
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0]["ts"] != "2001-02-03T04:05:06.000000007Z" {
		t.Errorf("ts = %v, want the record time", got[0]["ts"])
	}
	if ts, ok := got[1]["ts"]; ok {
		t.Errorf("zero record time must be omitted, got ts = %v", ts)
	}
}

func TestCallerComesFromTheRecord(t *testing.T) {
	t.Run("direct slog call", func(t *testing.T) {
		logger, records := jsonLogger(t, zapcore.DebugLevel)
		l := slog.New(NewHandler(logger))

		_, file, line, _ := runtime.Caller(0)
		l.Info("here") // must be reported as this line

		want := file + ":" + strconv.Itoa(line+1)
		if got := onlyRecord(t, records())["caller"]; got != want {
			t.Errorf("caller = %v, want %v", got, want)
		}
	})

	t.Run("zero PC has no caller", func(t *testing.T) {
		logger, records := jsonLogger(t, zapcore.DebugLevel)
		_ = NewHandler(logger).Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0))
		if got, ok := onlyRecord(t, records())["caller"]; ok {
			t.Errorf("caller = %v, want none", got)
		}
	})

	t.Run("WithCaller(false)", func(t *testing.T) {
		logger, records := jsonLogger(t, zapcore.DebugLevel)
		slog.New(NewHandler(logger, WithCaller(false))).Info("m")
		if got, ok := onlyRecord(t, records())["caller"]; ok {
			t.Errorf("caller = %v, want none", got)
		}
	})
}

func TestStacktraceOptions(t *testing.T) {
	tests := []struct {
		name      string
		opts      []Option
		level     slog.Level
		wantStack bool
	}{
		{"default at Error", nil, slog.LevelError, true},
		{"default not at Warn", nil, slog.LevelWarn, false},
		{"WithStacktraceAt(Warn)", []Option{WithStacktraceAt(slog.LevelWarn)}, slog.LevelWarn, true},
		{"WithoutStacktrace", []Option{WithoutStacktrace()}, slog.LevelError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, records := jsonLogger(t, zapcore.DebugLevel)
			slog.New(NewHandler(logger, tt.opts...)).Log(context.Background(), tt.level, "m")

			_, hasStack := onlyRecord(t, records())["stacktrace"]
			if hasStack != tt.wantStack {
				t.Errorf("stacktrace present = %v, want %v", hasStack, tt.wantStack)
			}
		})
	}
}

func discardLogger(level zapcore.Level) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	return zap.New(zapcore.NewCore(enc, zapcore.AddSync(io.Discard), level))
}

func benchmarkHandler(b *testing.B, l *slog.Logger, level slog.Level) {
	b.ReportAllocs()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		l.Log(ctx, level, "msg", "a", 1, "b", "x", slog.Group("g", slog.Int("c", 2)))
	}
}

func BenchmarkHandlerEnabled(b *testing.B) {
	benchmarkHandler(b, slog.New(NewHandler(discardLogger(zapcore.DebugLevel), WithCaller(false))), slog.LevelInfo)
}

func BenchmarkHandlerEnabledWithCaller(b *testing.B) {
	benchmarkHandler(b, slog.New(NewHandler(discardLogger(zapcore.DebugLevel))), slog.LevelInfo)
}

func BenchmarkHandlerDisabled(b *testing.B) {
	benchmarkHandler(b, slog.New(NewHandler(discardLogger(zapcore.InfoLevel))), slog.LevelDebug)
}

func BenchmarkHandlerWithAttrs(b *testing.B) {
	l := slog.New(NewHandler(discardLogger(zapcore.DebugLevel), WithCaller(false))).
		With("svc", "api", "env", "prod", "ver", "1.2.3", "region", "us")
	benchmarkHandler(b, l, slog.LevelInfo)
}

func TestCallerThroughLoggerHelpers(t *testing.T) {
	zapLogger, records := jsonLogger(t, zapcore.DebugLevel)
	ctx := logger.WithContext(context.Background(), slog.New(NewHandler(zapLogger)))

	_, file, line, _ := runtime.Caller(0)
	logger.Info(ctx, "helper") // must be reported as this line

	want := file + ":" + strconv.Itoa(line+1)
	if got := onlyRecord(t, records())["caller"]; got != want {
		t.Errorf("caller = %v, want %v", got, want)
	}
}

func TestContextAttributesThroughTheBridge(t *testing.T) {
	zapLogger, records := jsonLogger(t, zapcore.DebugLevel)
	l := slog.New(logger.NewContextHandler(NewHandler(zapLogger, WithCaller(false))))
	ctx := logger.WithAttrs(context.Background(), slog.String("request_id", "r1"))

	l.WithGroup("db").InfoContext(ctx, "query", "rows", 3)

	r := onlyRecord(t, records())
	assertJSON(t, []any{r["request_id"], r["db"]}, `["r1", {"rows": 3}]`)
}

func TestErrorSerializationThroughTheBridge(t *testing.T) {
	zapLogger, records := jsonLogger(t, zapcore.DebugLevel)
	l := slog.New(logger.NewErrorHandler(NewHandler(zapLogger, WithCaller(false), WithoutStacktrace())))

	l.Error("failed", "error", fmt.Errorf("dial: %w", errors.New("refused")))

	r := onlyRecord(t, records())
	assertJSON(t, []any{r["error.type"], r["error.message"]}, `["*fmt.wrapError", "dial: refused"]`)
}
