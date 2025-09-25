# YAML Workflow Engine Dockerfile
# This builds a workflow engine that executes YAML-defined workflow recipes
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/workflow-example/Dockerfile -t yaml-workflow-engine .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/workflow-example
RUN go mod download

# Build the workflow engine binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o yaml-workflow-engine .

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 workflow && \
    adduser -D -s /bin/sh -u 1001 -G workflow workflow

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/workflow-example/)
COPY --from=builder /app/examples/workflow-example/yaml-workflow-engine .

# Create workflows directory for YAML definitions
RUN mkdir -p workflows && chown -R workflow:workflow workflows

# Copy any pre-defined workflow templates if they exist
# COPY --from=builder /app/workflows/ ./workflows/

# Change ownership to non-root user
RUN chown -R workflow:workflow /app

# Switch to non-root user
USER workflow

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
# ENV AI_MAX_TOKENS=800

# Workflow Engine Configuration:
# ENV WORKFLOWS_DIR=./workflows
# ENV MAX_PARALLEL_WORKFLOWS=5
# ENV DEFAULT_WORKFLOW_TIMEOUT=30m
# ENV ENABLE_AI_ENHANCEMENT=true

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV SERVICE_NAME=yaml-workflow-engine
# ENV NAMESPACE=default

# Monitoring Configuration:
# ENV METRICS_ENABLED=true
# ENV PROMETHEUS_PORT=9090
# ENV LOG_LEVEL=info

# Start the workflow engine service
CMD ["./yaml-workflow-engine"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="YAML Workflow Engine for executing recipe-based workflows with dependency management"
LABEL org.opencontainers.image.source="https://github.com/neelabhtripathi/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/neelabhtripathi/gomind/tree/main/examples/workflow-example"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL workflow.features="yaml-recipes,dependency-management,parallel-execution,ai-enhancement,real-time-monitoring"
LABEL workflow.supported-formats="yaml,yml"