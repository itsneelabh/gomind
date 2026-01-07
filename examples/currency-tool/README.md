# Currency Tool

A GoMind tool that provides currency conversion using the [Frankfurter](https://frankfurter.dev/) API.

## Features

- **Currency Conversion**: Convert amounts between currencies
- **Exchange Rates**: Get current rates for any base currency
- **Free API**: No API key required, unlimited requests
- **Distributed Tracing**: Built-in trace context propagation

## Capabilities

### convert_currency

Converts an amount from one currency to another.

```bash
curl -X POST http://localhost:8097/api/capabilities/convert_currency \
  -H "Content-Type: application/json" \
  -d '{"from": "USD", "to": "JPY", "amount": 1000}'
```

### get_exchange_rates

Gets exchange rates for a base currency.

```bash
curl -X POST http://localhost:8097/api/capabilities/get_exchange_rates \
  -H "Content-Type: application/json" \
  -d '{"base": "USD", "currencies": ["EUR", "GBP", "JPY"]}'
```

## Prerequisites

Before running this tool, ensure you have:

- **Docker**: Required for building and running containers
- **Kind**: Kubernetes in Docker for local cluster ([install guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
- **Go 1.25+**: For local development
- **Redis**: For service discovery (deployed automatically by setup.sh)

### Configure Environment

```bash
cd examples/currency-tool
cp .env.example .env
# Edit .env if needed (defaults work for local development)
```

## Quick Start

```bash
# Ensure .env is configured per Prerequisites
./setup.sh run
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | (required) | Redis connection URL |
| `PORT` | `8097` | HTTP server port |
