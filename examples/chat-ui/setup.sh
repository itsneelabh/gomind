#!/bin/bash

# setup.sh - One-click setup for GoMind Chat UI
# A simple static frontend that connects to travel-chat-agent backend
#
# Usage:
#   ./setup.sh          - Full deployment (build, load, deploy)
#   ./setup.sh build    - Build Docker image only
#   ./setup.sh deploy   - Deploy to existing cluster
#   ./setup.sh rebuild  - Rebuild and redeploy
#   ./setup.sh logs     - View pod logs
#   ./setup.sh status   - Check deployment status
#   ./setup.sh forward  - Start port forwarding
#   ./setup.sh clean    - Remove deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Configuration
CLUSTER_NAME="gomind-demo-$(whoami)"
NAMESPACE="gomind-examples"
APP_NAME="chat-ui"
LOCAL_PORT=8096

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
    echo -e "${BLUE}║           GoMind Chat UI - Static Frontend            ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        echo "Please install Docker from https://www.docker.com/"
        exit 1
    fi
    log_success "Docker installed"

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        echo "Please install kubectl from https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi
    log_success "kubectl installed"

    # Check kind
    if ! command -v kind &> /dev/null; then
        log_error "Kind is not installed"
        echo "Please install Kind from https://kind.sigs.k8s.io/"
        exit 1
    fi
    log_success "Kind installed"

    echo ""
}

# Detect Kind cluster
detect_cluster() {
    # Try to find an existing gomind cluster
    EXISTING_CLUSTER=$(kind get clusters 2>/dev/null | grep "gomind-demo" | head -1 || true)
    if [ -n "$EXISTING_CLUSTER" ]; then
        CLUSTER_NAME="$EXISTING_CLUSTER"
        log_info "Detected Kind cluster: $CLUSTER_NAME"
    else
        log_error "No gomind-demo cluster found"
        echo "Please run setup.sh from travel-chat-agent first to create the cluster"
        exit 1
    fi
}

# Build Docker image
build_image() {
    local no_cache=""
    if [ "$1" == "--no-cache" ]; then
        no_cache="--no-cache"
        log_info "Building with --no-cache"
    fi

    log_info "Building Docker image..."
    docker build $no_cache -t ${APP_NAME}:latest .
    log_success "${APP_NAME}:latest built"
}

# Load image into Kind
load_image() {
    log_info "Loading image into Kind cluster..."
    kind load docker-image ${APP_NAME}:latest --name "$CLUSTER_NAME"
    log_success "Image loaded to Kind"
}

# Deploy to Kubernetes
deploy() {
    log_info "Creating namespace if needed..."
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    log_info "Applying Kubernetes manifests..."
    kubectl apply -f k8-deployment.yaml -n $NAMESPACE

    log_info "Waiting for deployment to be ready..."
    kubectl rollout status deployment/${APP_NAME} -n $NAMESPACE --timeout=60s

    log_success "${APP_NAME} deployed successfully!"
}

# Start port forwarding
port_forward() {
    log_info "Starting port forward on localhost:${LOCAL_PORT}..."
    log_info "Press Ctrl+C to stop"
    echo ""
    kubectl port-forward svc/${APP_NAME}-service ${LOCAL_PORT}:80 -n $NAMESPACE
}

# Show logs
show_logs() {
    log_info "Showing ${APP_NAME} logs..."
    kubectl logs -l app=${APP_NAME} -n $NAMESPACE --tail=50 -f
}

# Show status
show_status() {
    log_info "Deployment Status:"
    kubectl get pods -l app=${APP_NAME} -n $NAMESPACE
    echo ""
    log_info "Service Status:"
    kubectl get svc -l app=${APP_NAME} -n $NAMESPACE
}

# Clean up deployment
clean() {
    log_info "Removing ${APP_NAME} deployment..."
    kubectl delete -f k8-deployment.yaml -n $NAMESPACE --ignore-not-found
    log_success "Cleanup complete"
}

# Main execution
main() {
    print_header

    case "${1:-}" in
        build)
            check_prerequisites
            build_image "${2:-}"
            ;;
        deploy)
            check_prerequisites
            detect_cluster
            deploy
            ;;
        rebuild)
            check_prerequisites
            detect_cluster
            log_info "Rebuilding ${APP_NAME}..."
            build_image --no-cache
            load_image
            deploy
            kubectl rollout restart deployment/${APP_NAME} -n $NAMESPACE
            kubectl rollout status deployment/${APP_NAME} -n $NAMESPACE --timeout=60s
            log_success "${APP_NAME} rebuilt and deployed!"
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        forward)
            port_forward
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            echo "Usage: ./setup.sh [command]"
            echo ""
            echo "Commands:"
            echo "  (none)    Full deployment (build, load, deploy)"
            echo "  build     Build Docker image only"
            echo "  deploy    Deploy to existing cluster"
            echo "  rebuild   Rebuild and redeploy with fresh image"
            echo "  logs      View pod logs"
            echo "  status    Check deployment status"
            echo "  forward   Start port forwarding (localhost:${LOCAL_PORT})"
            echo "  clean     Remove deployment"
            echo "  help      Show this help message"
            echo ""
            echo "Prerequisites:"
            echo "  - Docker"
            echo "  - kubectl"
            echo "  - Kind cluster (created by travel-chat-agent setup)"
            ;;
        *)
            # Default: full deployment
            check_prerequisites
            detect_cluster
            build_image
            load_image
            deploy
            echo ""
            log_success "Chat UI is ready!"
            echo ""
            echo "Access the UI:"
            echo "  Option 1: kubectl port-forward svc/${APP_NAME}-service ${LOCAL_PORT}:80 -n $NAMESPACE"
            echo "            Then open http://localhost:${LOCAL_PORT}"
            echo ""
            echo "  Option 2: Use NodePort http://localhost:30096 (if cluster has port mapping)"
            echo ""
            echo "Commands:"
            echo "  ./setup.sh logs     - View logs"
            echo "  ./setup.sh status   - Check status"
            echo "  ./setup.sh forward  - Start port forwarding"
            echo "  ./setup.sh rebuild  - Rebuild and redeploy"
            ;;
    esac
}

main "$@"
