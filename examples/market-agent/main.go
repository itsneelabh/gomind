package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/itsneelabh/gomind"
)

// MarketData represents market information
type MarketData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	Volume    int64     `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

// MarketAnalysis represents the analysis result
type MarketAnalysis struct {
	Symbol     string    `json:"symbol"`
	Trend      string    `json:"trend"`      // "bullish", "bearish", "neutral"
	Confidence float64   `json:"confidence"` // 0-1
	Prediction string    `json:"prediction"`
	Reasoning  string    `json:"reasoning"`
	Timestamp  time.Time `json:"timestamp"`
}

// TrendPrediction represents price prediction
type TrendPrediction struct {
	Symbol         string    `json:"symbol"`
	CurrentPrice   float64   `json:"current_price"`
	PredictedPrice float64   `json:"predicted_price"`
	TimeHorizon    string    `json:"time_horizon"`
	Confidence     float64   `json:"confidence"`
	Timestamp      time.Time `json:"timestamp"`
}

// MarketAgent demonstrates AI-powered market analysis
type MarketAgent struct {
	framework.BaseAgent
}

// @capability: get_market_data
// @description: Retrieves current market data for a given symbol
// @input: symbol string "Stock symbol (e.g., AAPL, GOOGL)"
// @output: market_data object "Current market data including price, volume, and change"
func (m *MarketAgent) GetMarketData(symbol string) MarketData {
	// Simulate market data (in real implementation, this would fetch from an API)
	basePrice := map[string]float64{
		"AAPL":  150.0,
		"GOOGL": 2800.0,
		"MSFT":  300.0,
		"TSLA":  800.0,
		"AMZN":  3200.0,
	}

	symbol = strings.ToUpper(symbol)
	price, exists := basePrice[symbol]
	if !exists {
		price = 100.0 // Default price for unknown symbols
	}

	// Add some random variation
	variation := (rand.Float64() - 0.5) * 0.1 // ±5% variation
	currentPrice := price * (1 + variation)
	change := currentPrice - price

	return MarketData{
		Symbol:    symbol,
		Price:     currentPrice,
		Change:    change,
		Volume:    rand.Int63n(1000000) + 100000, // Random volume
		Timestamp: time.Now(),
	}
}

// @capability: analyze_market
// @description: Performs AI-powered market analysis for a given symbol
// @input: symbol string "Stock symbol to analyze"
// @output: analysis object "Detailed market analysis with trend and prediction"
func (m *MarketAgent) AnalyzeMarket(symbol string) MarketAnalysis {
	// Get current market data
	data := m.GetMarketData(symbol)

	// Try to use AI if available
	ctx := context.Background()
	var aiResponse string
	var err error

	if os.Getenv("OPENAI_API_KEY") != "" {
		prompt := fmt.Sprintf(`Analyze the market data for %s:
		Current Price: $%.2f
		Price Change: $%.2f
		Volume: %d
		
		Provide a brief analysis including:
		1. Trend (bullish/bearish/neutral)
		2. Confidence level (0-1)
		3. Short reasoning
		
		Respond in a structured format.`, data.Symbol, data.Price, data.Change, data.Volume)

		// Use the AI capability through the base agent if available
		if response, aiErr := m.ContactAgent(ctx, "ai-assistant", prompt); aiErr == nil {
			aiResponse = response
		} else {
			err = aiErr
		}
	}

	if err != nil || aiResponse == "" {
		// Fallback to rule-based analysis if AI fails
		return m.fallbackAnalysis(data)
	}

	// Parse AI response and create analysis
	trend := "neutral"
	confidence := 0.5
	reasoning := aiResponse

	// Simple parsing logic (in production, use more sophisticated parsing)
	if strings.Contains(strings.ToLower(aiResponse), "bullish") {
		trend = "bullish"
		confidence = 0.7
	} else if strings.Contains(strings.ToLower(aiResponse), "bearish") {
		trend = "bearish"
		confidence = 0.7
	}

	// Determine prediction based on trend
	prediction := "Hold"
	switch trend {
	case "bullish":
		prediction = "Buy"
	case "bearish":
		prediction = "Sell"
	}

	return MarketAnalysis{
		Symbol:     data.Symbol,
		Trend:      trend,
		Confidence: confidence,
		Prediction: prediction,
		Reasoning:  reasoning,
		Timestamp:  time.Now(),
	}
}

// @capability: predict_price
// @description: Predicts future price movement for a given symbol
// @input: symbol string "Stock symbol to predict"
// @input: horizon string "Time horizon (1d, 1w, 1m)"
// @output: prediction object "Price prediction with confidence interval"
func (m *MarketAgent) PredictPrice(symbol, horizon string) TrendPrediction {
	data := m.GetMarketData(symbol)

	// Simple prediction logic (in production, use ML models)
	var multiplier float64
	switch horizon {
	case "1d":
		multiplier = 1 + (rand.Float64()-0.5)*0.05 // ±2.5% for 1 day
	case "1w":
		multiplier = 1 + (rand.Float64()-0.5)*0.15 // ±7.5% for 1 week
	case "1m":
		multiplier = 1 + (rand.Float64()-0.5)*0.30 // ±15% for 1 month
	default:
		multiplier = 1 + (rand.Float64()-0.5)*0.10 // ±5% default
	}

	predictedPrice := data.Price * multiplier
	confidence := 0.6 + rand.Float64()*0.3 // 60-90% confidence

	return TrendPrediction{
		Symbol:         data.Symbol,
		CurrentPrice:   data.Price,
		PredictedPrice: predictedPrice,
		TimeHorizon:    horizon,
		Confidence:     confidence,
		Timestamp:      time.Now(),
	}
}

// @capability: market_sentiment
// @description: Analyzes overall market sentiment using AI
// @input: query string "Market query or concern"
// @output: sentiment string "AI-generated market sentiment analysis"
func (m *MarketAgent) MarketSentiment(query string) string {
	ctx := context.Background()

	prompt := fmt.Sprintf(`As a market analyst, provide insights on: %s
	
	Consider current market conditions, recent trends, and provide a balanced perspective.
	Keep the response concise but informative.`, query)

	// Try to use AI via agent collaboration
	if response, err := m.ContactAgent(ctx, "ai-assistant", prompt); err == nil {
		return response
	}

	return fmt.Sprintf("Market sentiment analysis for '%s': Based on current indicators, "+
		"market conditions appear stable with normal trading patterns. "+
		"Please consult additional resources for detailed analysis.", query)
}

// fallbackAnalysis provides rule-based analysis when AI is unavailable
func (m *MarketAgent) fallbackAnalysis(data MarketData) MarketAnalysis {
	trend := "neutral"
	confidence := 0.5
	prediction := "Hold"
	reasoning := "Basic technical analysis based on price movement"

	changePercent := (data.Change / (data.Price - data.Change)) * 100

	if changePercent > 2 {
		trend = "bullish"
		prediction = "Buy"
		confidence = 0.6
		reasoning = fmt.Sprintf("Strong upward movement (+%.2f%%)", changePercent)
	} else if changePercent < -2 {
		trend = "bearish"
		prediction = "Sell"
		confidence = 0.6
		reasoning = fmt.Sprintf("Strong downward movement (%.2f%%)", changePercent)
	}

	return MarketAnalysis{
		Symbol:     data.Symbol,
		Trend:      trend,
		Confidence: confidence,
		Prediction: prediction,
		Reasoning:  reasoning,
		Timestamp:  time.Now(),
	}
}

func main() {
	// Create framework instance
	fw, err := framework.NewFramework(
		framework.WithPort(8080),
		framework.WithAgentName("market-agent"),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// Create the market agent
	marketAgent := &MarketAgent{}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping framework...")
		cancel()
	}()

	log.Println("Starting market analysis agent on port 8080...")
	log.Println("Visit http://localhost:8080 for the chat interface")
	log.Println("Available capabilities:")
	log.Println("  - get_market_data: Get real-time market data")
	log.Println("  - analyze_market: AI-powered market analysis")
	log.Println("  - predict_price: Price prediction with confidence")
	log.Println("  - market_sentiment: AI market sentiment analysis")

	if os.Getenv("OPENAI_API_KEY") != "" {
		log.Println("OpenAI API key detected - AI features enabled")
	} else {
		log.Println("No OpenAI API key - running with fallback logic")
	}

	// Initialize and start the framework
	if err := fw.InitializeAgent(ctx, marketAgent); err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	// Start the HTTP server
	if err := fw.StartHTTPServer(ctx, marketAgent); err != nil {
		log.Fatalf("Framework error: %v", err)
	}

	log.Println("Market agent stopped gracefully")
}
