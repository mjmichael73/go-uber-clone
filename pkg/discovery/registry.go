package discovery

import (
	"fmt"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServiceRegistry manages gRPC client connections
type ServiceRegistry struct {
	connections map[string]*grpc.ClientConn
	mu          sync.RWMutex
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		connections: make(map[string]*grpc.ClientConn),
	}
}

func (r *ServiceRegistry) GetConnection(serviceName, address string) (*grpc.ClientConn, error) {
	r.mu.RLock()
	if conn, exists := r.connections[serviceName]; exists {
		r.mu.RUnlock()
		return conn, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := r.connections[serviceName]; exists {
		return conn, nil
	}

	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", serviceName, address, err)
	}

	r.connections[serviceName] = conn
	log.Printf("Connected to service: %s at %s", serviceName, address)
	return conn, nil
}

func (r *ServiceRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, conn := range r.connections {
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection to %s: %v", name, err)
		}
	}
}
