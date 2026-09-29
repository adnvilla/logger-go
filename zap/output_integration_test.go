package zap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	logger "github.com/adnvilla/logger-go"
	loggerzap "github.com/adnvilla/logger-go/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// These tests run the root module's compatibility fixture through the Zap
// bridge. testdata/structured_events.golden.json mirrors the root copy.

func TestProductionJSONOutputCompatibility(t *testing.T) {
	want := readGoldenEvents(t)

	var output bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(testZapEncoderConfig()),
		zapcore.AddSync(&output),
		zapcore.DebugLevel,
	)
	emitCompatibilityFixture(t, loggerzap.NewHandler(zap.New(core)))

	got := decodeNDJSON(t, output.String())
	assertEventsEqual(t, got, want)
}

func TestDevelopmentConsoleOutputCompatibility(t *testing.T) {
	want := readGoldenEvents(t)

	var output bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(testZapEncoderConfig()),
		zapcore.AddSync(&output),
		zapcore.DebugLevel,
	)
	emitCompatibilityFixture(t, loggerzap.NewHandler(zap.New(core)))

	got := decodeZapConsole(t, output.String())
	assertEventsEqual(t, got, want)
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
