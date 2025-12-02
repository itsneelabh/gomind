#!/bin/bash

# GoMind Agent with Telemetry - One-Click Setup Script
# This script sets up everything needed including monitoring infrastructure

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
APP_NAME="research-agent-telemetry"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Functions
print_header() {
    echo -e "${BLUE}╔═══════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  GoMind Agent with Telemetry Setup            ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"
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

load_env() {
    print_step "Loading environment variables..."

    if [ -f "$SCRIPT_DIR/.env" ]; then
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
  # Agent port
  - containerPort: 30092
    hostPort: 8092
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
    echo ""
}

setup_infrastructure() {
    print_step "Setting up monitoring infrastructure..."

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
    echo ""
}

setup_api_keys() {
    print_step "Setting up API keys..."

    # Check for API keys (loaded from .env)
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        print_warning "No AI API keys found in .env file"
        echo ""
        echo "To enable AI features, add API keys to your .env file:"
        echo "  OPENAI_API_KEY=your-key"
        echo "  # or"
        echo "  ANTHROPIC_API_KEY=your-key"
        echo "  # or"
        echo "  GROQ_API_KEY=your-key"
        echo ""
    else
        print_success "Using API keys from .env file"
    fi

    # Create secret with available keys (empty string for unset keys - won't be detected as available)
    kubectl create secret generic ai-provider-keys \
        --from-literal=OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
        --from-literal=ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
        --from-literal=GROQ_API_KEY="${GROQ_API_KEY:-}" \
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

    print_step "Deploying agent with telemetry..."
    kubectl apply -f "$SCRIPT_DIR/k8-deployment.yaml"

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
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8092:80 >/dev/null 2>&1 &
    PF_PID=$!
    sleep 3

    # Test health endpoint
    echo "Testing health endpoint..."
    if curl -s http://localhost:8092/health | grep -q "healthy"; then
        print_success "Health check passed"
    else
        print_warning "Health check failed"
    fi

    # Test capabilities
    echo "Testing capabilities endpoint..."
    if curl -s http://localhost:8092/api/capabilities | grep -q "capabilities"; then
        print_success "Capabilities endpoint working"
    else
        print_warning "Capabilities endpoint not responding"
    fi

    # Kill port forward
    kill $PF_PID 2>/dev/null || true
    echo ""
}

setup_port_forwards() {
    print_step "Setting up port forwarding for monitoring..."

    # Kill existing port forwards
    pkill -f "kubectl.*port-forward.*$NAMESPACE" || true
    sleep 2

    # Start port forwarding in background
    kubectl port-forward -n $NAMESPACE svc/$APP_NAME-service 8092:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/grafana 3000:80 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/prometheus 9090:9090 >/dev/null 2>&1 &
    kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686 >/dev/null 2>&1 &

    sleep 2
    print_success "Port forwarding active"
    echo ""
}

print_summary() {
    echo -e "${GREEN}╔═══════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       Setup Complete! 🎉                      ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Your agent with telemetry is now running!"
    echo ""
    echo -e "${BLUE}🚀 Agent Endpoint:${NC}"
    echo "  http://localhost:8092/health"
    echo ""
    echo -e "${BLUE}📊 Monitoring Dashboards:${NC}"
    echo "  Grafana:    http://localhost:3000 (admin/admin)"
    echo "  Prometheus: http://localhost:9090"
    echo "  Jaeger:     http://localhost:16686"
    echo ""
    echo -e "${BLUE}🧪 Test the agent:${NC}"
    echo "  curl -X POST http://localhost:8092/api/capabilities/research_topic \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"topic\": \"latest AI trends\", \"use_ai\": true}'"
    echo ""
    echo -e "${BLUE}📈 View telemetry:${NC}"
    echo "  1. Open Grafana: http://localhost:3000"
    echo "  2. Default credentials: admin/admin"
    echo "  3. Metrics are sent to Prometheus via OTEL Collector"
    echo "  4. Traces are viewable in Jaeger"
    echo ""
    echo -e "${BLUE}🔧 Useful commands:${NC}"
    echo "  kubectl get pods -n $NAMESPACE"
    echo "  kubectl logs -n $NAMESPACE -l app=$APP_NAME"
    echo "  make logs      - View agent logs"
    echo "  make status    - Check deployment status"
    echo "  make clean     - Delete everything"
    echo ""
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GROQ_API_KEY" ]; then
        print_warning "Remember to set an API key for full AI functionality:"
        echo "  export OPENAI_API_KEY=your-key"
        echo "  kubectl create secret generic ai-provider-keys \\"
        echo "    --from-literal=OPENAI_API_KEY=\$OPENAI_API_KEY -n $NAMESPACE"
    fi
    echo ""
    echo -e "${YELLOW}💡 Port forwards are running in the background${NC}"
    echo "   To stop them: pkill -f 'kubectl.*port-forward.*$NAMESPACE'"
}

# Main execution
main() {
    clear
    print_header

    load_env
    check_prerequisites
    create_kind_cluster
    setup_infrastructure
    setup_api_keys
    build_and_deploy
    test_deployment
    setup_port_forwards
    print_summary
}

# Handle Ctrl+C
trap 'echo -e "\n${YELLOW}Setup interrupted${NC}"; exit 1' INT

# Run main function
main
