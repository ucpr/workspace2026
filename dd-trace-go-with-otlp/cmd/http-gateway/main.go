// http-gateway exposes a small REST API and forwards requests to grpc-service1
// (the order service) over gRPC. It is the entry point of the call chain:
//
//	client -> http-gateway (HTTP) -> grpc-service1 -> grpc-service2 -> postgres
//
// The inbound HTTP handler and the outbound gRPC client are both traced with
// dd-trace-go so a single request produces one distributed trace.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	grpctrace "github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2"
	httptrace "github.com/DataDog/dd-trace-go/contrib/net/http/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	orderv1 "github.com/ucpr/workspace2026/dd-trace-go-with-otlp/gen/order/v1"
	"github.com/ucpr/workspace2026/dd-trace-go-with-otlp/internal/tracing"
)

const serviceName = "http-gateway"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	stopTracer := tracing.Start(serviceName)
	defer stopTracer()

	addr := envOr("LISTEN_ADDR", ":8080")
	orderAddr := envOr("ORDER_ADDR", "grpc-service1:50051")

	conn, err := grpc.NewClient(orderAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpctrace.UnaryClientInterceptor(grpctrace.WithService(serviceName))),
		grpc.WithStreamInterceptor(grpctrace.StreamClientInterceptor(grpctrace.WithService(serviceName))),
	)
	if err != nil {
		return fmt.Errorf("dial order service: %w", err)
	}
	defer conn.Close()
	h := &handler{orders: orderv1.NewOrderServiceClient(conn)}

	mux := httptrace.NewServeMux(httptrace.WithService(serviceName))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /orders", h.createOrder)
	mux.HandleFunc("GET /orders", h.listOrders)
	mux.HandleFunc("GET /orders/{id}", h.getOrder)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("http-gateway listening", "addr", addr, "order", orderAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
		}
	}()

	waitForSignal()
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

type handler struct {
	orders orderv1.OrderServiceClient
}

type orderJSON struct {
	ID        int64  `json:"id"`
	Item      string `json:"item"`
	Quantity  int32  `json:"quantity"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func toJSON(o *orderv1.Order) orderJSON {
	return orderJSON{
		ID:        o.GetId(),
		Item:      o.GetItem(),
		Quantity:  o.GetQuantity(),
		Status:    o.GetStatus(),
		CreatedAt: o.GetCreatedAt(),
	}
}

func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Item     string `json:"item"`
		Quantity int32  `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	resp, err := h.orders.CreateOrder(r.Context(), &orderv1.CreateOrderRequest{
		Item:     body.Item,
		Quantity: body.Quantity,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJSON(resp.GetOrder()))
}

func (h *handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	resp, err := h.orders.GetOrder(r.Context(), &orderv1.GetOrderRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toJSON(resp.GetOrder()))
}

func (h *handler) listOrders(w http.ResponseWriter, r *http.Request) {
	var limit int32
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = int32(n)
	}
	resp, err := h.orders.ListOrders(r.Context(), &orderv1.ListOrdersRequest{Limit: limit})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]orderJSON, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		out = append(out, toJSON(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": out})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// writeGRPCError maps gRPC status codes returned from downstream services onto
// sensible HTTP status codes.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	code := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	}
	writeError(w, code, st.Message())
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
