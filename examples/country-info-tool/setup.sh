#!/bin/bash

################################################################################
# Country Info Tool - Complete Setup Script
# Provides 1-click deployment with full observability stack
################################################################################

set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="country-info-tool"
PORT=${PORT:-8098}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

################################################################################
# Utility Functions
################################################################################

print_header() {
    echo -e "\n${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

load_env() {
    if [ -f .env ]; then
        print_info "Loading environment from .env"
        export $(grep -v '^#' .env | xargs)
    elif [ -f .env.example ]; then
        print_info "Loading environment from .env.example"
        export $(grep -v '^#' .env.example | xargs)
    else
        print_info "No .env file found, using defaults"
    fi
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$1 is not installed. Please install it first."
        return 1
    fi
    print_success "$1 is installed"
    return 0
}

check_redis() {
    print_info "Checking Redis connection..."
    if redis-cli -u "$REDIS_URL" ping &> /dev/null; then
        print_success "Redis is running and accessible"
        return 0
    else
        print_error "Redis is not accessible at $REDIS_URL"
        print_info "Please start Redis or update REDIS_URL in .env"
        return 1
    fi
}

################################################################################
# Build Commands
################################################################################

cmd_build() {
    print_header "Building $APP_NAME"

    print_info "Tidying Go modules..."
    GOWORK=off go mod tidy

    print_info "Building application..."
    GOWORK=off go build -o "$APP_NAME" .

    print_success "Build complete: ./$APP_NAME"
}

cmd_run() {
    print_header "Running $APP_NAME locally"

    load_env

    # Check Redis first
    if ! check_redis; then
        print_error "Cannot start application without Redis"
        print_info "Start Redis with: docker run -d -p 6379:6379 redis:alpine"
        exit 1
    fi

    # Build if needed
    if [ ! -f "$APP_NAME" ]; then
        cmd_build
    fi

    print_info "Starting $APP_NAME on port $PORT..."
    print_info "Press Ctrl+C to stop"
    ./"$APP_NAME"
}

cmd_docker_build() {
    print_header "Building Docker image"

    if ! check_command docker; then
        exit 1
    fi

    print_info "Building docker image: $APP_NAME:latest"
    docker build -t "$APP_NAME:latest" .

    print_success "Docker image built successfully"
    docker images | grep "$APP_NAME"
}

################################################################################
# Cluster Management
################################################################################

cmd_cluster() {
    print_header "Creating Kind cluster: $CLUSTER_NAME"

    if ! check_command kind; then
        print_error "Please install Kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi

    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Cluster $CLUSTER_NAME already exists"
        return 0
    fi

    print_info "Creating cluster with port mappings..."

    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30098
        hostPort: 8098
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

    print_success "Cluster $CLUSTER_NAME created successfully"
    kubectl cluster-info --context "kind-$CLUSTER_NAME"
}

cmd_infra() {
    print_header "Setting up infrastructure (Redis, Grafana, Prometheus, Jaeger)"

    if [ ! -f "../k8-deployment/setup-infrastructure.sh" ]; then
        print_error "Infrastructure setup script not found"
        print_info "Expected: ../k8-deployment/setup-infrastructure.sh"
        exit 1
    fi

    print_info "Running infrastructure setup..."
    bash ../k8-deployment/setup-infrastructure.sh

    print_success "Infrastructure setup complete"
}

setup_api_keys() {
    print_header "Setting up AI API keys"

    load_env

    # Check if secret already exists
    if kubectl get secret ai-api-keys -n "$NAMESPACE" &> /dev/null; then
        print_info "AI API keys secret already exists"
        read -p "Do you want to update it? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 0
        fi
        kubectl delete secret ai-api-keys -n "$NAMESPACE"
    fi

    # Collect API keys
    if [ -z "$OPENAI_API_KEY" ]; then
        read -p "Enter OpenAI API Key (or press Enter to skip): " OPENAI_API_KEY
    fi

    if [ -z "$ANTHROPIC_API_KEY" ]; then
        read -p "Enter Anthropic API Key (or press Enter to skip): " ANTHROPIC_API_KEY
    fi

    if [ -z "$GROQ_API_KEY" ]; then
        read -p "Enter Groq API Key (or press Enter to skip): " GROQ_API_KEY
    fi

    # Create secret with available keys
    local secret_args=""
    [ -n "$OPENAI_API_KEY" ] && secret_args="$secret_args --from-literal=OPENAI_API_KEY=$OPENAI_API_KEY"
    [ -n "$ANTHROPIC_API_KEY" ] && secret_args="$secret_args --from-literal=ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY"
    [ -n "$GROQ_API_KEY" ] && secret_args="$secret_args --from-literal=GROQ_API_KEY=$GROQ_API_KEY"

    if [ -z "$secret_args" ]; then
        print_error "No API keys provided"
        exit 1
    fi

    kubectl create secret generic ai-api-keys -n "$NAMESPACE" $secret_args
    print_success "AI API keys secret created"
}

################################################################################
# Deployment Commands
################################################################################

cmd_deploy() {
    print_header "Deploying $APP_NAME to Kubernetes"

    # Check prerequisites
    if ! check_command kubectl; then
        exit 1
    fi

    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        print_info "Create cluster with: ./setup.sh cluster"
        exit 1
    fi

    # Build docker image
    cmd_docker_build

    # Load image to Kind
    print_info "Loading image to Kind cluster..."
    if command -v kind &> /dev/null && kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        kind load docker-image "$APP_NAME:latest" --name "$CLUSTER_NAME"
        print_success "Image loaded to Kind cluster"
    else
        print_info "Not a Kind cluster or cluster not found, skipping image load"
    fi

    # Create namespace if needed
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_info "Creating namespace: $NAMESPACE"
        kubectl create namespace "$NAMESPACE"
    fi

    # Setup secrets
    setup_api_keys

    # Apply manifests
    if [ ! -f "k8-deployment.yaml" ]; then
        print_error "k8-deployment.yaml not found"
        exit 1
    fi

    print_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    # Wait for deployment
    print_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/"$APP_NAME" -n "$NAMESPACE" 2>/dev/null || true

    # Check status
    print_success "$APP_NAME deployed successfully"
    kubectl get pods -n "$NAMESPACE" -l app="$APP_NAME"
}

cmd_full_deploy() {
    print_header "ONE-CLICK DEPLOYMENT: Complete setup"

    print_info "This will:"
    print_info "  1. Create Kind cluster"
    print_info "  2. Setup infrastructure (Redis, Grafana, Prometheus, Jaeger)"
    print_info "  3. Deploy $APP_NAME"
    print_info "  4. Setup port forwarding"
    echo

    # Create cluster
    cmd_cluster

    # Wait a moment for cluster to stabilize
    sleep 5

    # Setup infrastructure
    cmd_infra

    # Deploy application
    cmd_deploy

    # Setup port forwarding in background
    print_info "Setting up port forwarding..."
    cmd_forward_all &

    sleep 3

    print_header "DEPLOYMENT COMPLETE"
    print_success "Cluster: $CLUSTER_NAME"
    print_success "Namespace: $NAMESPACE"
    print_success "Application: $APP_NAME"
    echo
    print_info "Access points:"
    print_info "  - Country Info Tool: http://localhost:8098"
    print_info "  - Grafana: http://localhost:3000 (admin/admin)"
    print_info "  - Prometheus: http://localhost:9090"
    print_info "  - Jaeger: http://localhost:16686"
    echo
    print_info "Test with: ./setup.sh test"
    print_info "View logs: ./setup.sh logs"
    print_info "Clean up: ./setup.sh clean-all"
}

################################################################################
# Testing Commands
################################################################################

cmd_test() {
    print_header "Testing $APP_NAME"

    if ! check_command curl; then
        print_error "curl is required for testing"
        exit 1
    fi

    local endpoint="http://localhost:${PORT}/api/capabilities/get_country_info"

    print_info "Testing endpoint: $endpoint"
    print_info "Request: Get country info for Japan"
    echo

    local response=$(curl -s -X POST "$endpoint" \
        -H "Content-Type: application/json" \
        -d '{"country":"Japan"}')

    if [ $? -eq 0 ]; then
        if command -v jq &> /dev/null; then
            echo "$response" | jq .
        else
            echo "$response"
        fi
        print_success "Test completed successfully"
    else
        print_error "Test failed - is the service running?"
        print_info "Start locally: ./setup.sh run"
        print_info "Or port forward: ./setup.sh forward"
        exit 1
    fi
}

################################################################################
# Port Forwarding Commands
################################################################################

cmd_forward() {
    print_header "Setting up port forwarding for $APP_NAME"

    if ! kubectl get deployment "$APP_NAME" -n "$NAMESPACE" &> /dev/null; then
        print_error "Deployment $APP_NAME not found in namespace $NAMESPACE"
        exit 1
    fi

    print_info "Forwarding port $PORT..."
    print_info "Access application at: http://localhost:$PORT"
    print_info "Press Ctrl+C to stop"

    kubectl port-forward -n "$NAMESPACE" deployment/"$APP_NAME" "$PORT:$PORT"
}

cmd_forward_all() {
    print_header "Setting up port forwarding for all services"

    # Function to forward a port with error handling
    forward_service() {
        local namespace=$1
        local service=$2
        local port=$3
        local label=$4

        if kubectl get deployment "$service" -n "$namespace" &> /dev/null 2>&1 || \
           kubectl get service "$service" -n "$namespace" &> /dev/null 2>&1; then
            print_info "Forwarding $label on port $port..."
            kubectl port-forward -n "$namespace" "service/$service" "$port:$port" &> /dev/null &
            return 0
        else
            print_info "$label not found, skipping..."
            return 1
        fi
    }

    # Kill existing port forwards
    pkill -f "kubectl port-forward" 2>/dev/null || true
    sleep 2

    # Forward application
    print_info "Forwarding $APP_NAME on port $PORT..."
    kubectl port-forward -n "$NAMESPACE" deployment/"$APP_NAME" "$PORT:$PORT" &> /dev/null &

    # Forward monitoring services
    forward_service "monitoring" "grafana" "3000" "Grafana"
    forward_service "monitoring" "prometheus-server" "9090" "Prometheus"
    forward_service "monitoring" "jaeger-query" "16686" "Jaeger"

    sleep 2

    print_success "Port forwarding setup complete"
    echo
    print_info "Access points:"
    print_info "  - Country Info Tool: http://localhost:$PORT"
    print_info "  - Grafana: http://localhost:3000"
    print_info "  - Prometheus: http://localhost:9090"
    print_info "  - Jaeger: http://localhost:16686"
    echo
    print_info "Running in background. To stop: pkill -f 'kubectl port-forward'"
    print_info "Or clean up everything: ./setup.sh clean-all"
}

################################################################################
# Monitoring Commands
################################################################################

cmd_logs() {
    print_header "Viewing logs for $APP_NAME"

    if ! kubectl get deployment "$APP_NAME" -n "$NAMESPACE" &> /dev/null; then
        print_error "Deployment $APP_NAME not found in namespace $NAMESPACE"
        exit 1
    fi

    print_info "Streaming logs... (Press Ctrl+C to stop)"
    kubectl logs -n "$NAMESPACE" -l app="$APP_NAME" --follow --tail=100
}

cmd_status() {
    print_header "Status of $APP_NAME"

    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        print_error "Namespace $NAMESPACE not found"
        exit 1
    fi

    echo -e "${YELLOW}Deployments:${NC}"
    kubectl get deployments -n "$NAMESPACE" -l app="$APP_NAME"
    echo

    echo -e "${YELLOW}Pods:${NC}"
    kubectl get pods -n "$NAMESPACE" -l app="$APP_NAME"
    echo

    echo -e "${YELLOW}Services:${NC}"
    kubectl get services -n "$NAMESPACE" -l app="$APP_NAME"
    echo

    # Show recent events
    echo -e "${YELLOW}Recent Events:${NC}"
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -10
}

################################################################################
# Rollout Command
################################################################################

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

        if command -v kind &> /dev/null && kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
            print_info "Loading image into kind cluster..."
            kind load docker-image "$APP_NAME:latest" --name "$CLUSTER_NAME"
            print_success "Image loaded"
        fi
    fi

    # Restart deployment
    print_info "Restarting deployment..."
    kubectl rollout restart deployment/"$APP_NAME" -n "$NAMESPACE"

    print_info "Waiting for rollout to complete..."
    if kubectl rollout status deployment/"$APP_NAME" -n "$NAMESPACE" --timeout=120s; then
        print_success "Rollout complete!"
    else
        print_error "Rollout failed"
        kubectl logs -n "$NAMESPACE" -l app="$APP_NAME" --tail=20
        exit 1
    fi
}

