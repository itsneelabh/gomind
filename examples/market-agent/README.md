# Market Agent Example

This example demonstrates an AI-powered market analysis agent using the GoMind Agent Framework.

## What This Example Shows

- Advanced agent with multiple related capabilities
- AI integration for market analysis (with fallback logic)
- Complex data structures and business logic
- Agent collaboration patterns
- Error handling and graceful degradation

## Agent Capabilities

The `MarketAgent` implements four market-related capabilities:

1. **get_market_data** - Retrieves current market data for a stock symbol
2. **analyze_market** - Performs AI-powered market analysis with trend prediction
3. **predict_price** - Predicts future price movement with confidence intervals
4. **market_sentiment** - Analyzes overall market sentiment using AI

## Prerequisites

For AI-powered features, you'll need an OpenAI API key:

```bash
export OPENAI_API_KEY="your-openai-api-key-here"
```

The agent will work without an API key but will use fallback logic instead of AI analysis.

## Running the Example

1. Navigate to this directory:
   ```bash
   cd examples/market-agent
   ```

2. (Optional) Set your OpenAI API key:
   ```bash
   export OPENAI_API_KEY="sk-..."
   ```

3. Run the agent:
   ```bash
   go run main.go
   ```

4. Open your browser to `http://localhost:8080`

## Testing the Agent

### Using the Web Interface
1. Visit `http://localhost:8080`
2. Ask questions like:
   - "Get market data for AAPL"
   - "Analyze the market for Tesla"
   - "Predict GOOGL price for 1 week"
   - "What's the market sentiment around tech stocks?"

### Using HTTP API

```bash
# Get market data
curl -X POST http://localhost:8080/agents/market-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "get_market_data", "input": {"symbol": "AAPL"}}'

# Analyze market
curl -X POST http://localhost:8080/agents/market-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "analyze_market", "input": {"symbol": "TSLA"}}'

# Price prediction
curl -X POST http://localhost:8080/agents/market-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "predict_price", "input": {"symbol": "GOOGL", "horizon": "1w"}}'

# Market sentiment
curl -X POST http://localhost:8080/agents/market-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "market_sentiment", "input": {"query": "tech stocks outlook"}}'
```

## Key Features Demonstrated

### 1. Complex Data Structures
```go
type MarketAnalysis struct {
    Symbol      string    `json:"symbol"`
    Trend       string    `json:"trend"`
    Confidence  float64   `json:"confidence"`
    Prediction  string    `json:"prediction"`
    Reasoning   string    `json:"reasoning"`
    Timestamp   time.Time `json:"timestamp"`
}
```

### 2. AI Integration with Fallback
```go
func (m *MarketAgent) AnalyzeMarket(symbol string) MarketAnalysis {
    // Try AI first
    if response, err := m.ContactAgent(ctx, "ai-assistant", prompt); err == nil {
        return parseAIResponse(response)
    }
    
    // Fallback to rule-based analysis
    return m.fallbackAnalysis(data)
}
```

### 3. Multiple Input Parameters
```go
// @capability: predict_price
// @input: symbol string "Stock symbol to predict"
// @input: horizon string "Time horizon (1d, 1w, 1m)"
func (m *MarketAgent) PredictPrice(symbol, horizon string) TrendPrediction {
    // Implementation
}
```

### 4. Environment-Based Configuration
```go
if os.Getenv("OPENAI_API_KEY") != "" {
    log.Println("OpenAI API key detected - AI features enabled")
} else {
    log.Println("No OpenAI API key - running with fallback logic")
}
```

## Sample Responses

### Market Data Response
```json
{
  "symbol": "AAPL",
  "price": 157.32,
  "change": 2.18,
  "volume": 845273,
  "timestamp": "2025-08-14T23:15:00Z"
}
```

### Market Analysis Response
```json
{
  "symbol": "AAPL",
  "trend": "bullish",
  "confidence": 0.7,
  "prediction": "Buy",
  "reasoning": "Strong upward momentum with positive sentiment indicators",
  "timestamp": "2025-08-14T23:15:00Z"
}
```

### Price Prediction Response
```json
{
  "symbol": "AAPL",
  "current_price": 157.32,
  "predicted_price": 162.45,
  "time_horizon": "1w",
  "confidence": 0.75,
  "timestamp": "2025-08-14T23:15:00Z"
}
```

## Extending the Agent

This example provides a foundation for building real market analysis tools:

1. **Real Data Integration**: Replace simulated data with real API calls (Alpha Vantage, Yahoo Finance, etc.)
2. **Advanced AI Models**: Integrate specialized financial AI models
3. **Technical Indicators**: Add support for RSI, MACD, Bollinger Bands, etc.
4. **Portfolio Management**: Extend to manage entire portfolios
5. **Real-time Streaming**: Add WebSocket support for live data feeds

## Next Steps

- Check out the [Multi-Agent Example](../multi-agent/) to see agents working together
- Review the [API Documentation](../../docs/API.md) for advanced framework features
- Explore the [Basic Agent Example](../basic-agent/) for simpler implementations
