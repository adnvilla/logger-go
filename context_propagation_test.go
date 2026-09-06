package logger_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	logger "github.com/adnvilla/logger-go"
)

type contextProbe struct {
	slog.Handler
	want                      context.Context
	enabledCalls, handleCalls int
	allow                     bool
	t                         *testing.T
	level                     slog.Level
}

func (h *contextProbe) Enabled(ctx context.Context, level slog.Level) bool {
	h.enabledCalls++
	if ctx != h.want {
		h.t.Error("Enabled did not receive the exact caller context")
	}
	if level != h.level {
		h.t.Errorf("Enabled level = %v, want %v", level, h.level)
	}
	return h.allow
}

func (h *contextProbe) Handle(ctx context.Context, r slog.Record) error {
	h.handleCalls++
	if ctx != h.want {
		h.t.Error("Handle did not receive the exact caller context")
	}
	if r.Level != h.level || r.Message != "operation finished" {
		h.t.Errorf("unexpected record: level=%v message=%q", r.Level, r.Message)
	}
	return nil
}

func TestHelpersPropagateContext(t *testing.T) {
	methods := []struct {
		name  string
		level slog.Level
		log   func(context.Context, string, ...any)
	}{
		{"Info", slog.LevelInfo, logger.Info},
		{"Warn", slog.LevelWarn, logger.Warn},
		{"Error", slog.LevelError, logger.Error},
		{"Debug", slog.LevelDebug, logger.Debug},
	}
	for _, method := range methods {
		for _, source := range []string{"context logger", "default logger"} {
			for _, scenario := range []string{"active", "cancelled", "disabled"} {
				t.Run(method.name+"/"+source+"/"+scenario, func(t *testing.T) {
					previous := slog.Default()
					t.Cleanup(func() { slog.SetDefault(previous) })
					ctx, cancel := context.WithCancel(context.Background())
					t.Cleanup(cancel)
					if scenario == "cancelled" {
						cancel()
					}
					h := &contextProbe{Handler: slog.NewJSONHandler(io.Discard, nil), t: t, level: method.level, allow: scenario != "disabled"}
					l := slog.New(h)
					var caller context.Context = ctx
					if source == "context logger" {
						caller = logger.WithContext(caller, l)
					} else {
						slog.SetDefault(l)
					}
					h.want = caller
					method.log(caller, "operation finished")
					if h.enabledCalls != 1 {
						t.Errorf("Enabled calls = %d, want 1", h.enabledCalls)
					}
					wantHandled := 1
					if !h.allow {
						wantHandled = 0
					}
					if h.handleCalls != wantHandled {
						t.Errorf("Handle calls = %d, want %d", h.handleCalls, wantHandled)
					}
				})
			}
		}
	}
}
