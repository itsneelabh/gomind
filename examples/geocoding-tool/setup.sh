#!/bin/bash
# Geocoding Tool Setup Script
# Provides commands for building, running, and deploying the geocoding tool

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
APP_NAME="geocoding-tool"
PORT=${PORT:-8095}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Geocoding Tool - $1${NC}"
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
    fi
}

# Create Kind cluster with port mappings
cmd_cluster() {
    print_header "Creating Kind Cluster"

    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Cluster ${CLUSTER_NAME} already exists"
        return 0
    fi

    print_info "Creating Kind cluster: ${CLUSTER_NAME}"
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30095
    hostPort: 8095
    protocol: TCP
  - containerPort: 30000
    hostPort: 3000
    protocol: TCP
  - containerPort: 30090
    hostPort: 9090
    protocol: TCP
  - containerPort: 30686
    hostPort: 16686
    protocol: TCP
EOF

    print_success "Kind cluster created: ${CLUSTER_NAME}"
    print_info "Setting kubectl context to kind-${CLUSTER_NAME}"
    kubectl config use-context "kind-${CLUSTER_NAME}"
}

# Setup infrastructure (Grafana, Prometheus, Jaeger, etc.)
cmd_infra() {
    print_header "Setting Up Infrastructure"

    if [ ! -f ../k8-deployment/setup-infrastructure.sh ]; then
        print_error "Infrastructure setup script not found: ../k8-deployment/setup-infrastructure.sh"
        exit 1
    fi

    print_info "Running infrastructure setup script..."
    bash ../k8-deployment/setup-infrastructure.sh

    print_success "Infrastructure setup complete"
}

# Setup API keys secret
setup_api_keys() {
    print_info "Setting up API keys secret..."

    load_env

    # Create namespace if it doesn't exist
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    # Check for required API keys
    local has_keys=false
    local secret_args=""

    if [ -n "$OPENAI_API_KEY" ]; then
        secret_args="${secret_args} --from-literal=OPENAI_API_KEY=${OPENAI_API_KEY}"
        has_keys=true
        print_info "Found OPENAI_API_KEY"
    fi

    if [ -n "$ANTHROPIC_API_KEY" ]; then
        secret_args="${secret_args} --from-literal=ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}"
        has_keys=true
        print_info "Found ANTHROPIC_API_KEY"
    fi

    if [ -n "$GROQ_API_KEY" ]; then
        secret_args="${secret_args} --from-literal=GROQ_API_KEY=${GROQ_API_KEY}"
        has_keys=true
        print_info "Found GROQ_API_KEY"
    fi

    if [ "$has_keys" = false ]; then
        print_error "No API keys found in .env file"
        print_info "Please set at least one of: OPENAI_API_KEY, ANTHROPIC_API_KEY, GROQ_API_KEY"
        exit 1
    fi

    # Create or update secret
    kubectl create secret generic ai-provider-keys \
        --namespace="${NAMESPACE}" \
        ${secret_args} \
        --dry-run=client -o yaml | kubectl apply -f -

    print_success "API keys secret created/updated"
}

# Build the tool
cmd_build() {
    print_header "Building Geocoding Tool"

    print_info "Running go mod tidy..."
    GOWORK=off go mod tidy

    print_info "Building binary..."
    GOWORK=off go build -o geocoding-tool .

    print_success "Build completed: geocoding-tool"
}

# Run the tool locally
cmd_run() {
    print_header "Running Geocoding Tool"

    load_env

    if [ -z "$REDIS_URL" ]; then
        print_error "REDIS_URL environment variable is required"
        print_info "Set it in .env file or export it: export REDIS_URL=redis://localhost:6379"
        exit 1
    fi

    # Build first
    cmd_build

    print_info "Starting geocoding-tool on port $PORT..."
    print_info "Redis URL: $REDIS_URL"
    echo ""

    ./geocoding-tool
}

# Build Docker image
cmd_docker_build() {
    print_header "Building Docker Image"

    docker build -t geocoding-tool:latest .

    print_success "Docker image built: geocoding-tool:latest"
}

