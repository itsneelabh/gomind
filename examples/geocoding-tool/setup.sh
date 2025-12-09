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

# Default values
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

    # Build Docker image first
    cmd_docker_build

    # Load image into kind cluster if available
    if command -v kind &> /dev/null; then
        print_info "Loading image into kind cluster..."
        kind load docker-image geocoding-tool:latest --name "gomind-demo-$(whoami)"
    fi

    print_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/geocoding-tool -n gomind-examples 2>/dev/null || true

    # Apply Kubernetes manifests
    print_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    print_success "Deployment complete"
    print_info "Check status: kubectl get pods -n gomind-examples -l app=geocoding-tool"
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
    echo "Geocoding Tool Setup Script"
    echo ""
    echo "Usage: ./setup.sh <command>"
    echo ""
    echo "Commands:"
    echo "  build         Build the tool binary"
    echo "  run           Build and run the tool locally"
    echo "  docker-build  Build Docker image"
    echo "  deploy        Deploy to Kubernetes"
    echo "  test          Run test requests"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  REDIS_URL     Redis connection URL (required for run)"
    echo "  PORT          HTTP server port (default: 8095)"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh build"
    echo "  REDIS_URL=redis://localhost:6379 ./setup.sh run"
    echo "  ./setup.sh test"
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
    deploy)
        cmd_deploy
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
