package telemetry

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
)

// mapCarrier adapts a plain map[string]string to
// propagation.TextMapCarrier, used to move W3C trace context through
// JSON payloads (Step Functions state input/output) rather than HTTP
// headers.
type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string { return c[key] }
func (c mapCarrier) Set(key, value string) { c[key] = value }
func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectMap serializes the trace context (and any baggage) active in
// ctx into a plain map suitable for embedding as a "traceContext" field
// in a Step Functions state payload. Step Functions has no built-in
// notion of trace context, so Releascope propagates it manually through
// the JSON that already flows between states (see README's "Trace
// Propagation" section for which boundaries this covers).
func InjectMap(ctx context.Context) map[string]string {
	carrier := mapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier
}

// ExtractMap rebuilds a context carrying the trace context serialized
// by InjectMap. A nil/empty map is valid input (e.g. an
// EventBridge-Scheduler-triggered execution has no upstream trace to
// continue) and simply yields ctx unchanged, so the next span started
// from it becomes a new trace root.
func ExtractMap(ctx context.Context, carrier map[string]string) context.Context {
	if len(carrier) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, mapCarrier(carrier))
}

// headerCarrier adapts API Gateway's single-value proxy-integration
// event.Headers (map[string]string) to propagation.TextMapCarrier with
// case-insensitive lookups, since HTTP header casing is not guaranteed
// to survive API Gateway's transformation.
type headerCarrier map[string]string

func (c headerCarrier) Get(key string) string {
	for k, v := range c {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
func (c headerCarrier) Set(key, value string) { c[key] = value }
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// ExtractHTTPHeaders rebuilds trace context from an API Gateway proxy
// event's Headers map. This is genuine W3C Trace Context propagation
// across the client -> API Gateway -> Lambda boundary when the caller
// sends a `traceparent` header; when it doesn't, extraction is a no-op
// and the Lambda's span becomes a fresh trace root.
func ExtractHTTPHeaders(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, headerCarrier(headers))
}
