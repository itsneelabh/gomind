package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"

	framework "github.com/itsneelabh/gomind"
)

// TechnicalAnalysisAgent provides technical analysis capabilities for financial markets
type TechnicalAnalysisAgent struct {
	*framework.BaseAgent
}

// NewTechnicalAnalysisAgent creates a new technical analysis agent
func NewTechnicalAnalysisAgent() *TechnicalAnalysisAgent {
	return &TechnicalAnalysisAgent{}
}

// @llm_prompt: "You are a Technical Analysis Agent specializing in financial market technical indicators, chart patterns, and trading signals. You analyze price movements, volume patterns, and technical indicators to provide trading insights."
// @specialties: ["technical-analysis", "chart-patterns", "trading-signals", "price-analysis", "volume-analysis", "indicators", "resistance-support", "momentum-analysis"]
// @capability: CalculateTechnicalIndicators
// @description: Calculate various technical indicators like RSI, MACD, Moving Averages for given price data
// @input_types: price-data,indicator-request,timeframe-selection
// @output_formats: indicator-values,signal-strength,trend-analysis
func (t *TechnicalAnalysisAgent) CalculateTechnicalIndicators(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	t.Logger().Info("Calculating technical indicators", input)

	// Extract input parameters
	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required")
	}

	indicators, ok := input["indicators"].([]interface{})
	if !ok {
		// Default indicators if none specified
		indicators = []interface{}{"RSI", "MACD", "SMA", "EMA"}
	}

	// Mock price data - in real implementation, this would come from market data agent
	priceData := t.generateMockPriceData(symbol, 50)

	results := make(map[string]interface{})
	results["symbol"] = symbol
	results["indicators"] = make(map[string]interface{})

	for _, ind := range indicators {
		indicator := ind.(string)
		switch indicator {
		case "RSI":
			rsi := t.calculateRSI(priceData, 14)
			results["indicators"].(map[string]interface{})["RSI"] = map[string]interface{}{
				"value":       rsi,
				"signal":      t.interpretRSI(rsi),
				"description": "Relative Strength Index - measures overbought/oversold conditions",
			}

		case "MACD":
			macd, signal, histogram := t.calculateMACD(priceData)
			results["indicators"].(map[string]interface{})["MACD"] = map[string]interface{}{
				"macd":           macd,
				"signal":         signal,
				"histogram":      histogram,
				"interpretation": t.interpretMACD(macd, signal, histogram),
				"description":    "Moving Average Convergence Divergence - trend following momentum indicator",
			}

		case "SMA":
			sma20 := t.calculateSMA(priceData, 20)
			sma50 := t.calculateSMA(priceData, 50)
			results["indicators"].(map[string]interface{})["SMA"] = map[string]interface{}{
				"SMA_20":      sma20,
				"SMA_50":      sma50,
				"signal":      t.interpretSMA(priceData[len(priceData)-1], sma20, sma50),
				"description": "Simple Moving Average - trend identification",
			}

		case "EMA":
			ema12 := t.calculateEMA(priceData, 12)
			ema26 := t.calculateEMA(priceData, 26)
			results["indicators"].(map[string]interface{})["EMA"] = map[string]interface{}{
				"EMA_12":      ema12,
				"EMA_26":      ema26,
				"signal":      t.interpretEMA(ema12, ema26),
				"description": "Exponential Moving Average - responsive trend indicator",
			}
		}
	}

	return results, nil
}

// @capability: IdentifyChartPatterns
// @description: Identify common chart patterns like head and shoulders, triangles, flags in price data
// @input_types: price-data,pattern-request,timeframe-selection
// @output_formats: pattern-identification,confidence-score,trading-implications
func (t *TechnicalAnalysisAgent) IdentifyChartPatterns(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	t.Logger().Info("Identifying chart patterns", input)

	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required")
	}

	// Mock price data with highs and lows
	priceData := t.generateMockPriceDataWithHL(symbol, 100)

	patterns := t.detectPatterns(priceData)

	return map[string]interface{}{
		"symbol":           symbol,
		"patterns_found":   patterns,
		"analysis_period":  "100 periods",
		"confidence_level": "moderate",
		"recommendations":  t.generatePatternRecommendations(patterns),
	}, nil
}

