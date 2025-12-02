#!/bin/bash

# GoMind Stock Market Tool - One-Click Setup Script
# This script sets up everything needed to run the stock market tool locally

set -e  # Exit on error

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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║    GoMind Stock Market Tool Setup      ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
    echo ""
}

print_step() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

check_command() {
    if ! command -v $1 &> /dev/null; then
        print_error "$1 is not installed"
        echo "Please install $1 and try again"
        echo "Installation guide: $2"
        exit 1
    fi
}

load_env() {
    print_step "Loading environment variables..."

    if [ -f "$SCRIPT_DIR/.env" ]; then
        # Export variables from .env file
        set -a
        source "$SCRIPT_DIR/.env"
        set +a
        print_success "Loaded .env file"
    elif [ -f "$SCRIPT_DIR/.env.example" ]; then
        print_warning "No .env file found, copying from .env.example"
        cp "$SCRIPT_DIR/.env.example" "$SCRIPT_DIR/.env"
        set -a
        source "$SCRIPT_DIR/.env"
        set +a
        print_success "Created .env from example"
    else
        print_warning "No .env file found"
    fi
    echo ""
}

check_prerequisites() {
    print_step "Checking prerequisites..."

    check_command "docker" "https://docs.docker.com/get-docker/"
    check_command "kind" "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    check_command "kubectl" "https://kubernetes.io/docs/tasks/tools/"

    print_success "All prerequisites installed"
    echo ""
}

create_kind_cluster() {
    print_step "Setting up Kind cluster ($CLUSTER_NAME)..."

    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        print_success "Cluster $CLUSTER_NAME already exists, reusing it"
    else
        cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  - containerPort: 30082
    hostPort: 8082
    protocol: TCP
  - containerPort: 30090
    hostPort: 8090
    protocol: TCP
EOF
        print_success "Kind cluster created"
    fi

    kubectl config use-context kind-$CLUSTER_NAME
    echo ""
}

setup_namespace() {
    print_step "Creating namespace..."
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    print_success "Namespace ready"
    echo ""
}

install_redis() {
    print_step "Setting up Redis..."

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
    echo ""
}

setup_api_keys() {
    print_step "Setting up API keys..."

    # Finnhub API key for stock data
    if [ -n "$FINNHUB_API_KEY" ] && [ "$FINNHUB_API_KEY" != "your-finnhub-api-key-here" ]; then
        print_success "Using Finnhub API key from environment"
    else
        print_warning "No Finnhub API key found"
        echo ""
        echo "The stock tool works with mock data without an API key."
        echo "For real stock market data, get a FREE API key from:"
        echo "  https://finnhub.io/register"
        echo ""
        echo "Then add to .env: FINNHUB_API_KEY=your-key-here"
        echo ""
        FINNHUB_API_KEY="mock-key"
    fi

    # Create secret
    kubectl create secret generic stock-tool-secrets \
        --from-literal=FINNHUB_API_KEY="${FINNHUB_API_KEY}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    print_success "API keys configured"
    echo ""
}

build_and_deploy() {
    print_step "Building Docker image..."
    docker build -t $APP_NAME:latest "$SCRIPT_DIR" >/dev/null 2>&1
    print_success "Docker image built"

    print_step "Loading image into Kind cluster..."
    kind load docker-image $APP_NAME:latest --name $CLUSTER_NAME
    print_success "Image loaded"

    print_step "Deploying stock tool..."
    kubectl apply -f "$SCRIPT_DIR/k8-deployment.yaml"

    echo "Waiting for deployment to be ready..."
    if kubectl wait --for=condition=available --timeout=120s deployment/$APP_NAME -n $NAMESPACE 2>/dev/null; then
        print_success "Stock tool deployed successfully!"
    else
        print_error "Deployment failed. Checking logs..."
        kubectl logs -n $NAMESPACE -l app=$APP_NAME --tail=20
        exit 1
    fi
    echo ""
}

test_deployment() {
    print_step "Testing deployment..."
    echo ""

    # Start port forward in background
    kubectl port-forward -n $NAMESPACE svc/stock-service 8082:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # Test health endpoint
    echo "Testing health endpoint..."
    if curl -s http://localhost:8082/health | grep -q "healthy"; then
        print_success "Health check passed"
    else
        print_warning "Health check failed"
    fi

    # Test capabilities
    echo "Testing capabilities endpoint..."
    if curl -s http://localhost:8082/api/capabilities | grep -q "capabilities"; then
        print_success "Capabilities endpoint working"
    else
        print_warning "Capabilities endpoint not responding"
    fi

    # Test stock quote
    echo "Testing stock quote endpoint..."
    QUOTE_RESULT=$(curl -s -X POST http://localhost:8082/api/capabilities/stock_quote \
        -H "Content-Type: application/json" \
        -d '{"symbol":"AAPL"}' 2>/dev/null)
    if echo "$QUOTE_RESULT" | grep -q "symbol"; then
        print_success "Stock quote endpoint working"
    else
        print_warning "Stock quote endpoint not responding"
    fi

    # Kill port forward
    kill $PF_PID 2>/dev/null || true
    echo ""
}

print_summary() {
    echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Setup Complete!                  ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your stock market tool is now running in the Kind cluster!"
    echo ""
    echo "To access the tool:"
    echo "  kubectl port-forward -n $NAMESPACE svc/stock-service 8082:80"
    echo "  Then visit: http://localhost:8082/health"
    echo ""
    echo "Test stock quote:"
    echo "  curl -X POST http://localhost:8082/api/capabilities/stock_quote \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"symbol\":\"AAPL\"}'"
    echo ""
    echo "Useful commands:"
    echo "  make logs      - View tool logs"
    echo "  make status    - Check deployment status"
    echo "  make test      - Run tests"
    echo "  make clean     - Delete everything"
    echo ""
    if [ "$FINNHUB_API_KEY" = "mock-key" ]; then
        print_warning "Using mock data. For real stock data:"
        echo "  1. Get FREE API key from: https://finnhub.io/register"
        echo "  2. Add to .env: FINNHUB_API_KEY=your-key"
        echo "  3. Run: ./setup.sh"
    fi
}

# Main execution
main() {
    clear
    print_header

    load_env
    check_prerequisites
    create_kind_cluster
    setup_namespace
    install_redis
    setup_api_keys
    build_and_deploy
    test_deployment
    print_summary
}

# Handle Ctrl+C
trap 'echo -e "\n${YELLOW}Setup interrupted${NC}"; exit 1' INT

# Run main function
main
