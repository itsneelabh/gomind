# AI Tools Showcase Dockerfile
# This builds a showcase of all built-in AI-powered tools from GoMind framework
# NOTE: Build this Dockerfile from the project root, not from this directory
# Usage: docker build -f examples/ai-tools-showcase/Dockerfile -t ai-tools-showcase .

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the entire project (needed for replace directive to work)
COPY . .

# Change to example directory and download dependencies
WORKDIR /app/examples/ai-tools-showcase
RUN go mod download

# Build the AI tools showcase binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o ai-tools-showcase ./main.go

# Final stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user for security
RUN addgroup -g 1001 -S aitools && \
    adduser -u 1001 -S aitools -G aitools

# Set working directory
WORKDIR /app

# Copy binary from builder stage (built in examples/ai-tools-showcase/)
COPY --from=builder /app/examples/ai-tools-showcase/ai-tools-showcase .

# Change ownership to non-root user
RUN chown -R aitools:aitools /app

# Switch to non-root user
USER aitools

# Expose the service port
EXPOSE 8080

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Set environment variables with defaults
ENV PORT=8080

# Document supported environment variables for configuration
# AI Configuration (REQUIRED - at least one AI provider):
# ENV OPENAI_API_KEY=your_openai_key_here
# ENV GROQ_API_KEY=your_groq_key_here
# ENV ANTHROPIC_API_KEY=your_anthropic_key_here
# ENV DEEPSEEK_API_KEY=your_deepseek_key_here
# ENV GEMINI_API_KEY=your_gemini_key_here

# Service Discovery Configuration:
# ENV REDIS_URL=redis://localhost:6379
# ENV GOMIND_NAMESPACE=examples

# Monitoring Configuration:
# ENV GOMIND_DEV_MODE=false
# ENV LOG_LEVEL=info

# Start the AI tools showcase
CMD ["./ai-tools-showcase"]

# Labels for container metadata
LABEL maintainer="GoMind Framework Team"
LABEL version="1.0.0"
LABEL description="AI Tools Showcase demonstrating built-in AI-powered tools from GoMind framework"
LABEL org.opencontainers.image.source="https://github.com/itsneelabh/gomind"
LABEL org.opencontainers.image.documentation="https://github.com/itsneelabh/gomind/tree/main/examples/ai-tools-showcase"
LABEL org.opencontainers.image.vendor="GoMind Framework"
LABEL tool.type="ai-tools-composite"
LABEL tool.architecture="multi-provider,built-in-tools,framework-integration"
LABEL tool.ai-dependency="required"
LABEL tool.capabilities="translation,summarization,sentiment-analysis,code-review,interactive-showcase"