package logger

import (
	"context"
	"log/slog"
	"testing"
)

// mockHandler is a test handler that captures log records
type mockHandler struct {
	records []slog.Record
}

func (m *mockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (m *mockHandler) Handle(ctx context.Context, r slog.Record) error {
	m.records = append(m.records, r)
	return nil
}

func (m *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m
}

func (m *mockHandler) WithGroup(name string) slog.Handler {
	return m
}

func TestSetLogger(t *testing.T) {
	ctx := context.Background()
	handler := &mockHandler{}

	newCtx := SetLogger(ctx, handler)

	// Verify that the context has a logger
	logger := FromContext(newCtx)
	if logger == nil {
		t.Error("Expected logger to be set in context")
	}

	// Verify that the default logger was set
	if slog.Default() == nil {
		t.Error("Expected default logger to be set")
	}
}

func TestInfo(t *testing.T) {
	handler := &mockHandler{}
	ctx := SetLogger(context.Background(), handler)

	Info(ctx, "test message", "key", "value")

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]
	if record.Level != slog.LevelInfo {
		t.Errorf("Expected level Info, got %v", record.Level)
	}
	if record.Message != "test message" {
		t.Errorf("Expected message 'test message', got %q", record.Message)
	}
}

func TestWarn(t *testing.T) {
	handler := &mockHandler{}
	ctx := SetLogger(context.Background(), handler)

	Warn(ctx, "warning message", "key", "value")

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]
	if record.Level != slog.LevelWarn {
		t.Errorf("Expected level Warn, got %v", record.Level)
	}
	if record.Message != "warning message" {
		t.Errorf("Expected message 'warning message', got %q", record.Message)
	}
}

func TestError(t *testing.T) {
	handler := &mockHandler{}
	ctx := SetLogger(context.Background(), handler)

	Error(ctx, "error message", "key", "value")

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]
	if record.Level != slog.LevelError {
		t.Errorf("Expected level Error, got %v", record.Level)
	}
	if record.Message != "error message" {
		t.Errorf("Expected message 'error message', got %q", record.Message)
	}
}

func TestDebug(t *testing.T) {
	handler := &mockHandler{}
	ctx := SetLogger(context.Background(), handler)

	Debug(ctx, "debug message", "key", "value")

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]
	if record.Level != slog.LevelDebug {
		t.Errorf("Expected level Debug, got %v", record.Level)
	}
	if record.Message != "debug message" {
		t.Errorf("Expected message 'debug message', got %q", record.Message)
	}
}

func TestLoggingWithAttributes(t *testing.T) {
	handler := &mockHandler{}
	ctx := SetLogger(context.Background(), handler)

	Info(ctx, "test with attrs", "string_attr", "value", "int_attr", 42)

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]

	// Check that attributes are present
	attrCount := 0
	record.Attrs(func(a slog.Attr) bool {
		attrCount++
		return true
	})

	if attrCount != 2 {
		t.Errorf("Expected 2 attributes, got %d", attrCount)
	}
}
