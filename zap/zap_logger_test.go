package zap

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewHandler(t *testing.T) {
	logger := zap.NewNop()
	handler := NewHandler(logger)

	if handler == nil {
		t.Error("Expected handler to be created")
	}

	// Verify that it implements slog.Handler
	var _ slog.Handler = handler
}

func TestZapHandler_Enabled(t *testing.T) {
	core, _ := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	ctx := context.Background()

	tests := []struct {
		level    slog.Level
		expected bool
	}{
		{slog.LevelDebug, false}, // Below InfoLevel
		{slog.LevelInfo, true},   // At InfoLevel
		{slog.LevelWarn, true},   // Above InfoLevel
		{slog.LevelError, true},  // Above InfoLevel
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if enabled := handler.Enabled(ctx, tt.level); enabled != tt.expected {
				t.Errorf("Expected Enabled(%v) = %v, got %v", tt.level, tt.expected, enabled)
			}
		})
	}
}

func TestZapHandler_Handle(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	ctx := context.Background()

	tests := []struct {
		level   slog.Level
		message string
	}{
		{slog.LevelDebug, "debug message"},
		{slog.LevelInfo, "info message"},
		{slog.LevelWarn, "warn message"},
		{slog.LevelError, "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			record := slog.NewRecord(time.Now(), tt.level, tt.message, 0)
			record.AddAttrs(slog.String("key1", "value1"), slog.Int("key2", 42))

			err := handler.Handle(ctx, record)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			// Check that the log was recorded
			entries := logs.All()
			if len(entries) == 0 {
				t.Error("Expected at least one log entry")
				return
			}

			lastEntry := entries[len(entries)-1]
			if lastEntry.Message != tt.message {
				t.Errorf("Expected message %q, got %q", tt.message, lastEntry.Message)
			}

			// Check that attributes were included
			if len(lastEntry.Context) < 2 {
				t.Errorf("Expected at least 2 context fields, got %d", len(lastEntry.Context))
			}
		})
	}
}

func TestZapHandler_HandleWithAttrs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(
		slog.String("string_attr", "string_value"),
		slog.Int("int_attr", 123),
		slog.Bool("bool_attr", true),
	)

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %q", entry.Message)
	}

	// Check that all attributes are present
	expectedFields := map[string]struct {
		stringVal string
		intVal    int64
		boolVal   bool
		fieldType zapcore.FieldType
	}{
		"string_attr": {stringVal: "string_value", fieldType: zapcore.StringType},
		"int_attr":    {intVal: 123, fieldType: zapcore.Int64Type},
		"bool_attr":   {boolVal: true, fieldType: zapcore.BoolType},
	}

	foundFields := 0
	for _, field := range entry.Context {
		if expected, exists := expectedFields[field.Key]; exists {
			foundFields++
			switch expected.fieldType {
			case zapcore.StringType:
				if field.String != expected.stringVal {
					t.Errorf("Expected field %s=%s, got %s", field.Key, expected.stringVal, field.String)
				}
			case zapcore.Int64Type:
				if field.Integer != expected.intVal {
					t.Errorf("Expected field %s=%d, got %d", field.Key, expected.intVal, field.Integer)
				}
			case zapcore.BoolType:
				expectedInt := int64(0)
				if expected.boolVal {
					expectedInt = 1
				}
				if field.Integer != expectedInt {
					t.Errorf("Expected field %s=%t (as %d), got %d", field.Key, expected.boolVal, expectedInt, field.Integer)
				}
			}
		}
	}

	if foundFields != len(expectedFields) {
		t.Errorf("Expected to find %d fields, found %d", len(expectedFields), foundFields)
	}
}

func TestZapHandler_WithAttrs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	attrs := []slog.Attr{
		slog.String("service", "test"),
		slog.String("version", "1.0"),
	}

	newHandler := handler.WithAttrs(attrs)
	if newHandler == handler {
		t.Error("Expected a new handler instance")
	}

	// Test that the new handler includes the attributes
	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)

	err := newHandler.Handle(ctx, record)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]

	// Check that the pre-set attributes are present
	foundService := false
	foundVersion := false
	for _, field := range entry.Context {
		if field.Key == "service" && field.String == "test" {
			foundService = true
		}
		if field.Key == "version" && field.String == "1.0" {
			foundVersion = true
		}
	}

	if !foundService {
		t.Error("Expected to find 'service' attribute")
	}
	if !foundVersion {
		t.Error("Expected to find 'version' attribute")
	}
}

func TestZapHandler_WithGroup(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	groupedHandler := handler.WithGroup("mygroup")
	if groupedHandler == handler {
		t.Error("Expected a new handler instance")
	}

	// Test that the grouped handler works
	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)

	err := groupedHandler.Handle(ctx, record)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.LoggerName != "mygroup" {
		t.Errorf("Expected logger name 'mygroup', got %q", entry.LoggerName)
	}
}

func TestConvertSlogLevel(t *testing.T) {
	tests := []struct {
		slogLevel slog.Level
		zapLevel  zapcore.Level
	}{
		{slog.LevelDebug, zapcore.DebugLevel},
		{slog.LevelInfo, zapcore.InfoLevel},
		{slog.LevelWarn, zapcore.WarnLevel},
		{slog.LevelError, zapcore.ErrorLevel},
		{slog.Level(100), zapcore.ErrorLevel}, // High level should map to Error
	}

	for _, tt := range tests {
		t.Run(tt.slogLevel.String(), func(t *testing.T) {
			zapLevel := convertSlogLevel(tt.slogLevel)
			if zapLevel != tt.zapLevel {
				t.Errorf("Expected convertSlogLevel(%v) = %v, got %v", tt.slogLevel, tt.zapLevel, zapLevel)
			}
		})
	}
}

func TestZapHandler_HandleDefaultLevel(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	handler := NewHandler(logger)

	ctx := context.Background()

	// Test with a custom level that doesn't match standard levels
	customLevel := slog.Level(50) // Between Info and Warn
	record := slog.NewRecord(time.Now(), customLevel, "custom level message", 0)

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "custom level message" {
		t.Errorf("Expected message 'custom level message', got %q", entry.Message)
	}

	// Custom levels should default to Info level in Zap
	if entry.Level != zapcore.InfoLevel {
		t.Errorf("Expected zap level Info for custom slog level, got %v", entry.Level)
	}
}

func TestZapHandler_Interface(t *testing.T) {
	// Test that ZapHandler implements slog.Handler interface
	logger := zap.NewNop()
	handler := NewHandler(logger)

	// This should compile without errors
	var _ slog.Handler = handler

	// Test all required methods exist and can be called
	ctx := context.Background()

	if !handler.Enabled(ctx, slog.LevelInfo) {
		// This test depends on the logger configuration
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	if err := handler.Handle(ctx, record); err != nil {
		t.Errorf("Handle returned error: %v", err)
	}

	newHandler := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	if newHandler == nil {
		t.Error("WithAttrs returned nil")
	}

	groupHandler := handler.WithGroup("group")
	if groupHandler == nil {
		t.Error("WithGroup returned nil")
	}
}
