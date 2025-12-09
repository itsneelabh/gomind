#!/bin/bash
# Agent with Telemetry Setup Script
# Provides commands for building, running, and deploying the research agent with full telemetry

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="research-agent-telemetry"
PORT=${PORT:-8092}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Agent with Telemetry - $1${NC}"
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

# Load .env file if it exists
load_env() {
    if [ -f .env ]; then
        print_info "Loading environment from .env file"
        set -a
        source .env
        set +a
    elif [ -f .env.example ]; then
        print_info "No .env file found, copying from .env.example"
        cp .env.example .env
        set -a
        source .env
        set +a
    fi
}

check_command() {
    if ! command -v $1 &> /dev/null; then
        print_error "$1 is not installed"
        echo "Please install $1 and try again"
        exit 1
    fi
}

# Build the agent
cmd_build() {
    print_header "Building Agent"

    print_info "Running go mod tidy..."
    GOWORK=off go mod tidy

    print_info "Building binary..."
    GOWORK=off go build -o research-agent-telemetry .

    print_success "Build completed: research-agent-telemetry"
}

# Run the agent locally
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

    print_info "Starting research-agent-telemetry on port $PORT..."
    print_info "Redis URL: $REDIS_URL"
    echo ""

    ./research-agent-telemetry
}

# Build Docker image
cmd_docker_build() {
    print_header "Building Docker Image"

    docker build -t $APP_NAME:latest .

    print_success "Docker image built: $APP_NAME:latest"
}

# Create Kind cluster with port mappings for monitoring
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
  - containerPort: 30092
    hostPort: 8092
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

# Setup monitoring infrastructure
cmd_infra() {
    print_header "Setting Up Monitoring Infrastructure"

    # Use the infrastructure setup script
    if [ -f "$SCRIPT_DIR/../k8-deployment/setup-infrastructure.sh" ]; then
        print_success "Found infrastructure setup script"
        echo ""

        # Run the infrastructure setup
        NAMESPACE=$NAMESPACE "$SCRIPT_DIR/../k8-deployment/setup-infrastructure.sh"

        echo ""
        print_success "Monitoring infrastructure ready"
    else
        print_error "Infrastructure setup script not found"
        echo "Please ensure k8-deployment/setup-infrastructure.sh exists"
        exit 1
    fi
}

# Setup API keys as Kubernetes secrets
setup_api_keys() {
    print_info "Setting up API keys..."

    # Check for AI API keys (loaded from .env)
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        print_info "No AI API keys found in .env file"
        echo ""
        echo "To enable AI features, add API keys to your .env file:"
        echo "  OPENAI_API_KEY=your-key"
        echo "  # or"
        echo "  ANTHROPIC_API_KEY=your-key"
        echo "  # or"
        echo "  GROQ_API_KEY=your-key"
        echo ""
    else
        print_success "Using AI API keys from .env file"
    fi

    # Create secret with available keys
    kubectl create secret generic ai-provider-keys \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
        --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    print_success "API keys configured"
}

# Deploy to Kubernetes
cmd_deploy() {
    print_header "Deploying to Kubernetes"

    load_env

    # Build Docker image first
    cmd_docker_build

    # Load image into kind cluster if available
    if command -v kind &> /dev/null; then
        print_info "Loading image into kind cluster..."
        kind load docker-image $APP_NAME:latest --name "$CLUSTER_NAME"
        print_success "Image loaded"
    fi

    # Create namespace
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    # Setup API keys
    setup_api_keys

    print_info "Waiting for any existing deployment..."
    kubectl wait --for=condition=available --timeout=30s deployment/$APP_NAME -n $NAMESPACE 2>/dev/null || true

    # Apply Kubernetes manifests
    print_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    print_info "Waiting for deployment to be ready..."
    if kubectl wait --for=condition=available --timeout=120s deployment/$APP_NAME -n $NAMESPACE 2>/dev/null; then
        print_success "$APP_NAME deployed successfully!"
    else
        print_error "Deployment failed. Checking logs..."
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi

    print_info "Check status: kubectl get pods -n $NAMESPACE -l app=$APP_NAME"
}

# Full deployment: cluster + infrastructure + agent
cmd_full_deploy() {
    print_header "Full Deployment"

    load_env

    # Step 1: Create Kind cluster
    cmd_cluster

    # Step 2: Setup monitoring infrastructure
    cmd_infra

    # Step 3: Deploy agent
    cmd_deploy

    # Step 4: Setup port forwards
    cmd_forward_all
}

# Run tests
cmd_test() {
    print_header "Running Tests"

    # Start port forward in background
    print_info "Starting port forward..."
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8092:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # Test health endpoint
    echo "Testing health endpoint..."
    if curl -s http://localhost:8092/health | grep -q "healthy"; then
        print_success "Health check passed"
    else
        print_error "Health check failed"
    fi

    # Test capabilities
    echo "Testing capabilities endpoint..."
    if curl -s http://localhost:8092/api/capabilities | grep -q "capabilities"; then
        print_success "Capabilities endpoint working"
    else
        print_error "Capabilities endpoint not responding"
    fi

    # Test research query
    echo ""
    print_info "Testing research query..."
    curl -s -X POST http://localhost:8092/api/capabilities/research_topic \
        -H "Content-Type: application/json" \
        -d '{"topic": "latest AI trends", "use_ai": false}' | jq . 2>/dev/null || echo "(install jq for pretty output)"

    # Kill port forward
    kill $PF_PID 2>/dev/null || true
}

