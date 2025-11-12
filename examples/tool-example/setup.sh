#!/bin/bash

# GoMind Agent Example - One-Click Setup Script
# This script sets up everything needed to run the agent example locally

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="gomind-demo"
NAMESPACE="gomind-examples"
APP_NAME="research-agent"

# Functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     GoMind Agent Example Setup         ║${NC}"
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

check_prerequisites() {
    print_step "Checking prerequisites..."

    check_command "docker" "https://docs.docker.com/get-docker/"
    check_command "kind" "https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    check_command "kubectl" "https://kubernetes.io/docs/tasks/tools/"

    print_success "All prerequisites installed"
    echo ""
}

create_kind_cluster() {
    print_step "Setting up Kind cluster..."

    if kind get clusters 2>/dev/null | grep -q "$CLUSTER_NAME"; then
        print_warning "Cluster $CLUSTER_NAME already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kind delete cluster --name $CLUSTER_NAME
        else
            print_warning "Using existing cluster"
        fi
    fi

    if ! kind get clusters 2>/dev/null | grep -q "$CLUSTER_NAME"; then
        cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
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
    print_step "Installing Redis..."

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

    # Check for API keys
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        print_warning "No AI API keys found in environment"
        echo ""
        echo "Would you like to:"
        echo "1) Enter an API key now"
        echo "2) Continue with dummy keys (limited functionality)"
        echo ""
        read -p "Choice (1/2): " choice

        if [ "$choice" = "1" ]; then
            echo ""
            echo "Select provider:"
            echo "1) OpenAI"
            echo "2) Anthropic"
            echo "3) Groq"
            read -p "Provider (1/2/3): " provider

            read -s -p "Enter API key: " api_key
            echo ""

            case $provider in
                1) OPENAI_API_KEY=$api_key ;;
                2) ANTHROPIC_API_KEY=$api_key ;;
                3) GROQ_API_KEY=$api_key ;;
            esac
        fi
    fi

    # Create secret
    kubectl create secret generic ai-provider-keys \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-dummy-key}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-dummy-key}" \
        --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-dummy-key}" \
        -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

    print_success "API keys configured"
    echo ""
}

build_and_deploy() {
    print_step "Building Docker image..."
    docker build -t $APP_NAME:latest . >/dev/null 2>&1
    print_success "Docker image built"

    print_step "Loading image into Kind cluster..."
    kind load docker-image $APP_NAME:latest --name $CLUSTER_NAME
    print_success "Image loaded"

    print_step "Deploying agent..."
    kubectl apply -f k8-deployment.yaml

    echo "Waiting for deployment to be ready..."
    if kubectl wait --for=condition=available --timeout=120s deployment/$APP_NAME -n $NAMESPACE 2>/dev/null; then
        print_success "Agent deployed successfully!"
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
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8090:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # Test health endpoint
    echo "Testing health endpoint..."
    if curl -s http://localhost:8090/health | grep -q "healthy"; then
        print_success "Health check passed"
    else
        print_warning "Health check failed"
    fi

    # Test capabilities
    echo "Testing capabilities endpoint..."
    if curl -s http://localhost:8090/api/capabilities | grep -q "capabilities"; then
        print_success "Capabilities endpoint working"
    else
        print_warning "Capabilities endpoint not responding"
    fi

    # Kill port forward
    kill $PF_PID 2>/dev/null || true
    echo ""
}

print_summary() {
    echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Setup Complete! 🎉               ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your agent is now running in the Kind cluster!"
    echo ""
    echo "To access the agent:"
    echo "  kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8090:80"
    echo "  Then visit: http://localhost:8090/health"
    echo ""
    echo "Useful commands:"
    echo "  make logs      - View agent logs"
    echo "  make status    - Check deployment status"
    echo "  make test      - Run tests"
    echo "  make clean     - Delete everything"
    echo ""
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        print_warning "Remember to set an API key for full AI functionality:"
        echo "  export OPENAI_API_KEY=your-key"
        echo "  make create-secrets"
    fi
}

# Main execution
main() {
    clear
    print_header

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