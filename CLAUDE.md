# CLAUDE.md

## Project Overview

`logger-go` is a small Go library (`github.com/adnvilla/logger-go`) that wraps the standard `log/slog` package to provide context-aware logging. It stores a `*slog.Logger` inside a `context.Context` and exposes convenience helpers for the four standard log levels. It also ships a Zap bridge so that `slog` can emit through `go.uber.org/zap`.

Go module: `github.com/adnvilla/logger-go`  
Go version: 1.22  
Current release: v1.0.0

---

## Repository Structure

```
logger-go/
├── context.go          # Context storage/retrieval helpers (WithContext, FromContext, With)
├── context_test.go     # Tests for context helpers
├── logger.go           # Logging level helpers (SetLogger, Info, Warn, Error, Debug)
├── logger_test.go      # Tests for level helpers
├── zap/
│   ├── zap_logger.go       # ZapHandler: slog.Handler backed by go.uber.org/zap
│   └── zap_logger_test.go  # Tests for ZapHandler
├── examples/
│   └── zap/
│       └── main.go         # Runnable example: Zap handler + context-aware logging
├── .github/
│   └── workflows/
│       ├── go.yml          # CI: build + test on push/PR to master
│       └── release.yml     # CD: semantic release triggered after CI passes on master
├── Makefile                # `make` / `make build`: tidy + build
├── .releaserc.json         # Semantic release configuration
├── CHANGELOG.md            # Release history
├── README.md               # User-facing documentation
└── go.mod / go.sum
```

---

## Public API

### Root package (`github.com/adnvilla/logger-go`)

| Symbol | Signature | Description |
|---|---|---|
| `SetLogger` | `func SetLogger(ctx context.Context, l slog.Handler) context.Context` | Creates a `*slog.Logger` from the handler, sets it as `slog.Default`, and stores it on the context. |
| `Info` | `func Info(ctx context.Context, msg string, attrs ...any)` | Log at Info level via the context logger. |
| `Warn` | `func Warn(ctx context.Context, msg string, attrs ...any)` | Log at Warn level. |
| `Error` | `func Error(ctx context.Context, msg string, attrs ...any)` | Log at Error level. |
| `Debug` | `func Debug(ctx context.Context, msg string, attrs ...any)` | Log at Debug level. |
| `WithContext` | `func WithContext(ctx context.Context, l *slog.Logger) context.Context` | Store a `*slog.Logger` on the context. |
| `FromContext` | `func FromContext(ctx context.Context) *slog.Logger` | Retrieve the logger from context; falls back to `slog.Default()`. |
| `With` | `func With(ctx context.Context, args ...any) context.Context` | Return a child context whose logger carries the extra attributes. |

The unexported `contextKey` type (a zero-size struct) is used as the map key to avoid collisions with other packages.

### Zap subpackage (`github.com/adnvilla/logger-go/zap`)

| Symbol | Description |
|---|---|
| `ZapHandler` | Struct that implements `slog.Handler` backed by a `*zap.Logger`. |
| `NewHandler(logger *zap.Logger) slog.Handler` | Wraps the Zap logger and adds a 4-level caller skip to compensate for the slog→handler call stack. |

`ZapHandler` methods:
- `Enabled` — delegates to `zap.Core().Enabled` after converting the slog level.
- `Handle` — converts `slog.Attr` values to `zap.Field` slices and dispatches on level.
- `WithAttrs` — creates a new handler with pre-set fields.
- `WithGroup` — creates a named child logger via `zap.Logger.Named`.

Custom/unknown slog levels fall through to `zap.Info` in `Handle` and to `zapcore.DebugLevel` in `convertSlogLevel`.

---

## Development Workflows

### Build

```bash
make          # runs go mod tidy + go build ./...
make build    # same as above
```

### Test

```bash
go test ./...
```

Tests use:
- Standard `testing` package
- `go.uber.org/zap/zaptest/observer` for capturing zap output in assertions
- A `mockHandler` (defined in `logger_test.go`) that captures `slog.Record` values

### Run the example

```bash
go run examples/zap/main.go
```

---

## CI/CD

- **`.github/workflows/go.yml`** — runs on every push to and PR against `master`. Delegates to the reusable workflow `adnvilla/gha-toolkit/.github/workflows/go-base.yml@v1.1.1` with Go 1.24.
- **`.github/workflows/release.yml`** — triggers after `go.yml` succeeds on `master`. Runs semantic release (Node 20) to bump the version and update `CHANGELOG.md`.
- Releases follow [Conventional Commits](https://www.conventionalcommits.org/) enforced by `.releaserc.json`.

---

## Code Conventions

1. **Context-first signatures** — every public function takes `context.Context` as its first parameter.
2. **Fallback to `slog.Default()`** — `FromContext` never panics; it falls back gracefully when no logger is stored.
3. **Immutability of handlers** — `WithAttrs` and `WithGroup` always return a new `ZapHandler`; the original is unchanged.
4. **Compile-time interface guard** — `var _ slog.Handler = (*ZapHandler)(nil)` at the top of `zap_logger.go` ensures the interface is satisfied at compile time. Keep this line when adding new handler implementations.
5. **Caller skip** — `NewHandler` adds `zap.AddCallerSkip(4)` to report the correct call site through the slog wrapper layers. Adjust if the call depth changes.
6. **No comments on obvious code** — comments are only added for non-obvious constraints or workarounds.
7. **Package name collision** — the `zap` subpackage is imported as `zap` while the Uber library is aliased `zaplib` in examples. Follow this convention to avoid shadowing.

---

## Dependencies

| Module | Version | Role |
|---|---|---|
| `go.uber.org/zap` | v1.27.0 | Zap logger (direct) |
| `go.uber.org/multierr` | v1.11.0 | Indirect (Zap dependency) |
| `github.com/stretchr/testify` | v1.8.1 | Test assertions |
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection in tests |

---

## Adding a New Handler

1. Create a new subpackage (e.g., `zerolog/`).
2. Define a struct that holds the backend logger.
3. Add `var _ slog.Handler = (*YourHandler)(nil)` for the compile-time guard.
4. Implement `Enabled`, `Handle`, `WithAttrs`, `WithGroup`.
5. In `Handle`, convert `slog.Attr` values via `attr.Value.Any()` to the backend's field type.
6. Adjust caller skip if the backend supports it.
7. Add tests using the backend's observer/test utilities.
8. Add a runnable example under `examples/<handler-name>/main.go`.
