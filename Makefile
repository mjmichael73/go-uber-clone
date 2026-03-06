PROTO_DIR=proto
GO_OUT=pkg/pb

.PHONY: proto build run docker-up docker-down

proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
		$(PROTO_DIR)/user/user.proto
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
		$(PROTO_DIR)/driver/driver.proto
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
		$(PROTO_DIR)/ride/ride.proto
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
		$(PROTO_DIR)/location/location.proto
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
		$(PROTO_DIR)/payment/payment.proto
	protoc --go_out=. --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=paths=import \
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
		docker exec -i uber-postgres psql -U uber -d uberdb < $$f; \
	done

generate-swagger:
	/home/ubuntu/go/bin/swag init -g services/api-gateway/main.go -o services/api-gateway/docs
