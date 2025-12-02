#!/bin/bash

# setup.sh - One-click setup for agent-with-resilience example
# This script sets up the local development environment and can deploy to Kubernetes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"

echo "=============================================="
echo "Setting up Research Agent with Resilience"
echo "=============================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Component directories (self-contained)
GROCERY_API_DIR="$EXAMPLES_DIR/mock-services/grocery-store-api"
GROCERY_TOOL_DIR="$EXAMPLES_DIR/grocery-tool"
AGENT_DIR="$SCRIPT_DIR"

# Check prerequisites
check_prerequisites() {
    echo "Checking prerequisites..."

    # Check Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        echo "Please install Go 1.23+ from https://golang.org/dl/"
        exit 1
    fi
    echo -e "${GREEN}✓ Go installed: $(go version)${NC}"

    # Check Docker (optional)
    if command -v docker &> /dev/null; then
        echo -e "${GREEN}✓ Docker installed${NC}"
        DOCKER_AVAILABLE=true
    else
        echo -e "${YELLOW}! Docker not found (optional for local development)${NC}"
        DOCKER_AVAILABLE=false
    fi

    # Check kubectl (optional)
    if command -v kubectl &> /dev/null; then
        echo -e "${GREEN}✓ kubectl installed${NC}"
        KUBECTL_AVAILABLE=true
    else
        echo -e "${YELLOW}! kubectl not found (optional for K8s deployment)${NC}"
        KUBECTL_AVAILABLE=false
    fi

    echo ""
}

# Setup Redis
setup_redis() {
    echo "Setting up Redis..."

    # Check if Redis is already running
    if command -v redis-cli &> /dev/null; then
        if redis-cli ping &> /dev/null; then
            echo -e "${GREEN}✓ Redis is already running${NC}"
            return 0
        fi
    fi

    # Try Docker Redis
    if [ "$DOCKER_AVAILABLE" = true ]; then
        echo "Starting Redis via Docker..."

        # Stop existing container if any
        docker stop gomind-redis 2>/dev/null || true
        docker rm gomind-redis 2>/dev/null || true

        # Start Redis
        docker run -d \
            --name gomind-redis \
            -p 6379:6379 \
            redis:7-alpine

        echo -e "${GREEN}✓ Redis started on port 6379${NC}"
    else
        echo -e "${RED}Error: Redis not available${NC}"
        echo "Please install Redis or Docker to run Redis"
        echo ""
        echo "Options:"
        echo "  1. Install Redis: brew install redis && brew services start redis"
        echo "  2. Use Docker: docker run -d -p 6379:6379 redis:7-alpine"
        exit 1
    fi

    echo ""
}

# Check for API keys
check_api_keys() {
    local has_key=false
    local key_source=""

    # Check environment variable first
    if [ -n "$OPENAI_API_KEY" ]; then
        has_key=true
        key_source="environment variable"
    # Then check .env file
    elif [ -f .env ] && grep -q "^OPENAI_API_KEY=sk-" .env; then
        has_key=true
        key_source=".env file"
    fi

    if [ "$has_key" = true ]; then
        log_success "OpenAI API key found ($key_source)"
        return 0
    else
        log_warn "No OpenAI API key configured"
        echo ""
        echo -e "${YELLOW}┌────────────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│  AI Features Require an OpenAI API Key                     │${NC}"
        echo -e "${YELLOW}├────────────────────────────────────────────────────────────┤${NC}"
        echo -e "${YELLOW}│  Without an API key, the agent will still work but AI      │${NC}"
        echo -e "${YELLOW}│  capabilities (summarization, analysis) will be disabled.  │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  To add your API key:                                      │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Option 1: Add to .env file                                │${NC}"
        echo -e "${YELLOW}│    echo 'OPENAI_API_KEY=sk-your-key' >> .env               │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Option 2: Export environment variable                     │${NC}"
        echo -e "${YELLOW}│    export OPENAI_API_KEY=sk-your-key                       │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  For Kubernetes: Create a secret                           │${NC}"
        echo -e "${YELLOW}│    kubectl create secret generic openai-api-key \\          │${NC}"
        echo -e "${YELLOW}│      --from-literal=api-key=sk-your-key \\                  │${NC}"
        echo -e "${YELLOW}│      -n gomind-examples                                    │${NC}"
        echo -e "${YELLOW}└────────────────────────────────────────────────────────────┘${NC}"
        echo ""
        return 1
    fi
}

