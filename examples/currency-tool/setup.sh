#!/bin/bash
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"

case "${1:-help}" in
    build)
        GOWORK=off go mod tidy && GOWORK=off go build -o currency-tool .
        echo "Build completed: currency-tool"
        ;;
    run)
        [ -f .env ] && source .env
        [ -z "$REDIS_URL" ] && echo "REDIS_URL required" && exit 1
        GOWORK=off go mod tidy && GOWORK=off go build -o currency-tool .
        ./currency-tool
        ;;
    docker-build)
        docker build -t currency-tool:latest .
        ;;
    deploy)
        docker build -t currency-tool:latest .
        command -v kind &>/dev/null && kind load docker-image currency-tool:latest --name "gomind-demo-$(whoami)"
        kubectl apply -f k8-deployment.yaml
        kubectl wait --for=condition=available --timeout=120s deployment/currency-tool -n gomind-examples 2>/dev/null || true
        echo "currency-tool deployed successfully!"
        ;;
    test)
        curl -s -X POST http://localhost:${PORT:-8097}/api/capabilities/convert_currency \
            -H "Content-Type: application/json" \
            -d '{"from": "USD", "to": "JPY", "amount": 1000}' | jq .
        ;;
    *)
        echo "Usage: ./setup.sh {build|run|docker-build|deploy|test}"
        ;;
esac
