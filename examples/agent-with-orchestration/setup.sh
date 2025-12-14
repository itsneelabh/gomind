#!/bin/bash

# setup.sh - One-click setup for travel-research-agent with orchestration
# This script sets up the local development environment and can deploy to Kubernetes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="travel-research-agent"
AGENT_PORT=8094

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
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  GoMind Travel Research Agent with Orchestration      ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Tool directories
GEOCODING_TOOL_DIR="$EXAMPLES_DIR/geocoding-tool"
WEATHER_TOOL_DIR="$EXAMPLES_DIR/weather-tool-v2"
CURRENCY_TOOL_DIR="$EXAMPLES_DIR/currency-tool"
COUNTRY_INFO_TOOL_DIR="$EXAMPLES_DIR/country-info-tool"
NEWS_TOOL_DIR="$EXAMPLES_DIR/news-tool"
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

    # Check kind (optional for k8s)
    if command -v kind &> /dev/null; then
        echo -e "${GREEN}✓ Kind installed${NC}"
        KIND_AVAILABLE=true
    else
        echo -e "${YELLOW}! Kind not found (optional for K8s deployment)${NC}"
        KIND_AVAILABLE=false
    fi

    echo ""
}

# Create Kind cluster with port mappings (like agent-with-telemetry)
create_kind_cluster() {
    log_info "Setting up Kind cluster ($CLUSTER_NAME)..."

    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_success "Cluster $CLUSTER_NAME already exists, reusing it"
    else
        cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  # Agent port
  - containerPort: 30094
    hostPort: $AGENT_PORT
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
        log_success "Kind cluster created with port mappings"
    fi

    kubectl config use-context kind-$CLUSTER_NAME
    echo ""
}

# Setup monitoring infrastructure (Prometheus, Grafana, Jaeger, OTEL Collector)
setup_monitoring_infrastructure() {
    log_info "Setting up monitoring infrastructure..."

    # Use the shared infrastructure setup script
    if [ -f "$EXAMPLES_DIR/k8-deployment/setup-infrastructure.sh" ]; then
        log_success "Found infrastructure setup script"
        echo ""

        # Run the infrastructure setup
        NAMESPACE=$NAMESPACE "$EXAMPLES_DIR/k8-deployment/setup-infrastructure.sh"

        echo ""
        log_success "Monitoring infrastructure ready"
    else
        log_warn "Infrastructure setup script not found"
        echo "  Monitoring will not be available"
        echo "  Ensure k8-deployment/setup-infrastructure.sh exists"
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
        echo -e "${YELLOW}│  AI Features Require an API Key                            │${NC}"
        echo -e "${YELLOW}├────────────────────────────────────────────────────────────┤${NC}"
        echo -e "${YELLOW}│  Without an API key, predefined workflows will work but    │${NC}"
        echo -e "${YELLOW}│  natural language orchestration will be limited.           │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  To add your API key:                                      │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Option 1: Add to .env file                                │${NC}"
        echo -e "${YELLOW}│    echo 'OPENAI_API_KEY=sk-your-key' >> .env               │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Option 2: Export environment variable                     │${NC}"
        echo -e "${YELLOW}│    export OPENAI_API_KEY=sk-your-key                       │${NC}"
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
            echo "PORT=8094" >> .env
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
    GOWORK=off go build -o travel-research-agent .

    echo -e "${GREEN}✓ Application built successfully${NC}"
    echo ""
}

# Build all travel tools locally
build_tools() {
    log_info "Building travel tools..."

    local tools=("geocoding-tool" "weather-tool-v2" "currency-tool" "country-info-tool" "news-tool")

    for tool in "${tools[@]}"; do
        local tool_dir="$EXAMPLES_DIR/$tool"
        if [ -d "$tool_dir" ]; then
            log_info "Building $tool..."
            (cd "$tool_dir" && GOWORK=off go build -o "$tool" . 2>/dev/null) && log_success "$tool built" || log_warn "$tool build failed (may not exist yet)"
        else
            log_warn "$tool directory not found"
        fi
    done
}

