# ADR 0002: Context storage and the default logger

- **Status:** Accepted
- **Date:** 2026-09-29
- **Issues:** #10, #23 (Phase 3 — configuration API)

## Context

`SetLogger(ctx, handler)` did three things at once:

1. It built a logger from a handler.
2. It replaced the process-wide `slog.Default()`.
3. It stored the logger in a context.

That couples one-time process configuration with request-scoped context
handling. Its name also suggests a logger argument, but it takes a handler.

The context helpers also had undefined nil behaviour:

- `FromContext(nil)` panicked.
- A typed nil `*slog.Logger` stored in a context was returned as nil, and the
  next log call panicked.

Storing a whole `*slog.Logger` in `context.Context` is a debated pattern.
The `log/slog` design deliberately left `NewContext`/`FromContext` out of the
standard library. What services actually need to travel with a request is
**data**: request ID, tenant, trace and span IDs. They do not need a
different logger per request.

## Decision

1. **Process configuration uses the standard library.** Build the logger once
   at startup and call `slog.SetDefault(l)`. `logger-go` does not add another
   global setter.
2. **`SetLogger` is deprecated, not removed.** It keeps working for v1 users.
   Its documentation shows the two explicit calls that replace it. Removal is
   only possible in a v2 (see #30 and `docs/RELEASING.md`).
3. **Nil behaviour is defined** and covered by tests:
   - A nil `context.Context` is treated as `context.Background()` in
     `WithContext`, `FromContext`, `With` and the level helpers, as
     `log/slog` does.
   - `WithContext(ctx, nil)` ignores the nil logger and returns `ctx`
     unchanged.
   - `FromContext` never returns nil. It falls back to `slog.Default()`.
   - `SetLogger(ctx, nil)` panics, like `slog.New(nil)`.
4. **Request-scoped data travels as attributes, not loggers** (#23).
   - `logger.WithAttrs(ctx, …)` stores attributes in the context.
   - `logger.NewContextHandler(next)` adds them to every record.
   - This works with plain `slog.InfoContext`, with the level helpers, and with
     any backend, including the Zap bridge.
   - It is the recommended pattern, and the Phase 4 trace-correlation and
     schema middleware build on it.
5. **`WithContext` / `FromContext` / `With` remain supported.** They still fit
   code that wants a logger per component. Dependency injection of a
   `*slog.Logger` is equally valid. The library does not require either.

## Consequences

- Existing callers keep compiling. Linters report `SetLogger` as deprecated,
  which is the intended nudge.
- Code that previously panicked on nil inputs now logs through the default
  logger.
- Services must wrap their handler once with `NewContextHandler` to benefit
  from context attributes. Phase 4's `NewProduction` does this for them.
