# Standalone Dockerfile for stock-market-tool
# This Dockerfile builds the tool independently without workspace dependencies
# Usage (from stock-market-tool directory): docker build -t stock-tool:latest .

FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev ca-certificates

WORKDIR /app

# Copy only the stock-market-tool directory (standalone build)
COPY go.mod go.sum ./
COPY *.go ./

# Configure Go to fetch from GitHub and ignore workspace
ENV GOWORK=off
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOPROXY=direct
ENV GONOSUMDB=github.com/itsneelabh/gomind/*

# Download dependencies from GitHub (v0.6.4)
RUN go mod download && go mod verify

# Verify dependencies
RUN go list -m all

# Build the binary with optimizations
RUN go build -a -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o stock-tool .

# Verify the binary was created
RUN ls -lh stock-tool

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy binary from builder stage
COPY --from=builder /app/stock-tool .

# Change ownership to non-root user
RUN chown appuser:appgroup /app/stock-tool
USER appuser

# Set default PORT if not provided
ENV PORT=8082

# Health check - uses PORT environment variable
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT}/health || exit 1

# Expose port (default 8082, can be overridden by PORT env var)
EXPOSE ${PORT}

# Run the binary
ENTRYPOINT ["./stock-tool"]
