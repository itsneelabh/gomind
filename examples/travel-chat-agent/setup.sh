#!/bin/bash

# setup.sh - One-click setup for travel-chat-agent and chat-ui
# This script sets up the local development environment and can deploy to Kubernetes
# Modeled after agent-with-orchestration/setup.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="$(dirname "$SCRIPT_DIR")"
CHAT_UI_DIR="$EXAMPLES_DIR/chat-ui"

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="travel-chat-agent"
AGENT_PORT=8095
UI_PORT=8096

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
    echo -e "${BLUE}║     GoMind Travel Chat Agent + Chat UI                ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        echo "Please install Go 1.23+ from https://golang.org/dl/"
        exit 1
    fi
    log_success "Go installed: $(go version)"

    # Check Docker (optional)
    if command -v docker &> /dev/null; then
        log_success "Docker installed"
        DOCKER_AVAILABLE=true
    else
        log_warn "Docker not found (required for K8s deployment)"
        DOCKER_AVAILABLE=false
    fi

    # Check kubectl (optional)
    if command -v kubectl &> /dev/null; then
        log_success "kubectl installed"
        KUBECTL_AVAILABLE=true
    else
        log_warn "kubectl not found (required for K8s deployment)"
        KUBECTL_AVAILABLE=false
    fi

    # Check kind (optional for k8s)
    if command -v kind &> /dev/null; then
        log_success "Kind installed"
        KIND_AVAILABLE=true
    else
        log_warn "Kind not found (required for local K8s deployment)"
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
  # Chat Agent port
  - containerPort: 30095
    hostPort: $AGENT_PORT
    protocol: TCP
  # Chat UI port
  - containerPort: 30096
    hostPort: $UI_PORT
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
    log_info "Setting up Redis..."

    # Check if Redis is already running
    if command -v redis-cli &> /dev/null; then
        if redis-cli ping &> /dev/null; then
            log_success "Redis is already running"
            return 0
        fi
    fi

    # Try Docker Redis
    if [ "$DOCKER_AVAILABLE" = true ]; then
        log_info "Starting Redis via Docker..."

        # Stop existing container if any
        docker stop gomind-redis 2>/dev/null || true
        docker rm gomind-redis 2>/dev/null || true

        # Start Redis
        docker run -d \
            --name gomind-redis \
            -p 6379:6379 \
            redis:7-alpine

        log_success "Redis started on port 6379"
    else
        log_error "Redis not available"
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
    log_info "Setting up environment..."

    if [ ! -f .env ]; then
        echo "REDIS_URL=redis://localhost:6379" > .env
        echo "PORT=8095" >> .env
        echo "APP_ENV=development" >> .env
        log_success "Created default .env file"
    else
        log_success ".env file already exists"
    fi

    # Check for API keys
    check_api_keys || true

    echo ""
}

# Load environment variables from .env file
load_env() {
    log_info "Loading environment variables..."

    if [ -f "$SCRIPT_DIR/.env" ]; then
        set -a
        source "$SCRIPT_DIR/.env"
        set +a
        log_success "Loaded .env file"
    else
        log_warn "No .env file found"
    fi
}

# Build the application (local only)
build_app() {
    log_info "Building travel-chat-agent..."

    cd "$SCRIPT_DIR"

    # Download dependencies
    GOWORK=off go mod download
    GOWORK=off go mod tidy

    # Build
    GOWORK=off go build -o travel-chat-agent .

    log_success "Application built successfully"
    echo ""
}

