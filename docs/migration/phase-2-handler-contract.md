# Migration notes: Phase 2 — slog handler contract

These notes cover the Phase 2 release (expected `v1.1.0`, together with
`zap/v1.1.0`).

- **Core-only services** (context helpers with a standard slog handler):
  the only change is a correct caller in `source` / `caller` fields.
- **Services that use the Zap bridge**: the emitted JSON changes for groups,
  `LogValuer`, custom levels, timestamps and caller. Dashboards, monitors and
  saved queries that depend on the old shape must be checked before
  upgrading.

Background: [ADR 0001](../adr/0001-zap-backend-and-library-direction.md).
Tracking: #19, #20, #21, #22, and the consumer inventory in #18.

## 1. Upgrade

The Zap bridge is now its own module. Services that import
`github.com/adnvilla/logger-go/zap` must require it explicitly:

```bash
go get github.com/adnvilla/logger-go@v1.1.0 github.com/adnvilla/logger-go/zap@v1.1.0
go mod tidy
```

`go get github.com/adnvilla/logger-go/zap@v1.1.0` alone is enough: it pulls
the matching root version. Upgrading only the root fails with
`cannot find module providing package github.com/adnvilla/logger-go/zap`
until the module is added (`go mod tidy` also adds it).

Core-only services just upgrade the root:

```bash
go get github.com/adnvilla/logger-go@v1.1.0
```

No code changes are required: `zap.NewHandler(zapLogger)` keeps its signature.

## 2. What changes in the output

Real output from the same program on `v1.0.1` and `v1.1.0`, using a Zap
logger with `zap.AddCaller()` and `zap.AddStacktrace(zapcore.ErrorLevel)`
(timestamps elided):

| Case | `v1.0.1` | `v1.1.0` |
|---|---|---|
| `l.WithGroup("request").Info("handled", "id", 7)` | `"logger":"request", "id":7` | `"request":{"id":7}` |
| `slog.Group("http", slog.Int("status", 200))` | `"http":[{"Key":"status","Value":{}}]` (value lost) | `"http":{"status":200}` |
| `"password", password("hunter2")` with a redacting `LogValue()` | `"password":"hunter2"` (**secret leaked**) | `"password":"[REDACTED]"` |
| `slog.Attr{}` | `"":null` | omitted |
| `l.Log(ctx, slog.Level(12), …)` | `"level":"info"` (and dropped by Warn/Error cores) | `"level":"error"` + stack trace |
| Record with an explicit time | current time | the record's time |
| Direct `slog` call: `caller` | a frame further up the stack, for example `runtime/proc.go:283` (wrong) | the calling line |
| `logger.Info(ctx, …)`: `caller` | calling line | calling line (unchanged) |
| `Handle` called directly | prints `Logger.check error: failed to get caller` to stderr | no stderr noise |

Other behaviour to know:

- **The logger name is no longer used for groups.** Queries on
  `logger:<group>` must move to the nested field, for example `request.id`.
- **Caller and stack traces are configured on the bridge**, not on the
  `zap.Logger`:
  - The bridge writes through `zapLogger.Core()`, so `zap.AddCaller()` and
    `zap.AddStacktrace()` set on the logger are not visible to it.
  - Defaults: caller **on** (printed when the encoder has a `CallerKey`),
    stack trace at **Error**. This matches `zap.NewProduction`.
  - `zap.NewDevelopment` users who relied on stack traces at Warn:
    `zap.NewHandler(zapLogger, zap.WithStacktraceAt(slog.LevelWarn))`.
  - Loggers built with `zap.New(core)` that had no caller now get one when
    the encoder has a `CallerKey`. Opt out with `zap.WithCaller(false)`.
- **Custom levels map by range** (`< Info` → debug, `< Warn` → info,
  `< Error` → warn, otherwise error). Records accepted by the level filter
  are never dropped.
- **Unchanged:** level, message, logger name set with `zapLogger.Named`,
  fields bound with `zapLogger.With`, flat attributes, `logger.With`
  attributes, and every output from the standard slog JSON and text
  handlers. The production JSON and console golden tests pass unchanged.

## 3. Versioning decision

This ships as a **minor** release (`v1.1.0`), not a major one:

- The Go API is source-compatible: `NewHandler` gains optional options, and
  `ZapHandler` remains as a deprecated alias.
- The output changes fix behaviour that contradicted the documented
  `slog.Handler` contract (lost values, leaked secrets, dropped records).
  They are bug fixes, not new semantics.
- A major version in Go means a new module path (`/v2`) and an import-path
  change in every service, which is disproportionate here. The risk is
  handled by these notes, the checklist below and the pilot (#28).

## 4. Checklist for service owners

Use the inventory in #18 to find the services and assets that need attention.

- [ ] Search the code for `WithGroup(`, `slog.Group(`, `LogValue()`, custom
      `slog.Level(` values and `zap.NewHandler(`.
- [ ] Dashboards, monitors, saved queries and pipelines:
  - [ ] Re-point anything filtering on `logger:<group>` to the nested field.
  - [ ] Replace array-shaped group fields with object paths.
  - [ ] Review alerts on `level:info` that received custom high levels,
        which now arrive as `error`.
- [ ] Check log volume and cost if stack traces at Error are new for the
      service.
- [ ] Decide on caller and stack-trace options (`WithCaller`,
      `WithStacktraceAt`, `WithoutStacktrace`).

## 5. Pilot validation (go/no-go)

Run this on the pilot service first (#28):

1. In staging, capture about 15 minutes of logs on `v1.0.1`.
2. Deploy `v1.1.0` to staging and capture the same traffic.
3. Compare the key paths per event type: new paths, removed paths,
   `level`, `caller` and `logger`. Every difference must be explained by
   section 2.
4. Check the dashboards and monitors from the checklist against the new
   data.
5. Compare p95 latency and allocation rate of logging-heavy endpoints.

**Go** when every difference is expected, dashboards and alerts are
updated, and there is no latency regression beyond noise. **No-go**
otherwise: record the reason in #28.

## 6. Rollback

```bash
go get github.com/adnvilla/logger-go@v1.0.1
```

Downgrading the root also removes the `github.com/adnvilla/logger-go/zap`
requirement automatically, and the `zap` package is served from the root
again. Revert any dashboard changes made for the new shape.