# Build Docker images
build_docker() {
    log_info "Building Docker images..."

    # Build using the standalone Dockerfile (fetches gomind from GitHub)
    docker build -t travel-research-agent:latest "$AGENT_DIR"
    log_success "travel-research-agent:latest built"
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
    kind load docker-image --name "$cluster_name" travel-research-agent:latest

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

    # Read API keys directly from .env file (more reliable than source)
    local OPENAI_KEY=""
    local ANTHROPIC_KEY=""
    local GROQ_KEY=""

    if [ -f "$AGENT_DIR/.env" ]; then
        OPENAI_KEY=$(grep "^OPENAI_API_KEY=" "$AGENT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
        ANTHROPIC_KEY=$(grep "^ANTHROPIC_API_KEY=" "$AGENT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
        GROQ_KEY=$(grep "^GROQ_API_KEY=" "$AGENT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
    fi

    # Check for AI API keys
    if [ -z "$OPENAI_KEY" ] && [ -z "$ANTHROPIC_KEY" ] && [ -z "$GROQ_KEY" ]; then
        log_warn "No AI API keys found in .env file"
        echo ""
        echo "To enable AI features, add API keys to your .env file:"
        echo "  OPENAI_API_KEY=your-key"
        echo ""
    else
        if [ -n "$OPENAI_KEY" ]; then
            log_success "Found OPENAI_API_KEY in .env"
        fi
        if [ -n "$ANTHROPIC_KEY" ]; then
            log_success "Found ANTHROPIC_API_KEY in .env"
        fi
        if [ -n "$GROQ_KEY" ]; then
            log_success "Found GROQ_API_KEY in .env"
        fi
    fi

    # Create AI provider keys secret
    kubectl create secret generic ai-provider-keys \
        --from-literal=OPENAI_API_KEY="${OPENAI_KEY}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY}" \
        --from-literal=GROQ_API_KEY="${GROQ_KEY}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    log_success "API keys configured as K8s secret"
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

    # Deploy the agent
    kubectl apply -f "$AGENT_DIR/k8-deployment.yaml"
    log_success "travel-research-agent deployed"

    # Force rollout to pick up new image (needed when using :latest tag)
    log_info "Rolling out new version..."
    kubectl rollout restart deployment/travel-research-agent -n gomind-examples
    kubectl rollout status deployment/travel-research-agent -n gomind-examples --timeout=120s

    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=travel-research-agent -n gomind-examples --timeout=120s 2>/dev/null || true

    log_success "Deployment complete!"
    log_info "Run '$0 forward' to set up port forwards"
}

# Port forward (agent only)
port_forward() {
    log_info "Setting up agent port forward..."

    # Kill existing port forwards for agent
    pkill -f "port-forward.*travel-research-agent" 2>/dev/null || true

    sleep 1

    kubectl port-forward -n $NAMESPACE svc/travel-research-agent-service $AGENT_PORT:80 &

    sleep 3

    log_success "Port forward established:"
    echo "  - Agent: http://localhost:$AGENT_PORT"
    echo ""
    echo "Available Endpoints:"
    echo "  POST /api/orchestrate/natural         - Natural language requests"
    echo "  POST /api/orchestrate/travel-research - Predefined travel workflow"
    echo "  POST /api/orchestrate/custom          - Custom workflow execution"
    echo "  GET  /api/orchestrate/workflows       - List available workflows"
    echo "  GET  /api/orchestrate/history         - Execution history"
    echo "  GET  /health                          - Health check with metrics"
    echo ""
    echo "Press Ctrl+C to stop port forwards"

    wait
}

# Port forward with monitoring (agent + Grafana, Prometheus, Jaeger)
port_forward_all() {
    log_info "Setting up port forwards for agent and monitoring..."

    # Kill existing port forwards
    pkill -f "port-forward.*$NAMESPACE" 2>/dev/null || true

    sleep 2

    # Start port forwarding in background
    kubectl port-forward -n $NAMESPACE svc/travel-research-agent-service $AGENT_PORT:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/otel-collector 4318:4318 >/dev/null 2>&1 &

    sleep 3

    log_success "Port forwards established"
    print_summary
}

# Print summary after deployment
print_summary() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Setup Complete! 🎉                              ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your Travel Research Agent with Orchestration is now running!"
    echo ""
    echo -e "${BLUE}🚀 Agent Endpoint:${NC}"
    echo "  http://localhost:$AGENT_PORT/health"
    echo ""
    echo -e "${BLUE}📊 Monitoring Dashboards:${NC}"
    echo "  Grafana:    http://localhost:3000 (admin/admin)"
    echo "  Prometheus: http://localhost:9090"
    echo "  Jaeger:     http://localhost:16686"
    echo ""
    echo -e "${BLUE}🧪 Test the orchestration:${NC}"
    echo "  # List available workflows"
    echo "  curl http://localhost:$AGENT_PORT/api/orchestrate/workflows | jq ."
    echo ""
    echo "  # Execute travel research workflow"
    echo "  curl -X POST http://localhost:$AGENT_PORT/api/orchestrate/travel-research \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"destination\": \"Tokyo, Japan\", \"country\": \"Japan\", \"base_currency\": \"USD\", \"amount\": 1000}'"
    echo ""
    echo -e "${BLUE}📈 View telemetry:${NC}"
    echo "  1. Open Grafana: http://localhost:3000"
    echo "  2. Traces in Jaeger: http://localhost:16686"
    echo "  3. Metrics in Prometheus: http://localhost:9090"
    echo ""
    echo -e "${BLUE}🔧 Useful commands:${NC}"
    echo "  kubectl get pods -n $NAMESPACE"
    echo "  kubectl logs -n $NAMESPACE -l app=$APP_NAME -f"
    echo "  $0 test            - Run orchestration test"
    echo "  $0 cleanup         - Delete everything"
    echo ""
    echo -e "${YELLOW}💡 Port forwards are running in the background${NC}"
    echo "   To stop them: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
}

# Test orchestration
test_orchestration() {
    log_info "Running orchestration test..."
    echo ""

    # Test list workflows
    log_info "Step 1: List available workflows"
    curl -s http://localhost:8094/api/orchestrate/workflows | jq . 2>/dev/null || echo "Request sent"
    echo ""

    # Test discover tools
    log_info "Step 2: Discover available tools"
    curl -s http://localhost:8094/api/discover | jq '.discovery_summary' 2>/dev/null || echo "Request sent"
    echo ""

    # Test health
    log_info "Step 3: Check health"
    curl -s http://localhost:8094/health | jq '{status, orchestrator, ai}' 2>/dev/null || echo "Request sent"
    echo ""

    # Test natural language (if AI is configured)
    log_info "Step 4: Test natural language orchestration"
    curl -s -X POST http://localhost:8094/api/orchestrate/natural \
        -H "Content-Type: application/json" \
        -d '{"request":"What is the weather like in Tokyo?","use_ai":true}' | jq '{request_id, tools_used, confidence}' 2>/dev/null || echo "Request sent"
    echo ""

    log_success "Orchestration test complete!"
}

# Rollout - restart deployment to pick up new secrets/config
rollout() {
    print_header
    log_info "Rolling out deployment..."

    local rebuild=false

    # Check for --build flag
    if [ "$2" = "--build" ] || [ "$2" = "build" ]; then
        rebuild=true
    fi

    # Load env to update secrets
    load_env

    # Update secrets from .env
    log_info "Updating secrets from .env..."
    setup_k8s_secrets

    # Rebuild if requested
    if [ "$rebuild" = true ]; then
        log_info "Rebuilding Docker image..."
        build_docker

        if command -v kind &> /dev/null; then
            log_info "Loading image into kind cluster..."
            load_to_kind
            log_success "Image loaded"
        fi
    fi

    # Restart deployment
    log_info "Restarting deployment..."
    kubectl rollout restart deployment/$APP_NAME -n $NAMESPACE

    log_info "Waiting for rollout to complete..."
    if kubectl rollout status deployment/$APP_NAME -n $NAMESPACE --timeout=120s; then
        log_success "Rollout complete!"
    else
        log_error "Rollout failed"
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."

    # Stop port forwards
    pkill -f "port-forward.*8094" 2>/dev/null || true

    # Delete K8s resources
    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

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
    echo "Starting Travel Research Agent with Orchestration..."
    echo ""
    echo "The agent will be available at: http://localhost:8094"
    echo ""
    echo "Endpoints:"
    echo "  POST /api/orchestrate/natural         - Natural language requests"
    echo "  POST /api/orchestrate/travel-research - Predefined travel workflow"
    echo "  POST /api/orchestrate/custom          - Custom workflow execution"
    echo "  GET  /api/orchestrate/workflows       - List available workflows"
    echo "  GET  /api/orchestrate/history         - Execution history"
    echo "  GET  /api/discover                    - Discover tools"
    echo "  GET  /health                          - Health with orchestrator status"
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
    export PORT=${PORT:-8094}

    ./travel-research-agent
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

    # 3. Build agent (tools should already be deployed)
    build_app

    # 4. Verify travel tools are available (port forward or check K8s)
    echo ""
    log_info "Checking for deployed travel tools..."
    local tools_available=0

    # Check if tools are accessible via K8s port-forwards
    for port in 8085 8086 8087 8088 8089; do
        if nc -z localhost "$port" 2>/dev/null; then
            tools_available=$((tools_available + 1))
        fi
    done

    if [ $tools_available -gt 0 ]; then
        log_success "Found $tools_available travel tools available"
    else
        log_warn "No travel tools found on expected ports"
        echo "  The agent will work but workflows may fail without tools"
        echo "  Deploy tools using: kubectl apply -f examples/*/k8-deployment.yaml"
    fi

    # 5. Run the agent in foreground
    run_app
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
    print_header
    cat << EOF
Usage: $0 <command>

Local Development Commands:
  setup      Setup the local development environment (default)
  run        Setup and run the agent only
  run-all    Build and run with smart tool detection (recommended)
             - Reuses existing Redis/services if available
             - Detects deployed travel tools
  redis      Setup Redis only
  build      Build the agent only
  tools      Build all travel tools locally

Kubernetes Cluster Commands:
  cluster        Create a Kind cluster with port mappings
  infra          Setup monitoring infrastructure (Prometheus, Grafana, Jaeger, OTEL)
  full-deploy    Complete deployment: cluster + infra + agent

Kubernetes Deployment Commands:
  docker         Build Docker images
  deploy         Build, load to Kind, and deploy to Kubernetes
  forward        Port forward agent only
  forward-all    Port forward agent + monitoring (recommended)
  test           Run the orchestration test scenario
  rollout        Restart deployment to pick up new secrets/config
                 Use --build flag to rebuild Docker image first
  cleanup        Remove all deployed resources (agent only)
  cleanup-all    Delete Kind cluster and all resources

Examples:
  # Quick local development
  $0 run-all          # Run with existing tools

  # Full Kubernetes deployment (recommended)
  $0 full-deploy      # Creates cluster, infrastructure, and deploys agent

  # Step-by-step deployment
  $0 cluster          # Create Kind cluster
  $0 infra            # Setup monitoring
  $0 deploy           # Deploy agent
  $0 forward-all      # Port forward everything

  # Test and observe
  $0 test             # Run orchestration test
  # Open Grafana: http://localhost:3000
  # Open Jaeger:  http://localhost:16686
EOF
}

# Full deployment: cluster + infrastructure + agent
full_deploy() {
    print_header
    log_info "Starting full deployment..."
    echo ""

    # Step 1: Create Kind cluster
    create_kind_cluster

    # Step 2: Setup monitoring infrastructure
    setup_monitoring_infrastructure

    # Step 3: Load environment for secrets
    load_env

    # Step 4: Build and deploy agent
    build_docker
    load_to_kind
    deploy_k8s

    # Step 5: Setup port forwards
    port_forward_all
}

# Cleanup everything including Kind cluster
cleanup_all() {
    log_info "Cleaning up everything..."

    # Stop port forwards
    pkill -f "port-forward.*$NAMESPACE" 2>/dev/null || true

    # Delete K8s resources
    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    # Stop local Redis
    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    # Delete Kind cluster
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Deleting Kind cluster $CLUSTER_NAME..."
        kind delete cluster --name $CLUSTER_NAME
        log_success "Kind cluster deleted"
    fi

    log_success "Full cleanup complete"
}

# Handle arguments
case "${1:-setup}" in
    setup)
        main
        ;;
    run)
        main run
        ;;
    run-all)
        check_prerequisites
        run_all
        ;;
    redis)
        setup_redis
        ;;
    build)
        build_app
        ;;
    tools)
        check_prerequisites
        build_tools
        ;;
    cluster)
        check_prerequisites
        print_header
        create_kind_cluster
        ;;
    infra)
        check_prerequisites
        print_header
        setup_monitoring_infrastructure
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
    full-deploy)
        check_prerequisites
        full_deploy
        ;;
    forward)
        port_forward
        ;;
    forward-all)
        port_forward_all
        ;;
    test)
        test_orchestration
        ;;
    cleanup)
        cleanup
        ;;
    rollout)
        rollout "$@"
        ;;
    cleanup-all)
        cleanup_all
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
