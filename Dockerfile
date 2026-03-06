# Build stage
FROM golang:1.25.5-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy module files
COPY go.mod go.sum ./
RUN go mod download || (go env -w GOPROXY=https://goproxy.io,direct && go mod download)

# Copy source code
COPY . .

# Build argument for which service to build
ARG SERVICE

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /service ./services/${SERVICE}

# Runtime stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /service .

EXPOSE 8080 50051 50052 50053 50054 50055 50056

CMD ["./service"]