#!/bin/bash
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

case "${1:-help}" in
    build) GOWORK=off go mod tidy && GOWORK=off go build -o country-info-tool . ;;
    run) [ -f .env ] && source .env; GOWORK=off go mod tidy && GOWORK=off go build -o country-info-tool . && ./country-info-tool ;;
    docker-build) docker build -t country-info-tool:latest . ;;
    deploy)
        docker build -t country-info-tool:latest .
        command -v kind &>/dev/null && kind load docker-image country-info-tool:latest --name "gomind-demo-$(whoami)"
        kubectl apply -f k8-deployment.yaml
        kubectl wait --for=condition=available --timeout=120s deployment/country-info-tool -n gomind-examples 2>/dev/null || true
        echo "country-info-tool deployed successfully!"
        ;;
    test) curl -s -X POST http://localhost:${PORT:-8098}/api/capabilities/get_country_info -H "Content-Type: application/json" -d '{"country": "Japan"}' | jq . ;;
    *) echo "Usage: ./setup.sh {build|run|docker-build|deploy|test}" ;;
esac