################################################################################
# Cleanup Commands
################################################################################

cmd_clean() {
    print_header "Cleaning up $APP_NAME"

    if kubectl get deployment "$APP_NAME" -n "$NAMESPACE" &> /dev/null; then
        print_info "Deleting Kubernetes resources..."
        kubectl delete -f k8-deployment.yaml 2>/dev/null || true
        print_success "Kubernetes resources deleted"
    else
        print_info "No Kubernetes resources found"
    fi

    if [ -f "$APP_NAME" ]; then
        print_info "Removing local binary..."
        rm "$APP_NAME"
        print_success "Local binary removed"
    fi

    print_success "Cleanup complete"
}

cmd_clean_all() {
    print_header "Complete cleanup - Removing cluster and all resources"

    print_info "This will delete:"
    print_info "  - Kind cluster: $CLUSTER_NAME"
    print_info "  - All deployed applications"
    print_info "  - All monitoring infrastructure"
    echo

    read -p "Are you sure? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Cleanup cancelled"
        exit 0
    fi

    # Stop port forwarding
    print_info "Stopping port forwarding..."
    pkill -f "kubectl port-forward" 2>/dev/null || true

    # Delete Kind cluster
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        print_info "Deleting Kind cluster: $CLUSTER_NAME"
        kind delete cluster --name "$CLUSTER_NAME"
        print_success "Cluster deleted"
    else
        print_info "Cluster $CLUSTER_NAME not found"
    fi

    # Clean local files
    if [ -f "$APP_NAME" ]; then
        rm "$APP_NAME"
        print_success "Local binary removed"
    fi

    print_success "Complete cleanup finished"
}

