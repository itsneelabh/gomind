#!/bin/bash

# GoMind Infrastructure Setup Script
# Intelligently deploys infrastructure components only if they don't exist
# Never deletes existing resources - always checks services first

set -e

# Colors
COLOR_RED='\033[0;31m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_BLUE='\033[0;34m'
COLOR_PURPLE='\033[0;35m'
COLOR_NC='\033[0m'

NAMESPACE=${NAMESPACE:-gomind-examples}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${COLOR_BLUE}"
echo "🏗️  GoMind Infrastructure Setup"
echo "================================"
echo -e "${COLOR_NC}"

# Function to check if a service exists and is healthy
check_service_exists() {
    local service_name=$1
    local namespace=$2

    if kubectl get service "$service_name" -n "$namespace" &>/dev/null; then
        # Service exists, check if it has endpoints (healthy)
        local endpoints=$(kubectl get endpoints "$service_name" -n "$namespace" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)
        if [ -n "$endpoints" ]; then
            return 0  # Service exists and is healthy
        else
            return 1  # Service exists but no healthy endpoints
        fi
    else
        return 2  # Service doesn't exist
    fi
}

# Function to check if deployment is ready
check_deployment_ready() {
    local deployment_name=$1
    local namespace=$2

    local ready=$(kubectl get deployment "$deployment_name" -n "$namespace" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null)
    if [ "$ready" = "True" ]; then
        return 0
    else
        return 1
    fi
}

# Function to create namespace if it doesn't exist
setup_namespace() {
    echo -e "${COLOR_YELLOW}📁 Checking namespace...${COLOR_NC}"

    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
        echo -e "${COLOR_GREEN}✅ Namespace '$NAMESPACE' already exists${COLOR_NC}"
    else
        echo -e "${COLOR_BLUE}📦 Creating namespace '$NAMESPACE'...${COLOR_NC}"
        kubectl apply -f "$SCRIPT_DIR/namespace.yaml"
        echo -e "${COLOR_GREEN}✅ Namespace created${COLOR_NC}"
    fi
    echo ""
}

# Function to deploy a component with checks
deploy_component() {
    local component_name=$1
    local service_name=$2
    local deployment_name=$3
    local yaml_file=$4

    echo -e "${COLOR_YELLOW}🔍 Checking ${component_name}...${COLOR_NC}"

    # Check if service exists (capture exit status without triggering set -e)
    local service_status=0
    check_service_exists "$service_name" "$NAMESPACE" && service_status=0 || service_status=$?

    if [ $service_status -eq 0 ]; then
        # Service exists and is healthy
        local deployment_ready=0
        check_deployment_ready "$deployment_name" "$NAMESPACE" && deployment_ready=0 || deployment_ready=$?
        if [ $deployment_ready -eq 0 ]; then
            echo -e "${COLOR_GREEN}✅ ${component_name} already running and healthy${COLOR_NC}"
            echo -e "${COLOR_BLUE}   Service: ${service_name}, Deployment: ${deployment_name}${COLOR_NC}"
            return 0
        else
            echo -e "${COLOR_YELLOW}⚠️  ${component_name} service exists but deployment not ready${COLOR_NC}"
            echo -e "${COLOR_BLUE}   Checking if we should redeploy...${COLOR_NC}"
        fi
    elif [ $service_status -eq 1 ]; then
        echo -e "${COLOR_YELLOW}⚠️  ${component_name} service exists but has no healthy endpoints${COLOR_NC}"
        echo -e "${COLOR_BLUE}   Will apply configuration to fix...${COLOR_NC}"
    else
        echo -e "${COLOR_BLUE}📦 ${component_name} not found, deploying...${COLOR_NC}"
    fi

    # Deploy or update the component
    kubectl apply -f "$SCRIPT_DIR/$yaml_file"

    # Wait for deployment to be ready
    echo -e "${COLOR_BLUE}⏳ Waiting for ${component_name} to be ready...${COLOR_NC}"
    if kubectl wait --for=condition=available --timeout=120s deployment/"$deployment_name" -n "$NAMESPACE" 2>/dev/null; then
        echo -e "${COLOR_GREEN}✅ ${component_name} is ready${COLOR_NC}"
    else
        echo -e "${COLOR_YELLOW}⚠️  ${component_name} deployment timeout, but may still be starting${COLOR_NC}"
    fi

    echo ""
}

# Function to show infrastructure status
show_status() {
    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo -e "${COLOR_GREEN}📊 Infrastructure Status${COLOR_NC}"
    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo ""

    echo -e "${COLOR_BLUE}Services:${COLOR_NC}"
    kubectl get svc -n "$NAMESPACE" -o wide 2>/dev/null || echo "No services found"
    echo ""

    echo -e "${COLOR_BLUE}Deployments:${COLOR_NC}"
    kubectl get deployments -n "$NAMESPACE" -o wide 2>/dev/null || echo "No deployments found"
    echo ""

    echo -e "${COLOR_BLUE}Pods:${COLOR_NC}"
    kubectl get pods -n "$NAMESPACE" -o wide 2>/dev/null || echo "No pods found"
    echo ""

    echo -e "${COLOR_BLUE}Persistent Volume Claims:${COLOR_NC}"
    kubectl get pvc -n "$NAMESPACE" 2>/dev/null || echo "No PVCs found"
    echo ""
}

# Function to check prerequisites
check_prerequisites() {
    echo -e "${COLOR_YELLOW}🔍 Checking prerequisites...${COLOR_NC}"

    if ! command -v kubectl &>/dev/null; then
        echo -e "${COLOR_RED}❌ kubectl not found${COLOR_NC}"
        echo "Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi

    # Check if kubectl can connect to cluster
    if ! kubectl cluster-info &>/dev/null; then
        echo -e "${COLOR_RED}❌ Cannot connect to Kubernetes cluster${COLOR_NC}"
        echo "Please ensure your kubeconfig is set up correctly"
        exit 1
    fi

    echo -e "${COLOR_GREEN}✅ Prerequisites OK${COLOR_NC}"
    echo -e "${COLOR_BLUE}   Cluster: $(kubectl config current-context)${COLOR_NC}"
    echo ""
}

# Function to verify files exist
verify_files() {
    echo -e "${COLOR_YELLOW}🔍 Verifying configuration files...${COLOR_NC}"

    local files=(
        "namespace.yaml"
        "redis.yaml"
        "otel-collector.yaml"
        "prometheus.yaml"
        "jaeger.yaml"
        "grafana.yaml"
        "metrics-server.yaml"
    )

    local missing=()
    for file in "${files[@]}"; do
        if [ ! -f "$SCRIPT_DIR/$file" ]; then
            missing+=("$file")
        fi
    done

    if [ ${#missing[@]} -ne 0 ]; then
        echo -e "${COLOR_RED}❌ Missing files: ${missing[*]}${COLOR_NC}"
        exit 1
    fi

    echo -e "${COLOR_GREEN}✅ All configuration files found${COLOR_NC}"
    echo ""
}

# Function to deploy metrics-server (in kube-system namespace)
deploy_metrics_server() {
    echo -e "${COLOR_YELLOW}🔍 Checking Metrics Server...${COLOR_NC}"

    # Check if metrics-server is already running in kube-system
    if kubectl get deployment metrics-server -n kube-system &>/dev/null; then
        local ready=$(kubectl get deployment metrics-server -n kube-system -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null)
        if [ "$ready" = "True" ]; then
            echo -e "${COLOR_GREEN}✅ Metrics Server already running and healthy${COLOR_NC}"
            return 0
        else
            echo -e "${COLOR_YELLOW}⚠️  Metrics Server exists but not ready, reapplying...${COLOR_NC}"
        fi
    else
        echo -e "${COLOR_BLUE}📦 Metrics Server not found, deploying...${COLOR_NC}"
    fi

    # Deploy metrics-server
    kubectl apply -f "$SCRIPT_DIR/metrics-server.yaml"

    # Wait for deployment to be ready
    echo -e "${COLOR_BLUE}⏳ Waiting for Metrics Server to be ready...${COLOR_NC}"
    if kubectl wait --for=condition=available --timeout=120s deployment/metrics-server -n kube-system 2>/dev/null; then
        echo -e "${COLOR_GREEN}✅ Metrics Server is ready${COLOR_NC}"
        echo -e "${COLOR_BLUE}   kubectl top pods/nodes will be available shortly${COLOR_NC}"
    else
        echo -e "${COLOR_YELLOW}⚠️  Metrics Server deployment timeout, but may still be starting${COLOR_NC}"
    fi

    echo ""
}

# Main deployment function
main() {
    check_prerequisites
    verify_files

    # Create namespace first
    setup_namespace

    # Deploy Metrics Server first (in kube-system, enables kubectl top)
    deploy_metrics_server

    # Deploy components in dependency order
    # Each component is checked before deployment

    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo -e "${COLOR_GREEN}🚀 Deploying Infrastructure Components${COLOR_NC}"
    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo ""

    # 1. Redis - Required by all components for service discovery
    deploy_component "Redis" "redis" "redis" "redis.yaml"

    # 2. OTEL Collector - Telemetry pipeline
    deploy_component "OTEL Collector" "otel-collector" "otel-collector" "otel-collector.yaml"

    # 3. Prometheus - Metrics storage
    deploy_component "Prometheus" "prometheus" "prometheus" "prometheus.yaml"

    # 4. Jaeger - Distributed tracing
    deploy_component "Jaeger" "jaeger-query" "jaeger" "jaeger.yaml"

    # 5. Grafana - Visualization
    deploy_component "Grafana" "grafana" "grafana" "grafana.yaml"

    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo -e "${COLOR_GREEN}✅ Infrastructure Setup Complete!${COLOR_NC}"
    echo -e "${COLOR_PURPLE}════════════════════════════════════════${COLOR_NC}"
    echo ""

    # Show final status
    show_status

    # Show access information
    echo -e "${COLOR_BLUE}🌐 Access Information:${COLOR_NC}"
    echo -e "   Redis:          ${COLOR_YELLOW}redis.${NAMESPACE}:6379${COLOR_NC}"
    echo -e "   OTEL Collector: ${COLOR_YELLOW}otel-collector.${NAMESPACE}:4318${COLOR_NC}"
    echo -e "   Prometheus:     ${COLOR_YELLOW}prometheus.${NAMESPACE}:9090${COLOR_NC}"
    echo -e "   Jaeger:         ${COLOR_YELLOW}jaeger-query.${NAMESPACE}:16686${COLOR_NC}"
    echo -e "   Grafana:        ${COLOR_YELLOW}grafana.${NAMESPACE}:80${COLOR_NC}"
    echo ""

    echo -e "${COLOR_BLUE}💡 To access from outside the cluster:${COLOR_NC}"
    echo -e "   kubectl port-forward -n ${NAMESPACE} svc/grafana 3000:80"
    echo -e "   kubectl port-forward -n ${NAMESPACE} svc/prometheus 9090:9090"
    echo -e "   kubectl port-forward -n ${NAMESPACE} svc/jaeger-query 16686:80"
    echo ""

    echo -e "${COLOR_BLUE}🔧 Useful commands:${COLOR_NC}"
    echo -e "   Status:  ${COLOR_YELLOW}./setup-infrastructure.sh status${COLOR_NC}"
    echo -e "   Logs:    ${COLOR_YELLOW}kubectl logs -n ${NAMESPACE} -l app=<component>${COLOR_NC}"
    echo -e "   Delete:  ${COLOR_YELLOW}kubectl delete namespace ${NAMESPACE}${COLOR_NC}"
    echo ""
}

# Status command
status_only() {
    check_prerequisites
    show_status
}

# Help command
show_help() {
    echo "GoMind Infrastructure Setup Script"
    echo ""
    echo "This script intelligently deploys infrastructure components"
    echo "and never deletes existing resources."
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  setup       Deploy infrastructure (checks existing first) - default"
    echo "  status      Show current infrastructure status"
    echo "  help        Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  NAMESPACE   Kubernetes namespace (default: gomind-examples)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Deploy with checks"
    echo "  $0 status             # Show current status"
    echo "  NAMESPACE=prod $0     # Deploy to 'prod' namespace"
    echo ""
    echo "Safety Features:"
    echo "  ✓ Checks if services exist before deploying"
    echo "  ✓ Skips deployment if service is healthy"
    echo "  ✓ Never deletes existing resources"
    echo "  ✓ Shows clear status of what exists vs what's created"
}

# Handle commands
case "${1:-setup}" in
    "setup")
        main
        ;;
    "status")
        status_only
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo -e "${COLOR_RED}❌ Unknown command: $1${COLOR_NC}"
        echo ""
        show_help
        exit 1
        ;;
esac
