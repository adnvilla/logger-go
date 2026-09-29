package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

var update = flag.Bool("update", false, "rewrite golden files")

// emitProductionFixture logs the representative records of the production
// schema: a request log, a client error, a server error, a redacted login and a
// grouped record with context attributes.
func emitProductionFixture(l *slog.Logger) {
	ctx := logger.WithAttrs(context.Background(), slog.String(logger.KeyRequestID, "req-123"))
	req := []any{
		slog.String(logger.KeyHTTPRequestMethod, "GET"),
		slog.String(logger.KeyHTTPRoute, "/items/:id"),
		slog.String(logger.KeyURLPath, "/items/42"),
		slog.String(logger.KeyClientAddress, "203.0.113.7"),
	}

	l.InfoContext(ctx, "http request completed", append(req,
		slog.Int(logger.KeyHTTPResponseStatusCode, 200),
		slog.Int(logger.KeyHTTPResponseBodySize, 512),
		slog.Int(logger.KeyDurationMS, 12),
	)...)
	l.WarnContext(ctx, "http request completed", append(req,
		slog.Int(logger.KeyHTTPResponseStatusCode, 404),
		slog.Int(logger.KeyDurationMS, 3),
	)...)
	l.ErrorContext(ctx, "item lookup failed",
		"error", fmt.Errorf("query items: %w", errors.New("connection refused")),
		slog.String(logger.KeyEndUserID, "u-1"),
	)
	l.InfoContext(ctx, "login", "user", "u-1", "password", "hunter2",
		slog.Group("headers", slog.String("Authorization", "Bearer abc"), slog.String("accept", "json")))
	l.WithGroup("db").InfoContext(ctx, "query", "rows", 3, "table", "items")
}

func newProductionForTest(buf *bytes.Buffer, cfg logger.Config) *slog.Logger {
	cfg.Output = buf
	if cfg.ServiceName == "" {
		cfg.ServiceName, cfg.ServiceVersion, cfg.Environment = "navi-api", "1.4.0", "production"
	}
	return logger.NewProduction(cfg)
}

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

func TestProductionGolden(t *testing.T) {
	var buf bytes.Buffer
	emitProductionFixture(newProductionForTest(&buf, logger.Config{}))

	got := decodeLines(t, &buf)
	for _, r := range got {
		if _, ok := r["time"].(string); !ok {
			t.Fatalf("record without time: %v", r)
		}
		delete(r, "time")
	}
	if raw := buf.String(); strings.Contains(raw, "hunter2") || strings.Contains(raw, "Bearer abc") {
		t.Fatalf("secret leaked:\n%s", raw)
	}

	path := filepath.Join("testdata", "production.golden.json")
	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	if *update {
		if err := os.WriteFile(path, append(gotJSON, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mustJSON(t, got, string(want))
}

func TestProductionFieldOrder(t *testing.T) {
	var buf bytes.Buffer
	l := newProductionForTest(&buf, logger.Config{})
	l.InfoContext(logger.WithAttrs(context.Background(), slog.String("request_id", "r1")), "m", "k", 1)

	line := buf.String()
	order := []string{`"time"`, `"level"`, `"msg"`, `"service.name"`, `"service.version"`, `"deployment.environment.name"`, `"request_id"`, `"k"`}
	last := -1
	for _, key := range order {
		i := strings.Index(line, key)
		if i < last {
			t.Fatalf("%s out of order in %s", key, line)
		}
		last = i
	}
}

func TestProductionOptions(t *testing.T) {
	t.Run("default level is Info", func(t *testing.T) {
		var buf bytes.Buffer
		l := newProductionForTest(&buf, logger.Config{})
		l.Debug("hidden")
		if buf.Len() != 0 {
			t.Errorf("debug record emitted: %s", buf.String())
		}
	})

	t.Run("level, source and stack trace", func(t *testing.T) {
		var buf bytes.Buffer
		l := newProductionForTest(&buf, logger.Config{Level: slog.LevelDebug, AddSource: true, ErrorStacktrace: true})
		l.Debug("shown")
		l.Error("failed", "error", errors.New("boom"))

		got := decodeLines(t, &buf)
		if len(got) != 2 || got[0]["source"] == nil {
			t.Fatalf("want a debug record with source, got %v", got)
		}
		if s, _ := got[1][logger.KeyExceptionStacktrace].(string); !strings.Contains(s, "TestProductionOptions") {
			t.Errorf("stack trace missing the log call: %q", s)
		}
	})

	t.Run("extra redact keys and extractors", func(t *testing.T) {
		var buf bytes.Buffer
		zone := func(context.Context) []slog.Attr { return []slog.Attr{slog.String("cloud.region", "us-east-1")} }
		l := newProductionForTest(&buf, logger.Config{RedactKeys: []string{"ssn"}, Extractors: []logger.ContextExtractor{zone}})
		l.WithGroup("g").InfoContext(context.Background(), "m", "ssn", "123")

		r := decodeLines(t, &buf)[0]
		mustJSON(t, []any{r["cloud.region"], r["g"]}, `["us-east-1", {"ssn": "[REDACTED]"}]`)
	})

	t.Run("empty service fields are omitted", func(t *testing.T) {
		var buf bytes.Buffer
		logger.NewProduction(logger.Config{Output: &buf, ServiceName: "svc"}).Info("m")
		r := decodeLines(t, &buf)[0]
		if r[logger.KeyServiceName] != "svc" || r[logger.KeyServiceVersion] != nil || r[logger.KeyDeploymentEnvironment] != nil {
			t.Errorf("unexpected resource fields: %v", r)
		}
	})
}

func ExampleNewProduction() {
	l := logger.NewProduction(logger.Config{
		ServiceName:    "navi-api",
		ServiceVersion: "1.4.0",
		Environment:    "production",
		Output:         os.Stdout,
		// Drop the timestamp so the example output is stable.
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	slog.SetDefault(l)

	ctx := logger.WithAttrs(context.Background(), slog.String(logger.KeyRequestID, "req-123"))
	slog.ErrorContext(ctx, "login failed", "user", "u-1", "password", "hunter2", "error", errors.New("bad credentials"))
	// Output:
	// {"level":"ERROR","msg":"login failed","service.name":"navi-api","service.version":"1.4.0","deployment.environment.name":"production","request_id":"req-123","user":"u-1","password":"[REDACTED]","error.type":"*errors.errorString","error.message":"bad credentials"}
}
