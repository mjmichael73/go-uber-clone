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
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/mjmichael73/go-uber-clone/pkg/config"
	"github.com/mjmichael73/go-uber-clone/pkg/middleware"
	driverPb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	paymentPb "github.com/mjmichael73/go-uber-clone/pkg/pb/payment"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/ride"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/handler"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/matcher"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/repository"
)

func main() {
	cfg := config.Load()
	cfg.ServicePort = getEnvOrDefault("SERVICE_PORT", "50053")

	// Database
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

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
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to NATS: %v (continuing without event streaming)", err)
	}

	// gRPC clients
	driverConn, err := grpc.Dial(cfg.DriverServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Driver Service: %v", err)
	}
	driverClient := driverPb.NewDriverServiceClient(driverConn)

	paymentConn, err := grpc.Dial(cfg.PaymentServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Payment Service: %v", err)
	}
	paymentClient := paymentPb.NewPaymentServiceClient(paymentConn)

	// Initialize
	rideRepo := repository.NewRideRepository(db)
	rideMatcher, err := matcher.NewRideMatcher(cfg.DriverServiceAddr, cfg.NotificationServiceAddr, rideRepo)
	if err != nil {
		log.Fatalf("Failed to create ride matcher: %v", err)
	}

	rideHandler := handler.NewRideHandler(rideRepo, rideMatcher, driverClient, paymentClient, rdb, nc)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.RecoveryInterceptor,
			middleware.UnaryLoggingInterceptor,
		),
	)

	pb.RegisterRideServiceServer(grpcServer, rideHandler)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("ride-service", grpc_health_v1.HealthCheckResponse_SERVING)
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
		log.Println("Shutting down Ride Service...")
		grpcServer.GracefulStop()
		if nc != nil {
			nc.Close()
		}
	}()

	log.Printf("Ride Service started on %s", address)
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
