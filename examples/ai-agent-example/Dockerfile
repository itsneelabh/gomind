# AI-First Agent Dockerfile
# This builds an AI-first agent where AI drives every decision and process
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/ai-agent-example/Dockerfile -t ai-first-agent .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/ai-agent-example
RUN go mod download

# Build the AI-first agent binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ai-first-agent .

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 aiagent && \
    adduser -D -s /bin/sh -u 1001 -G aiagent aiagent

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/ai-agent-example/)
COPY --from=builder /app/examples/ai-agent-example/ai-first-agent .

# Change ownership to non-root user
RUN chown -R aiagent:aiagent /app

# Switch to non-root user
USER aiagent

# Expose the service port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Set environment variables with defaults
ENV PORT=8080

# Document supported environment variables for configuration
# AI Configuration (REQUIRED - AI-first agent cannot function without AI):
# ENV AI_PROVIDER=openai
# ENV OPENAI_API_KEY=your_key_here
# ENV GROQ_API_KEY=your_groq_key_here
# ENV ANTHROPIC_API_KEY=your_anthropic_key_here
# ENV AI_MODEL=gpt-4
# ENV AI_TEMPERATURE=0.3
# ENV AI_MAX_TOKENS=1500

# AI-First Agent Configuration:
# ENV PLANNING_ENABLED=true
# ENV ADAPTIVE_EXECUTION=true
# ENV CONTEXT_WINDOW=8000
# ENV GOAL_DECOMPOSITION_DEPTH=5

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV SERVICE_NAME=ai-first-agent
# ENV NAMESPACE=default

# Monitoring Configuration:
# ENV METRICS_ENABLED=true
# ENV PROMETHEUS_PORT=9090
# ENV LOG_LEVEL=info

# Performance Configuration:
# ENV MAX_CONCURRENT_AI_REQUESTS=5
# ENV AI_REQUEST_TIMEOUT=30s
# ENV CACHE_AI_RESPONSES=true

# Start the AI-first agent
CMD ["./ai-first-agent"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="AI-First Agent where AI drives every decision and process from start to finish"
LABEL org.opencontainers.image.source="https://github.com/itsneelabh/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/itsneelabh/gomind/tree/main/examples/ai-agent-example"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL agent.type="ai-first"
LABEL agent.architecture="ai-centric,planning-first,adaptive-execution,intent-driven"
LABEL agent.ai-dependency="required"
LABEL agent.capabilities="intelligent-query,goal-oriented-execution,service-orchestration,adaptive-problem-solving"