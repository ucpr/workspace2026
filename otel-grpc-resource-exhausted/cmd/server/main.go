// server is a small HTTP API instrumented with the OTel Go SDK. Spans are
// exported over OTLP/gRPC so that oversized export requests surface as
// "rpc error: code = ResourceExhausted desc = grpc: received message larger
// than max".
//
// GET  /work?children=5&attr_bytes=8000&events=0&stack=0
//
//	Emits one server span plus `children` child spans. Each child span carries
//	an attribute of `attr_bytes` bytes (think: a long SQL statement) and
//	`events` span events.
//
// POST /admin/stall?d=10s
//
//	Holds a shared lock for d. Every /work request needs that lock before it
//	can run its children, so in-flight requests pile up and then all finish at
//	once when the stall ends. This mimics a DB pool or downstream that gets
//	stuck and recovers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

var tracer = otel.Tracer("otel-grpc-resource-exhausted/server")

// stallMu is read-locked by every request and write-locked by /admin/stall.
var stallMu sync.RWMutex

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tp, err := newTracerProvider(ctx)
	if err != nil {
		log.Fatalf("init tracer provider: %v", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = tp.Shutdown(sctx)
	}()

	mux := http.NewServeMux()
	mux.Handle("GET /work", otelhttp.NewHandler(http.HandlerFunc(work), "GET /work"))
	mux.HandleFunc("POST /admin/stall", stall)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	addr := envOr("LISTEN_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
}

func newTracerProvider(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// Log every OTel SDK error (this is where "traces export: ..." shows up).
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Printf("[otel] %v", err)
	}))

	// OTEL_EXPORTER_OTLP_ENDPOINT / OTEL_EXPORTER_OTLP_INSECURE and the
	// OTEL_BSP_* variables are read by the SDK itself.
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithDialOption(grpc.WithUnaryInterceptor(logExportSize)),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(envOr("OTEL_SERVICE_NAME", "api-server")),
	))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

// logExportSize logs the serialized size and span count of every OTLP export
// request so we can see how close each batch is to the 4 MiB gRPC limit.
func logExportSize(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	r, ok := req.(*coltracepb.ExportTraceServiceRequest)
	if !ok {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	spans := 0
	for _, rs := range r.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			spans += len(ss.GetSpans())
		}
	}
	size := proto.Size(r)
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	status := "ok"
	if err != nil {
		status = err.Error()
	}
	log.Printf("[export] spans=%d bytes=%d (%.2f MiB, %.0f B/span) took=%s result=%s",
		spans, size, float64(size)/(1<<20), float64(size)/float64(max(spans, 1)), time.Since(start).Round(time.Millisecond), status)
	return err
}

func work(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	children := intParam(q.Get("children"), envInt("DEFAULT_CHILDREN", 5))
	attrBytes := intParam(q.Get("attr_bytes"), envInt("DEFAULT_ATTR_BYTES", 200))
	events := intParam(q.Get("events"), envInt("DEFAULT_EVENTS", 0))
	withStack := q.Get("stack") == "1"

	// Wait for the shared resource (blocked while /admin/stall is active).
	waitStart := time.Now()
	stallMu.RLock()
	stallMu.RUnlock()
	waited := time.Since(waitStart)
	trace.SpanFromContext(ctx).SetAttributes(attribute.Int64("app.wait_ms", waited.Milliseconds()))

	payload := strings.Repeat("x", attrBytes)
	for i := range children {
		_, span := tracer.Start(ctx, fmt.Sprintf("db.query-%d", i))
		span.SetAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.statement", payload),
		)
		for j := range events {
			span.AddEvent("retry", trace.WithAttributes(attribute.Int("attempt", j)))
		}
		if withStack && waited > time.Second {
			// What a timeout path typically does: record the error with stack.
			span.RecordError(fmt.Errorf("query waited %s: %w", waited, context.DeadlineExceeded), trace.WithStackTrace(true))
			span.SetStatus(codes.Error, "slow query")
		}
		span.End()
	}
	fmt.Fprintf(w, "ok children=%d attr_bytes=%d waited=%s\n", children, attrBytes, waited)
}

func stall(w http.ResponseWriter, r *http.Request) {
	d, err := time.ParseDuration(r.URL.Query().Get("d"))
	if err != nil || d <= 0 {
		http.Error(w, "d must be a positive duration, e.g. ?d=10s", http.StatusBadRequest)
		return
	}
	go func() {
		stallMu.Lock()
		log.Printf("[stall] begin (%s)", d)
		time.Sleep(d)
		stallMu.Unlock()
		log.Printf("[stall] end")
	}()
	fmt.Fprintf(w, "stalling for %s\n", d)
}

func intParam(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v >= 0 {
		return v
	}
	return def
}

func envInt(k string, def int) int { return intParam(os.Getenv(k), def) }

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
