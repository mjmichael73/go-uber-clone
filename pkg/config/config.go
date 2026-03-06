package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Service
	ServiceName string
	ServicePort string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// NATS
	NatsURL string

	// JWT
	JWTSecret     string
	JWTExpiration int // hours

	// Service Discovery
	UserServiceAddr         string
	DriverServiceAddr       string
	RideServiceAddr         string
	LocationServiceAddr     string
	PaymentServiceAddr      string
	NotificationServiceAddr string
}

func Load() *Config {
	return &Config{
		ServiceName:             getEnv("SERVICE_NAME", "uber-service"),
		ServicePort:             getEnv("SERVICE_PORT", "8080"),
		DBHost:                  getEnv("DB_HOST", "localhost"),
		DBPort:                  getEnv("DB_PORT", "5432"),
		DBUser:                  getEnv("DB_USER", "uber"),
		DBPassword:              getEnv("DB_PASSWORD", "uber123"),
		DBName:                  getEnv("DB_NAME", "uberdb"),
		DBSSLMode:               getEnv("DB_SSLMODE", "disable"),
		RedisAddr:               getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:           getEnv("REDIS_PASSWORD", ""),
		RedisDB:                 getEnvInt("REDIS_DB", 0),
		NatsURL:                 getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:               getEnv("JWT_SECRET", "your-super-secret-key-change-in-production"),
		JWTExpiration:           getEnvInt("JWT_EXPIRATION", 24),
		UserServiceAddr:         getEnv("USER_SERVICE_ADDR", "localhost:50051"),
		DriverServiceAddr:       getEnv("DRIVER_SERVICE_ADDR", "localhost:50052"),
		RideServiceAddr:         getEnv("RIDE_SERVICE_ADDR", "localhost:50053"),
		LocationServiceAddr:     getEnv("LOCATION_SERVICE_ADDR", "localhost:50054"),
		PaymentServiceAddr:      getEnv("PAYMENT_SERVICE_ADDR", "localhost:50055"),
		NotificationServiceAddr: getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:50056"),
	}
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
