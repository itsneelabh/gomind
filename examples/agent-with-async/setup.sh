#!/bin/bash

# setup.sh - One-click setup for Async Travel Research Agent
# Demonstrates GoMind async task system for long-running multi-tool orchestration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"
GOMIND_ROOT="$(dirname "$EXAMPLES_DIR")"

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="async-travel-agent"
AGENT_PORT=8098

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
    echo -e "${BLUE}║  GoMind Async Travel Research Agent                   ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Tool directories (dependencies)
GEOCODING_TOOL_DIR="$EXAMPLES_DIR/geocoding-tool"
WEATHER_TOOL_DIR="$EXAMPLES_DIR/weather-tool-v2"
CURRENCY_TOOL_DIR="$EXAMPLES_DIR/currency-tool"
NEWS_TOOL_DIR="$EXAMPLES_DIR/news-tool"
STOCK_TOOL_DIR="$EXAMPLES_DIR/stock-market-tool"
AGENT_DIR="$SCRIPT_DIR"

# Check prerequisites
check_prerequisites() {
    echo "Checking prerequisites..."

    # Check Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        echo "Please install Go 1.25+ from https://golang.org/dl/"
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

# Create Kind cluster with port mappings
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
  - containerPort: 30098
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

# Setup monitoring infrastructure
setup_monitoring_infrastructure() {
    log_info "Setting up monitoring infrastructure..."

    if [ -f "$EXAMPLES_DIR/k8-deployment/setup-infrastructure.sh" ]; then
        log_success "Found infrastructure setup script"
        echo ""
        NAMESPACE=$NAMESPACE "$EXAMPLES_DIR/k8-deployment/setup-infrastructure.sh"
        echo ""
        log_success "Monitoring infrastructure ready"
    else
        log_warn "Infrastructure setup script not found"
        echo "  Monitoring will not be available"
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
        exit 1
    fi

    echo ""
}

# Check for API keys
check_api_keys() {
    local found_keys=""

    # Check OpenAI
    if [ -n "$OPENAI_API_KEY" ]; then
        found_keys="OpenAI (env)"
    elif [ -f .env ] && grep -q "^OPENAI_API_KEY=sk-" .env; then
        found_keys="OpenAI (.env)"
    fi

    # Check Anthropic
    if [ -n "$ANTHROPIC_API_KEY" ]; then
        [ -n "$found_keys" ] && found_keys="$found_keys, "
        found_keys="${found_keys}Anthropic (env)"
    elif [ -f .env ] && grep -q "^ANTHROPIC_API_KEY=sk-ant-" .env; then
        [ -n "$found_keys" ] && found_keys="$found_keys, "
        found_keys="${found_keys}Anthropic (.env)"
    fi

    # Check Groq
    if [ -n "$GROQ_API_KEY" ]; then
        [ -n "$found_keys" ] && found_keys="$found_keys, "
        found_keys="${found_keys}Groq (env)"
    elif [ -f .env ] && grep -q "^GROQ_API_KEY=gsk_" .env; then
        [ -n "$found_keys" ] && found_keys="$found_keys, "
        found_keys="${found_keys}Groq (.env)"
    fi

    if [ -n "$found_keys" ]; then
        log_success "AI provider key(s) found: $found_keys"
        return 0
    else
        log_warn "No AI provider API keys configured"
        echo ""
        echo -e "${YELLOW}┌────────────────────────────────────────────────────────────┐${NC}"
        echo -e "${YELLOW}│  AI synthesis requires an API key                          │${NC}"
        echo -e "${YELLOW}├────────────────────────────────────────────────────────────┤${NC}"
        echo -e "${YELLOW}│  Without an API key, task processing works but AI          │${NC}"
        echo -e "${YELLOW}│  synthesis of results will be disabled.                    │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Configure at least ONE provider in your .env file:        │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│    OPENAI_API_KEY=sk-your-key                              │${NC}"
        echo -e "${YELLOW}│    ANTHROPIC_API_KEY=sk-ant-your-key                       │${NC}"
        echo -e "${YELLOW}│    GROQ_API_KEY=gsk_your-key                               │${NC}"
        echo -e "${YELLOW}│                                                            │${NC}"
        echo -e "${YELLOW}│  Multiple providers enable automatic failover.             │${NC}"
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
            echo "PORT=$AGENT_PORT" >> .env
            echo "WORKER_COUNT=3" >> .env
            echo -e "${GREEN}✓ Created default .env file${NC}"
        fi
    else
        echo -e "${GREEN}✓ .env file already exists${NC}"
    fi

    check_api_keys || true

    echo ""
}

# Build the application
build_app() {
    echo "Building application..."

    cd "$AGENT_DIR"
    GOWORK=off go mod download
    GOWORK=off go mod tidy
    GOWORK=off go build -o async-travel-agent .

    echo -e "${GREEN}✓ Application built successfully${NC}"
    echo ""
}

# Build all travel tools locally
build_tools() {
    log_info "Building travel tools..."

    local tools=("geocoding-tool" "weather-tool-v2" "currency-tool" "news-tool" "stock-market-tool")

    for tool in "${tools[@]}"; do
        local tool_dir="$EXAMPLES_DIR/$tool"
        if [ -d "$tool_dir" ]; then
            log_info "Building $tool..."
            (cd "$tool_dir" && GOWORK=off go build -o "$tool" . 2>/dev/null) && log_success "$tool built" || log_warn "$tool build failed"
        else
            log_warn "$tool directory not found"
        fi
    done
}

# Build Docker images
# Set DOCKER_NO_CACHE=true to rebuild with fresh dependencies
build_docker() {
    log_info "Building Docker images..."

    local no_cache_flag=""
    if [ "$DOCKER_NO_CACHE" = "true" ]; then
        log_info "Building with --no-cache (fresh dependency download)"
        no_cache_flag="--no-cache"
    fi

    # Build from agent directory using standalone Dockerfile (downloads from GitHub)
    cd "$AGENT_DIR"
    docker build $no_cache_flag -t $APP_NAME:latest .
    log_success "$APP_NAME:latest built"
}

# Load images to Kind
load_to_kind() {
    if ! command -v kind >/dev/null 2>&1; then
        log_warn "Kind not found, skipping image load"
        return
    fi

    local context=$(kubectl config current-context 2>/dev/null)
    local cluster_name=""

    if [[ "$context" == kind-* ]]; then
        cluster_name="${context#kind-}"
        log_info "Detected Kind cluster: $cluster_name"
    else
        cluster_name=$(kind get clusters 2>/dev/null | head -1)
        if [ -z "$cluster_name" ]; then
            log_error "No Kind clusters found"
            return 1
        fi
        log_info "Using Kind cluster: $cluster_name"
    fi

    log_info "Loading images to Kind cluster '$cluster_name'..."
    kind load docker-image --name "$cluster_name" $APP_NAME:latest

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
    else
        log_warn "No .env file found"
    fi
}

# Setup API keys as Kubernetes secrets
setup_k8s_secrets() {
    log_info "Setting up API keys as secrets..."

    local OPENAI_KEY=""
    local ANTHROPIC_KEY=""

    if [ -f "$AGENT_DIR/.env" ]; then
        OPENAI_KEY=$(grep "^OPENAI_API_KEY=" "$AGENT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
        ANTHROPIC_KEY=$(grep "^ANTHROPIC_API_KEY=" "$AGENT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
    fi

    if [ -z "$OPENAI_KEY" ] && [ -z "$ANTHROPIC_KEY" ]; then
        log_warn "No AI API keys found in .env file"
    else
        [ -n "$OPENAI_KEY" ] && log_success "Found OPENAI_API_KEY in .env"
        [ -n "$ANTHROPIC_KEY" ] && log_success "Found ANTHROPIC_API_KEY in .env"
    fi

    kubectl create secret generic ai-provider-keys-async-agent \
        --from-literal=OPENAI_API_KEY="${OPENAI_KEY}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -n $NAMESPACE -f -

    log_success "API keys configured as K8s secret (ai-provider-keys-async-agent)"
}

# Deploy to Kubernetes
deploy_k8s() {
    log_info "Deploying to Kubernetes..."

    load_env

    # Create namespace if not exists
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup secrets
    setup_k8s_secrets

    # Deploy the agent
    kubectl apply -f "$AGENT_DIR/k8-deployment.yaml"
    log_success "$APP_NAME deployed"

    # Rollout
    log_info "Rolling out new version..."
    kubectl rollout restart deployment/$APP_NAME -n $NAMESPACE
    kubectl rollout status deployment/$APP_NAME -n $NAMESPACE --timeout=120s

    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=$APP_NAME -n $NAMESPACE --timeout=120s 2>/dev/null || true

    log_success "Deployment complete!"
    log_info "Run '$0 forward' to set up port forwards"
}

# Deploy to Kubernetes in production mode (separate API and Worker)
deploy_k8s_production() {
    log_info "Deploying to Kubernetes in PRODUCTION mode (separate API + Worker)..."

    load_env

    # Create namespace if not exists
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup secrets
    setup_k8s_secrets

    # Deploy API component
    log_info "Deploying API component..."
    kubectl apply -f "$AGENT_DIR/k8-deployment-api.yaml"
    log_success "async-travel-agent-api deployed"

    # Deploy Worker component
    log_info "Deploying Worker component..."
    kubectl apply -f "$AGENT_DIR/k8-deployment-worker.yaml"
    log_success "async-travel-agent-worker deployed"

    # Rollout API
    log_info "Rolling out API..."
    kubectl rollout restart deployment/async-travel-agent-api -n $NAMESPACE 2>/dev/null || true
    kubectl rollout status deployment/async-travel-agent-api -n $NAMESPACE --timeout=120s 2>/dev/null || true

    # Rollout Worker
    log_info "Rolling out Worker..."
    kubectl rollout restart deployment/async-travel-agent-worker -n $NAMESPACE 2>/dev/null || true
    kubectl rollout status deployment/async-travel-agent-worker -n $NAMESPACE --timeout=120s 2>/dev/null || true

    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=async-travel-agent-api -n $NAMESPACE --timeout=120s 2>/dev/null || true
    kubectl wait --for=condition=ready pod -l app=async-travel-agent-worker -n $NAMESPACE --timeout=120s 2>/dev/null || true

    log_success "Production deployment complete!"
    log_info "API and Worker running as separate deployments"
    log_info "Run '$0 forward' to set up port forwards"

    # Show deployment status
    echo ""
    echo "Deployment Status:"
    kubectl get deployments -n $NAMESPACE -l framework=gomind | grep async-travel-agent || true
}

# Port forward (agent only)
port_forward() {
    log_info "Setting up agent port forward..."

    pkill -f "port-forward.*$APP_NAME" 2>/dev/null || true

    sleep 1

    kubectl port-forward -n $NAMESPACE svc/${APP_NAME}-service $AGENT_PORT:80 &

    sleep 3

    log_success "Port forward established:"
    echo "  - Agent: http://localhost:$AGENT_PORT"
    echo ""
    echo "Available Endpoints:"
    echo "  POST /api/v1/tasks           - Submit async task"
    echo "  GET  /api/v1/tasks/:id       - Get task status/result"
    echo "  POST /api/v1/tasks/:id/cancel - Cancel task"
    echo "  GET  /health                 - Health check"
    echo ""
    echo "Press Ctrl+C to stop port forwards"

    wait
}

# Port forward with monitoring
port_forward_all() {
    log_info "Setting up port forwards for agent and monitoring..."

    # Only kill this agent's port forward (preserve shared services for other agents)
    pkill -f "port-forward.*$APP_NAME" 2>/dev/null || true

    sleep 1

    # Start agent port forward
    kubectl port-forward -n $NAMESPACE svc/${APP_NAME}-service $AGENT_PORT:80 >/dev/null 2>&1 &

    # Start shared monitoring forwards ONLY if not already running
    if ! nc -z localhost 3000 2>/dev/null; then
        kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
        log_success "Grafana: http://localhost:3000"
    else
        log_success "Grafana: http://localhost:3000 (already forwarded, reusing)"
    fi

    if ! nc -z localhost 9090 2>/dev/null; then
        kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
        log_success "Prometheus: http://localhost:9090"
    else
        log_success "Prometheus: http://localhost:9090 (already forwarded, reusing)"
    fi

    if ! nc -z localhost 16686 2>/dev/null; then
        kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:80 >/dev/null 2>&1 &
        log_success "Jaeger: http://localhost:16686"
    else
        log_success "Jaeger: http://localhost:16686 (already forwarded, reusing)"
    fi

    sleep 2

    log_success "Port forwards established"
    print_summary
}

# Print summary
print_summary() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Setup Complete!                                 ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your Async Travel Research Agent is now running!"
    echo ""
    echo -e "${BLUE}Agent Endpoint:${NC}"
    echo "  http://localhost:$AGENT_PORT/health"
    echo ""
    echo -e "${BLUE}Monitoring Dashboards:${NC}"
    echo "  Grafana:    http://localhost:3000 (admin/admin)"
    echo "  Prometheus: http://localhost:9090"
    echo "  Jaeger:     http://localhost:16686"
    echo ""
    echo -e "${BLUE}Test the async task system:${NC}"
    echo "  # Submit a travel research task"
    echo "  curl -X POST http://localhost:$AGENT_PORT/api/v1/tasks \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"type\":\"travel_research\",\"input\":{\"destination\":\"Tokyo, Japan\",\"include_news\":true}}'"
    echo ""
    echo "  # Poll for status"
    echo "  curl http://localhost:$AGENT_PORT/api/v1/tasks/{TASK_ID}"
    echo ""
    echo -e "${BLUE}Useful commands:${NC}"
    echo "  kubectl get pods -n $NAMESPACE"
    echo "  kubectl logs -n $NAMESPACE -l app=$APP_NAME -f"
    echo "  $0 test            - Run test scenario"
    echo "  $0 cleanup         - Delete everything"
    echo ""
}

# Test async task
test_task() {
    log_info "Running async task test..."
    echo ""

    log_info "Step 1: Check health"
    curl -s http://localhost:$AGENT_PORT/health | jq . 2>/dev/null || echo "Request sent"
    echo ""

    log_info "Step 2: Submit travel research task"
    RESPONSE=$(curl -s -X POST http://localhost:$AGENT_PORT/api/v1/tasks \
        -H "Content-Type: application/json" \
        -d '{"type":"travel_research","input":{"destination":"Tokyo, Japan","include_news":true,"include_stocks":false}}')
    echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

    TASK_ID=$(echo "$RESPONSE" | jq -r '.task_id // .id' 2>/dev/null)
    echo ""

    if [ -n "$TASK_ID" ] && [ "$TASK_ID" != "null" ]; then
        log_info "Step 3: Poll for task status (task_id: $TASK_ID)"
        for i in {1..10}; do
            echo "  Polling attempt $i..."
            STATUS=$(curl -s http://localhost:$AGENT_PORT/api/v1/tasks/$TASK_ID)
            echo "$STATUS" | jq '{id, status, progress}' 2>/dev/null || echo "$STATUS"

            TASK_STATUS=$(echo "$STATUS" | jq -r '.status' 2>/dev/null)
            if [ "$TASK_STATUS" = "completed" ] || [ "$TASK_STATUS" = "failed" ]; then
                echo ""
                log_info "Final result:"
                echo "$STATUS" | jq . 2>/dev/null || echo "$STATUS"
                break
            fi
            sleep 3
        done
    fi

    echo ""
    log_success "Test complete!"
}

# Run the application locally
run_app() {
    echo "Starting Async Travel Agent..."
    echo ""
    echo "The agent will be available at: http://localhost:$AGENT_PORT"
    echo ""
    echo "Endpoints:"
    echo "  POST /api/v1/tasks           - Submit async task"
    echo "  GET  /api/v1/tasks/:id       - Get task status/result"
    echo "  POST /api/v1/tasks/:id/cancel - Cancel task"
    echo "  GET  /health                 - Health check"
    echo ""
    echo "Press Ctrl+C to stop"
    echo "=============================================="
    echo ""

    if [ -f .env ]; then
        export $(cat .env | grep -v '^#' | xargs)
    fi

    export REDIS_URL=${REDIS_URL:-"redis://localhost:6379"}
    export PORT=${PORT:-$AGENT_PORT}
    export WORKER_COUNT=${WORKER_COUNT:-3}

    ./async-travel-agent
}

# Run all components locally (smart detection)
run_all() {
    log_info "Starting all components for local development..."
    echo ""

    local started_pids=()

    cleanup_local() {
        echo ""
        log_info "Shutting down..."
        for pid in "${started_pids[@]}"; do
            kill "$pid" 2>/dev/null || true
        done
        pkill -f "async-travel-agent" 2>/dev/null || true
        pkill -f "geocoding-tool" 2>/dev/null || true
        pkill -f "weather-tool-v2" 2>/dev/null || true
        pkill -f "currency-tool" 2>/dev/null || true
        pkill -f "news-tool" 2>/dev/null || true
        pkill -f "stock-market-tool" 2>/dev/null || true
        log_success "All components stopped"
    }
    trap cleanup_local EXIT INT TERM

    # 1. Ensure Redis
    setup_redis

    # 2. Load environment
    setup_env

    # 3. Build agent
    build_app

    # 4. Check for deployed tools
    echo ""
    log_info "Checking for deployed travel tools..."
    local tools_available=0

    for port in 8085 8086 8087 8088 8089; do
        if nc -z localhost "$port" 2>/dev/null; then
            tools_available=$((tools_available + 1))
        fi
    done

    if [ $tools_available -gt 0 ]; then
        log_success "Found $tools_available travel tools available"
    else
        log_warn "No travel tools found on expected ports"
        echo "  Starting tools locally..."
        build_tools

        # Start tools in background
        for tool_info in "geocoding-tool:8085" "weather-tool-v2:8086" "currency-tool:8087" "stock-market-tool:8088" "news-tool:8089"; do
            tool="${tool_info%%:*}"
            port="${tool_info##*:}"
            tool_dir="$EXAMPLES_DIR/$tool"
            if [ -f "$tool_dir/$tool" ]; then
                log_info "Starting $tool on port $port..."
                (cd "$tool_dir" && REDIS_URL="redis://localhost:6379" PORT=$port ./$tool) &
                started_pids+=($!)
                sleep 1
            fi
        done
        sleep 2
    fi

    # 5. Run agent
    run_app
}

# Rollout - restart deployment
rollout() {
    print_header
    log_info "Rolling out deployment..."

    local rebuild=false
    if [ "$2" = "--build" ] || [ "$2" = "build" ]; then
        rebuild=true
    fi

    load_env
    setup_k8s_secrets

    if [ "$rebuild" = true ]; then
        log_info "Rebuilding Docker image..."
        build_docker
        load_to_kind
    fi

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

    pkill -f "port-forward.*$AGENT_PORT" 2>/dev/null || true
    pkill -f "async-travel-agent" 2>/dev/null || true

    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    log_success "Cleanup complete"
}

# Rebuild with no-cache and redeploy
# This ensures fresh dependencies are downloaded from GitHub
cmd_rebuild() {
    print_header
    log_info "Rebuilding with Fresh Dependencies"

    load_env

    # Build Docker image with --no-cache
    log_info "Building Docker image with --no-cache..."
    DOCKER_NO_CACHE=true build_docker

    # Load image into kind cluster if available
    if command -v kind &> /dev/null; then
        log_info "Loading image into kind cluster..."
        kind load docker-image $APP_NAME:latest --name "$CLUSTER_NAME"
        log_success "Image loaded"
    fi

    # Create namespace
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup API keys if function exists
    if type setup_k8s_secrets &>/dev/null; then
        setup_k8s_secrets
    fi

    # Apply Kubernetes manifests
    log_info "Applying Kubernetes manifests..."
    kubectl apply -f "$AGENT_DIR/k8-deployment.yaml"

    # Restart deployment to pick up new image
    log_info "Restarting deployment..."
    kubectl rollout restart deployment/$APP_NAME -n $NAMESPACE

    log_info "Waiting for deployment to be ready..."
    if kubectl rollout status deployment/$APP_NAME -n $NAMESPACE --timeout=120s; then
        log_success "$APP_NAME rebuilt and deployed with fresh dependencies!"
    else
        log_error "Deployment failed. Checking logs..."
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi
}

# Full deployment
full_deploy() {
    print_header
    log_info "Starting full deployment..."
    echo ""

    create_kind_cluster
    setup_monitoring_infrastructure
    load_env
    build_docker
    load_to_kind
    deploy_k8s
    port_forward_all
}

# Cleanup all including Kind cluster
cleanup_all() {
    log_info "Cleaning up everything..."

    pkill -f "port-forward.*$NAMESPACE" 2>/dev/null || true

    kubectl delete -f "$AGENT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Deleting Kind cluster $CLUSTER_NAME..."
        kind delete cluster --name $CLUSTER_NAME
        log_success "Kind cluster deleted"
    fi

    log_success "Full cleanup complete"
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
  redis      Setup Redis only
  build      Build the agent only
  tools      Build all travel tools locally

Kubernetes Cluster Commands:
  cluster        Create a Kind cluster with port mappings
  infra          Setup monitoring infrastructure (Prometheus, Grafana, Jaeger)
  full-deploy    Complete deployment: cluster + infra + agent

Kubernetes Deployment Commands:
  docker         Build Docker images
  deploy         Build, load to Kind, and deploy to Kubernetes (embedded mode)
  rebuild        Rebuild with --no-cache and redeploy (fresh dependencies)
  deploy-prod    Deploy in production mode (separate API + Worker)
  forward        Port forward agent only
  forward-all    Port forward agent + monitoring (recommended)
  test           Run async task test scenario
  rollout        Restart deployment (use --build to rebuild image first)
  cleanup        Remove deployed resources
  cleanup-all    Delete Kind cluster and all resources

Examples:
  # Quick local development
  $0 run-all          # Run with tools

  # Full Kubernetes deployment
  $0 full-deploy      # Creates cluster, infrastructure, deploys agent

  # Step-by-step deployment
  $0 cluster          # Create Kind cluster
  $0 infra            # Setup monitoring
  $0 deploy           # Deploy agent
  $0 forward-all      # Port forward everything

  # Test async tasks
  $0 test             # Submit and poll a travel research task
EOF
}

# Handle arguments
case "${1:-setup}" in
    setup)
        print_header
        check_prerequisites
        setup_redis
        setup_env
        build_app
        echo "=============================================="
        echo "Setup complete!"
        echo "Run '$0 run' to start the agent"
        echo "=============================================="
        ;;
    run)
        print_header
        check_prerequisites
        setup_redis
        setup_env
        build_app
        run_app
        ;;
    run-all)
        print_header
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
    rebuild)
        cmd_rebuild
        ;;
    deploy-prod)
        check_prerequisites
        build_docker
        load_to_kind
        # First cleanup embedded deployment if exists
        kubectl delete deployment/$APP_NAME -n $NAMESPACE 2>/dev/null || true
        deploy_k8s_production
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
        test_task
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
