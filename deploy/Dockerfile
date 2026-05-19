# Build stage
FROM golang:1.25-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and gcc for ONNX runtime
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for ONNX runtime
RUN CGO_ENABLED=1 GOOS=linux go build -a -o fraudsvc ./cmd/fraudsvc

# Final stage
FROM alpine:latest

# Install ca-certificates and ONNX runtime libraries
RUN apk --no-cache add ca-certificates libstdc++

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app/

# Copy the binary from builder stage
COPY --from=builder /app/fraudsvc .

# Copy configs
COPY --from=builder /app/configs ./configs

# Copy ML models
COPY --from=builder /app/ml/models ./ml/models

# Change ownership
RUN chown -R appuser:appgroup /app/

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./fraudsvc"]
