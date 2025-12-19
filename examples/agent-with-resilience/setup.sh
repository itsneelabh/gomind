#!/bin/bash

# setup.sh - One-click setup for agent-with-resilience example
# This script sets up the local development environment and can deploy to Kubernetes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration
CLUSTER_NAME=${CLUSTER_NAME:-"gomind-demo-$(whoami)"}
NAMESPACE="gomind-examples"
APP_NAME="research-agent-resilience"
PORT=${PORT:-8093}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

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

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Agent with Resilience - $1${NC}"
    echo -e "${BLUE}================================================${NC}"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

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

# Load .env file if it exists
load_env() {
    if [ -f "$AGENT_DIR/.env" ]; then
        print_info "Loading environment from .env file"
        set -a
        source "$AGENT_DIR/.env"
        set +a
        print_success "Loaded .env file"
    elif [ -f "$AGENT_DIR/.env.example" ]; then
        print_info "No .env file found, copying from .env.example"
        cp "$AGENT_DIR/.env.example" "$AGENT_DIR/.env"
        set -a
        source "$AGENT_DIR/.env"
        set +a
        print_success "Created .env from example"
    else
        log_warn "No .env file found"
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
            # Use the configured CLUSTER_NAME
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                cluster_name="$CLUSTER_NAME"
                log_info "Using Kind cluster: $cluster_name"
            else
                # Use the first available Kind cluster
                cluster_name=$(kind get clusters 2>/dev/null | head -1)
                if [ -z "$cluster_name" ]; then
                    log_error "No Kind clusters found. Please create one with: ./setup.sh cluster"
                    return 1
                fi
                log_info "Using Kind cluster: $cluster_name"
            fi
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
    kubectl create secret generic ai-provider-keys-resilience-agent \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
        --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
        -n gomind-examples --dry-run=client -o yaml | kubectl apply -n $NAMESPACE -f -

    log_success "API keys configured"
}

# Deploy to Kubernetes
deploy_k8s() {
    log_info "Deploying to Kubernetes..."

    # Load environment and setup secrets
    load_env

    # Create namespace if not exists
    kubectl create namespace gomind-examples --dry-run=client -o yaml | kubectl apply -n $NAMESPACE -f -

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
    log_info "Run './setup.sh forward-all' to set up port forwards"
}

# Port forward (legacy - agent + dependencies)
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
        -d '{"topic":"groceries","sources":["grocery-service"],"ai_synthesis":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Request sent"
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
            -d '{"topic":"groceries","sources":["grocery-service"],"ai_synthesis":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Request sent"
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
        -d '{"topic":"groceries","sources":["grocery-service"],"ai_synthesis":false}' | jq '{success_rate, partial}' 2>/dev/null || echo "Recovery request sent"

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

# Check if a service is available (local or K8s port-forward)
check_service_available() {
    local port=$1
    local name=$2
    if nc -z localhost "$port" 2>/dev/null; then
        log_success "$name already available on port $port"
        return 0
    fi
    return 1
}

# Check if Redis is available (local, Docker, or K8s)
check_redis_available() {
    # Check local Redis
    if redis-cli ping 2>/dev/null | grep -q PONG; then
        log_success "Redis available (local)"
        export REDIS_URL="redis://localhost:6379"
        return 0
    fi

    # Check Docker Redis
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q gomind-redis; then
        log_success "Redis available (Docker: gomind-redis)"
        export REDIS_URL="redis://localhost:6379"
        return 0
    fi

    # Check K8s Redis via port-forward or service
    if nc -z localhost 6379 2>/dev/null; then
        log_success "Redis available on port 6379 (existing connection)"
        export REDIS_URL="redis://localhost:6379"
        return 0
    fi

    return 1
}

