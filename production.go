package logger

import (
	"io"
	"log/slog"
	"os"
)

// Config configures NewProduction. Only the service fields are usually needed.
type Config struct {
	// Resource fields, bound once as service.name, service.version and
	// deployment.environment.name. Empty fields are omitted.
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Output receives newline-delimited JSON. Defaults to os.Stdout.
	Output io.Writer
	// Level is the minimum level. Defaults to slog.LevelInfo.
	Level slog.Leveler
	// AddSource adds the source (function, file, line) of each log call.
	AddSource bool
	// ReplaceAttr is passed to the JSON handler (see slog.HandlerOptions). It runs
	// after redaction and error serialization.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// Handler, when set, replaces the JSON handler as the final encoder, for
	// example the Zap bridge (github.com/adnvilla/logger-go/zap) during a
	// migration. Output, Level, AddSource and ReplaceAttr are then ignored.
	Handler slog.Handler

	// Extractors derive correlation attributes from the context, for example
	// SpanContext from github.com/adnvilla/logger-go/otel.
	Extractors []ContextExtractor
	// RedactKeys are redacted in addition to DefaultRedactKeys.
	RedactKeys []string
	// ErrorStacktrace adds exception.stacktrace to records at slog.LevelError
	// and above that carry an error.
	ErrorStacktrace bool
}

// NewProduction returns a logger that emits the production log schema
// (docs/schema.md, version SchemaVersion).
//
// Records flow through, in order:
//  1. NewContextHandler: context attributes (WithAttrs) and the configured
//     extractors, always at the top level;
//  2. NewRedactHandler: DefaultRedactKeys plus Config.RedactKeys;
//  3. NewErrorHandler: error.type and error.message, plus
//     exception.stacktrace when Config.ErrorStacktrace is set;
//  4. slog's JSON handler on Config.Output, or Config.Handler.
//
// The resource fields are bound on top. NewProduction does not change the
// process default: call slog.SetDefault with the result if needed.
func NewProduction(cfg Config) *slog.Logger {
	base := cfg.Handler
	if base == nil {
		out := cfg.Output
		if out == nil {
			out = os.Stdout
		}
		base = slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level:       cfg.Level,
			AddSource:   cfg.AddSource,
			ReplaceAttr: cfg.ReplaceAttr,
		})
	}

	var errorOpts []ErrorOption
	if cfg.ErrorStacktrace {
		errorOpts = append(errorOpts, WithErrorStacktrace(slog.LevelError))
	}
	h := NewErrorHandler(base, errorOpts...)
	h = NewRedactHandler(h, RedactKeys(cfg.RedactKeys...))
	h = NewContextHandler(h, cfg.Extractors...)

	var resource []any
	for _, f := range []struct{ key, value string }{
		{KeyServiceName, cfg.ServiceName},
		{KeyServiceVersion, cfg.ServiceVersion},
		{KeyDeploymentEnvironment, cfg.Environment},
	} {
		if f.value != "" {
			resource = append(resource, slog.String(f.key, f.value))
		}
	}
	return slog.New(h).With(resource...)
}
