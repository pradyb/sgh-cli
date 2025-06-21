# Multi-stage build for sgh-cli
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sgh .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata git

# Create non-root user
RUN addgroup -g 1001 -S sgh && \
    adduser -u 1001 -S sgh -G sgh

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/sgh .

# Change ownership to non-root user
RUN chown -R sgh:sgh /app

# Switch to non-root user
USER sgh

# Expose port (if needed for health checks)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./sgh"]

# Default command
CMD ["--help"] 