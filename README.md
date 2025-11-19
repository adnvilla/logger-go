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

If you plan to use the Zap handler, also pull in Zap:

```bash
go get go.uber.org/zap
```

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
    ctx := context.Background()

    // Attach slog's default text handler to the context.
    ctx = logger.SetLogger(ctx, slog.NewTextHandler(os.Stdout, nil))

    logger.Info(ctx, "Hello from logger-go", "version", "v1")
}
```

The logger helpers expect the `context.Context` used during `SetLogger`. If no logger is found on the context, the helpers fall back to `slog.Default()`.

### Adding request scoped attributes

Use `logger.With` to enrich the context with attributes. The function returns a new context containing a child logger.

```go
ctx := logger.With(ctx, "request_id", reqID, "user_id", userID)
logger.Info(ctx, "handling request")
```

Every log emitted with the derived context includes those attributes.

### Using the Zap handler

The `zap` subpackage implements `slog.Handler`, which allows slog to write to Zap. This means you can continue using the familiar `zap.Logger` ecosystem while adopting `log/slog`.

```go
import (
    "context"

    "github.com/adnvilla/logger-go"
    "github.com/adnvilla/logger-go/zap"

    zaplib "go.uber.org/zap"
)

func main() {
    ctx := context.Background()

    zapLogger, _ := zaplib.NewDevelopment()
    ctx = logger.SetLogger(ctx, zap.NewHandler(zapLogger))

    logger.Info(ctx, "Hello, World!", "component", "demo")
    logger.Debug(ctx, "Structured logging", "key", "value", "count", 123)
}
```

Refer to [`examples/zap`](examples/zap/main.go) for a runnable program that mirrors the snippet above.

## Testing

Run the tests with:

```bash
go test ./...
```
