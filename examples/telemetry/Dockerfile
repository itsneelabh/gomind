# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/telemetry/Dockerfile -t telemetry-example .
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/telemetry
RUN go mod download

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o telemetry-example ./main.go

FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy binary from builder stage (built in examples/telemetry/)
COPY --from=builder /app/examples/telemetry/telemetry-example .

# Change ownership to non-root user
RUN chown appuser:appgroup /app/telemetry-example
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Expose port (if application serves HTTP)
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./telemetry-example"]
