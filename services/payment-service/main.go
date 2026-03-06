package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/mjmichael73/go-uber-clone/pkg/auth"
	"github.com/mjmichael73/go-uber-clone/pkg/config"
	"github.com/mjmichael73/go-uber-clone/pkg/metrics"
	"github.com/mjmichael73/go-uber-clone/pkg/middleware"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/payment"
	"github.com/mjmichael73/go-uber-clone/pkg/tracing"
	"github.com/mjmichael73/go-uber-clone/services/payment-service/handler"
)

func main() {
	cfg := config.Load()
	cfg.ServicePort = getEnvOrDefault("SERVICE_PORT", "50055")
	metricsPort := getEnvOrDefault("METRICS_PORT", "9091")
	jaegerEndpoint := getEnvOrDefault("JAEGER_ENDPOINT", "jaeger:4317")

	// Tracing
	shutdown, err := tracing.InitTracer("payment-service", jaegerEndpoint)
	if err != nil {
		log.Printf("Warning: Failed to initialize tracer: %v", err)
	} else {
		defer shutdown(context.Background())
	}

	// Metrics
	go func() {
		log.Printf("Starting metrics server on :%s", metricsPort)
		if err := metrics.StartMetricsServer(fmt.Sprintf(":%s", metricsPort)); err != nil {
			log.Printf("Warning: Failed to start metrics server: %v", err)
		}
	}()

	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	paymentHandler := handler.NewPaymentHandler(db)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, time.Duration(cfg.JWTExpiration)*time.Hour)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.GetUnaryInterceptors(jwtManager, nil)...,
		),
		grpc.ChainStreamInterceptor(
			middleware.GetStreamInterceptors()...,
		),
	)
	metrics.RegisterServer(grpcServer)

	pb.RegisterPaymentServiceServer(grpcServer, paymentHandler)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("payment-service", grpc_health_v1.HealthCheckResponse_SERVING)
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
		log.Println("Shutting down Payment Service...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Payment Service started on %s", address)
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
