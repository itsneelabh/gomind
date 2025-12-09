#!/bin/bash
# Context Propagation Example Setup Script
# Provides commands for building, running, and deploying the context propagation demo

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
APP_NAME="context-propagation-example"
PORT=${PORT:-8100}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Context Propagation Example - $1${NC}"
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

# Build the example
cmd_build() {
    print_header "Building Context Propagation Example"

    print_info "Running go mod tidy..."
    GOWORK=off go mod tidy

    print_info "Building binary..."
    GOWORK=off go build -o context-propagation-example .

    print_success "Build completed: context-propagation-example"
}

# Run the example locally
cmd_run() {
    print_header "Running Context Propagation Example"

    load_env

    # Build first
    cmd_build

    print_info "Starting context propagation demo..."
    echo ""
    print_info "This example demonstrates context propagation through request lifecycle"
    print_info "Watch how baggage automatically flows through all function calls"
    echo ""

    ./context-propagation-example
}

# Build Docker image
cmd_docker_build() {
    print_header "Building Docker Image"

    if [ ! -f "Dockerfile" ]; then
        print_info "This is a local-only example (no Dockerfile present)"
        print_info "Use './setup.sh run' to run locally instead"
        echo ""
        print_info "To containerize this example, create a Dockerfile with:"
        echo "  FROM golang:1.25-alpine"
        echo "  WORKDIR /app"
        echo "  COPY . ."
        echo "  RUN go build -o context-propagation-example ."
        echo "  CMD [\"./context-propagation-example\"]"
        return 0
    fi

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
  # Context propagation example port
  - containerPort: 30100
    hostPort: 8100
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
        echo "Note: This example doesn't require AI API keys"
        echo "It's a telemetry/context propagation demonstration"
        echo ""
    else
        print_success "Using AI API keys from .env file"

        # Create AI provider secret with available keys
        kubectl create secret generic ai-provider-keys \
            --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
            --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
            --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
            -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

        print_success "AI API keys configured"
    fi
}