################################################################################
# Help Command
################################################################################

cmd_help() {
    cat <<EOF

${BLUE}Country Info Tool - Setup Script${NC}

${YELLOW}USAGE:${NC}
    ./setup.sh [COMMAND]

${YELLOW}BUILD COMMANDS:${NC}
    ${GREEN}build${NC}               Build the application locally
    ${GREEN}run${NC}                 Build and run the application locally (requires Redis)
    ${GREEN}docker-build${NC}        Build Docker image

${YELLOW}CLUSTER COMMANDS:${NC}
    ${GREEN}cluster${NC}             Create Kind cluster with port mappings
    ${GREEN}infra${NC}               Setup infrastructure (Redis, Grafana, Prometheus, Jaeger)

${YELLOW}DEPLOYMENT COMMANDS:${NC}
    ${GREEN}deploy${NC}              Deploy application to Kubernetes
    ${GREEN}full-deploy${NC}         ONE-CLICK: Create cluster + infra + deploy + port forward

${YELLOW}TESTING COMMANDS:${NC}
    ${GREEN}test${NC}                Test the application with sample request

${YELLOW}PORT FORWARDING:${NC}
    ${GREEN}forward${NC}             Port forward application only
    ${GREEN}forward-all${NC}         Port forward application + monitoring services

${YELLOW}MONITORING COMMANDS:${NC}
    ${GREEN}logs${NC}                View application logs
    ${GREEN}status${NC}              Show deployment status
    ${GREEN}rollout${NC}             Restart deployment to pick up new secrets/config
                        Use --build flag to rebuild Docker image first

${YELLOW}CLEANUP COMMANDS:${NC}
    ${GREEN}clean${NC}               Remove application deployment
    ${GREEN}clean-all${NC}           Delete Kind cluster and all resources

${YELLOW}HELP:${NC}
    ${GREEN}help${NC}                Show this help message

${YELLOW}CONFIGURATION:${NC}
    Cluster Name:    $CLUSTER_NAME
    Namespace:       $NAMESPACE
    Application:     $APP_NAME
    Port:            $PORT
    Redis URL:       $REDIS_URL

${YELLOW}EXAMPLES:${NC}
    # One-click deployment
    ./setup.sh full-deploy

    # Build and test locally
    ./setup.sh build
    ./setup.sh run

    # Manual deployment
    ./setup.sh cluster
    ./setup.sh infra
    ./setup.sh deploy
    ./setup.sh forward-all

    # Monitoring
    ./setup.sh status
    ./setup.sh logs

    # Cleanup
    ./setup.sh clean-all

${YELLOW}ACCESS POINTS (after deployment):${NC}
    - Country Info Tool: http://localhost:$PORT
    - Grafana:          http://localhost:3000 (admin/admin)
    - Prometheus:       http://localhost:9090
    - Jaeger:           http://localhost:16686

EOF
}

################################################################################
# Main Entry Point
################################################################################

main() {
    local command=${1:-help}

    case "$command" in
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
            print_error "Unknown command: $command"
            echo
            cmd_help
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
