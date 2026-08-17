// Package logging provides JSON structured logging that automatically
// correlates log records with the active OpenTelemetry trace/span via the
// context passed to each log call.
package logging

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler wraps an slog.Handler and injects trace_id/span_id
// attributes from the OpenTelemetry span active in ctx, enabling
// traces/logs correlation in the backend (e.g. Datadog, CloudWatch).
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// New builds a JSON structured logger. level accepts debug/info/warn/error
// (case-insensitive); unrecognized values fall back to info.
func New(level string) *slog.Logger {
	return slog.New(traceHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     parseLevel(level),
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Align with common "severity"/"message" field names used by
			// CloudWatch/Datadog log processors, while keeping slog's
			// defaults for everything else.
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
			case slog.MessageKey:
				a.Key = "message"
			case slog.TimeKey:
				a.Key = "timestamp"
			}
			return a
		},
	})})
}

// SetDefault builds a logger via New and installs it as slog's package
// default, so plain slog.InfoContext(ctx, ...) calls anywhere in the
// process pick up trace correlation and JSON formatting.
func SetDefault(level string) *slog.Logger {
	logger := New(level)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN", "warning", "WARNING":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