# Deploy to Kubernetes
cmd_deploy() {
    print_header "Deploying to Kubernetes"

    if [ ! -f "k8-deployment.yaml" ]; then
        print_info "This is a local-only example (no k8-deployment.yaml present)"
        echo ""
        print_info "This example demonstrates context propagation through request lifecycle"
        print_info "It simulates HTTP requests with telemetry context flowing through all layers"
        echo ""
        print_success "Use './setup.sh run' to run the demo locally instead"
        echo ""
        print_info "The demo will show:"
        echo "  - Request-level context (request_id, user_id, tenant_id)"
        echo "  - Layer-specific context (db.operation, cache.operation, etc.)"
        echo "  - Automatic context propagation through nested function calls"
        echo "  - Telemetry metrics with inherited labels"
        echo ""
        print_info "Example output includes:"
        echo "  - Processing multiple simulated requests"
        echo "  - Authentication attempts with context"
        echo "  - Database queries with inherited labels"
        echo "  - Cache operations with baggage"
        echo "  - Telemetry summary with metrics count"
        return 0
    fi

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

# Full deployment: cluster + infrastructure + example
cmd_full_deploy() {
    print_header "Full Deployment"

    print_info "This is a local-only example - use './setup.sh run' instead"
    echo ""
    print_info "If you want to deploy monitoring infrastructure for other examples:"
    echo ""
    echo "  # Create cluster with monitoring"
    echo "  ./setup.sh cluster"
    echo "  ./setup.sh infra"
    echo ""
    echo "  # Run this example locally to see telemetry"
    echo "  ./setup.sh run"
    echo ""
}

# Run tests
cmd_test() {
    print_header "Running Tests"

    print_info "Building example..."
    cmd_build

    print_info "Running context propagation demo..."
    echo ""

    # Run the example and capture output
    if ./context-propagation-example > /tmp/context-propagation-test.log 2>&1; then
        print_success "Demo completed successfully"

        # Verify expected output
        if grep -q "Context Propagation Example" /tmp/context-propagation-test.log && \
           grep -q "Telemetry Summary" /tmp/context-propagation-test.log; then
            print_success "Output verification passed"
            echo ""
            print_info "Telemetry metrics emitted:"
            grep "Metrics Emitted:" /tmp/context-propagation-test.log || true
        else
            print_error "Output verification failed"
            cat /tmp/context-propagation-test.log
            exit 1
        fi
    else
        print_error "Demo failed"
        cat /tmp/context-propagation-test.log
        exit 1
    fi

    # Cleanup
    rm -f /tmp/context-propagation-test.log

    echo ""
    print_success "All tests passed!"
}

# Port forward (not applicable for local-only example)
cmd_forward() {
    print_header "Port Forwarding"

    print_info "This is a local-only example"
    print_info "Port forwarding is not needed - run directly with './setup.sh run'"
    echo ""
    print_info "If you have monitoring infrastructure deployed:"
    echo "  ./setup.sh forward-all    # Access monitoring dashboards"
}

# Port forward for monitoring only
cmd_forward_all() {
    print_header "Port Forwarding (Monitoring)"

    # Check if monitoring is deployed
    if ! kubectl get namespace $NAMESPACE &> /dev/null; then
        print_error "Namespace $NAMESPACE not found"
        print_info "Deploy monitoring first: ./setup.sh cluster && ./setup.sh infra"
        exit 1
    fi

    # Kill existing port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true
    sleep 2

    # Start port forwarding in background
    print_info "Starting port forwards for monitoring..."
    kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686 >/dev/null 2>&1 &

    sleep 2
    print_success "Port forwarding active"

    echo ""
    echo "Monitoring Dashboards:"
    echo "  Grafana:      http://localhost:3000 (admin/admin)"
    echo "  Prometheus:   http://localhost:9090"
    echo "  Jaeger:       http://localhost:16686"
    echo ""
    print_info "Run './setup.sh run' in another terminal to generate telemetry"
    echo ""
    echo "Press Ctrl+C or run: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
}

# View logs (not applicable for local-only example)
cmd_logs() {
    print_header "Viewing Logs"

    print_info "This is a local-only example"
    print_info "Logs are displayed directly when running: ./setup.sh run"
    echo ""
    print_info "Example output shows:"
    echo "  - Request processing with context propagation"
    echo "  - Telemetry metrics with inherited labels"
    echo "  - Summary of metrics emitted"
}

# Check status
cmd_status() {
    print_header "Status Check"

    # Check if binary exists
    if [ -f "context-propagation-example" ]; then
        print_success "Binary built: context-propagation-example"
    else
        print_info "Binary not built yet. Run: ./setup.sh build"
    fi

    echo ""
    print_info "This is a local-only demonstration example"
    print_info "It shows context propagation patterns in action"
    echo ""

    # Check if monitoring infrastructure exists
    if kubectl get namespace $NAMESPACE &> /dev/null 2>&1; then
        echo "Monitoring Infrastructure Status:"
        kubectl get pods -n $NAMESPACE -l "app in (prometheus,grafana,otel-collector,jaeger)" 2>/dev/null || \
            print_info "No monitoring pods found in $NAMESPACE"
    else
        print_info "No Kubernetes monitoring deployed"
        echo "  Deploy with: ./setup.sh cluster && ./setup.sh infra"
    fi
}

# Clean up
cmd_clean() {
    print_header "Cleaning Up"

    print_info "Removing built binary..."
    rm -f context-propagation-example

    print_success "Cleanup complete"
}

# Clean up everything including cluster
cmd_clean_all() {
    print_header "Cleaning Up Everything"

    # Remove binary
    rm -f context-propagation-example

    # Kill port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" 2>/dev/null || true

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
    echo "Context Propagation Example Setup Script"
    echo ""
    echo "This example demonstrates context propagation and telemetry patterns."
    echo "It simulates HTTP requests with automatic label propagation through nested calls."
    echo ""
    echo "Usage: ./setup.sh <command>"
    echo ""
    echo "Local Development Commands:"
    echo "  build         Build the example binary"
    echo "  run           Build and run the demonstration"
    echo "  test          Run the demo and verify output"
    echo ""
    echo "Kubernetes Infrastructure Commands (Optional):"
    echo "  cluster       Create Kind cluster for monitoring"
    echo "  infra         Setup monitoring infrastructure (Prometheus, Grafana, Jaeger)"
    echo "  forward-all   Port forward monitoring dashboards"
    echo ""
    echo "Docker Commands (Not Needed):"
    echo "  docker-build  Show info about containerization (no Dockerfile)"
    echo "  deploy        Show info about deployment (local-only example)"
    echo ""
    echo "Utility Commands:"
    echo "  status        Check build and infrastructure status"
    echo "  logs          Show info about logs (run shows output directly)"
    echo "  forward       Show info about port forwarding (not needed)"
    echo "  clean         Remove built binary"
    echo "  clean-all     Remove binary and delete Kind cluster"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  PORT              Port for future HTTP server (default: 8100)"
    echo "  REDIS_URL         Redis URL for future integrations (default: redis://localhost:6379)"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh run              # Run the context propagation demo"
    echo "  ./setup.sh test             # Run and verify the demo"
    echo "  ./setup.sh cluster          # Create Kind cluster for monitoring"
    echo "  ./setup.sh infra            # Deploy monitoring stack"
    echo "  ./setup.sh forward-all      # Access monitoring dashboards"
    echo ""
    echo "What This Demo Shows:"
    echo "  - Context creation at system edge (request entry point)"
    echo "  - Automatic baggage propagation through function calls"
    echo "  - Layer-specific context addition (db, cache, business logic)"
    echo "  - Telemetry metrics with inherited labels"
    echo "  - Multi-tenant request tracking"
    echo ""
    echo "Expected Output:"
    echo "  === Context Propagation Example ==="
    echo "  Processing request req-001"
    echo "  ..."
    echo "  === Telemetry Summary ==="
    echo "  Metrics Emitted: 47"
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
