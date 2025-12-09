#!/bin/bash
# Weather Tool v2 Setup Script
# Provides commands for building, running, and deploying the weather tool

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default values
PORT=${PORT:-8096}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Weather Tool v2 - $1${NC}"
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

load_env() {
    if [ -f .env ]; then
        print_info "Loading environment from .env file"
        set -a
        source .env
        set +a
    fi
}

cmd_build() {
    print_header "Building Weather Tool v2"

    print_info "Running go mod tidy..."
    GOWORK=off go mod tidy

    print_info "Building binary..."
    GOWORK=off go build -o weather-tool-v2 .

    print_success "Build completed: weather-tool-v2"
}

cmd_run() {
    print_header "Running Weather Tool v2"

    load_env

    if [ -z "$REDIS_URL" ]; then
        print_error "REDIS_URL environment variable is required"
        exit 1
    fi

    cmd_build

    print_info "Starting weather-tool-v2 on port $PORT..."
    print_info "Redis URL: $REDIS_URL"
    print_info "API: Open-Meteo (free, no API key required)"
    echo ""

    ./weather-tool-v2
}

cmd_docker_build() {
    print_header "Building Docker Image"

    docker build -t weather-tool-v2:latest .

    print_success "Docker image built: weather-tool-v2:latest"
}

cmd_deploy() {
    print_header "Deploying to Kubernetes"

    cmd_docker_build

    if command -v kind &> /dev/null; then
        print_info "Loading image into kind cluster..."
        kind load docker-image weather-tool-v2:latest --name "gomind-demo-$(whoami)"
    fi

    print_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/weather-tool-v2 -n gomind-examples 2>/dev/null || true

    print_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    print_success "Deployment complete"
    print_info "Check status: kubectl get pods -n gomind-examples -l app=weather-tool-v2"
}

cmd_test() {
    print_header "Running Tests"

    print_info "Testing weather forecast endpoint (Tokyo)..."
    curl -s -X POST http://localhost:$PORT/api/capabilities/get_weather_forecast \
        -H "Content-Type: application/json" \
        -d '{"lat": 35.6762, "lon": 139.6503, "days": 3}' | jq .

    echo ""
    print_info "Testing current weather endpoint (New York)..."
    curl -s -X POST http://localhost:$PORT/api/capabilities/get_current_weather \
        -H "Content-Type: application/json" \
        -d '{"lat": 40.7128, "lon": -74.0060}' | jq .
}

cmd_help() {
    echo "Weather Tool v2 Setup Script"
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
    echo "  PORT          HTTP server port (default: 8096)"
    echo ""
    echo "API: Open-Meteo (https://open-meteo.com)"
    echo "  - Free, unlimited, no API key required"
    echo "  - Provides current weather and 16-day forecasts"
}

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
