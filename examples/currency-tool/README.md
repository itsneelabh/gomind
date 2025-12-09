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

## Quick Start

```bash
cp .env.example .env
./setup.sh run
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | (required) | Redis connection URL |
| `PORT` | `8097` | HTTP server port |
