#!/bin/bash
# News Tool Setup Script
# Deploys to existing Kind cluster without modifying common infrastructure
# NOTE: Requires GNEWS_API_KEY from https://gnews.io/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="news-tool"
PORT=8099

print_step() { echo -e "${BLUE}▶ $1${NC}"; }
print_success() { echo -e "${GREEN}✓ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠ $1${NC}"; }
print_error() { echo -e "${RED}✗ $1${NC}"; }

load_env() {
    if [ -f .env ]; then
        set -a; source .env; set +a
        print_success "Loaded .env file"
    fi
}

cmd_build() {
    print_step "Building news-tool..."
    GOWORK=off go mod tidy
    GOWORK=off go build -o news-tool .
    print_success "Build completed"
}

cmd_run() {
    load_env
    [ -z "$REDIS_URL" ] && print_error "REDIS_URL required" && exit 1
    if [ -z "$GNEWS_API_KEY" ] || [ "$GNEWS_API_KEY" = "your_api_key_here" ]; then
        print_warning "GNEWS_API_KEY not set - news API will not work"
        print_warning "Get a FREE API key at: https://gnews.io/"
    fi
    cmd_build
    ./news-tool
}

cmd_docker_build() {
    print_step "Building Docker image..."
    docker build -t $APP_NAME:latest .
    print_success "Docker image built"
}

cmd_deploy() {
    print_step "Deploying $APP_NAME to Kind cluster..."

    # Load .env for API key
    load_env

    # Check for API key
    if [ -z "$GNEWS_API_KEY" ] || [ "$GNEWS_API_KEY" = "your_api_key_here" ]; then
        print_warning "GNEWS_API_KEY not found in .env"
        print_warning "The tool will deploy but news search will not work"
        print_warning "Get a FREE API key at: https://gnews.io/"
        GNEWS_API_KEY=""
    else
        print_success "Using GNEWS_API_KEY from .env"
    fi

    cmd_docker_build

    print_step "Loading image into Kind cluster..."
    kind load docker-image $APP_NAME:latest --name $CLUSTER_NAME
    print_success "Image loaded"

    # Create secret for API key
    print_step "Creating API key secret..."
    kubectl create secret generic external-api-keys \
        --from-literal=GNEWS_API_KEY="${GNEWS_API_KEY}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    print_success "Secret created"

    print_step "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml

    print_step "Waiting for deployment..."
    if kubectl wait --for=condition=available --timeout=120s deployment/$APP_NAME -n $NAMESPACE 2>/dev/null; then
        print_success "$APP_NAME deployed successfully!"
    else
        print_error "Deployment failed"
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi

    echo ""
    if [ -z "$GNEWS_API_KEY" ]; then
        print_warning "Remember to add GNEWS_API_KEY to .env and redeploy"
    fi
    echo "To test:"
    echo "  kubectl port-forward -n $NAMESPACE svc/${APP_NAME}-service 8099:80 &"
    echo "  curl -X POST http://localhost:8099/api/capabilities/search_news \\"
    echo "    -H 'Content-Type: application/json' -d '{\"query\":\"Tokyo travel\",\"max_results\":3}'"
}

cmd_test() {
    print_step "Testing news-tool..."
    if ! curl -s http://localhost:$PORT/health >/dev/null 2>&1; then
        kubectl port-forward -n $NAMESPACE svc/${APP_NAME}-service $PORT:80 >/dev/null 2>&1 &
        sleep 3
    fi
    curl -s -X POST http://localhost:$PORT/api/capabilities/search_news \
        -H "Content-Type: application/json" \
        -d '{"query": "Tokyo travel", "max_results": 3}' | jq .
}

cmd_logs() { kubectl logs -n $NAMESPACE -l app=$APP_NAME -f; }
cmd_status() { kubectl get pods,svc -n $NAMESPACE -l app=$APP_NAME; }

cmd_help() {
    echo "News Tool Setup Script"
    echo ""
    echo "Usage: ./setup.sh {build|run|docker-build|deploy|test|logs|status|help}"
    echo ""
    echo "API: GNews.io - REQUIRES API KEY"
    echo "  Get FREE API key at: https://gnews.io/"
    echo "  Free tier: 100 requests/day"
    echo ""
    echo "Set GNEWS_API_KEY in .env file before deploying"
}

case "${1:-help}" in
    build) cmd_build ;; run) cmd_run ;; docker-build) cmd_docker_build ;;
    deploy) cmd_deploy ;; test) cmd_test ;; logs) cmd_logs ;; status) cmd_status ;;
    help|--help|-h) cmd_help ;; *) print_error "Unknown: $1"; exit 1 ;;
esac