// @capability: GenerateTradingSignals
// @description: Generate buy/sell/hold trading signals based on technical analysis
// @input_types: technical-analysis-request,risk-parameters,timeframe-selection
// @output_formats: trading-signals,confidence-score,risk-assessment
func (t *TechnicalAnalysisAgent) GenerateTradingSignals(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	t.Logger().Info("Generating trading signals", input)

	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required")
	}

	riskLevel := "moderate"
	if risk, exists := input["risk_level"].(string); exists {
		riskLevel = risk
	}

	// Generate comprehensive analysis
	priceData := t.generateMockPriceData(symbol, 30)
	currentPrice := priceData[len(priceData)-1]

	// Calculate multiple indicators for signal generation
	rsi := t.calculateRSI(priceData, 14)
	macd, signal, _ := t.calculateMACD(priceData)
	sma20 := t.calculateSMA(priceData, 20)
	sma50 := t.calculateSMA(priceData, 50)

	// Generate overall signal
	signals := []string{}
	confidence := 0.0

	// RSI signals
	if rsi < 30 {
		signals = append(signals, "RSI oversold - potential buy")
		confidence += 0.25
	} else if rsi > 70 {
		signals = append(signals, "RSI overbought - potential sell")
		confidence += 0.25
	}

	// MACD signals
	if macd > signal {
		signals = append(signals, "MACD bullish crossover")
		confidence += 0.25
	} else {
		signals = append(signals, "MACD bearish crossover")
		confidence += 0.25
	}

	// Moving average signals
	if currentPrice > sma20 && sma20 > sma50 {
		signals = append(signals, "Price above moving averages - bullish trend")
		confidence += 0.25
	} else if currentPrice < sma20 && sma20 < sma50 {
		signals = append(signals, "Price below moving averages - bearish trend")
		confidence += 0.25
	}

	// Generate final recommendation
	overallSignal := t.generateOverallSignal(rsi, macd, signal, currentPrice, sma20, sma50)

	return map[string]interface{}{
		"symbol":             symbol,
		"overall_signal":     overallSignal,
		"confidence":         fmt.Sprintf("%.1f%%", confidence*100),
		"risk_level":         riskLevel,
		"individual_signals": signals,
		"current_price":      currentPrice,
		"support_level":      sma50,
		"resistance_level":   math.Max(sma20, currentPrice*1.05),
		"analysis_timestamp": "2024-01-15T10:30:00Z",
	}, nil
}

// Helper functions for technical analysis calculations

func (t *TechnicalAnalysisAgent) generateMockPriceData(symbol string, periods int) []float64 {
	// Generate realistic-looking price data
	basePrice := 100.0
	if symbol == "AAPL" {
		basePrice = 175.0
	} else if symbol == "TSLA" {
		basePrice = 250.0
	} else if symbol == "GOOGL" {
		basePrice = 135.0
	}

	prices := make([]float64, periods)
	price := basePrice

	for i := 0; i < periods; i++ {
		// Simulate price movement with some randomness and trend
		change := (math.Sin(float64(i)*0.1) * 2) + ((float64(i%7) - 3) * 0.5)
		price += change
		if price < basePrice*0.8 {
			price = basePrice * 0.8
		}
		if price > basePrice*1.3 {
			price = basePrice * 1.3
		}
		prices[i] = price
	}

	return prices
}

func (t *TechnicalAnalysisAgent) generateMockPriceDataWithHL(symbol string, periods int) map[string][]float64 {
	closes := t.generateMockPriceData(symbol, periods)
	highs := make([]float64, periods)
	lows := make([]float64, periods)

	for i, close := range closes {
		highs[i] = close * (1.0 + float64(i%3)*0.01)
		lows[i] = close * (1.0 - float64(i%3)*0.01)
	}

	return map[string][]float64{
		"close": closes,
		"high":  highs,
		"low":   lows,
	}
}

func (t *TechnicalAnalysisAgent) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0 // Neutral RSI if not enough data
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain/loss
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return rsi
}

func (t *TechnicalAnalysisAgent) calculateMACD(prices []float64) (float64, float64, float64) {
	if len(prices) < 26 {
		return 0, 0, 0
	}

	ema12 := t.calculateEMA(prices, 12)
	ema26 := t.calculateEMA(prices, 26)
	macd := ema12 - ema26

	// For simplicity, calculate signal line as 9-period EMA of MACD
	// In real implementation, you'd need historical MACD values
	signal := macd * 0.9 // Simplified signal line
	histogram := macd - signal

	return macd, signal, histogram
}

func (t *TechnicalAnalysisAgent) calculateSMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1] // Return last price if not enough data
	}

	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}

	return sum / float64(period)
}

func (t *TechnicalAnalysisAgent) calculateEMA(prices []float64, period int) float64 {
	if len(prices) == 0 {
		return 0
	}
	if len(prices) < period {
		return t.calculateSMA(prices, len(prices))
	}

	multiplier := 2.0 / (float64(period) + 1.0)
	ema := t.calculateSMA(prices[:period], period)

	for i := period; i < len(prices); i++ {
		ema = (prices[i] * multiplier) + (ema * (1 - multiplier))
	}

	return ema
}

func (t *TechnicalAnalysisAgent) interpretRSI(rsi float64) string {
	if rsi > 70 {
		return "Overbought - potential sell signal"
	} else if rsi < 30 {
		return "Oversold - potential buy signal"
	}
	return "Neutral"
}

