# Country Info Tool

A GoMind tool that provides country information using the [RestCountries](https://restcountries.com/) API.

## Features

- **Country Details**: Get capital, population, languages, currency, timezones
- **Flag Emoji & URL**: Unicode flag emoji and PNG flag URL
- **Free API**: No API key required

## Capabilities

### get_country_info

```bash
curl -X POST http://localhost:8098/api/capabilities/get_country_info \
  -H "Content-Type: application/json" \
  -d '{"country": "Japan"}'
```

**Response:**
```json
{
  "name": "Japan",
  "capital": "Tokyo",
  "region": "Asia",
  "population": 125836021,
  "languages": ["Japanese"],
  "currency": {"code": "JPY", "name": "Japanese yen", "symbol": "円"},
  "timezones": ["UTC+09:00"],
  "flag": "🇯🇵"
}
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
| `PORT` | `8098` | HTTP server port |
