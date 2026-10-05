// receiver is a minimal OTLP/gRPC trace receiver for running the experiment
// without Docker. Like the OTel Collector's otlp receiver it keeps gRPC's
// default 4 MiB max receive size unless MAX_RECV_MSG_SIZE_MIB is set.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type traceServer struct {
	coltracepb.UnimplementedTraceServiceServer
}

func (traceServer) Export(_ context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	spans := 0
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			spans += len(ss.GetSpans())
		}
	}
	log.Printf("received spans=%d bytes=%d", spans, proto.Size(req))
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

func main() {
	addr := ":4317"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	var opts []grpc.ServerOption
	if v, err := strconv.Atoi(os.Getenv("MAX_RECV_MSG_SIZE_MIB")); err == nil && v > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(v<<20))
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	s := grpc.NewServer(opts...)
	coltracepb.RegisterTraceServiceServer(s, traceServer{})
	log.Printf("OTLP/gRPC receiver listening on %s", addr)
	log.Fatal(s.Serve(lis))
}