# Ensure Redis is available, starting it only if needed
ensure_redis() {
    if check_redis_available; then
        return 0
    fi

    log_info "Redis not found, starting..."
    setup_redis
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

# Run all components locally (smart - reuses existing infrastructure)
run_all() {
    log_info "Starting all components for local development..."
    echo ""

    # Track what we started (for cleanup on exit)
    local started_pids=()

    # Trap to cleanup background processes on exit
    cleanup_local() {
        echo ""
        log_info "Shutting down..."
        for pid in "${started_pids[@]}"; do
            kill "$pid" 2>/dev/null || true
        done
        log_success "All components stopped"
    }
    trap cleanup_local EXIT INT TERM

    # 1. Ensure Redis is available
    ensure_redis

    # 2. Load environment
    setup_env

    # 3. Build all components
    build_all

    # 4. Start grocery-store-api if not already running
    if check_service_available 8081 "grocery-store-api"; then
        log_info "Using existing grocery-store-api on port 8081"
    else
        log_info "Starting grocery-store-api on port 8081..."
        (cd "$GROCERY_API_DIR" && PORT=8081 ./grocery-store-api) &
        started_pids+=($!)
        sleep 2
    fi

    # 5. Start grocery-tool if not already running
    if check_service_available 8083 "grocery-tool"; then
        log_info "Using existing grocery-tool on port 8083"
    else
        if [ -f "$GROCERY_TOOL_DIR/grocery-tool" ]; then
            log_info "Starting grocery-tool on port 8083..."
            (cd "$GROCERY_TOOL_DIR" && PORT=8083 REDIS_URL="${REDIS_URL:-redis://localhost:6379}" ./grocery-tool) &
            started_pids+=($!)
            sleep 2
        else
            log_warn "grocery-tool not found, skipping (tool discovery may be limited)"
        fi
    fi

    # 6. Verify services are up
    echo ""
    log_info "Service Status:"
    if nc -z localhost 8081 2>/dev/null; then
        echo "  ✓ grocery-store-api: http://localhost:8081"
    else
        echo "  ✗ grocery-store-api: NOT RUNNING"
    fi
    if nc -z localhost 8083 2>/dev/null; then
        echo "  ✓ grocery-tool:      http://localhost:8083"
    else
        echo "  - grocery-tool:      NOT RUNNING (optional)"
    fi
    echo "  → agent:             http://localhost:8093 (starting...)"
    echo ""

    # 7. Run the agent in foreground
    run_app
}

#############################################
# STANDARDIZED 1-CLICK DEPLOYMENT COMMANDS
#############################################

# Create Kind cluster with port mappings
cmd_cluster() {
    print_header "Creating Kind Cluster"

    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        print_success "Cluster $CLUSTER_NAME already exists, reusing it"
    else
        cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  # Agent port
  - containerPort: 30093
    hostPort: 8093
    protocol: TCP
  # Grafana
  - containerPort: 30030
    hostPort: 3000
    protocol: TCP
  # Prometheus
  - containerPort: 30090
    hostPort: 9090
    protocol: TCP
  # Jaeger
  - containerPort: 31686
    hostPort: 16686
    protocol: TCP
EOF
        print_success "Kind cluster created"
    fi

    kubectl config use-context kind-$CLUSTER_NAME
}

# Setup infrastructure (Redis, OTEL, Prometheus, Jaeger, Grafana)
cmd_infra() {
    print_header "Setting Up Infrastructure"

    # Use the infrastructure setup script
    if [ -f "$SCRIPT_DIR/../k8-deployment/setup-infrastructure.sh" ]; then
        print_success "Found infrastructure setup script"
        echo ""

        # Run the infrastructure setup
        NAMESPACE=$NAMESPACE "$SCRIPT_DIR/../k8-deployment/setup-infrastructure.sh"

        echo ""
        print_success "Infrastructure ready"
    else
        print_error "Infrastructure setup script not found"
        echo "Please ensure k8-deployment/setup-infrastructure.sh exists"
        exit 1
    fi
}

# Build Docker image
cmd_docker_build() {
    print_header "Building Docker Image"

    build_docker

    print_success "Docker images built"
}

# Deploy to Kubernetes (standardized)
cmd_deploy() {
    print_header "Deploying to Kubernetes"

    load_env

    # Build Docker images first
    cmd_docker_build

    # Load images into kind cluster if available
    if command -v kind &> /dev/null; then
        print_info "Loading images into kind cluster..."
        load_to_kind
        print_success "Images loaded"
    fi

    # Deploy to Kubernetes
    deploy_k8s

    print_success "Deployment complete!"
    print_info "Run './setup.sh forward-all' to access services"
}

# Full deployment: cluster + infrastructure + agent
cmd_full_deploy() {
    print_header "Full Deployment (1-Click)"

    load_env

    # Step 1: Create Kind cluster
    cmd_cluster

    # Step 2: Setup infrastructure (Redis, monitoring, etc.)
    cmd_infra

    # Step 3: Deploy agent and dependencies
    cmd_deploy

    # Step 4: Setup port forwards
    cmd_forward_all
}

# Port forward for agent only (standardized)
cmd_forward() {
    print_header "Port Forwarding (Agent)"

    print_info "Starting port forward on localhost:8093..."
    print_info "Press Ctrl+C to stop"
    kubectl port-forward -n $NAMESPACE svc/research-agent-resilience 8093:8093
}

# Port forward for agent and monitoring
cmd_forward_all() {
    print_header "Port Forwarding (All Services)"

    # Kill existing port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true
    sleep 2

    # Start port forwarding in background
    print_info "Starting port forwards..."
    kubectl port-forward -n $NAMESPACE svc/research-agent-resilience 8093:8093 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grocery-store-api 8081:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grocery-tool-service 8083:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686 >/dev/null 2>&1 &

    sleep 2
    print_success "Port forwarding active"

    echo ""
    echo "Agent:             http://localhost:8093/health"
    echo "grocery-store-api: http://localhost:8081"
    echo "grocery-tool:      http://localhost:8083"
    echo "Grafana:           http://localhost:3000 (admin/admin)"
    echo "Prometheus:        http://localhost:9090"
    echo "Jaeger:            http://localhost:16686"
    echo ""
    echo "Press Ctrl+C or run: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
}

# Run tests (standardized)
cmd_test() {
    print_header "Running Tests"

    # Use the existing test_resilience function
    test_resilience
}

# Clean up agent only (standardized)
cmd_clean() {
    print_header "Cleaning Up Agent"

    cleanup

    print_success "Agent cleanup complete"
}

# Clean up everything including cluster (standardized)
cmd_clean_all() {
    print_header "Cleaning Up Everything"

    # Kill port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true

    # Delete agent
    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true
    kubectl delete -f "$GROCERY_TOOL_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true
    kubectl delete -f "$GROCERY_API_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    # Stop local Redis
    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    # Delete Kind cluster
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Deleting Kind cluster $CLUSTER_NAME..."
        kind delete cluster --name $CLUSTER_NAME
        print_success "Kind cluster deleted"
    fi

    print_success "Full cleanup complete"
}

# View logs
cmd_logs() {
    print_header "Viewing Logs"

    kubectl logs -n $NAMESPACE -l app=$APP_NAME -f --tail=100
}

# Check status
cmd_status() {
    print_header "Deployment Status"

    echo "Agent Pod:"
    kubectl get pods -n $NAMESPACE -l app=$APP_NAME
    echo ""
    echo "Agent Service:"
    kubectl get svc -n $NAMESPACE -l app=$APP_NAME
    echo ""
    echo "Dependencies:"
    kubectl get pods -n $NAMESPACE -l "app in (grocery-store-api,grocery-tool)"
    echo ""
    echo "Infrastructure:"
    kubectl get pods -n $NAMESPACE -l "app in (redis,prometheus,grafana,otel-collector,jaeger)"
}

# Rollout - restart deployment to pick up new secrets/config
cmd_rollout() {
    print_header "Rolling Out Deployment"

    local rebuild=false

    # Check for --build flag
    if [ "$2" = "--build" ] || [ "$2" = "build" ]; then
        rebuild=true
    fi

    # Load env to update secrets
    load_env

    # Update secrets from .env
    print_info "Updating secrets from .env..."
    setup_k8s_secrets

    # Rebuild if requested
    if [ "$rebuild" = true ]; then
        print_info "Rebuilding Docker images..."
        build_docker

        if command -v kind &> /dev/null; then
            print_info "Loading images into kind cluster..."
            load_to_kind
            print_success "Images loaded"
        fi
    fi

    # Restart deployment
    print_info "Restarting deployment..."
    kubectl rollout restart deployment/$APP_NAME -n $NAMESPACE

    print_info "Waiting for rollout to complete..."
    if kubectl rollout status deployment/$APP_NAME -n $NAMESPACE --timeout=120s; then
        print_success "Rollout complete!"
    else
        print_error "Rollout failed"
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi
}

# Build the agent (standardized)
cmd_build() {
    print_header "Building Agent"

    build_app

    print_success "Build completed: research-agent-resilience"
}

# Run the agent locally (standardized)
cmd_run() {
    print_header "Running Agent"

    load_env

    if [ -z "$REDIS_URL" ]; then
        print_error "REDIS_URL environment variable is required"
        print_info "Set it in .env file or export it: export REDIS_URL=redis://localhost:6379"
        exit 1
    fi

    # Build first
    cmd_build

    print_info "Starting research-agent-resilience on port $PORT..."
    print_info "Redis URL: $REDIS_URL"
    echo ""

    run_app
}

#############################################
# LEGACY COMMANDS (kept for compatibility)
#############################################

# Main setup (legacy)
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

STANDARDIZED 1-CLICK DEPLOYMENT COMMANDS:
  cluster       Create Kind cluster with port mappings
  infra         Setup infrastructure (Redis, Prometheus, Grafana, Jaeger)
  full-deploy   ONE-CLICK: cluster + infra + deploy + port forwards
  deploy        Build Docker images and deploy to Kubernetes
  forward       Port forward the agent service only
  forward-all   Port forward agent + monitoring dashboards
  test          Run resilience test scenario
  rollout       Restart deployment to pick up new secrets/config
                Use --build flag to rebuild Docker image first
  clean         Remove agent deployment only
  clean-all     Delete Kind cluster and all resources

Local Development Commands:
  build         Build the agent binary
  run           Build and run the agent locally
  run-all       Build and run ALL components locally (recommended)
                - Reuses existing Redis/services if available
                - Starts grocery-store-api + grocery-tool + agent

Kubernetes Deployment Commands:
  docker-build  Build Docker images for all components
  logs          View agent logs
  status        Check deployment status

Legacy Commands:
  setup         Setup the local development environment (default)
  redis         Setup Redis only
  build-all     Build all components (agent + mock-services)
  docker        Build Docker images (alias for docker-build)
  forward       Set up port forwards (legacy - use forward-all)
  cleanup       Remove deployed resources (alias for clean)

Environment Variables:
  CLUSTER_NAME      Kind cluster name (default: gomind-demo-\$(whoami))
  NAMESPACE         Kubernetes namespace (default: gomind-examples)
  PORT              HTTP server port (default: 8093)
  REDIS_URL         Redis connection URL (default: redis://localhost:6379)
  OPENAI_API_KEY    OpenAI API key (optional)
  ANTHROPIC_API_KEY Anthropic API key (optional)
  GROQ_API_KEY      Groq API key (optional)

Examples:
  $0 full-deploy    # ONE-CLICK: Complete deployment with monitoring
  $0 cluster        # Create Kind cluster only
  $0 infra          # Setup infrastructure only
  $0 deploy         # Deploy to existing cluster
  $0 forward-all    # Access all dashboards
  $0 test           # Run resilience tests
  $0 clean-all      # Delete everything

  $0 run-all        # Quick start: run everything locally
  REDIS_URL=redis://localhost:6379 $0 run
EOF
}

# Handle arguments
case "${1:-help}" in
    # Standardized commands
    cluster)
        cmd_cluster
        ;;
    infra)
        cmd_infra
        ;;
    full-deploy)
        cmd_full_deploy
        ;;
    deploy)
        cmd_deploy
        ;;
    forward)
        cmd_forward
        ;;
    forward-all)
        cmd_forward_all
        ;;
    test)
        cmd_test
        ;;
    clean)
        cmd_clean
        ;;
    clean-all)
        cmd_clean_all
        ;;
    build)
        cmd_build
        ;;
    run)
        cmd_run
        ;;
    docker-build)
        cmd_docker_build
        ;;
    logs)
        cmd_logs
        ;;
    status)
        cmd_status
        ;;
    rollout)
        cmd_rollout "$@"
        ;;
    # Legacy commands (kept for compatibility)
    setup)
        main
        ;;
    run-all)
        check_prerequisites
        run_all
        ;;
    redis)
        setup_redis
        ;;
    build-all)
        check_prerequisites
        build_all
        ;;
    docker)
        check_prerequisites
        build_docker
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
