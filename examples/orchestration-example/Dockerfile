# Multi-Modal Orchestrator Dockerfile
# This builds a comprehensive orchestration service supporting autonomous, workflow, and hybrid modes
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/orchestration-example/Dockerfile -t multi-modal-orchestrator .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/orchestration-example
RUN go mod download

# Build the orchestrator binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o multi-modal-orchestrator .

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 orchestrator && \
    adduser -D -s /bin/sh -u 1001 -G orchestrator orchestrator

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/orchestration-example/)
COPY --from=builder /app/examples/orchestration-example/multi-modal-orchestrator .

# Copy any configuration files if they exist
# COPY --from=builder /app/config/ ./config/

# Change ownership to non-root user
RUN chown -R orchestrator:orchestrator /app

# Switch to non-root user
USER orchestrator

# Expose the service port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Set environment variables with defaults
ENV PORT=8080
ENV GIN_MODE=release

# Document supported environment variables for configuration
# AI Configuration:
# ENV AI_PROVIDER=openai
# ENV AI_MODEL=gpt-4
# ENV OPENAI_API_KEY=your_key_here
# ENV AI_TEMPERATURE=0.3
# ENV AI_MAX_TOKENS=1000

# Orchestration Configuration:
# ENV ORCHESTRATOR_MODE=hybrid
# ENV MAX_CONCURRENCY=10
# ENV STEP_TIMEOUT=30s
# ENV TOTAL_TIMEOUT=2m
# ENV RETRY_ATTEMPTS=2

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV SERVICE_NAME=multi-modal-orchestrator
# ENV NAMESPACE=default

# Monitoring Configuration:
# ENV METRICS_ENABLED=true
# ENV PROMETHEUS_PORT=9090
# ENV LOG_LEVEL=info

# Start the orchestrator service
CMD ["./multi-modal-orchestrator"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="Multi-Modal Orchestrator supporting autonomous, workflow, and hybrid orchestration patterns"
LABEL org.opencontainers.image.source="https://github.com/neelabhtripathi/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/neelabhtripathi/gomind/tree/main/examples/orchestration-example"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL orchestration.modes="autonomous,workflow,hybrid"
LABEL orchestration.capabilities="ai-routing,dependency-management,parallel-execution,synthesis"