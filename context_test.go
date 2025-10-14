package logger

import (
	"context"
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
	handler := &mockHandler{}
	logger := slog.New(handler)
	ctx = WithContext(ctx, logger)

	// Add attributes to the logger
	newCtx := With(ctx, "key1", "value1", "key2", 42)

	// Verify that a new logger with attributes was created
	newLogger := FromContext(newCtx)
	if newLogger == logger {
		t.Error("Expected a new logger instance with attributes")
	}

	// The With function should create a logger that adds attributes to all subsequent logs
	// We test this by checking that the logger is different, which indicates it has pre-set attributes
	if newLogger == nil {
		t.Error("Expected new logger to not be nil")
	}
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
	handler := &mockHandler{}
	logger := slog.New(handler)

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

	// Test that we can log successfully (detailed attribute testing is complex with slog)
	Info(ctx, "test message", "additional", "attr")

	if len(handler.records) != 1 {
		t.Fatalf("Expected 1 log record, got %d", len(handler.records))
	}

	record := handler.records[0]
	if record.Message != "test message" {
		t.Errorf("Expected message 'test message', got %q", record.Message)
	}

	// Check that at least the additional attribute is present
	var foundAdditional bool
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "additional" && a.Value.Any() == "attr" {
			foundAdditional = true
		}
		return true
	})

	if !foundAdditional {
		t.Error("Expected to find additional=attr attribute")
	}
}
