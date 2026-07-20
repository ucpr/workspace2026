// grpc-service2 is the storage service. It persists orders in PostgreSQL and
// is called by grpc-service1 (the order service). Database access goes through
// the dd-trace-go database/sql integration so every query becomes a span.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	sqltrace "github.com/DataDog/dd-trace-go/contrib/database/sql/v2"
	grpctrace "github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2"
	"github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	storagev1 "github.com/ucpr/workspace2026/dd-trace-go-with-otlp/gen/storage/v1"
	"github.com/ucpr/workspace2026/dd-trace-go-with-otlp/internal/tracing"
)

const serviceName = "grpc-service2"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	stopTracer := tracing.Start(serviceName)
	defer stopTracer()

	addr := envOr("LISTEN_ADDR", ":50052")
	dsn := envOr("DATABASE_URL", "postgres://app:app@postgres:5432/orders?sslmode=disable")

	db, err := openDB(dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpctrace.UnaryServerInterceptor(grpctrace.WithService(serviceName))),
		grpc.StreamInterceptor(grpctrace.StreamServerInterceptor(grpctrace.WithService(serviceName))),
	)
	storagev1.RegisterStorageServiceServer(srv, &server{db: db})

	// Health + reflection make the service easy to poke with grpcurl / compose healthchecks.
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)

	go func() {
		slog.Info("grpc-service2 listening", "addr", addr)
		if err := srv.Serve(lis); err != nil {
			slog.Error("serve", "err", err)
		}
	}()

	waitForSignal()
	slog.Info("shutting down")
	srv.GracefulStop()
	return nil
}

func openDB(dsn string) (*sql.DB, error) {
	// Register the pgx stdlib driver under a traced name.
	sqltrace.Register("pgx", &stdlib.Driver{}, sqltrace.WithService(serviceName))
	db, err := sqltrace.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Retry ping: Postgres may still be starting when we come up.
	var pingErr error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pingErr = db.PingContext(ctx)
		cancel()
		if pingErr == nil {
			slog.Info("connected to postgres")
			return db, nil
		}
		slog.Warn("waiting for postgres", "attempt", i+1, "err", pingErr)
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("ping postgres: %w", pingErr)
}

type server struct {
	storagev1.UnimplementedStorageServiceServer
	db *sql.DB
}

func (s *server) SaveOrder(ctx context.Context, req *storagev1.SaveOrderRequest) (*storagev1.SaveOrderResponse, error) {
	if req.GetItem() == "" || req.GetQuantity() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "item required and quantity must be > 0")
	}
	const q = `INSERT INTO orders (item, quantity, status)
	           VALUES ($1, $2, 'created')
	           RETURNING id, item, quantity, status, created_at`
	row := s.db.QueryRowContext(ctx, q, req.GetItem(), req.GetQuantity())
	o, err := scanOrder(row)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert order: %v", err)
	}
	slog.Info("saved order", "id", o.GetId(), "item", o.GetItem())
	return &storagev1.SaveOrderResponse{Order: o}, nil
}

func (s *server) LoadOrder(ctx context.Context, req *storagev1.LoadOrderRequest) (*storagev1.LoadOrderResponse, error) {
	const q = `SELECT id, item, quantity, status, created_at FROM orders WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, req.GetId())
	o, err := scanOrder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "order %d not found", req.GetId())
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load order: %v", err)
	}
	return &storagev1.LoadOrderResponse{Order: o}, nil
}

func (s *server) ListOrders(ctx context.Context, req *storagev1.ListOrdersRequest) (*storagev1.ListOrdersResponse, error) {
	limit := req.GetLimit()
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `SELECT id, item, quantity, status, created_at FROM orders ORDER BY id DESC LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list orders: %v", err)
	}
	defer rows.Close()

	var out []*storagev1.StoredOrder
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "scan order: %v", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "iterate orders: %v", err)
	}
	return &storagev1.ListOrdersResponse{Orders: out}, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(s scanner) (*storagev1.StoredOrder, error) {
	var (
		o         storagev1.StoredOrder
		createdAt time.Time
	)
	if err := s.Scan(&o.Id, &o.Item, &o.Quantity, &o.Status, &createdAt); err != nil {
		return nil, err
	}
	o.CreatedAt = createdAt.Format(time.RFC3339)
	return &o, nil
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
