#!/bin/bash
set -e

# Financial Intelligence System Deployment Script
# This script deploys the multi-agent financial intelligence system to a Kind cluster

echo "🚀 Starting Financial Intelligence System Deployment"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="financial-intelligence"
NAMESPACE="financial-intelligence"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v kind &> /dev/null; then
        print_error "Kind is not installed. Please install Kind first."
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    print_success "All prerequisites are available"
}

# Create Kind cluster
create_cluster() {
    print_status "Creating Kind cluster: $CLUSTER_NAME"
    
    if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        print_warning "Cluster $CLUSTER_NAME already exists. Skipping creation."
        return
    fi
    
    cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
- role: worker
EOF
    
    print_success "Kind cluster created successfully"
}

# Install NGINX Ingress Controller
install_ingress() {
    print_status "Installing NGINX Ingress Controller..."
    
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    
    print_status "Waiting for ingress controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=90s
    
    print_success "NGINX Ingress Controller installed successfully"
}

# Build Docker images
build_images() {
    print_status "Building agent Docker images..."
    
    # Create Dockerfiles for each agent
    create_dockerfiles
    
    # Build and load images into Kind
    agents=("market-data" "news-analysis" "chat-ui" "technical-analysis" "portfolio-advisor")
    
    for agent in "${agents[@]}"; do
        print_status "Building $agent-agent image..."
        
        cd "agents/$agent"
        docker build -t "$agent-agent:latest" .
        kind load docker-image "$agent-agent:latest" --name $CLUSTER_NAME
        cd "../.."
        
        print_success "$agent-agent image built and loaded"
    done
}

# Create Dockerfiles for agents
create_dockerfiles() {
    agents=("market-data" "news-analysis" "chat-ui" "technical-analysis" "portfolio-advisor")
    
    for agent in "${agents[@]}"; do
        cat > "agents/$agent/Dockerfile" <<EOF
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o $agent-agent .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/$agent-agent .

EXPOSE 8080
CMD ["./$agent-agent"]
EOF
    done
    
    print_success "Dockerfiles created for all agents"
}

# Deploy to Kubernetes
deploy_to_k8s() {
    print_status "Deploying to Kubernetes..."
    
    # Set kubectl context to the Kind cluster
    kubectl cluster-info --context kind-$CLUSTER_NAME
    
    # Apply Kubernetes manifests in order
    kubectl apply -f k8s/namespace.yaml
    kubectl apply -f k8s/redis.yaml
    kubectl apply -f k8s/agents-part1.yaml
    kubectl apply -f k8s/agents-part2.yaml
    kubectl apply -f k8s/ingress.yaml
    
    print_success "Kubernetes manifests applied"
}

# Wait for deployment
wait_for_deployment() {
    print_status "Waiting for deployments to be ready..."
    
    # Wait for Redis
    kubectl wait --for=condition=available --timeout=300s deployment/redis -n $NAMESPACE
    
    # Wait for agents
    agents=("market-data-agent" "news-analysis-agent" "chat-ui-agent" "technical-analysis-agent" "portfolio-advisor-agent")
    
    for agent in "${agents[@]}"; do
        print_status "Waiting for $agent to be ready..."
        kubectl wait --for=condition=available --timeout=300s deployment/$agent -n $NAMESPACE
        print_success "$agent is ready"
    done
}

# Update /etc/hosts
update_hosts() {
    print_status "Updating /etc/hosts for local access..."
    
    if ! grep -q "financial-intelligence.local" /etc/hosts; then
        echo "127.0.0.1 financial-intelligence.local" | sudo tee -a /etc/hosts
        print_success "Added financial-intelligence.local to /etc/hosts"
    else
        print_warning "financial-intelligence.local already exists in /etc/hosts"
    fi
}

# Display status
show_status() {
    print_status "Deployment Status:"
    echo ""
    
    kubectl get pods -n $NAMESPACE
    echo ""
    
    kubectl get services -n $NAMESPACE
    echo ""
    
    print_success "🎉 Financial Intelligence System deployed successfully!"
    echo ""
    echo "Access the system at:"
    echo "  🌐 Web Interface: http://financial-intelligence.local"
    echo "  📊 Chat UI: http://financial-intelligence.local/chat"
    echo ""
    echo "Direct API access:"
    echo "  📈 Market Data: http://financial-intelligence.local/api/market-data"
    echo "  📰 News Analysis: http://financial-intelligence.local/api/news-analysis" 
    echo "  📊 Technical Analysis: http://financial-intelligence.local/api/technical-analysis"
    echo "  💼 Portfolio Advisor: http://financial-intelligence.local/api/portfolio-advisor"
    echo ""
    echo "To check agent discovery:"
    echo "  kubectl logs -n $NAMESPACE -l app=chat-ui-agent"
    echo ""
    echo "To test the system:"
    echo "  curl http://financial-intelligence.local/api/market-data/health"
}

# Cleanup function
cleanup() {
    if [ "$1" = "--cleanup" ]; then
        print_status "Cleaning up..."
        kind delete cluster --name $CLUSTER_NAME
        print_success "Cluster deleted"
        exit 0
    fi
}

# Main execution
main() {
    cleanup "$@"
    
    check_prerequisites
    create_cluster
    install_ingress
    build_images
    deploy_to_k8s
    wait_for_deployment
    update_hosts
    show_status
}

main "$@"
