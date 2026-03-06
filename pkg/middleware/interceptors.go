package middleware

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mjmichael73/go-uber-clone/pkg/auth"
)

// UnaryLoggingInterceptor logs all unary gRPC calls
func UnaryLoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	statusCode := codes.OK
	if err != nil {
		statusCode = status.Code(err)
	}

	log.Printf("[gRPC] %s | %s | %v | %v",
		info.FullMethod,
		statusCode,
		duration,
		err,
	)

	return resp, err
}

// StreamLoggingInterceptor logs all streaming gRPC calls
func StreamLoggingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	err := handler(srv, ss)
	duration := time.Since(start)

	log.Printf("[gRPC Stream] %s | %v | %v",
		info.FullMethod,
		duration,
		err,
	)

	return err
}

// UnaryAuthInterceptor validates JWT tokens
func UnaryAuthInterceptor(jwtManager *auth.JWTManager, publicMethods map[string]bool) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for public methods
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		claims, err := extractAndValidateToken(ctx, jwtManager)
		if err != nil {
			return nil, err
		}

		// Add user info to context
		ctx = context.WithValue(ctx, "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "user_type", claims.UserType)

		return handler(ctx, req)
	}
}

func extractAndValidateToken(ctx context.Context, jwtManager *auth.JWTManager) (*auth.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "metadata not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "authorization token not provided")
	}

	token := values[0]
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	claims, err := jwtManager.Verify(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return claims, nil
}

// RecoveryInterceptor recovers from panics
func RecoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC RECOVERY] %s: %v", info.FullMethod, r)
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}