# Deploy to Kubernetes
cmd_deploy() {
    print_header "Deploying to Kubernetes"

    # Load environment
    load_env

    # Build Docker image
    cmd_docker_build

    # Load image into kind cluster
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Loading image into kind cluster..."
        kind load docker-image geocoding-tool:latest --name "${CLUSTER_NAME}"
    else
        print_error "Kind cluster ${CLUSTER_NAME} not found. Run './setup.sh cluster' first."
        exit 1
    fi

    # Create namespace
    print_info "Creating namespace: ${NAMESPACE}"
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    # Setup API keys
    setup_api_keys

    # Apply Kubernetes manifests
    print_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    print_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/geocoding-tool -n "${NAMESPACE}" 2>/dev/null || true

    print_success "Deployment complete"
    print_info "Check status: kubectl get pods -n ${NAMESPACE} -l app=geocoding-tool"
}

# ONE-CLICK deployment: cluster + infra + deploy + forward-all
cmd_full_deploy() {
    print_header "ONE-CLICK Full Deployment"

    print_info "Step 1/4: Creating Kind cluster..."
    cmd_cluster

    print_info "Step 2/4: Setting up infrastructure..."
    cmd_infra

    print_info "Step 3/4: Deploying application..."
    cmd_deploy

    print_info "Step 4/4: Setting up port forwarding..."
    cmd_forward_all

    echo ""
    print_success "=========================================="
    print_success "  FULL DEPLOYMENT COMPLETE!"
    print_success "=========================================="
    echo ""
    print_info "Services available at:"
    print_info "  - Geocoding Tool: http://localhost:8095"
    print_info "  - Grafana: http://localhost:3000 (admin/admin)"
    print_info "  - Prometheus: http://localhost:9090"
    print_info "  - Jaeger: http://localhost:16686"
    echo ""
    print_info "Test the service:"
    print_info "  ./setup.sh test"
    echo ""
    print_info "View logs:"
    print_info "  ./setup.sh logs"
    echo ""
}

# Port forward tool service only
cmd_forward() {
    print_header "Port Forwarding - Tool Only"

    print_info "Forwarding ${APP_NAME} service (8095:80)..."
    kubectl port-forward -n "${NAMESPACE}" "service/${APP_NAME}" 8095:80 &
    local PID=$!
    echo $PID > /tmp/geocoding-tool-forward.pid

    print_success "Port forwarding started for ${APP_NAME} (PID: $PID)"
    print_info "Service available at: http://localhost:8095"
    print_info "To stop: kill $PID"
}

# Port forward all services (tool + Grafana + Prometheus + Jaeger)
cmd_forward_all() {
    print_header "Port Forwarding - All Services"

    # Kill existing port-forwards
    if [ -f /tmp/geocoding-tool-forward-all.pid ]; then
        print_info "Stopping existing port forwards..."
        pkill -F /tmp/geocoding-tool-forward-all.pid 2>/dev/null || true
        rm -f /tmp/geocoding-tool-forward-all.pid
    fi

    # Start new port forwards in background
    print_info "Starting port forwards..."

    # Tool service
    kubectl port-forward -n "${NAMESPACE}" "service/${APP_NAME}" 8095:80 > /tmp/geocoding-tool-pf.log 2>&1 &
    echo $! > /tmp/geocoding-tool-forward-all.pid

    # Grafana
    kubectl port-forward -n monitoring service/grafana 3000:3000 > /tmp/grafana-pf.log 2>&1 &
    echo $! >> /tmp/geocoding-tool-forward-all.pid

    # Prometheus
    kubectl port-forward -n monitoring service/prometheus 9090:9090 > /tmp/prometheus-pf.log 2>&1 &
    echo $! >> /tmp/geocoding-tool-forward-all.pid

    # Jaeger
    kubectl port-forward -n monitoring service/jaeger 16686:16686 > /tmp/jaeger-pf.log 2>&1 &
    echo $! >> /tmp/geocoding-tool-forward-all.pid

    sleep 2

    print_success "All port forwards started!"
    echo ""
    print_info "Services available at:"
    print_info "  - Geocoding Tool: http://localhost:8095"
    print_info "  - Grafana: http://localhost:3000 (admin/admin)"
    print_info "  - Prometheus: http://localhost:9090"
    print_info "  - Jaeger: http://localhost:16686"
    echo ""
    print_info "PIDs saved to: /tmp/geocoding-tool-forward-all.pid"
    print_info "To stop all: pkill -F /tmp/geocoding-tool-forward-all.pid"
}

