// Package telemetry bootstraps the OpenTelemetry SDK for every
// Releascope binary. Application code never imports a vendor-specific
// SDK (e.g. Datadog's dd-trace-go) — it only talks to the standard
// OpenTelemetry API, and this package wires that API to an OTLP
// exporter. Where the OTLP data ends up (Collector -> Datadog, Jaeger,
// etc.) is purely a matter of deployment configuration, never of code.
package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ShutdownFunc flushes and closes exporters. Callers must invoke it
// (typically via defer) before the process/Lambda invocation ends so
// batched spans and metrics are not lost — this matters especially in
// Lambda, where the runtime can freeze the execution environment as
// soon as the handler returns.
type ShutdownFunc func(context.Context) error

// Config controls telemetry bootstrap. Empty OTLPEndpoint intentionally
// keeps the application fully functional without a Collector: traces
// and metrics are written to stdout (visible in CloudWatch Logs) rather
// than dropped, which keeps "Collector is optional" true (spec section
// 27) while still letting local development inspect telemetry output.
type Config struct {
	ServiceName  string
	Environment  string
	OTLPEndpoint string // host:port or URL; empty disables OTLP export
}

// Setup configures global TracerProvider, MeterProvider and
// TextMapPropagator (W3C tracecontext + baggage). It returns a shutdown
// function and never an error for missing OTLP endpoint - that is a
// valid, supported configuration.
func Setup(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(cfg.ServiceName),
		semconv.DeploymentEnvironment(cfg.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	traceShutdown, err := setupTraces(ctx, res, cfg.OTLPEndpoint)
	if err != nil {
		return nil, err
	}

	metricShutdown, err := setupMetrics(ctx, res, cfg.OTLPEndpoint)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		err1 := traceShutdown(ctx)
		err2 := metricShutdown(ctx)
		if err1 != nil {
			return err1
		}
		return err2
	}, nil
}

func setupTraces(ctx context.Context, res *resource.Resource, endpoint string) (ShutdownFunc, error) {
	var sp sdktrace.SpanExporter
	var err error
	if endpoint == "" {
		sp, err = stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	} else {
		sp, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint), otlptracehttp.WithInsecure())
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry: build trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(sp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func setupMetrics(ctx context.Context, res *resource.Resource, endpoint string) (ShutdownFunc, error) {
	var reader metric.Reader
	if endpoint == "" {
		exp, err := stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("telemetry: build metric exporter: %w", err)
		}
		reader = metric.NewPeriodicReader(exp)
	} else {
		exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(endpoint), otlpmetrichttp.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("telemetry: build metric exporter: %w", err)
		}
		reader = metric.NewPeriodicReader(exp)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	return mp.Shutdown, nil
}

// TracerProvider returns the globally configured TracerProvider,
// primarily for handing to Lambda instrumentation (otellambda), which
// needs an explicit reference rather than reading the OTel global.
func TracerProvider() *sdktrace.TracerProvider {
	tp, _ := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	return tp
}
