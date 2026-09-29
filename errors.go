package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
)

// maxStackFrames bounds exception.stacktrace.
const maxStackFrames = 32

// noStacktrace disables stack traces: no slog level reaches it.
const noStacktrace = slog.Level(1<<31 - 1)

// ErrorOption configures NewErrorHandler.
type ErrorOption func(*errorHandler)

// WithErrorStacktrace adds exception.stacktrace, the goroutine stack at the log
// call (at most 32 frames), to records at or above level that carry an error.
// Stack traces are off by default.
func WithErrorStacktrace(level slog.Level) ErrorOption {
	return func(h *errorHandler) { h.stackAt = level }
}

// NewErrorHandler returns a handler that serializes errors following the
// production schema (docs/schema.md), then delegates to next.
//
// An attribute named "error" or "err" whose value is an error is replaced by
// error.type (the error's dynamic Go type, for example *fmt.wrapError) and
// error.message (err.Error(), so wrapped and joined errors keep their full
// text). This applies to record attributes and to attributes bound with
// Logger.With; attributes inside slog.Group values are left as they are. Like
// any call-site attribute, the error fields follow open groups.
//
// NewErrorHandler panics if next is nil.
func NewErrorHandler(next slog.Handler, opts ...ErrorOption) slog.Handler {
	if next == nil {
		panic("logger: NewErrorHandler called with a nil handler")
	}
	h := &errorHandler{next: next, stackAt: noStacktrace}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type errorHandler struct {
	next    slog.Handler
	stackAt slog.Level
}

func (h *errorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *errorHandler) Handle(ctx context.Context, r slog.Record) error {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if _, ok := asError(a); ok {
			found = true
			return false
		}
		return true
	})
	if !found {
		return h.next.Handle(ctx, r)
	}

	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(expandError(a)...)
		return true
	})
	if r.Level >= h.stackAt {
		if stack := stackFrom(r.PC); stack != "" {
			nr.AddAttrs(slog.String(KeyExceptionStacktrace, stack))
		}
	}
	return h.next.Handle(ctx, nr)
}

func (h *errorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	expanded := make([]slog.Attr, 0, len(attrs)+1)
	for _, a := range attrs {
		expanded = append(expanded, expandError(a)...)
	}
	return &errorHandler{next: h.next.WithAttrs(expanded), stackAt: h.stackAt}
}

func (h *errorHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &errorHandler{next: h.next.WithGroup(name), stackAt: h.stackAt}
}

// asError reports whether a is an "error"/"err" attribute holding an error.
func asError(a slog.Attr) (error, bool) {
	if a.Key != "error" && a.Key != "err" {
		return nil, false
	}
	v := a.Value.Resolve()
	if v.Kind() != slog.KindAny {
		return nil, false
	}
	err, ok := v.Any().(error)
	return err, ok && err != nil
}

func expandError(a slog.Attr) []slog.Attr {
	err, ok := asError(a)
	if !ok {
		return []slog.Attr{a}
	}
	return []slog.Attr{
		slog.String(KeyErrorType, fmt.Sprintf("%T", err)),
		slog.String(KeyErrorMessage, err.Error()),
	}
}

// stackFrom formats the current goroutine stack starting at the frame that
// made the log call (pc), in the style of a Go traceback.
func stackFrom(pc uintptr) string {
	if pc == 0 {
		return ""
	}
	pcs := make([]uintptr, 64+maxStackFrames)
	pcs = pcs[:runtime.Callers(1, pcs)]
	start := -1
	for i, p := range pcs {
		if p == pc {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	pcs = pcs[start:]
	if len(pcs) > maxStackFrames {
		pcs = pcs[:maxStackFrames]
	}

	var b strings.Builder
	frames := runtime.CallersFrames(pcs)
	for {
		f, more := frames.Next()
		b.WriteString(f.Function)
		b.WriteString("\n\t")
		b.WriteString(f.File)
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(f.Line))
		b.WriteByte('\n')
		if !more {
			break
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
