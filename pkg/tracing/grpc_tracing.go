package tracing

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor for tracing.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return otelgrpc.UnaryServerInterceptor()
}

// StreamServerInterceptor returns a gRPC stream server interceptor for tracing.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return otelgrpc.StreamServerInterceptor()
}

// UnaryClientInterceptor returns a gRPC unary client interceptor for tracing.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return otelgrpc.UnaryClientInterceptor()
}

// StreamClientInterceptor returns a gRPC stream client interceptor for tracing.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return otelgrpc.StreamClientInterceptor()
}
