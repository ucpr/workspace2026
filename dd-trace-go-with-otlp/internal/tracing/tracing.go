// Package tracing wires up the Datadog tracer (dd-trace-go v2) for the
// services in this repo. Configuration is driven by the standard DD_* env
// vars (DD_AGENT_HOST, DD_ENV, DD_VERSION, ...) that dd-trace-go reads
// automatically, plus a service name passed explicitly.
package tracing

import (
	"log/slog"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
)

// Start initializes the global tracer for the given service and returns a
// stop function that flushes and shuts the tracer down. Call it once at
// process startup and defer the returned function.
func Start(service string) func() {
	// Export mode (OTLP vs Datadog agent), endpoint, env, version and
	// propagation style are all driven by env vars (OTEL_TRACES_EXPORTER,
	// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT, DD_ENV, DD_VERSION,
	// DD_TRACE_PROPAGATION_STYLE), so nothing else is configured here.
	tracer.Start(
		tracer.WithService(service),
	)
	slog.Info("tracer started", "service", service)
	return func() {
		tracer.Stop()
		slog.Info("tracer stopped", "service", service)
	}
}