# Create .env file
setup_env() {
    echo "Setting up environment..."

    if [ ! -f .env ]; then
        if [ -f .env.example ]; then
            cp .env.example .env
            echo -e "${GREEN}✓ Created .env file from .env.example${NC}"
        else
            echo "REDIS_URL=redis://localhost:6379" > .env
            echo "PORT=8093" >> .env
            echo -e "${GREEN}✓ Created default .env file${NC}"
        fi
    else
        echo -e "${GREEN}✓ .env file already exists${NC}"
    fi

    # Check for API keys
    check_api_keys || true

    echo ""
}

# Build the application (local only)
build_app() {
    echo "Building application..."

    # Download dependencies
    GOWORK=off go mod download
    GOWORK=off go mod tidy

    # Build
    GOWORK=off go build -o research-agent-resilience .

    echo -e "${GREEN}✓ Application built successfully${NC}"
    echo ""
}

# Build all components locally
build_all() {
    log_info "Building all components..."

    # Build grocery-store-api
    log_info "Building grocery-store-api..."
    if [ -d "$GROCERY_API_DIR" ]; then
        (cd "$GROCERY_API_DIR" && GOWORK=off go build -o grocery-store-api .)
        log_success "grocery-store-api built"
    else
        log_error "grocery-store-api not found at $GROCERY_API_DIR"
        exit 1
    fi

    # Build grocery-tool
    log_info "Building grocery-tool..."
    if [ -d "$GROCERY_TOOL_DIR" ]; then
        (cd "$GROCERY_TOOL_DIR" && GOWORK=off go build -o grocery-tool .)
        log_success "grocery-tool built"
    else
        log_warn "grocery-tool not found at $GROCERY_TOOL_DIR (may not be needed)"
    fi

    # Build agent
    log_info "Building research-agent-resilience..."
    (cd "$AGENT_DIR" && GOWORK=off go build -o research-agent-resilience .)
    log_success "research-agent-resilience built"
}

# Build Docker images
build_docker() {
    log_info "Building Docker images..."

    if [ -d "$GROCERY_API_DIR" ]; then
        docker build -t grocery-store-api:latest "$GROCERY_API_DIR"
        log_success "grocery-store-api:latest built"
    fi

    if [ -d "$GROCERY_TOOL_DIR" ]; then
        docker build -t grocery-tool:latest "$GROCERY_TOOL_DIR"
        log_success "grocery-tool:latest built"
    fi

    docker build -t research-agent-resilience:latest "$AGENT_DIR"
    log_success "research-agent-resilience:latest built"
}

# Load images to Kind
load_to_kind() {
    if ! command -v kind >/dev/null 2>&1; then
        log_warn "Kind not found, skipping image load"
        return
    fi

    # Detect Kind cluster name from current kubectl context
    local context=$(kubectl config current-context 2>/dev/null)
    local cluster_name=""

    if [[ "$context" == kind-* ]]; then
        cluster_name="${context#kind-}"
        log_info "Detected Kind cluster: $cluster_name"
    else
        # Check if there's a default 'kind' cluster
        if kind get clusters 2>/dev/null | grep -q "^kind$"; then
            cluster_name="kind"
            log_info "Using default Kind cluster: kind"
        else
            # Use the first available Kind cluster
            cluster_name=$(kind get clusters 2>/dev/null | head -1)
            if [ -z "$cluster_name" ]; then
                log_error "No Kind clusters found. Please create one with: kind create cluster --name <name>"
                return 1
            fi
            log_info "Using Kind cluster: $cluster_name"
        fi
    fi

    log_info "Loading images to Kind cluster '$cluster_name'..."
    kind load docker-image --name "$cluster_name" grocery-store-api:latest research-agent-resilience:latest

    # Only load grocery-tool if it exists
    if docker images grocery-tool:latest --format "{{.Repository}}" | grep -q grocery-tool; then
        kind load docker-image --name "$cluster_name" grocery-tool:latest
    fi

    log_success "Images loaded to Kind"
}

