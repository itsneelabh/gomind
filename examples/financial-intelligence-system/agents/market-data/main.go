package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	framework "github.com/itsneelabh/gomind"
)

// MarketDataAgent provides real-time financial market data using Alpha Vantage API
type MarketDataAgent struct {
	framework.BaseAgent
	apiKey string
	client *http.Client
}

// Initialize sets up the market data agent with API credentials
func (m *MarketDataAgent) Initialize(ctx context.Context) error {
	m.apiKey = os.Getenv("ALPHA_VANTAGE_API_KEY")
	if m.apiKey == "" {
		return fmt.Errorf("ALPHA_VANTAGE_API_KEY environment variable is required")
	}

	m.client = &http.Client{
		Timeout: 10 * time.Second,
	}

	m.Logger().Info("Market Data Agent initialized", map[string]interface{}{
		"agent_id":       m.GetAgentID(),
		"api_configured": true,
	})

	return nil
}

// @capability: get-stock-price
// @description: Retrieves real-time stock price for any publicly traded company
// @domain: financial-data
// @complexity: low
// @latency: 1-3s
// @cost: low
// @confidence: 0.95
// @business_value: investment-decisions,portfolio-tracking,market-analysis
// @llm_prompt: Ask me for current stock prices like 'What is AAPL trading at?' or 'Give me the latest price for Tesla'
// @specialties: real-time quotes,NYSE,NASDAQ,market indices,after-hours trading
// @use_cases: portfolio-tracking,investment-decisions,market-research,price-alerts
// @input_types: stock-symbol,company-name
// @output_formats: price-data,market-info,trading-volume
func (m *MarketDataAgent) GetStockPrice(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Alpha Vantage API call for real-time quote
	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=%s&apikey=%s", symbol, m.apiKey)

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock data: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse Alpha Vantage response
	quote, ok := result["Global Quote"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from Alpha Vantage")
	}

	price, _ := strconv.ParseFloat(quote["05. price"].(string), 64)
	change, _ := strconv.ParseFloat(quote["09. change"].(string), 64)
	changePercent := quote["10. change percent"].(string)

	return map[string]interface{}{
		"symbol":             symbol,
		"price":              price,
		"change":             change,
		"change_percent":     strings.TrimSuffix(changePercent, "%"),
		"volume":             quote["06. volume"],
		"latest_trading_day": quote["07. latest trading day"],
		"previous_close":     quote["08. previous close"],
		"open":               quote["02. open"],
		"high":               quote["03. high"],
		"low":                quote["04. low"],
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
		"source":             "Alpha Vantage",
	}, nil
}

// @capability: get-market-overview
// @description: Provides comprehensive market overview including major indices
// @domain: financial-data
// @complexity: medium
// @latency: 2-5s
// @cost: low
// @confidence: 0.90
// @business_value: market-analysis,risk-assessment,investment-strategy
// @llm_prompt: Ask me for market overview like 'How are the markets doing today?' or 'Give me the latest market indices'
// @specialties: market indices,S&P 500,NASDAQ,Dow Jones,market sentiment
// @use_cases: market-analysis,daily-briefings,investment-planning
// @input_types: index-symbols,market-sectors
// @output_formats: market-summary,index-data,sector-performance
func (m *MarketDataAgent) GetMarketOverview(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Major market indices
	indices := []string{"SPY", "QQQ", "DIA", "IWM"} // S&P 500, NASDAQ, Dow, Russell 2000

	var marketData []map[string]interface{}

	for _, symbol := range indices {
		priceData, err := m.GetStockPrice(ctx, map[string]interface{}{"symbol": symbol})
		if err != nil {
			m.Logger().Warn("Failed to fetch index data", map[string]interface{}{
				"symbol": symbol,
				"error":  err.Error(),
			})
			continue
		}
		marketData = append(marketData, priceData)
	}

	return map[string]interface{}{
		"market_indices":  marketData,
		"market_status":   "OPEN", // Simplified - would need market hours logic
		"last_updated":    time.Now().UTC().Format(time.RFC3339),
		"data_source":     "Alpha Vantage",
		"trading_session": "Regular Hours",
	}, nil
}

// @capability: get-historical-data
// @description: Retrieves historical stock price data for trend analysis
// @domain: financial-data
// @complexity: medium
// @latency: 3-8s
// @cost: low
// @confidence: 0.92
// @business_value: trend-analysis,backtesting,historical-research
// @llm_prompt: Ask me for historical data like 'Show me AAPL price history for the last month'
// @specialties: historical prices,trend analysis,time series data,chart data
// @use_cases: backtesting,trend-analysis,research,chart-generation
// @input_types: stock-symbol,time-period,date-range
// @output_formats: time-series-data,historical-prices,ohlcv-data
func (m *MarketDataAgent) GetHistoricalData(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Default to daily data for the last 100 days
	function := "TIME_SERIES_DAILY"
	if period, exists := input["period"]; exists {
		switch period.(string) {
		case "intraday":
			function = "TIME_SERIES_INTRADAY"
		case "weekly":
			function = "TIME_SERIES_WEEKLY"
		case "monthly":
			function = "TIME_SERIES_MONTHLY"
		}
	}

	url := fmt.Sprintf("https://www.alphavantage.co/query?function=%s&symbol=%s&apikey=%s", function, symbol, m.apiKey)
	if function == "TIME_SERIES_INTRADAY" {
		url += "&interval=5min"
	}

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical data: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract time series data
	var timeSeriesKey string
	for key := range result {
		if strings.Contains(key, "Time Series") {
			timeSeriesKey = key
			break
		}
	}

	if timeSeriesKey == "" {
		return nil, fmt.Errorf("no time series data found in response")
	}

	timeSeries := result[timeSeriesKey].(map[string]interface{})

	return map[string]interface{}{
		"symbol":       symbol,
		"time_series":  timeSeries,
		"data_type":    function,
		"last_updated": time.Now().UTC().Format(time.RFC3339),
		"source":       "Alpha Vantage",
		"record_count": len(timeSeries),
	}, nil
}

func main() {
	agent := &MarketDataAgent{}

	// Start the agent with framework auto-configuration
	f, err := framework.NewFramework(
		framework.WithAgentName("market-data-agent"),
		framework.WithPort(8080),
		framework.WithRedisURL(os.Getenv("REDIS_URL")),
	)
	if err != nil {
		fmt.Printf("Failed to create framework: %v\n", err)
		os.Exit(1)
	}

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize the agent
	if err := f.InitializeAgent(ctx, agent); err != nil {
		fmt.Printf("Failed to initialize agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting Market Data Agent on port 8080...")
	fmt.Println("Available capabilities:")
	fmt.Println("  - get-stock-price: Get real-time stock prices")
	fmt.Println("  - get-market-overview: Market indices overview")
	fmt.Println("  - get-historical-data: Historical price data")

	// Start HTTP server
	if err := f.StartHTTPServer(ctx, agent); err != nil {
		fmt.Printf("Failed to start market data agent: %v\n", err)
		os.Exit(1)
	}
}
