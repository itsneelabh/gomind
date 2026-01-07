# News Tool

A GoMind tool that provides news search capabilities using the [GNews.io](https://gnews.io/) API.

## Features

- **News Search**: Search for articles by topic or keyword
- **Multi-Language**: Support for multiple languages
- **Configurable**: Control number of results
- **Distributed Tracing**: Built-in trace context propagation

## Prerequisites

Before running this tool, ensure you have:

- **Docker**: Required for building and running containers
- **Kind**: Kubernetes in Docker for local cluster ([install guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
- **Go 1.25+**: For local development
- **Redis**: For service discovery (deployed automatically by setup.sh)
- **GNews.io API Key**: Required for news data (free tier available)

### Get Your GNews API Key

1. Sign up at https://gnews.io/
2. Get your API key from the dashboard
3. Free tier includes 100 requests/day

### Configure Environment

```bash
cd examples/news-tool
cp .env.example .env
# Edit .env and add your GNEWS_API_KEY
```

## Capabilities

### search_news

Searches for news articles related to a query.

```bash
curl -X POST http://localhost:8099/api/capabilities/search_news \
  -H "Content-Type: application/json" \
  -d '{"query": "Tokyo travel", "max_results": 5}'
```

**Response:**
```json
{
  "success": true,
  "data": {
    "total_articles": 100,
    "articles": [
      {
        "title": "Best Time to Visit Tokyo",
        "description": "A guide to Tokyo's seasons...",
        "url": "https://...",
        "source": "Travel Weekly",
        "published_at": "2024-12-01T10:00:00Z"
      }
    ]
  }
}
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
| `PORT` | `8099` | HTTP server port |
| `GNEWS_API_KEY` | (required) | GNews.io API key |

## Rate Limiting

The free tier allows 100 requests per day. The tool will return a clear error when the limit is exceeded.
