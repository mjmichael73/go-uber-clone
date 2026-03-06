package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/mjmichael73/go-uber-clone/pkg/config"
	"github.com/mjmichael73/go-uber-clone/pkg/middleware"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	"github.com/mjmichael73/go-uber-clone/services/driver-service/handler"
	"github.com/mjmichael73/go-uber-clone/services/driver-service/repository"
)

func main() {
	cfg := config.Load()
	cfg.ServicePort = getEnvOrDefault("SERVICE_PORT", "50052")

	// Database
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Initialize
	driverRepo := repository.NewDriverRepository(db)
	driverHandler := handler.NewDriverHandler(driverRepo, rdb)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.RecoveryInterceptor,
			middleware.UnaryLoggingInterceptor,
		),
		grpc.ChainStreamInterceptor(
			middleware.StreamLoggingInterceptor,
		),
	)

	pb.RegisterDriverServiceServer(grpcServer, driverHandler)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("driver-service", grpc_health_v1.HealthCheckResponse_SERVING)

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
		log.Println("Shutting down Driver Service...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Driver Service started on %s", address)
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
