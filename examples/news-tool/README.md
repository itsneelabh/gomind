# News Tool

A GoMind tool that provides news search capabilities using the [GNews.io](https://gnews.io/) API.

## Features

- **News Search**: Search for articles by topic or keyword
- **Multi-Language**: Support for multiple languages
- **Configurable**: Control number of results
- **Distributed Tracing**: Built-in trace context propagation

## API Key Required

GNews.io requires a free API key:

1. Sign up at https://gnews.io/
2. Get your API key from the dashboard
3. Set `GNEWS_API_KEY` environment variable

**Free Tier:** 100 requests/day

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
# Set up environment
cp .env.example .env
# Edit .env and add your GNEWS_API_KEY

# Run
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
