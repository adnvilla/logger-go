# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

- **Build:** `make` or `go build ./...`
- **Test core module:** `go test ./...`
- **Test Zap module:** `cd zap && go test ./...` (separate module, `zap/go.mod`)
- **Test single test:** `cd zap && go test -run TestGroupsAndAttributes ./...`
- **Tidy deps:** `go mod tidy && (cd zap && go mod tidy)`

## Architecture

This is a Go library (`github.com/adnvilla/logger-go`) that wraps `log/slog` with context-aware convenience helpers and a pluggable Zap backend.

### Root package (`logger`)

- **`context.go`** — Stores/retrieves `*slog.Logger` in `context.Context` using a private key. `FromContext` falls back to `slog.Default()`. `With` creates a child logger with extra attributes.
- **`logger.go`** — `SetLogger` initializes both the context logger and `slog.Default()`. Level helpers (`Info`, `Warn`, `Error`, `Debug`) extract the logger from context and delegate to it.

### `zap/` module (`github.com/adnvilla/logger-go/zap`)

- Separate Go module so core users do not inherit Zap; released in lockstep with the root (`zap/vX.Y.Z`).
- **`zap_logger.go`** — `NewHandler(*zap.Logger, ...Option)` delegates to `go.uber.org/zap/exp/zapslog` (ADR 0001). Options: `WithCaller`, `WithStacktraceAt`, `WithoutStacktrace`. Levels map by range for both `Enabled` and `Handle`.

### Key design points

- All logging functions take `context.Context` as the first argument — the logger travels through the call stack via context, not globals.
- `SetLogger` both sets the context logger and `slog.SetDefault`, so code using either path gets the configured handler.
- Level helpers build the `slog.Record` themselves (`runtime.Callers`) so the reported caller is the application line.
- Tests use a `mockHandler` (in `logger_test.go`) that captures `slog.Record` entries; zap tests assert emitted JSON.

## Branching & releases

Roadmap work is grouped by phase milestones and released **once per phase** (see `docs/RELEASING.md`):

- Stack one PR per issue on `phase-N/main` (branches `phase-N/<issue>-<slug>`), squash-merge them into the phase branch, then merge `phase-N/main` → `master` with a merge commit in a PR that carries the phase milestone.
- semantic-release only runs on `master`; the release is then renamed after the phase milestone.
- Use Conventional Commit titles (`feat`, `fix`, `refactor`, `docs`, `test`, `ci`, `chore`); `docs`/`test`/`chore`/`ci` alone do not produce a version.
