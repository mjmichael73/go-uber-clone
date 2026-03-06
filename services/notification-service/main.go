package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/mjmichael73/go-uber-clone/pkg/config"
	"github.com/mjmichael73/go-uber-clone/pkg/middleware"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/notification"
	"github.com/mjmichael73/go-uber-clone/services/notification-service/handler"
)

func main() {
	cfg := config.Load()
	cfg.ServicePort = getEnvOrDefault("SERVICE_PORT", "50056")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// NATS
	var nc *nats.Conn
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Printf("Warning: NATS connection failed: %v", err)
	}

	notifHandler := handler.NewNotificationHandler(rdb, nc)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.RecoveryInterceptor,
			middleware.UnaryLoggingInterceptor,
		),
	)

	pb.RegisterNotificationServiceServer(grpcServer, notifHandler)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("notification-service", grpc_health_v1.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	address := fmt.Sprintf(":%s", cfg.ServicePort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down Notification Service...")
		grpcServer.GracefulStop()
		if nc != nil {
			nc.Close()
		}
	}()

	log.Printf("Notification Service started on %s", address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
