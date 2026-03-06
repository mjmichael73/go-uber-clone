package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mjmichael73/go-uber-clone/pkg/auth"
	"github.com/mjmichael73/go-uber-clone/pkg/config"
	"github.com/mjmichael73/go-uber-clone/pkg/metrics"
	"github.com/mjmichael73/go-uber-clone/pkg/tracing"
	driverPb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	notifPb "github.com/mjmichael73/go-uber-clone/pkg/pb/notification"
	ridePb "github.com/mjmichael73/go-uber-clone/pkg/pb/ride"
	userPb "github.com/mjmichael73/go-uber-clone/pkg/pb/user"
	"github.com/mjmichael73/go-uber-clone/services/api-gateway/handlers"
	"github.com/mjmichael73/go-uber-clone/services/api-gateway/router"
)

// @title Go Uber Clone API
// @version 1.0
// @description This is a sample Uber clone API server.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	cfg := config.Load()
	cfg.ServicePort = getEnvOrDefault("SERVICE_PORT", "8080")
	metricsPort := getEnvOrDefault("METRICS_PORT", "9091")
	jaegerEndpoint := getEnvOrDefault("JAEGER_ENDPOINT", "jaeger:4317")

	// Tracing
	shutdown, err := tracing.InitTracer("api-gateway", jaegerEndpoint)
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

	// Connect to all gRPC services
	userConn := mustConnect("user-service", cfg.UserServiceAddr)
	defer userConn.Close()

	driverConn := mustConnect("driver-service", cfg.DriverServiceAddr)
	defer driverConn.Close()

	rideConn := mustConnect("ride-service", cfg.RideServiceAddr)
	defer rideConn.Close()

	notifConn := mustConnect("notification-service", cfg.NotificationServiceAddr)
	defer notifConn.Close()

	// Create gRPC clients
	userClient := userPb.NewUserServiceClient(userConn)
	driverClient := driverPb.NewDriverServiceClient(driverConn)
	rideClient := ridePb.NewRideServiceClient(rideConn)
	notifClient := notifPb.NewNotificationServiceClient(notifConn)

	// Create JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, time.Duration(cfg.JWTExpiration)*time.Hour)

	// Create HTTP handlers
	userHandler := handlers.NewUserHTTPHandler(userClient)
	driverHandler := handlers.NewDriverHTTPHandler(driverClient)
	rideHandler := handlers.NewRideHTTPHandler(rideClient)
	wsHandler := handlers.NewWebSocketHandler(rideClient, driverClient, notifClient)

	// Setup router
	r := router.SetupRouter(userHandler, driverHandler, rideHandler, wsHandler, jwtManager)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down API Gateway...")
		os.Exit(0)
	}()

	address := fmt.Sprintf(":%s", cfg.ServicePort)
	log.Printf("API Gateway started on %s", address)
	if err := r.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func mustConnect(name, addr string) *grpc.ClientConn {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
		grpc.WithChainUnaryInterceptor(
			tracing.UnaryClientInterceptor(),
			metrics.UnaryClientInterceptor(),
		),
		grpc.WithChainStreamInterceptor(
			tracing.StreamClientInterceptor(),
			metrics.StreamClientInterceptor(),
		),
	)
	if err != nil {
		log.Fatalf("Failed to connect to %s at %s: %v", name, addr, err)
	}
	log.Printf("Connected to %s at %s", name, addr)
	return conn
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
