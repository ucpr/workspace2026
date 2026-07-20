// grpc-service1 is the order service. It receives requests from the HTTP
// gateway and delegates persistence to grpc-service2 (the storage service).
// Trace context propagates gateway -> service1 -> service2 via the dd-trace-go
// gRPC interceptors.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpctrace "github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	orderv1 "github.com/ucpr/workspace2026/dd-trace-go-with-otlp/gen/order/v1"
	storagev1 "github.com/ucpr/workspace2026/dd-trace-go-with-otlp/gen/storage/v1"
	"github.com/ucpr/workspace2026/dd-trace-go-with-otlp/internal/tracing"
)

const serviceName = "grpc-service1"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	stopTracer := tracing.Start(serviceName)
	defer stopTracer()

	addr := envOr("LISTEN_ADDR", ":50051")
	storageAddr := envOr("STORAGE_ADDR", "grpc-service2:50052")

	// Client to grpc-service2, traced so spans chain across the call.
	conn, err := grpc.NewClient(storageAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpctrace.UnaryClientInterceptor(grpctrace.WithService(serviceName))),
		grpc.WithStreamInterceptor(grpctrace.StreamClientInterceptor(grpctrace.WithService(serviceName))),
	)
	if err != nil {
		return fmt.Errorf("dial storage: %w", err)
	}
	defer conn.Close()
	storage := storagev1.NewStorageServiceClient(conn)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpctrace.UnaryServerInterceptor(grpctrace.WithService(serviceName))),
		grpc.StreamInterceptor(grpctrace.StreamServerInterceptor(grpctrace.WithService(serviceName))),
	)
	orderv1.RegisterOrderServiceServer(srv, &server{storage: storage})

	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)

	go func() {
		slog.Info("grpc-service1 listening", "addr", addr, "storage", storageAddr)
		if err := srv.Serve(lis); err != nil {
			slog.Error("serve", "err", err)
		}
	}()

	waitForSignal()
	slog.Info("shutting down")
	srv.GracefulStop()
	return nil
}

type server struct {
	orderv1.UnimplementedOrderServiceServer
	storage storagev1.StorageServiceClient
}

func (s *server) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	resp, err := s.storage.SaveOrder(ctx, &storagev1.SaveOrderRequest{
		Item:     req.GetItem(),
		Quantity: req.GetQuantity(),
	})
	if err != nil {
		return nil, err
	}
	slog.Info("order created", "id", resp.GetOrder().GetId())
	return &orderv1.CreateOrderResponse{Order: toOrder(resp.GetOrder())}, nil
}

func (s *server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	resp, err := s.storage.LoadOrder(ctx, &storagev1.LoadOrderRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	return &orderv1.GetOrderResponse{Order: toOrder(resp.GetOrder())}, nil
}

func (s *server) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	resp, err := s.storage.ListOrders(ctx, &storagev1.ListOrdersRequest{Limit: req.GetLimit()})
	if err != nil {
		return nil, err
	}
	orders := make([]*orderv1.Order, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		orders = append(orders, toOrder(o))
	}
	return &orderv1.ListOrdersResponse{Orders: orders}, nil
}

func toOrder(o *storagev1.StoredOrder) *orderv1.Order {
	if o == nil {
		return nil
	}
	return &orderv1.Order{
		Id:        o.GetId(),
		Item:      o.GetItem(),
		Quantity:  o.GetQuantity(),
		Status:    o.GetStatus(),
		CreatedAt: o.GetCreatedAt(),
	}
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
