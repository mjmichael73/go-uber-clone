package metrics

import (
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor for metrics.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return grpc_prometheus.UnaryServerInterceptor
}

// StreamServerInterceptor returns a gRPC stream server interceptor for metrics.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return grpc_prometheus.StreamServerInterceptor
}

// UnaryClientInterceptor returns a gRPC unary client interceptor for metrics.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return grpc_prometheus.UnaryClientInterceptor
}

// StreamClientInterceptor returns a gRPC stream client interceptor for metrics.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return grpc_prometheus.StreamClientInterceptor
}

// RegisterServer registers a gRPC server for metrics.
func RegisterServer(s *grpc.Server) {
	grpc_prometheus.Register(s)
}