# Build Docker images (chat agent and chat UI)
build_docker() {
    log_info "Building Docker images..."

    local no_cache_flag=""
    if [ "$DOCKER_NO_CACHE" = "true" ]; then
        log_info "Building with --no-cache (fresh dependency download)"
        no_cache_flag="--no-cache"
    fi

    # Build chat agent using workspace Dockerfile (includes local module changes)
    local GOMIND_ROOT="$(dirname "$EXAMPLES_DIR")"
    docker build $no_cache_flag -f "$SCRIPT_DIR/Dockerfile.workspace" -t travel-chat-agent:latest "$GOMIND_ROOT"
    log_success "travel-chat-agent:latest built"

    # Build chat UI (using nginx)
    if [ -d "$CHAT_UI_DIR" ]; then
        log_info "Building chat-ui Docker image..."

        # Use existing Dockerfile (has proper permissions and nginx config)
        if [ -f "$CHAT_UI_DIR/Dockerfile" ]; then
            docker build $no_cache_flag -t chat-ui:latest "$CHAT_UI_DIR"
            log_success "chat-ui:latest built"
        else
            log_error "chat-ui Dockerfile not found"
        fi
    else
        log_warn "chat-ui directory not found"
    fi
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
        # Check if there's a matching cluster
        if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
            cluster_name="$CLUSTER_NAME"
            log_info "Using Kind cluster: $cluster_name"
        else
            # Use the first available Kind cluster
            cluster_name=$(kind get clusters 2>/dev/null | head -1)
            if [ -z "$cluster_name" ]; then
                log_error "No Kind clusters found. Please create one with: $0 cluster"
                return 1
            fi
            log_info "Using Kind cluster: $cluster_name"
        fi
    fi

    log_info "Loading images to Kind cluster '$cluster_name'..."
    kind load docker-image --name "$cluster_name" travel-chat-agent:latest

    # Load chat-ui if built
    if docker image inspect chat-ui:latest &>/dev/null; then
        kind load docker-image --name "$cluster_name" chat-ui:latest
        log_success "chat-ui image loaded"
    fi

    log_success "Images loaded to Kind"
}