# Load environment variables from .env file
load_env() {
    log_info "Loading environment variables..."

    if [ -f "$AGENT_DIR/.env" ]; then
        set -a
        source "$AGENT_DIR/.env"
        set +a
        log_success "Loaded .env file"
    elif [ -f "$AGENT_DIR/.env.example" ]; then
        log_warn "No .env file found, copying from .env.example"
        cp "$AGENT_DIR/.env.example" "$AGENT_DIR/.env"
        set -a
        source "$AGENT_DIR/.env"
        set +a
        log_success "Created .env from example"
    else
        log_warn "No .env file found"
    fi
}

# Setup API keys as Kubernetes secrets
setup_k8s_secrets() {
    log_info "Setting up API keys as secrets..."

    # Check for AI API keys (loaded from .env)
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        log_warn "No AI API keys found in .env file"
        echo ""
        echo "To enable AI features, add API keys to your .env file:"
        echo "  OPENAI_API_KEY=your-key"
        echo "  # or"
        echo "  ANTHROPIC_API_KEY=your-key"
        echo "  # or"
        echo "  GROQ_API_KEY=your-key"
        echo ""
    else
        log_success "Using AI API keys from .env file"
    fi

    # Create AI provider keys secret (empty string for unset keys - won't be detected as available)
    kubectl create secret generic ai-provider-keys \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
        --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
        -n gomind-examples --dry-run=client -o yaml | kubectl apply -f -

    log_success "API keys configured"
}

# Deploy to Kubernetes
deploy_k8s() {
    log_info "Deploying to Kubernetes..."

    # Load environment and setup secrets
    load_env

    # Create namespace if not exists
    kubectl create namespace gomind-examples --dry-run=client -o yaml | kubectl apply -f -

    # Setup secrets
    setup_k8s_secrets

    # Deploy components
    if [ -f "$GROCERY_API_DIR/k8-deployment.yaml" ]; then
        kubectl apply -f "$GROCERY_API_DIR/k8-deployment.yaml"
        log_success "grocery-store-api deployed"
    fi

    if [ -f "$GROCERY_TOOL_DIR/k8-deployment.yaml" ]; then
        kubectl apply -f "$GROCERY_TOOL_DIR/k8-deployment.yaml"
        log_success "grocery-tool deployed"
    fi

    kubectl apply -f "$AGENT_DIR/k8-deployment.yaml"
    log_success "research-agent-resilience deployed"

    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=grocery-store-api -n gomind-examples --timeout=120s 2>/dev/null || true
    kubectl wait --for=condition=ready pod -l app=grocery-tool -n gomind-examples --timeout=120s 2>/dev/null || true
    kubectl wait --for=condition=ready pod -l app=research-agent-resilience -n gomind-examples --timeout=120s 2>/dev/null || true

    log_success "Deployment complete!"
    log_info "Run '$0 forward' to set up port forwards"
}

# Port forward
port_forward() {
    log_info "Setting up port forwards..."

    # Kill existing port forwards
    pkill -f "port-forward.*8081" 2>/dev/null || true
    pkill -f "port-forward.*8083" 2>/dev/null || true
    pkill -f "port-forward.*8093" 2>/dev/null || true

    sleep 1

    kubectl port-forward -n gomind-examples svc/grocery-store-api 8081:80 &
    kubectl port-forward -n gomind-examples svc/grocery-tool-service 8083:80 &
    kubectl port-forward -n gomind-examples svc/research-agent-resilience 8093:8093 &

    sleep 3

    log_success "Port forwards established:"
    echo "  - grocery-store-api: http://localhost:8081"
    echo "  - grocery-tool:      http://localhost:8083"
    echo "  - agent:             http://localhost:8093"
    echo ""
    echo "Press Ctrl+C to stop port forwards"

    wait
}