func (t *TechnicalAnalysisAgent) interpretMACD(macd, signal, histogram float64) string {
	if macd > signal && histogram > 0 {
		return "Bullish momentum"
	} else if macd < signal && histogram < 0 {
		return "Bearish momentum"
	}
	return "Neutral momentum"
}

func (t *TechnicalAnalysisAgent) interpretSMA(currentPrice, sma20, sma50 float64) string {
	if currentPrice > sma20 && sma20 > sma50 {
		return "Strong uptrend"
	} else if currentPrice < sma20 && sma20 < sma50 {
		return "Strong downtrend"
	} else if currentPrice > sma20 {
		return "Short-term bullish"
	} else {
		return "Short-term bearish"
	}
}

func (t *TechnicalAnalysisAgent) interpretEMA(ema12, ema26 float64) string {
	if ema12 > ema26 {
		return "Bullish crossover"
	} else {
		return "Bearish crossover"
	}
}

func (t *TechnicalAnalysisAgent) detectPatterns(priceData map[string][]float64) []map[string]interface{} {
	patterns := []map[string]interface{}{}

	// Simple pattern detection logic
	closes := priceData["close"]
	highs := priceData["high"]
	lows := priceData["low"]

	if len(closes) < 20 {
		return patterns
	}

	// Detect support/resistance levels
	recentHighs := highs[len(highs)-20:]
	recentLows := lows[len(lows)-20:]

	sort.Float64s(recentHighs)
	sort.Float64s(recentLows)

	resistance := recentHighs[len(recentHighs)-3] // 3rd highest
	support := recentLows[2]                      // 3rd lowest

	patterns = append(patterns, map[string]interface{}{
		"pattern":    "Support/Resistance",
		"support":    support,
		"resistance": resistance,
		"confidence": "High",
		"type":       "levels",
	})

	// Simple trend detection
	recentPrices := closes[len(closes)-10:]

	if recentPrices[len(recentPrices)-1] > recentPrices[0]*1.05 {
		patterns = append(patterns, map[string]interface{}{
			"pattern":    "Uptrend",
			"confidence": "Moderate",
			"type":       "trend",
			"strength":   "Strong",
		})
	} else if recentPrices[len(recentPrices)-1] < recentPrices[0]*0.95 {
		patterns = append(patterns, map[string]interface{}{
			"pattern":    "Downtrend",
			"confidence": "Moderate",
			"type":       "trend",
			"strength":   "Strong",
		})
	}

	return patterns
}

func (t *TechnicalAnalysisAgent) generatePatternRecommendations(patterns []map[string]interface{}) []string {
	recommendations := []string{}

	for _, pattern := range patterns {
		patternType := pattern["pattern"].(string)
		switch patternType {
		case "Support/Resistance":
			recommendations = append(recommendations, "Consider buying near support level and selling near resistance")
		case "Uptrend":
			recommendations = append(recommendations, "Consider buying on pullbacks in the uptrend")
		case "Downtrend":
			recommendations = append(recommendations, "Consider selling on rallies in the downtrend")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No clear patterns detected - wait for better setup")
	}

	return recommendations
}

func (t *TechnicalAnalysisAgent) generateOverallSignal(rsi, macd, signal, currentPrice, sma20, sma50 float64) string {
	bullishCount := 0
	bearishCount := 0

	// RSI analysis
	if rsi < 30 {
		bullishCount++
	} else if rsi > 70 {
		bearishCount++
	}

	// MACD analysis
	if macd > signal {
		bullishCount++
	} else {
		bearishCount++
	}

	// Moving average analysis
	if currentPrice > sma20 && sma20 > sma50 {
		bullishCount++
	} else if currentPrice < sma20 && sma20 < sma50 {
		bearishCount++
	}

	if bullishCount > bearishCount {
		return "BUY"
	} else if bearishCount > bullishCount {
		return "SELL"
	}
	return "HOLD"
}

func main() {
	agent := &TechnicalAnalysisAgent{}

	// Start the agent with framework auto-configuration
	f, err := framework.NewFramework(
		framework.WithAgentName("technical-analysis-agent"),
		framework.WithPort(8084),
		framework.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
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

	fmt.Println("Starting Technical Analysis Agent on port 8084...")
	fmt.Println("Available capabilities:")
	fmt.Println("  - CalculateTechnicalIndicators: Calculate RSI, MACD, Moving Averages")
	fmt.Println("  - IdentifyChartPatterns: Detect chart patterns and trends")
	fmt.Println("  - GenerateTradingSignals: Generate buy/sell/hold signals")

	// Start HTTP server
	if err := f.StartHTTPServer(ctx, agent); err != nil {
		fmt.Printf("Failed to start technical analysis agent: %v\n", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
