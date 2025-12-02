# Grocery Store API

A mock REST API for an online grocery store with **error injection middleware** for resilience testing. This API is designed to work with the `agent-with-resilience` example to demonstrate circuit breaker patterns.

## Features

- **Product Catalog**: Browse and search grocery products
- **Shopping Cart**: Create and manage shopping carts
- **Order Processing**: Create and retrieve orders
- **Error Injection**: Configurable error injection for resilience testing
- **In-Memory Storage**: Fast, thread-safe in-memory data storage

## Error Injection Modes

This API supports three modes for testing resilience patterns:

| Mode | Description | Use Case |
|------|-------------|----------|
| `normal` | All requests succeed | Baseline testing |
| `rate_limit` | Returns 429 after N requests | Test retry with backoff |
| `server_error` | Returns 500 with probability | Test circuit breaker opening |

## Admin Endpoints

Control error injection via these endpoints (bypasses middleware):

```bash
# Set error injection mode
curl -X POST http://localhost:8081/admin/inject-error \
  -H "Content-Type: application/json" \
  -d '{"mode":"rate_limit","rate_limit_after":2,"retry_after_secs":5}'

# Check current status
curl http://localhost:8081/admin/status

# Reset to normal mode
curl -X POST http://localhost:8081/admin/reset
```

### Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `mode` | string | `normal`, `rate_limit`, or `server_error` |
| `rate_limit_after` | int | Number of requests before returning 429 |
| `server_error_rate` | float | Probability of 500 (0.0-1.0) |
| `retry_after_secs` | int | Value for Retry-After header |

## Quick Start

### Running Locally

```bash
# Build and run
go build -o grocery-store-api .
./grocery-store-api

# Or use go run
go run .
```

The server starts on `http://localhost:8081`.

### Running with Docker

```bash
# Build the Docker image
docker build -t grocery-store-api:latest .

# Run the container
docker run -p 8081:8081 grocery-store-api:latest
```

### Deploy to Kubernetes

```bash
# Build and load image to Kind
docker build -t grocery-store-api:latest .
kind load docker-image grocery-store-api:latest

# Deploy
kubectl apply -f k8-deployment.yaml

# Port forward for local access
kubectl port-forward -n gomind-examples svc/grocery-store-api 8081:80
```

## API Endpoints

### Health Check
```
GET /health
```

### Products
```
GET /products              # List all products
GET /products?category=X   # Filter by category
GET /products/{productId}  # Get specific product
```

### Carts
```
POST /carts                       # Create cart
GET /carts/{cartId}               # Get cart
POST /carts/{cartId}/items        # Add item to cart
PATCH /carts/{cartId}/items/{id}  # Update item quantity
DELETE /carts/{cartId}/items/{id} # Remove item
```

### Orders
```
POST /orders              # Create order from cart
GET /orders/{orderId}     # Get order details
```

## Integration with agent-with-resilience

This API is designed to work with the `agent-with-resilience` example:

```
agent-with-resilience → grocery-tool → grocery-store-api
                                              ↑
                                    (error injection here)
```

The `grocery-tool` proxies requests to this API. When error injection is enabled, the agent's circuit breaker and retry mechanisms are exercised.

## Project Structure

```
grocery-store-api/
├── main.go              # Entry point, route setup
├── handlers/            # HTTP request handlers
│   └── handlers.go
├── middleware/          # Error injection middleware
│   └── error_injection.go
├── models/              # Data models
│   └── models.go
├── store/               # In-memory data storage
│   └── store.go
├── go.mod               # Go module definition
├── Dockerfile           # Docker build configuration
├── k8-deployment.yaml   # Kubernetes manifests
└── README.md            # This file
```

## License

MIT License - See [LICENSE](../../../LICENSE) for details.