# Test resilience
test_resilience() {
    log_info "Running resilience test..."
    echo ""

    # Reset to normal mode
    log_info "Step 1: Reset to normal mode"
    curl -s -X POST http://localhost:8081/admin/reset | jq . 2>/dev/null || echo "Reset sent"
    echo ""

    # Test normal operation
    log_info "Step 2: Test normal operation"
    curl -s -X POST http://localhost:8093/api/capabilities/research_topic \
        -H "Content-Type: application/json" \
        -d '{"topic":"groceries","sources":["grocery-service"],"use_ai":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Request sent"
    echo ""

    # Enable rate limiting
    log_info "Step 3: Enable rate limiting (429 after 1 request)"
    curl -s -X POST http://localhost:8081/admin/inject-error \
        -H "Content-Type: application/json" \
        -d '{"mode":"rate_limit","rate_limit_after":1}' | jq . 2>/dev/null || echo "Rate limit enabled"
    echo ""

    # Make failing requests
    log_info "Step 4: Make requests (should trigger retries and failures)"
    for i in 1 2 3; do
        echo "Request $i:"
        curl -s -X POST http://localhost:8093/api/capabilities/research_topic \
            -H "Content-Type: application/json" \
            -d '{"topic":"groceries","sources":["grocery-service"],"use_ai":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Request sent"
    done
    echo ""

    # Check circuit breaker
    log_info "Step 5: Check circuit breaker status"
    curl -s http://localhost:8093/health | jq '.circuit_breakers["grocery-service"]' 2>/dev/null || echo "Health check sent"
    echo ""

    # Reset
    log_info "Step 6: Reset and recover"
    curl -s -X POST http://localhost:8081/admin/reset | jq . 2>/dev/null || echo "Reset sent"
    curl -s -X POST http://localhost:8093/api/capabilities/research_topic \
        -H "Content-Type: application/json" \
        -d '{"topic":"groceries","sources":["grocery-service"],"use_ai":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Recovery request sent"

    log_success "Resilience test complete!"
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."

    # Stop port forwards
    pkill -f "port-forward.*8081" 2>/dev/null || true
    pkill -f "port-forward.*8083" 2>/dev/null || true
    pkill -f "port-forward.*8093" 2>/dev/null || true

    # Delete K8s resources
    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true
    kubectl delete -f "$GROCERY_TOOL_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true
    kubectl delete -f "$GROCERY_API_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    # Stop local Redis
    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    log_success "Cleanup complete"
}

# Run the application
run_app() {
    echo "Starting Research Agent with Resilience..."
    echo ""
    echo "The agent will be available at: http://localhost:8093"
    echo ""
    echo "Endpoints:"
    echo "  POST /api/capabilities/research_topic - Resilient research"
    echo "  GET  /api/capabilities/discover_tools - Tool discovery"
    echo "  GET  /health                          - Health with CB states"
    echo ""
    echo "Press Ctrl+C to stop"
    echo "=============================================="
    echo ""

    # Load .env if exists
    if [ -f .env ]; then
        export $(cat .env | grep -v '^#' | xargs)
    fi

    # Set defaults if not set
    export REDIS_URL=${REDIS_URL:-"redis://localhost:6379"}
    export PORT=${PORT:-8093}

    ./research-agent-resilience
}

# Main setup
main() {
    check_prerequisites
    setup_redis
    setup_env
    build_app

    echo "=============================================="
    echo "Setup complete!"
    echo "=============================================="
    echo ""
    echo "To start the agent:"
    echo "  ./setup.sh run"
    echo ""

    # Check for run argument
    if [ "$1" = "run" ]; then
        run_app
    fi
}

# Show help
show_help() {
    cat << EOF
Usage: $0 <command>

Local Development Commands:
  setup      Setup the local development environment (default)
  run        Setup and run the application locally
  redis      Setup Redis only
  build      Build the application only

Kubernetes Deployment Commands:
  build-all  Build all components (agent + mock-services)
  docker     Build Docker images for all components
  deploy     Build, load to Kind, and deploy to Kubernetes
  forward    Set up port forwards to Kubernetes services
  test       Run the resilience test scenario
  cleanup    Remove all deployed resources

Examples:
  $0 run          # Quick start: run locally
  $0 deploy       # Full deployment to Kubernetes
  $0 forward      # Port forward after deployment
  $0 test         # Run resilience test
EOF
}

# Handle arguments
case "${1:-setup}" in
    setup)
        main
        ;;
    run)
        main run
        ;;
    redis)
        setup_redis
        ;;
    build)
        build_app
        ;;
    build-all)
        check_prerequisites
        build_all
        ;;
    docker)
        check_prerequisites
        build_docker
        ;;
    deploy)
        check_prerequisites
        build_docker
        load_to_kind
        deploy_k8s
        ;;
    forward)
        port_forward
        ;;
    test)
        test_resilience
        ;;
    cleanup)
        cleanup
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
