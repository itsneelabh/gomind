# Multi-Provider AI with Fallback Dockerfile
# This builds both tool and agent implementations with multi-provider AI fallback patterns
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/ai-multi-provider/Dockerfile -t multi-provider-ai .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/ai-multi-provider
RUN go mod download

# Build the multi-provider AI binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o multi-provider-ai .

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 multiprovider && \
    adduser -D -s /bin/sh -u 1001 -G multiprovider multiprovider

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/ai-multi-provider/)
COPY --from=builder /app/examples/ai-multi-provider/multi-provider-ai .

# Change ownership to non-root user
RUN chown -R multiprovider:multiprovider /app

# Switch to non-root user
USER multiprovider

# Expose the service ports (tool: 8085, agent: 8086)
EXPOSE 8085 8086

# Health check endpoint (checks primary deployment mode)
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${HEALTH_CHECK_PORT:-8085}/health || exit 1

# Set environment variables with defaults
ENV DEPLOYMENT_MODE=tool
ENV TOOL_PORT=8085
ENV AGENT_PORT=8086
ENV HEALTH_CHECK_PORT=8085

# Document supported environment variables for configuration
# Multi-Provider AI Configuration (REQUIRED - minimum 2 providers for fallback):
# ENV OPENAI_API_KEY=your_openai_key_here          # Primary provider (recommended)
# ENV GROQ_API_KEY=your_groq_key_here              # Fallback provider (ultra-fast)
# ENV ANTHROPIC_API_KEY=your_anthropic_key_here    # Secondary provider (analysis)
# ENV DEEPSEEK_API_KEY=your_deepseek_key_here      # Additional provider (reasoning)
# ENV GEMINI_API_KEY=your_gemini_key_here          # Additional provider (Google AI)

# Deployment Configuration:
# ENV DEPLOYMENT_MODE=tool                          # Options: tool, agent, both
# ENV TOOL_PORT=8085                               # Port for tool deployment
# ENV AGENT_PORT=8086                              # Port for agent deployment

# Multi-Provider Configuration:
# ENV PRIMARY_PROVIDER=openai                      # Explicitly set primary provider
# ENV FALLBACK_PROVIDER=groq                       # Explicitly set fallback provider
# ENV MAX_RETRY_ATTEMPTS=3                         # Max fallback attempts

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV NAMESPACE=examples

# Monitoring Configuration:
# ENV LOG_LEVEL=info
# ENV METRICS_ENABLED=true

# Performance Configuration:
# ENV PARALLEL_EXECUTION=true                      # Enable parallel provider execution
# ENV PROVIDER_TIMEOUT=30s                         # Timeout per provider
# ENV HEALTH_CHECK_INTERVAL=60s                    # Provider health check interval

# Start the multi-provider AI service
CMD ["./multi-provider-ai"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="Multi-Provider AI with automatic fallback patterns for both tools and agents"
LABEL org.opencontainers.image.source="https://github.com/itsneelabh/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/itsneelabh/gomind/tree/main/examples/ai-multi-provider"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL component.type="multi-provider-ai"
LABEL component.architecture="fallback-patterns,multi-provider,tool-and-agent"
LABEL component.ai-dependency="multi-required"
LABEL component.capabilities="automatic-fallback,provider-comparison,best-response,health-monitoring,discovery-planning,orchestration,problem-solving,task-routing"
LABEL component.providers="openai,groq,anthropic,deepseek,gemini"
LABEL component.deployment-modes="tool,agent,both"