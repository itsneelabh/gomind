# Enhanced Research Agent Dockerfile
# This builds an AI-enhanced research agent with comprehensive AI integration
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/agent-example-enhanced/Dockerfile -t research-agent-enhanced .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/agent-example-enhanced
RUN go mod download

# Build the agent binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o research-agent-enhanced .

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 agent && \
    adduser -D -s /bin/sh -u 1001 -G agent agent

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/agent-example-enhanced/)
COPY --from=builder /app/examples/agent-example-enhanced/research-agent-enhanced .

# Change ownership to non-root user
RUN chown -R agent:agent /app

# Switch to non-root user
USER agent

# Expose the service port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Set environment variables with defaults
ENV PORT=8080

# Document supported environment variables for configuration
# AI Configuration (REQUIRED for enhanced AI features):
# ENV AI_PROVIDER=openai
# ENV OPENAI_API_KEY=your_key_here
# ENV GROQ_API_KEY=your_groq_key_here
# ENV ANTHROPIC_API_KEY=your_anthropic_key_here

# Agent Configuration:
# ENV RESEARCH_DEPTH=comprehensive
# ENV MAX_CONCURRENT_REQUESTS=10
# ENV CACHE_ENABLED=true

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV SERVICE_NAME=research-agent-enhanced
# ENV NAMESPACE=default

# Monitoring Configuration:
# ENV METRICS_ENABLED=true
# ENV PROMETHEUS_PORT=9090
# ENV LOG_LEVEL=info

# Start the enhanced research agent
CMD ["./research-agent-enhanced"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="Enhanced Research Agent with comprehensive AI integration across all capabilities"
LABEL org.opencontainers.image.source="https://github.com/itsneelabh/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/itsneelabh/gomind/tree/main/examples/agent-example-enhanced"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL agent.type="enhanced-research"
LABEL agent.ai-capabilities="comprehensive,analysis,synthesis,intelligent-routing"