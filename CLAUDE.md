# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

- **Build:** `make` or `go build ./...`
- **Test all:** `go test ./...`
- **Test single package:** `go test ./zap/`
- **Test single test:** `go test -run TestZapHandler_Handle ./zap/`
- **Tidy deps:** `go mod tidy`

## Architecture

This is a Go library (`github.com/adnvilla/logger-go`) that wraps `log/slog` with context-aware convenience helpers and a pluggable Zap backend.

### Root package (`logger`)

- **`context.go`** — Stores/retrieves `*slog.Logger` in `context.Context` using a private key. `FromContext` falls back to `slog.Default()`. `With` creates a child logger with extra attributes.
- **`logger.go`** — `SetLogger` initializes both the context logger and `slog.Default()`. Level helpers (`Info`, `Warn`, `Error`, `Debug`) extract the logger from context and delegate to it.

### `zap/` subpackage

- **`zap_logger.go`** — `ZapHandler` implements `slog.Handler`, bridging slog calls to a `zap.Logger`. `NewHandler` adds `CallerSkip(4)` to align caller frames. Level mapping via `convertSlogLevel`.

### Key design points

- All logging functions take `context.Context` as the first argument — the logger travels through the call stack via context, not globals.
- `SetLogger` both sets the context logger and `slog.SetDefault`, so code using either path gets the configured handler.
- Tests use a `mockHandler` (in `logger_test.go`) that captures `slog.Record` entries for assertions; zap tests use `zaptest/observer`.
