# logger-go

`logger-go` is a small helper library that wraps Go's standard [`log/slog`](https://pkg.go.dev/log/slog) package.
It keeps a logger inside a `context.Context`, exposes convenience helpers for the common log levels, and ships with a Zap handler implementation so you can plug the logger into existing logging setups.

## Features

- **Context aware logging** – store an entire `*slog.Logger` on the request/operation context and retrieve it from anywhere in your call stack.
- **Simple level helpers** – `logger.Info`, `logger.Warn`, `logger.Error`, and `logger.Debug` forward to the logger associated with the context.
- **Zap bridge** – use the provided `zap.NewHandler` to emit structured logs through [`go.uber.org/zap`](https://pkg.go.dev/go.uber.org/zap).

## Installation

```bash
go get github.com/adnvilla/logger-go
```

The core module has no third-party dependencies. The Zap bridge is a separate
module, so only services that use it pull in Zap:

```bash
go get github.com/adnvilla/logger-go/zap
```

Both modules are released together with the same version (tags `vX.Y.Z` and
`zap/vX.Y.Z`).

Upgrading from `v1.0.x`? Read the [Phase 2 migration notes](docs/migration/phase-2-handler-contract.md).

## Quick start

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/adnvilla/logger-go"
)

func main() {
    l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(l) // process-wide configuration, once at startup

    ctx := logger.WithContext(context.Background(), l)
    logger.Info(ctx, "Hello from logger-go", "version", "v1")
}
```

The helpers use the logger carried by the context. If there is none, they fall
back to `slog.Default()`. `FromContext` never returns nil, and a nil context is
treated as `context.Background()`, as in `log/slog`.

`logger.SetLogger(ctx, handler)` still works but is deprecated: it mixes the
process-wide default with context storage. See
[ADR 0002](docs/adr/0002-context-and-default-logger.md).

### Trace correlation (OpenTelemetry)

The `otel` module adds the active span's `trace_id`, `span_id` and `trace_flags`
to every record, so logs can be joined with traces. It is a separate module, so
services without OpenTelemetry do not depend on it:

```bash
go get github.com/adnvilla/logger-go/otel
```

```go
import loggerotel "github.com/adnvilla/logger-go/otel"

l := slog.New(loggerotel.NewHandler(slog.NewJSONHandler(os.Stdout, nil)))
// With the otelhttp or otelgin middleware in place:
slog.InfoContext(r.Context(), "handling request")
// {"msg":"handling request","trace_id":"4bf9…","span_id":"00f0…","trace_flags":"01"}
```

`loggerotel.SpanContext` is a `logger.ContextExtractor`. You can also pass it to
`logger.NewContextHandler` together with your own extractors.

### Request-scoped attributes in the context (recommended)

Put request data in the context as attributes, and wrap your handler once with
`logger.NewContextHandler`. Every record logged with that context gets the
attributes, whether it comes from the level helpers, from plain
`slog.InfoContext`, or from any backend (including the Zap bridge):

```go
l := slog.New(logger.NewContextHandler(slog.NewJSONHandler(os.Stdout, nil)))
slog.SetDefault(l)

// In the HTTP middleware:
ctx := logger.WithAttrs(r.Context(), slog.String("request_id", reqID))

slog.InfoContext(ctx, "handling request")            // includes request_id
l.WithGroup("db").InfoContext(ctx, "query", "rows", 3)
// {"msg":"query","request_id":"...","db":{"rows":3}}
```

Context attributes are emitted at the top level, before the record's own
attributes, even when groups are open, so correlation fields keep a stable path.
They are not de-duplicated against attributes with the same key. See
[ADR 0002](docs/adr/0002-context-and-default-logger.md).

### Adding request scoped attributes

Use `logger.With` to enrich the context with attributes. The function returns a new context containing a child logger.

```go
ctx := logger.With(ctx, "request_id", reqID, "user_id", userID)
logger.Info(ctx, "handling request")
```

Every log emitted with the derived context includes those attributes.

The level helpers also forward the caller's context to the handler's `Enabled`
and `Handle` methods, including when using the default logger. For request logs,
pass the request context (or a context derived from it):

```go
ctx := logger.With(r.Context(), "request_id", reqID)
logger.Info(ctx, "handling request")
```

A context-aware handler can use this context to extract trace/span IDs for
log correlation. The standard JSON/text handlers and the included Zap bridge
do not extract these values automatically. Cancellation does not by itself
suppress logging, so failures and timeouts can still be recorded with their
request context.

### Using the Zap handler

The `zap` module (`github.com/adnvilla/logger-go/zap`) implements `slog.Handler`, which allows slog to write to Zap. This means you can continue using the familiar `zap.Logger` ecosystem while adopting `log/slog`.

```go
import (
    "context"
    "log/slog"

    "github.com/adnvilla/logger-go"
    "github.com/adnvilla/logger-go/zap"

    zaplib "go.uber.org/zap"
)

func main() {
    ctx := context.Background()

    zapLogger, _ := zaplib.NewDevelopment()
    l := slog.New(zap.NewHandler(zapLogger))
    slog.SetDefault(l)
    ctx = logger.WithContext(ctx, l)

    logger.Info(ctx, "Hello, World!", "component", "demo")
    logger.Debug(ctx, "Hello, World!", "key", "value", "key2", 123)
}
```

See [`ExampleNewHandler`](zap/example_test.go) for a runnable example.

The bridge delegates to [`zapslog`](https://pkg.go.dev/go.uber.org/zap/exp/zapslog)
and follows the full `slog.Handler` contract
(see [ADR 0001](docs/adr/0001-zap-backend-and-library-direction.md)):

- `WithGroup` and `slog.Group` nest attributes; they never change the Zap logger name.
- Empty attributes and empty groups are dropped, and empty-key groups are inlined.
- `slog.LogValuer` values are resolved, so types can redact themselves.
- The record time and the caller from `slog.Record` are preserved.

```go
type password string

func (password) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

log := slog.New(zap.NewHandler(zapLogger))
log.WithGroup("request").Info("login", "user", "u1", "password", password("hunter2"))
```

```json
{"level":"info","ts":"2026-09-29T00:00:00Z","caller":"app/main.go:42","msg":"login","request":{"user":"u1","password":"[REDACTED]"}}
```

Levels map by range, the same way for filtering and for emission:

| slog level | Zap level |
|---|---|
| below `Info` | `Debug` |
| `Info` up to below `Warn` | `Info` |
| `Warn` up to below `Error` | `Warn` |
| `Error` and above | `Error` |

`NewHandler` writes through the logger's core, so the level, encoder, outputs,
hooks, logger name and fields added with `zapLogger.With` still apply. Caller and
stack-trace settings of the `zap.Logger` itself are not visible to the bridge.
Configure them with options instead:

```go
zap.NewHandler(zapLogger,
    zap.WithCaller(true),                  // default: true (printed when the encoder has a CallerKey)
    zap.WithStacktraceAt(slog.LevelError), // default: Error; zap.WithoutStacktrace() disables it
)
```

## Testing

Run the tests with:

```bash
go test -race -cover ./...                # core module
(cd zap && go test -race -cover ./...)    # Zap bridge module
(cd otel && go test -race -cover ./...)   # OpenTelemetry correlation module
```

The integration suite verifies that production JSON remains newline-delimited and
machine-parseable, while development console output preserves the same semantic
fields without depending on timestamps, field order, or whitespace.
