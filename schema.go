package logger

// SchemaVersion is the version of the production log schema described in
// docs/schema.md. Adding keys is a minor change; renaming or removing keys, or
// changing their types, is a major change.
const SchemaVersion = "1.0.0"

// Canonical keys of the production log schema (docs/schema.md). Names follow
// the OpenTelemetry semantic conventions where one exists. The slog built-in
// keys (time, level, msg, source) are kept as emitted by log/slog.
const (
	// Resource: set once per process by NewProduction.
	KeyServiceName           = "service.name"
	KeyServiceVersion        = "service.version"
	KeyDeploymentEnvironment = "deployment.environment.name"

	// Correlation: taken from the request context.
	KeyTraceID    = "trace_id"
	KeySpanID     = "span_id"
	KeyTraceFlags = "trace_flags"
	KeyRequestID  = "request_id"

	// HTTP server and client.
	KeyHTTPRequestMethod      = "http.request.method"
	KeyHTTPRoute              = "http.route"
	KeyHTTPResponseStatusCode = "http.response.status_code"
	KeyHTTPResponseBodySize   = "http.response.body.size"
	KeyURLPath                = "url.path"
	KeyClientAddress          = "client.address"
	KeyDurationMS             = "duration_ms"

	// RPC.
	KeyRPCSystem  = "rpc.system"
	KeyRPCService = "rpc.service"
	KeyRPCMethod  = "rpc.method"

	// Identity.
	KeyEndUserID = "enduser.id"

	// Errors (see NewErrorHandler).
	KeyErrorType           = "error.type"
	KeyErrorMessage        = "error.message"
	KeyExceptionStacktrace = "exception.stacktrace"
)
