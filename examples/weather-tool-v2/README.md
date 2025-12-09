# Weather Tool v2

A GoMind tool that provides weather forecast capabilities using the [Open-Meteo](https://open-meteo.com/) API.

## Features

- **Current Weather**: Get current temperature, humidity, wind, and conditions
- **Multi-Day Forecast**: Up to 16-day weather forecast
- **Free API**: No API key required
- **Distributed Tracing**: Built-in trace context propagation
- **Coordinate-Based**: Works with any lat/lon coordinates

## Why v2?

This tool uses Open-Meteo instead of commercial weather APIs:
- **Free forever**: No API key, no credit card
- **Unlimited requests**: No rate limiting for reasonable use
- **Accurate data**: Uses NOAA, DWD, and other meteorological sources
- **Works with geocoding**: Accepts coordinates from geocoding-tool

## Capabilities

### get_weather_forecast

Gets current weather and multi-day forecast for coordinates.

**Request:**
```json
{
  "lat": 35.6762,
  "lon": 139.6503,
  "days": 7
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "lat": 35.6762,
    "lon": 139.6503,
    "timezone": "Asia/Tokyo",
    "temperature_current": 12.5,
    "temperature_min": 5,
    "temperature_max": 15,
    "temperature_avg": 10.5,
    "condition": "Partly cloudy",
    "weather_code": 2,
    "humidity": 55,
    "wind_speed": 12.3,
    "forecast": [
      {
        "date": "2024-12-03",
        "temperature_min": 5,
        "temperature_max": 15,
        "condition": "Partly cloudy",
        "precipitation": 0,
        "wind_speed_max": 15
      }
    ]
  }
}
```

### get_current_weather

Gets current weather conditions only (no forecast).

**Request:**
```json
{
  "lat": 40.7128,
  "lon": -74.0060
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "lat": 40.7128,
    "lon": -74.006,
    "timezone": "America/New_York",
    "temperature_current": 8.5,
    "condition": "Clear sky",
    "humidity": 45,
    "wind_speed": 8.2
  }
}
```

## Quick Start

### Prerequisites

- Go 1.25+
- Redis (for service discovery)

### Local Development

```bash
# Copy environment template
cp .env.example .env

# Start Redis (if not running)
docker run -d --name redis -p 6379:6379 redis:alpine

# Build and run
./setup.sh run
```

### Docker

```bash
# Build image
./setup.sh docker-build

# Run container
docker run -p 8096:8096 \
  -e REDIS_URL=redis://host.docker.internal:6379 \
  weather-tool-v2:latest
```

### Kubernetes

```bash
# Deploy to cluster
./setup.sh deploy

# Check status
kubectl get pods -n gomind-examples -l app=weather-tool-v2
```

## API Usage

### Test with curl

```bash
# Weather forecast (with coordinates from geocoding)
curl -X POST http://localhost:8096/api/capabilities/get_weather_forecast \
  -H "Content-Type: application/json" \
  -d '{"lat": 35.6762, "lon": 139.6503, "days": 7}'

# Current weather only
curl -X POST http://localhost:8096/api/capabilities/get_current_weather \
  -H "Content-Type: application/json" \
  -d '{"lat": 40.7128, "lon": -74.0060}'

# Health check
curl http://localhost:8096/health

# List capabilities
curl http://localhost:8096/api/capabilities
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | (required) | Redis connection URL |
| `PORT` | `8096` | HTTP server port |
| `APP_ENV` | `development` | Environment (development/staging/production) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | - | OpenTelemetry collector endpoint |
| `DEV_MODE` | `false` | Enable development mode |
| `GOMIND_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

## Weather Codes

The tool converts WMO weather codes to human-readable conditions:

| Code | Condition |
|------|-----------|
| 0 | Clear sky |
| 1-3 | Mainly clear to Overcast |
| 45-48 | Fog |
| 51-57 | Drizzle |
| 61-67 | Rain |
| 71-77 | Snow |
| 80-82 | Rain showers |
| 85-86 | Snow showers |
| 95-99 | Thunderstorm |

## Distributed Tracing

This tool uses `otelhttp` to propagate trace context. When called from an orchestrator:

```
orchestrator (parent span)
  └── geocoding-tool (child span)
        └── weather-tool-v2 (sibling span)
              └── open-meteo-api-call (grandchild span)
```

## Part of agent-with-orchestration

This tool is designed to work with the Smart Travel Research Assistant example. It depends on geocoding-tool to provide coordinates for weather lookups.

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/capabilities` | GET | List all capabilities |
| `/api/capabilities/get_weather_forecast` | POST | Weather forecast |
| `/api/capabilities/get_weather_forecast/schema` | GET | Input schema |
| `/api/capabilities/get_current_weather` | POST | Current weather |
| `/api/capabilities/get_current_weather/schema` | GET | Input schema |
