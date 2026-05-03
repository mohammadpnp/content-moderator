# deploy/docker/Dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
# -ldflags="-s -w" strips debug info to reduce binary size
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/bin/server \
    ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bin/server /app/server

# Copy any configuration files if they exist
COPY --from=builder /app/deploy/docker/entrypoint.sh /app/entrypoint.sh
COPY --from=builder /app/deploy/migrations /app/deploy/migrations
RUN chmod +x /app/entrypoint.sh

# Use non-root user
USER appuser

# Expose ports
# 8080: HTTP API
# 9090: gRPC API  
# 6060: pprof (debug)
EXPOSE 8080 9090 6060

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the server
ENTRYPOINT ["/app/entrypoint.sh"]