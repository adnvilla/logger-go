package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestWithContext(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(nil, nil))

	newCtx := WithContext(ctx, logger)

	// Verify that the logger was stored in the context
	retrievedLogger := FromContext(newCtx)
	if retrievedLogger != logger {
		t.Error("Expected the same logger instance to be returned from context")
	}
}

func TestFromContext(t *testing.T) {
	t.Run("with logger in context", func(t *testing.T) {
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(nil, nil))
		ctx = WithContext(ctx, logger)

		retrievedLogger := FromContext(ctx)
		if retrievedLogger != logger {
			t.Error("Expected the same logger instance to be returned from context")
		}
	})

	t.Run("without logger in context", func(t *testing.T) {
		ctx := context.Background()

		retrievedLogger := FromContext(ctx)
		if retrievedLogger != slog.Default() {
			t.Error("Expected default logger when no logger in context")
		}
	})

	t.Run("with wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), loggerKey, "not a logger")

		retrievedLogger := FromContext(ctx)
		if retrievedLogger != slog.Default() {
			t.Error("Expected default logger when wrong type in context")
		}
	})
}

func TestWith(t *testing.T) {
	ctx := context.Background()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx = WithContext(ctx, logger)

	// Add attributes to the logger
	newCtx := With(ctx, "key1", "value1", "key2", 42)

	// Verify that a new logger with attributes was created
	newLogger := FromContext(newCtx)
	if newLogger == logger {
		t.Error("Expected a new logger instance with attributes")
	}

	Info(newCtx, "test message")
	record := decodeJSONRecord(t, &output)
	assertJSONField(t, record, "key1", "value1")
	assertJSONField(t, record, "key2", float64(42))
}

func TestContextKey(t *testing.T) {
	// Test that contextKey struct is used correctly
	key1 := contextKey{}
	key2 := contextKey{}

	// Two instances of contextKey should be equal
	if key1 != key2 {
		t.Error("Expected contextKey instances to be equal")
	}

	// Test that the global loggerKey is of the correct type
	ctx := context.WithValue(context.Background(), loggerKey, slog.Default())
	value := ctx.Value(loggerKey)
	if value == nil {
		t.Error("Expected value to be stored with loggerKey")
	}
}

func TestMultipleContextOperations(t *testing.T) {
	ctx := context.Background()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))

	// Set initial logger
	ctx = WithContext(ctx, logger)

	// Add some attributes
	ctx = With(ctx, "service", "test", "version", "1.0")

	// Add more attributes
	ctx = With(ctx, "request_id", "12345")

	// Verify that each With call creates a new logger
	finalLogger := FromContext(ctx)
	if finalLogger == logger {
		t.Error("Expected final logger to be different from original")
	}

	Info(ctx, "test message", "additional", "attr")
	record := decodeJSONRecord(t, &output)
	assertJSONField(t, record, "msg", "test message")
	assertJSONField(t, record, "service", "test")
	assertJSONField(t, record, "version", "1.0")
	assertJSONField(t, record, "request_id", "12345")
	assertJSONField(t, record, "additional", "attr")
}

func decodeJSONRecord(t *testing.T, output *bytes.Buffer) map[string]any {
	t.Helper()

	var record map[string]any
	if err := json.NewDecoder(output).Decode(&record); err != nil {
		t.Fatalf("decode JSON log record: %v", err)
	}

	return record
}

func assertJSONField(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()

	if got := record[key]; got != want {
		t.Errorf("field %q = %#v, want %#v", key, got, want)
	}
}
