package logger

import (
	"context"
	"log/slog"
	"strings"
)

// Redacted replaces the value of sensitive attributes.
const Redacted = "[REDACTED]"

// DefaultRedactKeys are the keys NewRedactHandler redacts by default.
var DefaultRedactKeys = []string{
	"password", "passwd", "secret", "client_secret", "private_key", "credentials",
	"token", "access_token", "refresh_token", "id_token",
	"api_key", "apikey", "authorization", "cookie", "set_cookie",
}

// RedactOption configures NewRedactHandler.
type RedactOption func(*redactHandler)

// RedactKeys adds keys to the redaction list (in addition to DefaultRedactKeys).
func RedactKeys(keys ...string) RedactOption {
	return func(h *redactHandler) {
		for _, k := range keys {
			h.keys[normalizeKey(k)] = struct{}{}
		}
	}
}

// NewRedactHandler returns a handler that replaces the value of sensitive
// attributes with Redacted before delegating to next. It is a safety net: types
// that hold secrets should also implement slog.LogValuer and redact themselves.
//
// A key matches when, compared case-insensitively and treating "-" as "_",
// the whole key, its last dot-separated segment, or a "_"-separated suffix of
// that segment is in the redaction list. So "Authorization", "X-Api-Key",
// "user_password" and "http.request.header.authorization" match, while
// "token_count" does not.
// Values are checked at any depth inside groups, after slog.LogValuer
// resolution, for record attributes and attributes bound with Logger.With.
// Maps and structs logged with slog.Any are not inspected.
//
// NewRedactHandler panics if next is nil.
func NewRedactHandler(next slog.Handler, opts ...RedactOption) slog.Handler {
	if next == nil {
		panic("logger: NewRedactHandler called with a nil handler")
	}
	h := &redactHandler{next: next, keys: map[string]struct{}{}}
	RedactKeys(DefaultRedactKeys...)(h)
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type redactHandler struct {
	next slog.Handler
	keys map[string]struct{}
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	sensitive := false
	r.Attrs(func(a slog.Attr) bool {
		sensitive = h.needsRedaction(a)
		return !sensitive
	})
	if !sensitive {
		return h.next.Handle(ctx, r)
	}

	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(h.redact(a))
		return true
	})
	return h.next.Handle(ctx, nr)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redact(a)
	}
	return &redactHandler{next: h.next.WithAttrs(redacted), keys: h.keys}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &redactHandler{next: h.next.WithGroup(name), keys: h.keys}
}

func (h *redactHandler) sensitiveKey(key string) bool {
	k := normalizeKey(key)
	if _, ok := h.keys[k]; ok {
		return true
	}
	if i := strings.LastIndexByte(k, '.'); i >= 0 {
		k = k[i+1:]
		if _, ok := h.keys[k]; ok {
			return true
		}
	}
	for i := strings.IndexByte(k, '_'); i >= 0; i = strings.IndexByte(k, '_') {
		k = k[i+1:]
		if _, ok := h.keys[k]; ok {
			return true
		}
	}
	return false
}

func (h *redactHandler) needsRedaction(a slog.Attr) bool {
	if h.sensitiveKey(a.Key) {
		return true
	}
	switch a.Value.Kind() {
	case slog.KindLogValuer:
		return h.needsRedaction(slog.Attr{Key: a.Key, Value: a.Value.Resolve()})
	case slog.KindGroup:
		for _, g := range a.Value.Group() {
			if h.needsRedaction(g) {
				return true
			}
		}
	}
	return false
}

func (h *redactHandler) redact(a slog.Attr) slog.Attr {
	if h.sensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	v := a.Value.Resolve()
	if v.Kind() != slog.KindGroup {
		return slog.Attr{Key: a.Key, Value: v}
	}
	group := v.Group()
	out := make([]slog.Attr, len(group))
	for i, g := range group {
		out[i] = h.redact(g)
	}
	return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
}

func normalizeKey(k string) string {
	return strings.ReplaceAll(strings.ToLower(k), "-", "_")
}