# Port forward for agent only
cmd_forward() {
    print_header "Port Forwarding (Agent)"

    print_info "Starting port forward on localhost:8092..."
    print_info "Press Ctrl+C to stop"
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8092:80
}

# Port forward for agent and monitoring
cmd_forward_all() {
    print_header "Port Forwarding (All)"

    # Kill existing port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true
    sleep 2

    # Start port forwarding in background
    print_info "Starting port forwards..."
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8092:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686 >/dev/null 2>&1 &

    sleep 2
    print_success "Port forwarding active"

    echo ""
    echo "Agent:      http://localhost:8092/health"
    echo "Grafana:    http://localhost:3000 (admin/admin)"
    echo "Prometheus: http://localhost:9090"
    echo "Jaeger:     http://localhost:16686"
    echo ""
    echo "Press Ctrl+C or run: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
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
    echo "Monitoring Pods:"
    kubectl get pods -n $NAMESPACE -l "app in (prometheus,grafana,otel-collector,jaeger)"
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
    setup_api_keys

    # Rebuild if requested
    if [ "$rebuild" = true ]; then
        print_info "Rebuilding Docker image..."
        cmd_docker_build

        if command -v kind &> /dev/null; then
            print_info "Loading image into kind cluster..."
            kind load docker-image $APP_NAME:latest --name "$CLUSTER_NAME"
            print_success "Image loaded"
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

# Clean up agent only
cmd_clean() {
    print_header "Cleaning Up Agent"

    print_info "Removing agent deployment..."
    kubectl delete -f k8-deployment.yaml --ignore-not-found
    print_success "Agent cleanup complete"
}

# Clean up everything including cluster
cmd_clean_all() {
    print_header "Cleaning Up Everything"

    # Kill port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true

    # Delete agent
    kubectl delete -f k8-deployment.yaml --ignore-not-found 2>/dev/null || true

    # Delete Kind cluster
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Deleting Kind cluster $CLUSTER_NAME..."
        kind delete cluster --name $CLUSTER_NAME
        print_success "Kind cluster deleted"
    fi

    print_success "Full cleanup complete"
}

# Show help
cmd_help() {
    echo "Agent with Telemetry Setup Script"
    echo ""
    echo "Usage: ./setup.sh <command>"
    echo ""
    echo "Local Development Commands:"
    echo "  build         Build the agent binary"
    echo "  run           Build and run the agent locally"
    echo ""
    echo "Kubernetes Cluster Commands:"
    echo "  cluster       Create Kind cluster with port mappings"
    echo "  infra         Setup monitoring infrastructure (Prometheus, Grafana, Jaeger)"
    echo "  full-deploy   Complete deployment: cluster + infra + agent + port forwards"
    echo ""
    echo "Kubernetes Deployment Commands:"
    echo "  docker-build  Build Docker image"
    echo "  deploy        Build, load, and deploy to Kubernetes"
    echo "  test          Run test requests against deployed agent"
    echo "  forward       Port forward the agent service only"
    echo "  forward-all   Port forward agent + monitoring dashboards"
    echo "  logs          View agent logs"
    echo "  status        Check deployment status"
    echo "  rollout       Restart deployment to pick up new secrets/config"
    echo "                Use --build flag to rebuild Docker image first"
    echo "  clean         Remove agent deployment only"
    echo "  clean-all     Delete Kind cluster and all resources"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  REDIS_URL         Redis connection URL (required for run)"
    echo "  PORT              HTTP server port (default: 8092)"
    echo "  OPENAI_API_KEY    OpenAI API key (optional)"
    echo "  ANTHROPIC_API_KEY Anthropic API key (optional)"
    echo "  GROQ_API_KEY      Groq API key (optional)"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh full-deploy    # One-click full deployment"
    echo "  ./setup.sh deploy         # Deploy to existing cluster"
    echo "  ./setup.sh forward-all    # Access all dashboards"
    echo "  ./setup.sh test           # Run tests"
    echo "  REDIS_URL=redis://localhost:6379 ./setup.sh run"
}

# Main entry point
case "${1:-help}" in
    build)
        cmd_build
        ;;
    run)
        cmd_run
        ;;
    docker-build)
        cmd_docker_build
        ;;
    cluster)
        cmd_cluster
        ;;
    infra)
        cmd_infra
        ;;
    deploy)
        cmd_deploy
        ;;
    full-deploy)
        cmd_full_deploy
        ;;
    test)
        cmd_test
        ;;
    forward)
        cmd_forward
        ;;
    forward-all)
        cmd_forward_all
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
    clean)
        cmd_clean
        ;;
    clean-all)
        cmd_clean_all
        ;;
    help|--help|-h)
        cmd_help
        ;;
    *)
        print_error "Unknown command: $1"
        cmd_help
        exit 1
        ;;
esac
