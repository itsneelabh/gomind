# Stock Market Tool

A GoMind tool that provides real-time stock market data using the [Finnhub.io](https://finnhub.io/) API. This tool demonstrates the passive tool pattern - it registers capabilities with the service mesh but does not discover other components.

## Features

- **Real-time Stock Quotes**: Get current price, change, high, low, and trading data
- **Company Profiles**: Retrieve company information, market cap, industry, and more
- **Company News**: Fetch recent news articles for specific stocks
- **Market News**: Get general market news and headlines
- **Automatic Service Discovery**: Registers with Redis for agent discovery
- **Schema Discovery**: Full support for 3-phase tool invocation pattern
- **Graceful Fallback**: Uses mock data when API key is not configured

## Prerequisites

- Go 1.23 or higher
- Docker (for containerization)
- Kubernetes cluster (local Kind or production)
- Redis (for service discovery)
- Finnhub API key (free tier available at [finnhub.io](https://finnhub.io/register))

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `FINNHUB_API_KEY` | Finnhub API key for real data | No* | Mock data used |
| `REDIS_URL` | Redis connection URL | Yes | - |
| `PORT` | HTTP server port | No | 8082 |
| `NAMESPACE` | Kubernetes namespace | No | default |
| `DEV_MODE` | Development mode flag | No | false |
| `GOMIND_LOG_LEVEL` | Logging level (error\|warn\|info\|debug) | No | info |
| `GOMIND_LOG_FORMAT` | Log format (json\|text) | No | json |

\* Tool works without API key but returns simulated data

## Getting Your Finnhub API Key

1. Visit [https://finnhub.io/register](https://finnhub.io/register)
2. Sign up for a free account
3. Navigate to your dashboard
4. Copy your API key
5. Free tier includes:
   - 60 API calls per minute
   - Real-time stock quotes
   - Company profiles
   - News articles
   - 1 year of historical data

## Local Development

### Quick Start

```bash
# Set environment variables
export FINNHUB_API_KEY="your-api-key-here"
export REDIS_URL="redis://localhost:6379"
export PORT=8082

# Run the tool
go run .
```

### Build Binary

```bash
go build -o stock-tool .
./stock-tool
```

## Registered Capabilities

The tool registers these capabilities with the service mesh:

### 1. Stock Quote (`stock_quote`)
**Endpoint**: `/api/capabilities/stock_quote`

Gets real-time stock price and trading data.

**Request**:
```json
{
  "symbol": "AAPL"
}
```

**Response**:
```json
{
  "symbol": "AAPL",
  "current_price": 178.25,
  "change": 2.45,
  "percent_change": 1.39,
  "high": 179.50,
  "low": 176.80,
  "open": 177.00,
  "previous_close": 175.80,
  "timestamp": 1704928800,
  "source": "Finnhub API"
}
```

### 2. Company Profile (`company_profile`)
**Endpoint**: `/api/capabilities/company_profile`

Gets comprehensive company information.

**Request**:
```json
{
  "symbol": "TSLA"
}
```

**Response**:
```json
{
  "name": "Tesla Inc.",
  "ticker": "TSLA",
  "exchange": "NASDAQ",
  "industry": "Auto Manufacturers",
  "country": "US",
  "currency": "USD",
  "market_capitalization": 789000.5,
  "ipo": "2010-06-29",
  "website": "https://www.tesla.com",
  "logo": "https://finnhub.io/api/logo?symbol=TSLA",
  "source": "Finnhub API"
}
```

### 3. Company News (`company_news`)
**Endpoint**: `/api/capabilities/company_news`

Fetches recent news articles for a specific stock.

**Request**:
```json
{
  "symbol": "NVDA",
  "from": "2024-01-01",
  "to": "2024-01-31"
}
```

**Response**:
```json
{
  "symbol": "NVDA",
  "news": [
    {
      "headline": "NVIDIA announces new AI chip",
      "summary": "Company reveals next-generation GPU for data centers...",
      "source": "TechCrunch",
      "url": "https://...",
      "image": "https://...",
      "published": 1704928800
    }
  ],
  "from": "2024-01-01",
  "to": "2024-01-31",
  "source": "Finnhub API"
}
```

### 4. Market News (`market_news`)
**Endpoint**: `/api/capabilities/market_news`

Gets general market news and headlines.

**Request**:
```json
{
  "category": "general"
}
```

Categories: `general`, `forex`, `crypto`, `merger`

## Kubernetes Deployment

### Deploy to Kind Cluster

```bash
# Build Docker image
docker build -t stock-tool:latest .

# Load into Kind
kind load docker-image stock-tool:latest

# Update API key in k8-deployment.yaml
# Edit the FINNHUB_API_KEY value in the Secret

# Deploy
kubectl apply -f k8-deployment.yaml

# Verify deployment
kubectl get pods -n gomind-examples -l app=stock-tool
kubectl logs -n gomind-examples -l app=stock-tool
```

### Test the Deployment

```bash
# Port forward to access locally
kubectl port-forward -n gomind-examples svc/stock-service 8082:80

# Test stock quote
curl -X POST http://localhost:8082/api/capabilities/stock_quote \
  -H "Content-Type: application/json" \
  -d '{"symbol": "AAPL"}'

# Test company profile
curl -X POST http://localhost:8082/api/capabilities/company_profile \
  -H "Content-Type: application/json" \
  -d '{"symbol": "GOOGL"}'
```

## Integration with Research Agent

Once deployed, the stock tool is automatically discovered by the research agent via Redis. You can query stock data through natural language:

```bash
# Query through research agent
curl -X POST http://localhost:8091/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "current price of Apple stock",
    "use_ai": true
  }'

# Multi-entity comparison
curl -X POST http://localhost:8091/api/capabilities/research_topic \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "Compare AAPL vs GOOGL vs MSFT stock prices",
    "use_ai": true
  }'
```

The research agent will:
1. Use AI to understand the query is about stock data
2. Discover the stock-service via Redis
3. Select it as the relevant tool
4. Generate the correct payload (symbol extraction)
5. Call the appropriate capability
6. Synthesize results with AI analysis

## Architecture

```
Stock Tool (Passive)
    ├── Registers capabilities in Redis
    ├── Receives requests from agents
    ├── Calls Finnhub API
    ├── Falls back to mock data if API fails
    └── Returns standardized responses

Research Agent (Active)
    ├── Discovers stock tool via Redis
    ├── Uses AI for tool selection
    ├── Generates payloads automatically
    └── Orchestrates multi-tool workflows
```

## API Rate Limits

Free tier limits (Finnhub):
- **60 calls per minute**
- **Monthly limit**: Varies by endpoint

The tool implements:
- 1-minute result caching for stock quotes
- Graceful fallback to mock data on errors
- Structured error logging for rate limit tracking

## Troubleshooting

### Tool not appearing in discovery

```bash
# Check Redis connection
kubectl exec -n default redis-0 -- redis-cli KEYS "gomind:*"

# Should show: gomind:service:stock-service
```

### API errors

```bash
# Check logs for API key issues
kubectl logs -n gomind-examples -l app=stock-tool | grep -i "api"

# Common issues:
# - Invalid API key: Check secret configuration
# - Rate limit: Wait 1 minute or upgrade Finnhub plan
# - Invalid symbol: Use valid US stock ticker symbols
```

### Mock data being used

If you see "Mock Data" in responses:
1. Verify FINNHUB_API_KEY is set in the secret
2. Check pod environment: `kubectl exec -n gomind-examples <pod-name> -- env | grep FINNHUB`
3. Restart deployment: `kubectl rollout restart deployment/stock-tool -n gomind-examples`

## Development

### Project Structure

```
stock-market-tool/
├── main.go                 # Entry point, framework setup
├── stock_tool.go           # Tool definition, capability registration
├── finnhub_client.go       # Finnhub API client
├── handlers.go             # HTTP handlers for each capability
├── go.mod                  # Go module definition
├── Dockerfile              # Container image definition
├── k8-deployment.yaml      # Kubernetes manifests
└── README.md              # This file
```

### Adding New Capabilities

1. Add request/response types in `stock_tool.go`
2. Register capability in `registerCapabilities()`
3. Implement handler in `handlers.go`
4. Add Finnhub client method in `finnhub_client.go` if needed

### Testing

```bash
# Local testing without Redis
export FINNHUB_API_KEY="your-key"
export REDIS_URL="redis://localhost:6379"
go run .

# Test endpoints directly
curl -X POST http://localhost:8082/api/capabilities/stock_quote \
  -H "Content-Type: application/json" \
  -d '{"symbol": "AAPL"}'
```

## License

Part of the GoMind framework examples.
