PROTO_DIR=proto
GO_OUT=pkg/pb

.PHONY: proto build run docker-up docker-down

proto:
	@echo "Generating protobuf code..."
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/user/user.proto
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/driver/driver.proto
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/ride/ride.proto
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/location/location.proto
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/payment/payment.proto
	protoc --go_out=$(GO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/notification/notification.proto

build:
	@echo "Building services..."
	go build -o bin/api-gateway ./services/api-gateway
	go build -o bin/user-service ./services/user-service
	go build -o bin/driver-service ./services/driver-service
	go build -o bin/ride-service ./services/ride-service
	go build -o bin/location-service ./services/location-service
	go build -o bin/payment-service ./services/payment-service
	go build -o bin/notification-service ./services/notification-service

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down -v

migrate:
	@for f in migrations/*.sql; do \
		psql "postgresql://uber:uber123@localhost:5432/uberdb?sslmode=disable" -f $$f; \
	done