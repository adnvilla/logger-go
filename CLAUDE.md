# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

- **Build:** `make` or `go build ./...`
- **Test core module:** `go test ./...`
- **Test Zap module:** `cd zap && go test ./...` (separate module, `zap/go.mod`)
- **Test OTel module:** `cd otel && go test ./...` (separate module, `otel/go.mod`)
- **Test single test:** `cd zap && go test -run TestGroupsAndAttributes ./...`
- **Tidy deps:** `go mod tidy && (cd zap && go mod tidy) && (cd otel && go mod tidy)`

## Architecture

This is a Go library (`github.com/adnvilla/logger-go`) that wraps `log/slog` with context-aware convenience helpers and a pluggable Zap backend.

### Root package (`logger`)

- **`context.go`** — Stores/retrieves `*slog.Logger` in `context.Context` using a private key. `FromContext` never returns nil (falls back to `slog.Default()`); nil contexts are treated as `context.Background()`; `WithContext(ctx, nil)` is a no-op. `With` creates a child logger with extra attributes.
- **`attrs.go`** — `WithAttrs`/`AttrsFromContext` carry request-scoped `slog.Attr`s in the context; `NewContextHandler(next)` adds them to every record at the top level (replaying `WithGroup`/`WithAttrs` on a pre-group handler when groups are open).
- **`logger.go`** — Level helpers (`Info`, `Warn`, `Error`, `Debug`) extract the logger from context and delegate to it. `SetLogger` is deprecated (ADR 0002): use `slog.SetDefault` + `WithContext`.

### `zap/` module (`github.com/adnvilla/logger-go/zap`)

- Separate Go module so core users do not inherit Zap; released in lockstep with the root (`zap/vX.Y.Z`).
- **`zap_logger.go`** — `NewHandler(*zap.Logger, ...Option)` delegates to `go.uber.org/zap/exp/zapslog` (ADR 0001). Options: `WithCaller`, `WithStacktraceAt`, `WithoutStacktrace`. Levels map by range for both `Enabled` and `Handle`.

### `otel/` module (`github.com/adnvilla/logger-go/otel`)

- Separate Go module (OpenTelemetry dependency stays out of the core); released in lockstep (`otel/vX.Y.Z`).
- `SpanContext` is a `logger.ContextExtractor` emitting `trace_id`/`span_id`/`trace_flags`; `NewHandler(next)` = `logger.NewContextHandler(next, SpanContext)`.

### Key design points

- All logging functions take `context.Context` as the first argument — the logger travels through the call stack via context, not globals.
- Process configuration (`slog.SetDefault`) is kept separate from context storage; request-scoped data should travel as context attributes (ADR 0002).
- Level helpers build the `slog.Record` themselves (`runtime.Callers`) so the reported caller is the application line.
- Tests use a `mockHandler` (in `logger_test.go`) that captures `slog.Record` entries; zap tests assert emitted JSON.

## Branching & releases

Roadmap work is grouped by phase milestones and released **once per phase** (see `docs/RELEASING.md`):

- Stack one PR per issue on `phase-N/main` (branches `phase-N/<issue>-<slug>`), squash-merge them into the phase branch, then merge `phase-N/main` → `master` with a merge commit in a PR that carries the phase milestone.
- semantic-release only runs on `master`; the release is then renamed after the phase milestone.
- Use Conventional Commit titles (`feat`, `fix`, `refactor`, `docs`, `test`, `ci`, `chore`); `docs`/`test`/`chore`/`ci` alone do not produce a version.