# Show logs
cmd_logs() {
    print_header "Viewing Logs"

    local pod=$(kubectl get pods -n "${NAMESPACE}" -l "app=${APP_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [ -z "$pod" ]; then
        print_error "No ${APP_NAME} pod found in namespace ${NAMESPACE}"
        exit 1
    fi

    print_info "Showing logs for pod: $pod"
    echo ""
    kubectl logs -n "${NAMESPACE}" "$pod" --tail=100 -f
}

# Show deployment status
cmd_status() {
    print_header "Deployment Status"

    echo ""
    print_info "Checking Kind cluster..."
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_success "Cluster '${CLUSTER_NAME}' is running"
    else
        print_error "Cluster '${CLUSTER_NAME}' not found"
        return 1
    fi

    echo ""
    print_info "Checking namespace..."
    if kubectl get namespace "${NAMESPACE}" > /dev/null 2>&1; then
        print_success "Namespace '${NAMESPACE}' exists"
    else
        print_error "Namespace '${NAMESPACE}' not found"
        return 1
    fi

    echo ""
    print_info "Deployment status:"
    kubectl get deployments -n "${NAMESPACE}" -l "app=${APP_NAME}" 2>/dev/null || print_error "No deployments found"

    echo ""
    print_info "Pod status:"
    kubectl get pods -n "${NAMESPACE}" -l "app=${APP_NAME}" 2>/dev/null || print_error "No pods found"

    echo ""
    print_info "Service status:"
    kubectl get services -n "${NAMESPACE}" -l "app=${APP_NAME}" 2>/dev/null || print_error "No services found"

    echo ""
    print_info "Recent events:"
    kubectl get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' | tail -10
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

        if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
            print_info "Loading image into kind cluster..."
            kind load docker-image geocoding-tool:latest --name "${CLUSTER_NAME}"
            print_success "Image loaded"
        fi
    fi

    # Restart deployment
    print_info "Restarting deployment..."
    kubectl rollout restart deployment/geocoding-tool -n "${NAMESPACE}"

    print_info "Waiting for rollout to complete..."
    if kubectl rollout status deployment/geocoding-tool -n "${NAMESPACE}" --timeout=120s; then
        print_success "Rollout complete!"
    else
        print_error "Rollout failed"
        kubectl logs -n "${NAMESPACE}" -l app=geocoding-tool --tail=20
        exit 1
    fi
}

# Clean tool only (remove deployment but keep cluster)
cmd_clean() {
    print_header "Cleaning Tool Deployment"

    print_info "Removing ${APP_NAME} deployment from namespace ${NAMESPACE}..."
    kubectl delete -f k8-deployment.yaml --ignore-not-found=true

    print_info "Removing API keys secret..."
    kubectl delete secret ai-provider-keys -n "${NAMESPACE}" --ignore-not-found=true

    print_success "Tool deployment removed"
    print_info "Cluster and infrastructure still running. Use 'clean-all' to remove everything."
}

# Clean everything (delete Kind cluster)
cmd_clean_all() {
    print_header "Cleaning Everything"

    # Stop port forwards
    if [ -f /tmp/geocoding-tool-forward-all.pid ]; then
        print_info "Stopping port forwards..."
        pkill -F /tmp/geocoding-tool-forward-all.pid 2>/dev/null || true
        rm -f /tmp/geocoding-tool-forward-all.pid
    fi

    if [ -f /tmp/geocoding-tool-forward.pid ]; then
        pkill -F /tmp/geocoding-tool-forward.pid 2>/dev/null || true
        rm -f /tmp/geocoding-tool-forward.pid
    fi

    # Delete Kind cluster
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Deleting Kind cluster: ${CLUSTER_NAME}"
        kind delete cluster --name "${CLUSTER_NAME}"
        print_success "Cluster deleted"
    else
        print_info "Cluster ${CLUSTER_NAME} not found"
    fi

    # Cleanup temp files
    rm -f /tmp/geocoding-tool-*.log
    rm -f /tmp/grafana-pf.log /tmp/prometheus-pf.log /tmp/jaeger-pf.log

    print_success "Complete cleanup finished"
}

# Run tests
cmd_test() {
    print_header "Running Tests"

    print_info "Testing geocode endpoint..."
    curl -s -X POST http://localhost:$PORT/api/capabilities/geocode_location \
        -H "Content-Type: application/json" \
        -d '{"location": "Tokyo, Japan"}' | jq .

    echo ""
    print_info "Testing reverse geocode endpoint..."
    curl -s -X POST http://localhost:$PORT/api/capabilities/reverse_geocode \
        -H "Content-Type: application/json" \
        -d '{"lat": 35.6762, "lon": 139.6503}' | jq .
}

# Show help
cmd_help() {
    echo "Geocoding Tool Setup Script - 1-Click Deployment"
    echo ""
    echo "Usage: ./setup.sh <command>"
    echo ""
    echo "=== ONE-CLICK DEPLOYMENT ==="
    echo "  full-deploy   Complete deployment: cluster + infra + deploy + port-forwarding"
    echo ""
    echo "=== CLUSTER MANAGEMENT ==="
    echo "  cluster       Create Kind cluster with port mappings"
    echo "  infra         Setup infrastructure (Grafana, Prometheus, Jaeger)"
    echo "  clean         Remove tool deployment only (keep cluster)"
    echo "  clean-all     Delete Kind cluster and all resources"
    echo ""
    echo "=== APPLICATION COMMANDS ==="
    echo "  build         Build the tool binary"
    echo "  run           Build and run the tool locally"
    echo "  docker-build  Build Docker image"
    echo "  deploy        Deploy to Kubernetes cluster"
    echo "  test          Run test requests against the service"
    echo ""
    echo "=== MONITORING & DEBUG ==="
    echo "  forward       Port forward tool service only (8095:80)"
    echo "  forward-all   Port forward all services (tool + Grafana + Prometheus + Jaeger)"
    echo "  logs          View application logs"
    echo "  status        Show deployment status"
    echo "  rollout       Restart deployment to pick up new secrets/config"
    echo "                Use --build flag to rebuild Docker image first"
    echo "  help          Show this help message"
    echo ""
    echo "Configuration:"
    echo "  CLUSTER_NAME: ${CLUSTER_NAME}"
    echo "  NAMESPACE: ${NAMESPACE}"
    echo "  APP_NAME: ${APP_NAME}"
    echo "  PORT: ${PORT}"
    echo "  REDIS_URL: ${REDIS_URL}"
    echo ""
    echo "Environment Variables (.env file):"
    echo "  REDIS_URL           Redis connection URL (required for run)"
    echo "  PORT                HTTP server port (default: 8095)"
    echo "  OPENAI_API_KEY      OpenAI API key (optional)"
    echo "  ANTHROPIC_API_KEY   Anthropic API key (optional)"
    echo "  GROQ_API_KEY        Groq API key (optional)"
    echo ""
    echo "Examples:"
    echo "  # Complete 1-click deployment"
    echo "  ./setup.sh full-deploy"
    echo ""
    echo "  # Step-by-step deployment"
    echo "  ./setup.sh cluster        # Create cluster"
    echo "  ./setup.sh infra          # Setup monitoring"
    echo "  ./setup.sh deploy         # Deploy application"
    echo "  ./setup.sh forward-all    # Setup port forwarding"
    echo ""
    echo "  # Local development"
    echo "  ./setup.sh build"
    echo "  REDIS_URL=redis://localhost:6379 ./setup.sh run"
    echo ""
    echo "  # Testing and monitoring"
    echo "  ./setup.sh test           # Test endpoints"
    echo "  ./setup.sh logs           # View logs"
    echo "  ./setup.sh status         # Check status"
    echo ""
    echo "  # Cleanup"
    echo "  ./setup.sh clean          # Remove app only"
    echo "  ./setup.sh clean-all      # Remove everything"
}

# Main entry point
case "${1:-help}" in
    cluster)
        cmd_cluster
        ;;
    infra)
        cmd_infra
        ;;
    full-deploy)
        cmd_full_deploy
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
    deploy)
        cmd_deploy
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
    test)
        cmd_test
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
