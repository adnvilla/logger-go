package logger

import (
	"context"
	"log/slog"
)

type attrsKey struct{}

// WithAttrs returns a copy of ctx that carries attrs in addition to the
// attributes ctx already carries. Handlers wrapped with NewContextHandler add
// them to every record logged with that context, whatever logger is used.
//
// A nil ctx is treated as context.Background(). Without attrs, ctx is returned
// unchanged.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(attrs) == 0 {
		return ctx
	}
	prev := AttrsFromContext(ctx)
	all := make([]slog.Attr, 0, len(prev)+len(attrs))
	all = append(append(all, prev...), attrs...)
	return context.WithValue(ctx, attrsKey{}, all)
}

// AttrsFromContext returns the attributes carried by ctx, oldest first. The
// returned slice must not be modified.
func AttrsFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs, _ := ctx.Value(attrsKey{}).([]slog.Attr)
	return attrs
}

// ContextExtractor derives attributes from a logging context, for example
// trace and span IDs from the active span (see the logger-go/otel module).
// It must be safe for concurrent use and should return nil when it has nothing
// to add.
type ContextExtractor func(ctx context.Context) []slog.Attr

// NewContextHandler returns a handler that adds attributes derived from the
// logging context to every record, then delegates to next: first those
// returned by each extractor, in order, then those carried with WithAttrs.
//
// Context attributes are always emitted at the top level, before the record's
// own attributes, even when groups are open (logger.WithGroup), so correlation
// fields such as request or trace IDs keep a stable path. They are not
// de-duplicated against attributes with the same key.
//
// Wrapping a context handler again adds the new extractors to it instead of
// nesting (without extractors, it is returned unchanged). NewContextHandler
// panics if next is nil.
func NewContextHandler(next slog.Handler, extractors ...ContextExtractor) slog.Handler {
	if next == nil {
		panic("logger: NewContextHandler called with a nil handler")
	}
	if h, ok := next.(*contextHandler); ok {
		if len(extractors) == 0 {
			return h
		}
		c := *h
		c.extractors = append(append([]ContextExtractor(nil), h.extractors...), extractors...)
		return &c
	}
	return &contextHandler{next: next, root: next, extractors: append([]ContextExtractor(nil), extractors...)}
}

// contextHandler keeps two views of the wrapped handler: next has every
// WithAttrs/WithGroup call applied, and root is next as it was before the first
// WithGroup. ops records the calls made since that first group so they can be
// replayed on top of root after adding context attributes at the top level.
type contextHandler struct {
	next       slog.Handler
	root       slog.Handler
	ops        []handlerOp
	extractors []ContextExtractor
}

type handlerOp struct {
	group string // WithGroup(group) when attrs is nil
	attrs []slog.Attr
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := h.contextAttrs(ctx)
	if len(attrs) == 0 {
		return h.next.Handle(ctx, r)
	}

	if len(h.ops) == 0 {
		// No open groups: context attributes go first in the record itself.
		nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		nr.AddAttrs(attrs...)
		r.Attrs(func(a slog.Attr) bool {
			nr.AddAttrs(a)
			return true
		})
		return h.next.Handle(ctx, nr)
	}

	// Open groups: bind context attributes before the first group, then replay.
	next := h.root.WithAttrs(attrs)
	for _, op := range h.ops {
		if op.attrs != nil {
			next = next.WithAttrs(op.attrs)
		} else {
			next = next.WithGroup(op.group)
		}
	}
	return next.Handle(ctx, r)
}

// contextAttrs returns the extractor attributes followed by WithAttrs ones.
func (h *contextHandler) contextAttrs(ctx context.Context) []slog.Attr {
	carried := AttrsFromContext(ctx)
	if len(h.extractors) == 0 {
		return carried
	}
	var attrs []slog.Attr
	for _, extract := range h.extractors {
		attrs = append(attrs, extract(ctx)...)
	}
	if len(attrs) == 0 {
		return carried
	}
	return append(attrs, carried...)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	next := h.next.WithAttrs(attrs)
	if len(h.ops) == 0 {
		return &contextHandler{next: next, root: next, extractors: h.extractors}
	}
	return &contextHandler{next: next, root: h.root, ops: h.withOp(handlerOp{attrs: attrs}), extractors: h.extractors}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &contextHandler{next: h.next.WithGroup(name), root: h.root, ops: h.withOp(handlerOp{group: name}), extractors: h.extractors}
}

// withOp returns a new slice so handlers derived from the same parent never
// share (and overwrite) each other's operations.
func (h *contextHandler) withOp(op handlerOp) []handlerOp {
	ops := make([]handlerOp, len(h.ops), len(h.ops)+1)
	copy(ops, h.ops)
	return append(ops, op)
}
