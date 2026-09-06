package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
	loggerzap "github.com/adnvilla/logger-go/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestProductionJSONOutputCompatibility(t *testing.T) {
	want := readGoldenEvents(t)
	tests := []struct {
		name       string
		newHandler func(io.Writer) slog.Handler
	}{
		{
			name: "slog JSON handler",
			newHandler: func(output io.Writer) slog.Handler {
				return slog.NewJSONHandler(output, &slog.HandlerOptions{
					ReplaceAttr: removeVolatileSlogAttrs,
				})
			},
		},
		{
			name: "Zap JSON bridge",
			newHandler: func(output io.Writer) slog.Handler {
				core := zapcore.NewCore(
					zapcore.NewJSONEncoder(testZapEncoderConfig()),
					zapcore.AddSync(output),
					zapcore.DebugLevel,
				)
				return loggerzap.NewHandler(zap.New(core))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			emitCompatibilityFixture(t, tt.newHandler(&output))

			got := decodeNDJSON(t, output.String())
			assertEventsEqual(t, got, want)
		})
	}
}

func TestDevelopmentConsoleOutputCompatibility(t *testing.T) {
	want := readGoldenEvents(t)

	t.Run("slog text handler", func(t *testing.T) {
		var output bytes.Buffer
		handler := slog.NewTextHandler(&output, &slog.HandlerOptions{
			ReplaceAttr: removeVolatileSlogAttrs,
		})
		emitCompatibilityFixture(t, handler)

		lines := nonEmptyLines(output.String())
		if len(lines) != len(want) {
			t.Fatalf("got %d console records, want %d\noutput:\n%s", len(lines), len(want), output.String())
		}
		for i := range want {
			assertSlogTextRecord(t, lines[i], want[i])
		}
	})

	t.Run("Zap console bridge", func(t *testing.T) {
		var output bytes.Buffer
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(testZapEncoderConfig()),
			zapcore.AddSync(&output),
			zapcore.DebugLevel,
		)
		emitCompatibilityFixture(t, loggerzap.NewHandler(zap.New(core)))

		got := decodeZapConsole(t, output.String())
		assertEventsEqual(t, got, want)
	})
}

func emitCompatibilityFixture(t *testing.T, handler slog.Handler) {
	t.Helper()

	// Keep this baseline limited to behavior supported today. Groups, custom
	// levels, context propagation, and source metadata get regression coverage
	// with issues #6 through #9 instead of being characterized as correct here.
	previousDefault := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(previousDefault)
	})

	ctx := logger.SetLogger(context.Background(), handler)
	ctx = logger.With(ctx,
		"service", "checkout",
		"environment", "test",
		"version", "v1.0.0",
		"dd.trace_id", "123456789",
		"dd.span_id", "987654321",
	)

	logger.Info(ctx, "request completed",
		"request_id", "req-123",
		"status", 200,
		"success", true,
	)
	logger.Warn(ctx, "retry scheduled",
		"request_id", "req-456",
		"attempt", 2,
		"success", false,
		"error", errors.New("temporary failure"),
	)
}

func removeVolatileSlogAttrs(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey || attr.Key == slog.SourceKey {
		return slog.Attr{}
	}
	return attr
}

func testZapEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		LevelKey:         "level",
		MessageKey:       "msg",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		ConsoleSeparator: "\t",
	}
}

func readGoldenEvents(t *testing.T) []map[string]any {
	t.Helper()

	path := filepath.Join("testdata", "structured_events.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden events: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("decode golden events: %v", err)
	}
	return events
}

func decodeNDJSON(t *testing.T, output string) []map[string]any {
	t.Helper()

	if output == "" || !strings.HasSuffix(output, "\n") {
		t.Fatalf("JSON output must contain newline-delimited records, got %q", output)
	}

	lines := nonEmptyLines(output)
	events := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode JSON record %d: %v\nrecord: %s", i, err, line)
		}
		events = append(events, event)
	}
	return events
}

func decodeZapConsole(t *testing.T, output string) []map[string]any {
	t.Helper()

	lines := nonEmptyLines(output)
	events := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			t.Fatalf("Zap console record %d has unexpected format: %q", i, line)
		}

		event := map[string]any{
			"level": parts[0],
			"msg":   parts[1],
		}
		if err := json.Unmarshal([]byte(parts[2]), &event); err != nil {
			t.Fatalf("decode Zap console fields for record %d: %v\nrecord: %s", i, err, line)
		}
		events = append(events, event)
	}
	return events
}

func assertSlogTextRecord(t *testing.T, line string, want map[string]any) {
	t.Helper()

	for key, value := range want {
		var encoded string
		switch value := value.(type) {
		case string:
			encoded = value
			if strings.ContainsAny(value, " =\t\n\"") {
				encoded = strconv.Quote(value)
			}
		case float64:
			encoded = strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			encoded = strconv.FormatBool(value)
		default:
			t.Fatalf("unsupported golden value for text assertion: %s=%#v", key, value)
		}

		token := key + "=" + encoded
		if !strings.Contains(line, token) {
			t.Errorf("console record does not contain %q\nrecord: %s", token, line)
		}
	}
}

func assertEventsEqual(t *testing.T, got, want []map[string]any) {
	t.Helper()

	if reflect.DeepEqual(got, want) {
		return
	}

	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	t.Errorf("structured events differ\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
}

func nonEmptyLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}
