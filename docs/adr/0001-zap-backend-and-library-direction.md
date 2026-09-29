# ADR 0001: Zap backend and library direction

- **Status:** Accepted
- **Date:** 2026-09-29
- **Issue:** #12 (Phase 1 — Zap backend decision)

## Context

`logger-go` has two layers:

| Layer | Size | Role |
|---|---|---|
| Core (`context.go`, `logger.go`) | ~50 LOC | Stores a `*slog.Logger` in `context.Context`, level helpers, `SetLogger` |
| Zap bridge (`zap/zap_logger.go`) | ~75 LOC | Custom `slog.Handler` that writes to a `*zap.Logger` |

The Zap bridge has known contract defects (#7 groups/attributes/`LogValuer`,
#8 custom levels, #9 time/source). The root module forces a Zap dependency on
every consumer. Several services need a common log schema for correlation and
dashboards (#11).

We evaluated three slog→Zap handlers side by side with identical inputs and a
JSON encoder: the current bridge, `go.uber.org/zap/exp/zapslog` v0.3.0, and
`github.com/samber/slog-zap/v2` v2.7.0.

### Behaviour (17 cases)

| Case | current | zapslog | slog-zap |
|---|---|---|---|
| `WithGroup("request")` + attrs | ❌ turns into the logger name | ✅ nested | ✅ nested |
| nested `slog.Group` | ❌ `[{"Key":..,"Value":{}}]`, data lost | ✅ | ✅ |
| sibling groups from one parent | ❌ | ✅ | ❌ **written under the wrong group** (shared slice in `WithGroup`) |
| `LogValuer` redaction | ❌ **secret leaked** | ✅ | ✅ |
| empty `Attr{}` / empty-key group | ❌ | ✅ | ✅ |
| `slog.Level(12)` with core at Warn | ❌ dropped | ✅ error | ❌ dropped |
| explicit `Record.Time` | ❌ ignored | ✅ | ✅ |
| caller on direct `slog` call | ❌ wrong frame | ✅ | ✅ |
| stable field order | ✅ | ✅ | ❌ random (goes through a map) |
| honours the Zap core level in `Enabled` | ✅ | ✅ | ❌ own `Level`, Debug by default |
| honours Zap caller/stacktrace config | partial | via options | ❌ clears them |
| context attribute extraction | ❌ | ❌ | ✅ `AttrFromContext` |

### Performance (3 attributes + 1 group per record; 4 CPU; indicative)

| Scenario | current | zapslog | slog-zap | `slog.JSONHandler` |
|---|---|---|---|---|
| enabled record | 1648 ns · 7 allocs | **1221 ns · 5 allocs** | 3461 ns · 25 allocs | 921 ns · 3 allocs |
| with caller | 3064 ns | **1482 ns** | 3913 ns | – |
| 4 pre-bound attrs (`With`) | 1546 ns | **1159 ns** | 4535 ns · 34 allocs | – |
| disabled level | 9 ns · 0 allocs | 9 ns · 0 allocs | 1291 ns · 10 allocs | – |

### Maintenance

| | current | zapslog | slog-zap |
|---|---|---|---|
| Stability | ours, with defects | experimental `v0.x`, last tag Oct 2024 | stable `v2`, active |
| Dependencies | zap | zap; imports `zap/internal/stacktrace` (couples it to the zap version) | zap + `samber/lo` + `slog-common` + `x/text` |
| Size | ~75 LOC | ~200 LOC, MIT | ~150 LOC + `slog-common` |

### Level helpers and caller

The helpers call `FromContext(ctx).InfoContext(...)`, so `slog.Logger` captures
the program counter (PC) inside the helper. With any handler that honours
`Record.PC`, the caller is reported as `logger-go/logger.go`. The current bridge
hides this with a fixed `zap.AddCallerSkip(4)`, which in turn breaks direct
`slog` calls.

Building the record in the helper with `runtime.Callers(2, …)` (the pattern
`log/slog` documents for wrapping output methods) reports the correct line with
every PC-honouring handler.

## Decision

1. **Stop owning a custom slog→Zap handler.** Delegate `zap.NewHandler` to
   `zapslog` (#19). It is the only adapter that passed every case, and it is
   the fastest.
2. **Reject `samber/slog-zap`.** It corrupts group placement, has
   nondeterministic field order, ignores the Zap core level, and is 2–3×
   slower.
3. **Zap is a migration bridge, not the target backend.** Native
   `slog.JSONHandler` beat every slog→Zap bridge. Zap moves to its own Go
   module (#21), so core users no longer inherit it.
4. **Fix the call site in the level helpers** (#20), independently of the
   backend.
5. **`logger-go` becomes the shared observability-schema layer.** Its value is
   the contract, not the plumbing: context attributes (#23), trace correlation
   (#24), redaction (#25), error serialization (#26), and a `NewProduction`
   factory (#27). All of these are built as backend-agnostic handler
   middleware.

## Consequences

- **Output changes.** For services using `WithGroup`, `slog.Group`,
  `LogValuer` or custom levels, the emitted JSON changes. The Phase 2 release
  is gated on the consumer inventory (#18) and migration notes (#22), and it
  rolls out through a pilot service first (#28).
- **Dependency on an experimental module.** The `zap/exp` version is pinned.
  - **Contingency:** if `zap/exp` stalls or breaks with a newer Zap, vendor
    `zapslog` (MIT, with attribution) into the adapter and replace its
    `internal/stacktrace` import. We would own ~200 lines of already-correct
    code instead of rewriting ours.
- **Stack traces.** `zapslog` adds a stack trace at Error by default. Our
  adapter must choose and document this default explicitly (#19).
- **Nested module.** The Zap module needs prefixed tags (`zap/vX.Y.Z`) and
  must require a root version that no longer contains the `zap` package, to
  avoid ambiguous imports (#21).
- **Releases.** Each phase ships as one version (see `docs/RELEASING.md`).

## Alternatives considered

- **Fix and keep the custom handler.** Rejected. It would reimplement
  `zapslog`, with higher maintenance cost and no capability that existing
  adapters lack.
- **Retire `logger-go` and use slog directly.** Viable for a single service.
  Rejected because several services benefit from one shared schema and
  correlation policy (Phase 4).
