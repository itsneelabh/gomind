#!/bin/bash
# Stock Market Tool Setup Script
# Provides commands for building, running, and deploying the stock market tool

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
APP_NAME="stock-tool"
PORT=${PORT:-8082}
REDIS_URL=${REDIS_URL:-redis://localhost:6379}

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  Stock Market Tool - $1${NC}"
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

# Build the tool
cmd_build() {
    print_header "Building Stock Market Tool"

    print_info "Running go mod tidy..."
    GOWORK=off go mod tidy

    print_info "Building binary..."
    GOWORK=off go build -o stock-tool .

    print_success "Build completed: stock-tool"
}

# Run the tool locally
cmd_run() {
    print_header "Running Stock Market Tool"

    load_env

    if [ -z "$REDIS_URL" ]; then
        print_error "REDIS_URL environment variable is required"
        print_info "Set it in .env file or export it: export REDIS_URL=redis://localhost:6379"
        exit 1
    fi

    # Build first
    cmd_build

    print_info "Starting stock-tool on port $PORT..."
    print_info "Redis URL: $REDIS_URL"
    echo ""

    ./stock-tool
}

# Build Docker image
cmd_docker_build() {
    print_header "Building Docker Image"

    docker build -t $APP_NAME:latest .

    print_success "Docker image built: $APP_NAME:latest"
}

# Setup Redis in Kubernetes
setup_redis() {
    print_info "Setting up Redis..."

    # Check if Redis already exists in the namespace
    if kubectl get deployment redis -n $NAMESPACE >/dev/null 2>&1; then
        print_success "Redis already running in $NAMESPACE"
        return 0
    fi

    kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-pvc
  namespace: $NAMESPACE
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: redis-storage
          mountPath: /data
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
      volumes:
      - name: redis-storage
        persistentVolumeClaim:
          claimName: redis-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: $NAMESPACE
spec:
  type: ClusterIP
  ports:
  - port: 6379
    targetPort: 6379
  selector:
    app: redis
EOF

    echo "Waiting for Redis to be ready..."
    kubectl wait --for=condition=available --timeout=60s deployment/redis -n $NAMESPACE 2>/dev/null || true
    print_success "Redis installed"
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

    # Setup Redis
    setup_redis

    # Setup API keys as secrets
    print_info "Setting up API keys..."
    if [ -n "$FINNHUB_API_KEY" ] && [ "$FINNHUB_API_KEY" != "your-finnhub-api-key-here" ]; then
        kubectl create secret generic stock-tool-secrets \
            --from-literal=FINNHUB_API_KEY="${FINNHUB_API_KEY}" \
            -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
        print_success "Finnhub API key configured"
    else
        print_info "No Finnhub API key found (will use mock data)"
        echo ""
        echo "For real stock data, get a FREE API key from:"
        echo "  https://finnhub.io/register"
        echo ""
        echo "Then add to .env: FINNHUB_API_KEY=your-key-here"
        # Create empty secret to avoid deployment errors
        kubectl create secret generic stock-tool-secrets \
            --from-literal=FINNHUB_API_KEY="" \
            -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    fi

    # AI API keys
    if [ -n "$OPENAI_API_KEY" ] || [ -n "$ANTHROPIC_API_KEY" ] || [ -n "$GROQ_API_KEY" ]; then
        kubectl create secret generic ai-provider-keys \
            --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
            --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
            --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
            -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
        print_success "AI API keys configured"
    fi

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

# Run tests
cmd_test() {
    print_header "Running Tests"

    # Start port forward in background
    print_info "Starting port forward..."
    kubectl port-forward -n $NAMESPACE svc/stock-service 8082:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # Test health endpoint
    echo "Testing health endpoint..."
    if curl -s http://localhost:8082/health | grep -q "healthy"; then
        print_success "Health check passed"
    else
        print_error "Health check failed"
    fi

    # Test capabilities
    echo "Testing capabilities endpoint..."
    if curl -s http://localhost:8082/api/capabilities | grep -q "capabilities"; then
        print_success "Capabilities endpoint working"
    else
        print_error "Capabilities endpoint not responding"
    fi

    # Test stock quote
    echo ""
    print_info "Testing stock quote..."
    curl -s -X POST http://localhost:8082/api/capabilities/stock_quote \
        -H "Content-Type: application/json" \
        -d '{"symbol": "AAPL"}' | jq . 2>/dev/null || echo "(install jq for pretty output)"

    # Kill port forward
    kill $PF_PID 2>/dev/null || true
}

# Port forward for local access
cmd_forward() {
    print_header "Port Forwarding"

    print_info "Starting port forward on localhost:8082..."
    print_info "Press Ctrl+C to stop"
    kubectl port-forward -n $NAMESPACE svc/stock-service 8082:80
}

# View logs
cmd_logs() {
    print_header "Viewing Logs"

    kubectl logs -n $NAMESPACE -l app=$APP_NAME -f --tail=100
}

# Check status
cmd_status() {
    print_header "Deployment Status"

    echo "Pods:"
    kubectl get pods -n $NAMESPACE -l app=$APP_NAME
    echo ""
    echo "Service:"
    kubectl get svc -n $NAMESPACE -l app=$APP_NAME
}

# Clean up
cmd_clean() {
    print_header "Cleaning Up"

    print_info "Removing deployment..."
    kubectl delete -f k8-deployment.yaml --ignore-not-found
    print_success "Cleanup complete"
}

# Show help
cmd_help() {
    echo "Stock Market Tool Setup Script"
    echo ""
    echo "Usage: ./setup.sh <command>"
    echo ""
    echo "Commands:"
    echo "  build         Build the tool binary"
    echo "  run           Build and run the tool locally"
    echo "  docker-build  Build Docker image"
    echo "  deploy        Build, load, and deploy to Kubernetes"
    echo "  test          Run test requests against deployed tool"
    echo "  forward       Port forward the service for local access"
    echo "  logs          View tool logs"
    echo "  status        Check deployment status"
    echo "  clean         Remove deployment"
    echo "  help          Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  REDIS_URL         Redis connection URL (required for run)"
    echo "  PORT              HTTP server port (default: 8082)"
    echo "  FINNHUB_API_KEY   Finnhub API key for real stock data (optional)"
    echo "  OPENAI_API_KEY    OpenAI API key (optional)"
    echo ""
    echo "Examples:"
    echo "  ./setup.sh build"
    echo "  ./setup.sh deploy"
    echo "  ./setup.sh test"
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
    deploy)
        cmd_deploy
        ;;
    test)
        cmd_test
        ;;
    forward)
        cmd_forward
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
    help|--help|-h)
        cmd_help
        ;;
    *)
        print_error "Unknown command: $1"
        cmd_help
        exit 1
        ;;
esac
