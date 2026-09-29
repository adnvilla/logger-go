# Logs, traces and metrics

- **Issue:** #14
- **Schema:** [docs/schema.md](schema.md)

`logger-go` is a **logging** library. Logs, traces and metrics are separate
signals with separate pipelines. This page states what the library does for
each one, and what it does not do.

| Signal | What it answers | How it leaves the service | What `logger-go` does |
|---|---|---|---|
| **Logs** | What happened, with detail, for one event | JSON lines on stdout, shipped by the platform collector | Everything: schema, context attributes, redaction, errors (`NewProduction`) |
| **Traces** | Where time went across services, for one request | OTLP, from the OpenTelemetry SDK | Only **correlation**: copies `trace_id`, `span_id` and `trace_flags` into each log record (`logger-go/otel`) |
| **Metrics** | How much and how often, aggregated over time | OTLP or a Prometheus scrape, from metric instruments | **Nothing.** Metrics are not derived from logs |

## Logs

- Output is newline-delimited JSON on stdout, following the production
  schema. The platform's collector (for `navi-api`, the OpenTelemetry
  collector feeding OpenObserve) ships stdout to the log backend.
- "OpenTelemetry support" in this library means **OpenTelemetry names and trace
  correlation inside JSON logs**. It does **not** mean exporting logs over
  OTLP.
  - That was a deliberate choice for Phase 4: stdout JSON keeps the logging
    path independent of the telemetry SDK.
  - A service that needs OTLP log export can add the official
    [`otelslog`](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog)
    bridge as an extra handler. That is outside what this library supports and
    tests.
- For querying logs outside the current platform, backends such as
  [Grafana Loki](https://grafana.com/oss/loki/) or
  [OpenSearch](https://opensearch.org/) ingest the same JSON lines.

## Traces

- Spans are created and exported by the OpenTelemetry SDK and instrumentation,
  such as `otelgin` or `otelhttp`, not by this library.
- **Correlation:** wrap the handler with `logger-go/otel`, or pass
  `loggerotel.SpanContext` to `NewProduction`.
  - Every record logged with a context that carries a valid span gets
    `trace_id`, `span_id` and `trace_flags` at the top level.
  - In the backend, jump from a trace to its logs by `trace_id`, and back.
- Log with the request context (`slog.InfoContext(ctx, …)` or
  `logger.Info(ctx, …)`). Without it there is no span to correlate.

## Metrics

- **Prometheus does not ingest application logs.** It stores numeric time
  series scraped from metric endpoints
  ([Prometheus FAQ](https://prometheus.io/docs/introduction/faq/#how-to-feed-logs-into-prometheus)).
  Integrating with Prometheus, or any metrics backend, means **instrumenting
  metrics** (counters, histograms) with the OpenTelemetry metrics API or a
  Prometheus client. Log formatting has no part in it.
- **Never turn log fields into metric labels** when they are high-cardinality:
  `request_id`, `trace_id`, `url.path`, `enduser.id`, `client.address`. Use
  low-cardinality attributes such as `http.route`, `http.request.method` and
  `http.response.status_code`. `navi-api` already follows this for its HTTP
  metrics.
- **Exemplars** link a metric data point to a trace (`trace_id`) without
  adding a label, so it is safe to use them for high-cardinality correlation.
  The OpenTelemetry SDK records exemplars from the active span. `logger-go`
  plays no part in this.

## Datadog

Datadog is **not** a supported target of this library. The platform ingests
OpenTelemetry names, so the schema has no `dd.*` fields.

A service that ships to Datadog would need two things:
- The Datadog agent reading the same JSON stdout.
- Its own `logger.ContextExtractor` that emits `dd.trace_id` and `dd.span_id`
  in the format Datadog expects. Alternatively, Datadog's OpenTelemetry
  ingestion, which maps the OpenTelemetry IDs.

Neither is tested here.

## Summary of guarantees

| Guaranteed by `logger-go` | Not provided by `logger-go` |
|---|---|
| Production JSON schema and its versioning | Log shipping (collector) |
| `trace_id` / `span_id` / `trace_flags` in logs when the context has a valid span | Creating or exporting spans |
| Redaction of sensitive keys and `LogValuer` resolution | Metrics, dashboards, alerts |
| `error.type` / `error.message` serialization | OTLP log export, Datadog-specific fields |