# Setup API keys as Kubernetes secrets
setup_k8s_secrets() {
    log_info "Setting up API keys as secrets..."

    # Read API keys directly from .env file
    local OPENAI_KEY=""
    local ANTHROPIC_KEY=""
    local GROQ_KEY=""

    if [ -f "$SCRIPT_DIR/.env" ]; then
        OPENAI_KEY=$(grep "^OPENAI_API_KEY=" "$SCRIPT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
        ANTHROPIC_KEY=$(grep "^ANTHROPIC_API_KEY=" "$SCRIPT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
        GROQ_KEY=$(grep "^GROQ_API_KEY=" "$SCRIPT_DIR/.env" 2>/dev/null | cut -d'=' -f2 || echo "")
    fi

    # Check for AI API keys
    if [ -z "$OPENAI_KEY" ] && [ -z "$ANTHROPIC_KEY" ] && [ -z "$GROQ_KEY" ]; then
        log_warn "No AI API keys found in .env file"
        echo ""
        echo "To enable AI features, add API keys to your .env file:"
        echo "  OPENAI_API_KEY=your-key"
        echo ""
    else
        [ -n "$OPENAI_KEY" ] && log_success "Found OPENAI_API_KEY in .env"
        [ -n "$ANTHROPIC_KEY" ] && log_success "Found ANTHROPIC_API_KEY in .env"
        [ -n "$GROQ_KEY" ] && log_success "Found GROQ_API_KEY in .env"
    fi

    # Create AI provider keys secret
    kubectl create secret generic ai-provider-keys-chat-agent \
        --from-literal=OPENAI_API_KEY="${OPENAI_KEY}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_KEY}" \
        --from-literal=GROQ_API_KEY="${GROQ_KEY}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -n $NAMESPACE -f -

    log_success "API keys configured as K8s secret (ai-provider-keys-chat-agent)"
}

# Deploy to Kubernetes (both chat agent and chat UI)
deploy_k8s() {
    log_info "Deploying to Kubernetes..."

    # Load environment and setup secrets
    load_env

    # Create namespace if not exists
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup secrets
    setup_k8s_secrets

    # Deploy the chat agent
    kubectl apply -f "$SCRIPT_DIR/k8-deployment.yaml"
    log_success "travel-chat-agent deployed"

    # Deploy the chat UI
    if [ -f "$CHAT_UI_DIR/k8-deployment.yaml" ]; then
        kubectl apply -f "$CHAT_UI_DIR/k8-deployment.yaml"
        log_success "chat-ui deployed"
    else
        log_warn "chat-ui k8-deployment.yaml not found, skipping UI deployment"
    fi

    # Force rollout to pick up new images
    log_info "Rolling out new versions..."
    kubectl rollout restart deployment/travel-chat-agent -n $NAMESPACE
    kubectl rollout status deployment/travel-chat-agent -n $NAMESPACE --timeout=120s

    if kubectl get deployment chat-ui -n $NAMESPACE &>/dev/null; then
        kubectl rollout restart deployment/chat-ui -n $NAMESPACE
        kubectl rollout status deployment/chat-ui -n $NAMESPACE --timeout=60s
    fi

    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=travel-chat-agent -n $NAMESPACE --timeout=120s 2>/dev/null || true

    log_success "Deployment complete!"
    log_info "Run '$0 forward' to set up port forwards"
}

# Port forward (agent only)
port_forward() {
    log_info "Setting up port forwards..."

    # Kill existing port forwards
    pkill -f "port-forward.*travel-chat-agent" 2>/dev/null || true
    pkill -f "port-forward.*chat-ui" 2>/dev/null || true

    sleep 1

    kubectl port-forward -n $NAMESPACE svc/travel-chat-agent-service $AGENT_PORT:80 &

    if kubectl get svc chat-ui-service -n $NAMESPACE &>/dev/null; then
        kubectl port-forward -n $NAMESPACE svc/chat-ui-service $UI_PORT:80 &
    fi

    sleep 3

    log_success "Port forwards established:"
    echo "  - Chat Agent: http://localhost:$AGENT_PORT"
    echo "  - Chat UI:    http://localhost:$UI_PORT"
    echo ""
    echo "Press Ctrl+C to stop port forwards"

    wait
}

# Port forward with monitoring
port_forward_all() {
    log_info "Setting up port forwards for agent, UI, and monitoring..."

    # Kill only port forwards for this agent's services (preserve jaeger, registry-viewer, etc.)
    pkill -f "port-forward.*travel-chat-agent" 2>/dev/null || true
    pkill -f "port-forward.*chat-ui" 2>/dev/null || true
    pkill -f "port-forward.*grafana" 2>/dev/null || true
    pkill -f "port-forward.*prometheus" 2>/dev/null || true
    pkill -f "port-forward.*otel-collector.*4318" 2>/dev/null || true

    sleep 2

    # Start port forwarding in background
    kubectl port-forward -n $NAMESPACE svc/travel-chat-agent-service $AGENT_PORT:80 >/dev/null 2>&1 &

    if kubectl get svc chat-ui-service -n $NAMESPACE &>/dev/null; then
        kubectl port-forward -n $NAMESPACE svc/chat-ui-service $UI_PORT:80 >/dev/null 2>&1 &
    fi

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
    echo -e "${GREEN}║       Setup Complete!                                 ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your Travel Chat Agent and UI are now running!"
    echo ""
    echo -e "${BLUE}Chat Application:${NC}"
    echo "  Chat UI:    http://localhost:$UI_PORT"
    echo "  Chat API:   http://localhost:$AGENT_PORT/health"
    echo ""
    echo -e "${BLUE}Monitoring Dashboards:${NC}"
    echo "  Grafana:    http://localhost:3000 (admin/admin)"
    echo "  Prometheus: http://localhost:9090"
    echo "  Jaeger:     http://localhost:16686"
    echo ""
    echo -e "${BLUE}Test the chat:${NC}"
    echo "  1. Open http://localhost:$UI_PORT in your browser"
    echo "  2. Or use curl:"
    echo ""
    echo "  # Create a session"
    echo "  curl -X POST http://localhost:$AGENT_PORT/chat/session | jq ."
    echo ""
    echo "  # Chat with SSE streaming"
    echo "  curl -N -X POST http://localhost:$AGENT_PORT/chat/stream \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"message\": \"What is the weather in Tokyo?\"}'"
    echo ""
    echo -e "${BLUE}Useful commands:${NC}"
    echo "  kubectl get pods -n $NAMESPACE"
    echo "  kubectl logs -n $NAMESPACE -l app=travel-chat-agent -f"
    echo "  $0 test            - Run API tests"
    echo "  $0 cleanup         - Delete everything"
    echo ""
    echo -e "${YELLOW}Port forwards are running in the background${NC}"
    echo "   To stop them: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
}

# Test the API
test_api() {
    local host="${1:-localhost:$AGENT_PORT}"

    log_info "Testing travel-chat-agent at $host..."
    echo ""

    # Health check
    log_info "Step 1: Health check"
    curl -s "http://$host/health" | jq . 2>/dev/null || echo "Request sent"
    echo ""

    # Create session
    log_info "Step 2: Create session"
    SESSION_RESPONSE=$(curl -s -X POST "http://$host/chat/session")
    echo "$SESSION_RESPONSE" | jq . 2>/dev/null || echo "$SESSION_RESPONSE"
    SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.session_id' 2>/dev/null)
    echo ""

    if [ "$SESSION_ID" != "null" ] && [ -n "$SESSION_ID" ]; then
        log_info "Session created: $SESSION_ID"
        echo ""

        # Test streaming chat
        log_info "Step 3: Test SSE chat stream"
        echo "Sending: 'What is the weather in Tokyo?'"
        echo ""
        curl -N -X POST "http://$host/chat/stream" \
            -H "Content-Type: application/json" \
            -d "{\"session_id\": \"$SESSION_ID\", \"message\": \"What is the weather in Tokyo?\"}" 2>/dev/null || echo "Request sent"
        echo ""
    fi

    log_success "Test complete"
}

# Run the application locally
run_app() {
    log_info "Starting Travel Chat Agent..."
    echo ""
    echo "The agent will be available at: http://localhost:8095"
    echo ""
    echo "Endpoints:"
    echo "  POST /chat/stream           - SSE streaming chat"
    echo "  POST /chat/session          - Create session"
    echo "  GET  /chat/session/{id}     - Get session info"
    echo "  GET  /health                - Health check"
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
    export PORT=${PORT:-8095}

    ./travel-chat-agent
}

# Run with Redis setup
run_all() {
    log_info "Starting all components for local development..."
    echo ""

    # 1. Ensure Redis is available
    if ! redis-cli ping 2>/dev/null | grep -q PONG; then
        setup_redis
    else
        log_success "Redis already running"
    fi

    # 2. Load environment
    setup_env

    # 3. Build agent
    build_app

    # 4. Run the agent
    run_app
}

# Full deployment: cluster + infrastructure + agent + UI
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

    # Step 4: Build and deploy
    build_docker
    load_to_kind
    deploy_k8s

    # Step 5: Setup port forwards
    port_forward_all
}

# Rebuild with no-cache and redeploy
rebuild() {
    log_info "Rebuilding with Fresh Dependencies"

    # Build Docker images with --no-cache
    log_info "Building Docker images with --no-cache..."
    DOCKER_NO_CACHE=true build_docker

    # Load images into kind cluster if available
    if command -v kind &> /dev/null; then
        local cluster_name=$(kubectl config current-context 2>/dev/null | sed 's/kind-//')
        if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
            log_info "Loading images into kind cluster..."
            load_to_kind
            log_success "Images loaded"
        fi
    fi

    # Create namespace
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup API keys from .env file
    setup_k8s_secrets

    # Apply Kubernetes manifests
    log_info "Applying Kubernetes manifests..."
    kubectl apply -f "$SCRIPT_DIR/k8-deployment.yaml"

    if [ -f "$CHAT_UI_DIR/k8-deployment.yaml" ]; then
        kubectl apply -f "$CHAT_UI_DIR/k8-deployment.yaml"
    fi

    # Restart deployments
    log_info "Restarting deployments..."
    kubectl rollout restart deployment/travel-chat-agent -n $NAMESPACE

    if kubectl get deployment chat-ui -n $NAMESPACE &>/dev/null; then
        kubectl rollout restart deployment/chat-ui -n $NAMESPACE
    fi

    log_info "Waiting for deployments to be ready..."
    if kubectl rollout status deployment/travel-chat-agent -n $NAMESPACE --timeout=120s; then
        log_success "travel-chat-agent rebuilt and deployed!"
    else
        log_error "Deployment failed. Checking logs..."
        kubectl logs -n $NAMESPACE -l app=travel-chat-agent --tail=20
        exit 1
    fi
}

# Show logs
logs() {
    log_info "Showing logs for travel-chat-agent..."
    kubectl logs -n "$NAMESPACE" -l app=travel-chat-agent -f
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."

    # Stop port forwards for this agent only (preserve jaeger, registry-viewer, etc.)
    pkill -f "port-forward.*travel-chat-agent" 2>/dev/null || true
    pkill -f "port-forward.*chat-ui" 2>/dev/null || true

    # Delete K8s resources
    kubectl delete -f "$SCRIPT_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true

    if [ -f "$CHAT_UI_DIR/k8-deployment.yaml" ]; then
        kubectl delete -f "$CHAT_UI_DIR/k8-deployment.yaml" --ignore-not-found 2>/dev/null || true
    fi

    # Stop local Redis
    docker stop gomind-redis 2>/dev/null || true
    docker rm gomind-redis 2>/dev/null || true

    # Remove local binary
    rm -f "$SCRIPT_DIR/travel-chat-agent"

    log_success "Cleanup complete"
}

# Cleanup everything including Kind cluster
cleanup_all() {
    log_info "Cleaning up everything..."

    cleanup

    # Delete Kind cluster
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
  setup      Setup the local development environment
  run        Build and run the agent locally
  run-all    Setup Redis, build, and run (recommended for local dev)
  build      Build the agent only
  redis      Setup Redis only

Kubernetes Cluster Commands:
  cluster        Create a Kind cluster with port mappings
  infra          Setup monitoring infrastructure (Prometheus, Grafana, Jaeger, OTEL)
  full-deploy    Complete deployment: cluster + infra + agent + UI (recommended)

Kubernetes Deployment Commands:
  docker         Build Docker images (agent + UI)
  deploy         Build, load to Kind, and deploy to Kubernetes
  rebuild        Rebuild with --no-cache and redeploy (fresh dependencies)
  forward        Port forward agent and UI only
  forward-all    Port forward agent + UI + monitoring (recommended)
  test           Run API tests
  logs           Show agent logs
  cleanup        Remove deployed resources
  cleanup-all    Delete Kind cluster and all resources

Examples:
  # Quick local development
  $0 run-all          # Setup Redis, build, and run locally

  # Full Kubernetes deployment (recommended)
  $0 full-deploy      # Creates cluster, infrastructure, deploys agent + UI

  # Step-by-step deployment
  $0 cluster          # Create Kind cluster
  $0 infra            # Setup monitoring
  $0 docker           # Build Docker images
  $0 deploy           # Deploy to K8s
  $0 forward-all      # Port forward everything

  # Test the chat
  $0 test             # Run API tests
  # Open Chat UI: http://localhost:$UI_PORT
  # Open Jaeger:  http://localhost:16686
EOF
}

# Handle arguments
case "${1:-help}" in
    setup)
        check_prerequisites
        setup_env
        build_app
        log_success "Setup complete! Run '$0 run' to start the agent"
        ;;
    run)
        check_prerequisites
        build_app
        run_app
        ;;
    run-all)
        check_prerequisites
        run_all
        ;;
    build)
        check_prerequisites
        build_app
        ;;
    redis)
        check_prerequisites
        setup_redis
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
        check_prerequisites
        rebuild
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
        test_api "${2:-localhost:$AGENT_PORT}"
        ;;
    logs)
        logs
        ;;
    cleanup)
        cleanup
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
