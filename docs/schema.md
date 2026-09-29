# Production log schema

- **Version:** `1.0.0` (`logger.SchemaVersion`)
- **Issue:** #11 (Phase 4 — observability schema)
- **Naming:** [OpenTelemetry semantic conventions](https://opentelemetry.io/docs/specs/semconv/)
  wherever a convention exists.

This is the contract `logger-go` guarantees for production logs. Services get it
with `logger.NewProduction` (#27). The key names are also exported as
`logger.Key*` constants, so application code never spells them by hand.

## Format

- **One JSON object per line** (newline-delimited JSON), written to **stdout**.
  The platform's collector ships stdout to the log backend. Traces and metrics
  go through OTLP as before (see `docs/observability.md`).
- Encoded by `log/slog`'s JSON handler, or by the Zap bridge when a service
  still uses Zap.
- Built-in keys stay as `log/slog` emits them:

| Key | Type | Notes |
|---|---|---|
| `time` | string, RFC 3339 with nanoseconds | record time |
| `level` | string | `DEBUG`, `INFO`, `WARN`, `ERROR` (custom levels as `INFO+2` etc.) |
| `msg` | string | a constant, human-readable event name; variable data goes in attributes |
| `source` | object `{function,file,line}` | only when source is enabled |

## Canonical keys

### Resource (once per process)

| Key | Constant | Type | Example |
|---|---|---|---|
| `service.name` | `KeyServiceName` | string | `navi-api` |
| `service.version` | `KeyServiceVersion` | string | `1.4.0` |
| `deployment.environment.name` | `KeyDeploymentEnvironment` | string | `production` |

### Correlation (from the request context)

| Key | Constant | Type | Example |
|---|---|---|---|
| `trace_id` | `KeyTraceID` | string, 32 lowercase hex | `4bf92f3577b34da6a3ce929d0e0e4736` |
| `span_id` | `KeySpanID` | string, 16 lowercase hex | `00f067aa0ba902b7` |
| `trace_flags` | `KeyTraceFlags` | string, 2 hex | `01` |
| `request_id` | `KeyRequestID` | string | `5f0c…` |

`trace_id`, `span_id` and `trace_flags` follow the OpenTelemetry log data model
field names. They are emitted only when the context carries a valid span (#24).

### HTTP

| Key | Constant | Type | Example |
|---|---|---|---|
| `http.request.method` | `KeyHTTPRequestMethod` | string | `GET` |
| `http.route` | `KeyHTTPRoute` | string, low-cardinality template | `/items/:id` |
| `http.response.status_code` | `KeyHTTPResponseStatusCode` | integer | `200` |
| `http.response.body.size` | `KeyHTTPResponseBodySize` | integer, bytes | `512` |
| `url.path` | `KeyURLPath` | string | `/items/42` |
| `client.address` | `KeyClientAddress` | string | `203.0.113.7` |
| `duration_ms` | `KeyDurationMS` | integer, milliseconds | `12` |

`duration_ms` has no semantic convention for logs. It is a library
convention, in integer milliseconds so it is easy to query.

### RPC

| Key | Constant | Type | Example |
|---|---|---|---|
| `rpc.system` | `KeyRPCSystem` | string | `grpc` |
| `rpc.service` | `KeyRPCService` | string | `items.v1.ItemService` |
| `rpc.method` | `KeyRPCMethod` | string | `GetItem` |

### Identity

| Key | Constant | Type | Notes |
|---|---|---|---|
| `enduser.id` | `KeyEndUserID` | string | an opaque identifier; never an email address or a name (see Redaction) |

### Errors

| Key | Constant | Type | Example |
|---|---|---|---|
| `error.type` | `KeyErrorType` | string, Go type of the error | `*net.OpError` |
| `error.message` | `KeyErrorMessage` | string, `err.Error()` | `dial tcp: connection refused` |
| `exception.stacktrace` | `KeyExceptionStacktrace` | string, opt-in | goroutine stack at the log call |

An attribute named `error` or `err` whose value is an `error` is replaced by
`error.type` and `error.message` (#26):

- `error.type` is the dynamic type of the error as logged, for example
  `*fmt.wrapError` or `*errors.joinError`.
- `error.message` is `err.Error()`, so wrapped and joined errors keep their
  full, deterministic text.
- Other attributes holding errors are logged as their message.

## Policies

### Duplicate keys
- Resource keys are bound once by `NewProduction`. Do not add them again.
- Handlers do not de-duplicate keys, so the same key may appear twice in one
  JSON object. Avoid it: each key has one owner.
  - Resource keys belong to `NewProduction`.
  - Correlation keys belong to the context and middleware.
  - Everything else belongs to the call site.

### Grouping
- **Keys set by the library are always top-level.** This covers resource keys
  and context attributes such as request and trace IDs, even inside
  `logger.WithGroup` (see ADR 0002).
- Call-site keys, including the `error.*` fields, follow open groups like any
  record attribute. Log canonical call-site keys (HTTP, RPC, errors) from
  loggers without open groups.
- Application-specific data may use groups, for example
  `{"db":{"rows":3}}`.

### Cardinality
- Logs may carry high-cardinality values (IDs, paths). Those values must
  **never** become metric labels.
- For HTTP metrics, use `http.route` (a template), not `url.path`.
- `msg` is a constant event name. Put variable data in attributes, not in the
  message, so events can be grouped and counted.

### Redaction
- Secrets and personal data must not be logged. Types that hold them implement
  `slog.LogValuer` and redact themselves.
- As a safety net, `NewProduction` replaces the values of sensitive keys, such
  as `password`, `token` and `authorization`, with `[REDACTED]` at any depth
  (#25).

## Versioning
- **Additive changes** (new keys or new optional fields) bump the **minor**
  schema version and ship in a minor library release.
- **Renaming or removing a key, or changing its type,** bumps the **major**
  schema version. It needs a migration note and a deprecation period in which
  both keys are emitted.
- `logger.SchemaVersion` reports the version the library implements. It is not
  emitted per record, to keep lines small.
