# Geocoding Tool

A GoMind tool that provides location geocoding capabilities using the [Nominatim](https://nominatim.org/) API (OpenStreetMap).

## Features

- **Forward Geocoding**: Convert location names to geographic coordinates
- **Reverse Geocoding**: Convert coordinates to location names
- **Distributed Tracing**: Built-in trace context propagation
- **Service Discovery**: Automatic registration with Redis

## Capabilities

### geocode_location

Converts a location name to geographic coordinates.

**Request:**
```json
{
  "location": "Tokyo, Japan"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "lat": 35.6762,
    "lon": 139.6503,
    "display_name": "Tokyo, Japan",
    "country_code": "jp",
    "country": "Japan",
    "city": "Tokyo"
  }
}
```

### reverse_geocode

Converts coordinates to a location name.

**Request:**
```json
{
  "lat": 35.6762,
  "lon": 139.6503
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "lat": 35.6762,
    "lon": 139.6503,
    "display_name": "Tokyo, Kanto Region, Japan",
    "country_code": "jp",
    "country": "Japan",
    "city": "Tokyo"
  }
}
```

## Prerequisites

Before running this tool, ensure you have:

- **Docker**: Required for building and running containers
- **Kind**: Kubernetes in Docker for local cluster ([install guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
- **Go 1.25+**: For local development
- **Redis**: For service discovery (deployed automatically by setup.sh)

### Configure Environment

```bash
cd examples/geocoding-tool
cp .env.example .env
# Edit .env if needed (defaults work for local development)
```

## Quick Start

### Local Development

```bash
# Start Redis (if not running)
docker run -d --name redis -p 6379:6379 redis:alpine

# Build and run (ensure .env is configured per Prerequisites)
./setup.sh run
```

### Docker

```bash
# Build image
./setup.sh docker-build

# Run container
docker run -p 8095:8095 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  geocoding-tool:latest
```

### Kubernetes

```bash
# Deploy to cluster
./setup.sh deploy

# Check status
kubectl get pods -n gomind-examples -l app=geocoding-tool
```

## API Usage

### Test with curl

```bash
# Forward geocoding
curl -X POST http://localhost:8095/api/capabilities/geocode_location \
  -H "Content-Type: application/json" \
  -d '{"location": "New York, USA"}'

# Reverse geocoding
curl -X POST http://localhost:8095/api/capabilities/reverse_geocode \
  -H "Content-Type: application/json" \
  -d '{"lat": 40.7128, "lon": -74.0060}'

# Health check
curl http://localhost:8095/health

# List capabilities
curl http://localhost:8095/api/capabilities
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | (required) | Redis connection URL |
| `PORT` | `8095` | HTTP server port |
| `APP_ENV` | `development` | Environment (development/staging/production) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `DEV_MODE` | `false` | Enable development mode |
| `GOMIND_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

## Rate Limiting

Nominatim API has a strict rate limit of **1 request per second**. This tool includes a built-in delay to respect this limit. For production use, consider:

- Implementing a proper rate limiter
- Caching geocoding results
- Using a commercial geocoding service

## Distributed Tracing

This tool uses `telemetry.NewTracedHTTPClient` to propagate trace context to the Nominatim API. When called from an orchestrator:

```
orchestrator (parent span)
  └── geocoding-tool (child span)
        └── nominatim-api-call (grandchild span)
```

## Part of agent-with-orchestration

This tool is designed to work with the Smart Travel Research Assistant example. It provides geocoding for travel destinations, which is used by the weather-tool-v2 to fetch weather data.

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/capabilities` | GET | List all capabilities |
| `/api/capabilities/geocode_location` | POST | Forward geocoding |
| `/api/capabilities/geocode_location/schema` | GET | Input schema |
| `/api/capabilities/reverse_geocode` | POST | Reverse geocoding |
| `/api/capabilities/reverse_geocode/schema` | GET | Input schema |
