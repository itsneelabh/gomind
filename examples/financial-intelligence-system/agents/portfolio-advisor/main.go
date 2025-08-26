package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	framework "github.com/itsneelabh/gomind"
)

// PortfolioAdvisorAgent provides comprehensive portfolio management and investment advice
type PortfolioAdvisorAgent struct {
	*framework.BaseAgent
}

// PortfolioAnalysis represents a comprehensive portfolio analysis
type PortfolioAnalysis struct {
	TotalValue      float64            `json:"total_value"`
	Allocations     map[string]float64 `json:"allocations"`
	RiskScore       float64            `json:"risk_score"`
	ExpectedReturn  float64            `json:"expected_return"`
	Diversification float64            `json:"diversification"`
	Holdings        []HoldingAnalysis  `json:"holdings"`
	Recommendations []string           `json:"recommendations"`
	RebalanceNeeded bool               `json:"rebalance_needed"`
}

// HoldingAnalysis represents analysis of an individual holding
type HoldingAnalysis struct {
	Symbol           string  `json:"symbol"`
	Shares           float64 `json:"shares"`
	CurrentPrice     float64 `json:"current_price"`
	Value            float64 `json:"value"`
	Weight           float64 `json:"weight"`
	Performance      float64 `json:"performance"`
	RiskContribution float64 `json:"risk_contribution"`
	Recommendation   string  `json:"recommendation"`
}

// @llm_prompt: "You are a Portfolio Advisor Agent specializing in investment portfolio management, asset allocation, risk assessment, and providing personalized investment recommendations. You analyze portfolios holistically and provide strategic guidance."
// @specialties: ["portfolio-management", "asset-allocation", "risk-assessment", "investment-strategy", "diversification", "rebalancing", "performance-analysis", "financial-planning"]
// @capability: AnalyzePortfolio
// @description: Analyze a portfolio for risk, diversification, performance, and provide improvement recommendations
// @input_types: portfolio-holdings,investment-goals,risk-tolerance
// @output_formats: portfolio-analysis,risk-metrics,allocation-breakdown,recommendations
func (p *PortfolioAdvisorAgent) AnalyzePortfolio(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	p.Logger().Info("Analyzing portfolio", input)

	// Extract portfolio holdings
	holdings, ok := input["holdings"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("holdings are required for portfolio analysis")
	}

	// Get risk tolerance (conservative, moderate, aggressive)
	riskTolerance := "moderate"
	if risk, exists := input["risk_tolerance"].(string); exists {
		riskTolerance = strings.ToLower(risk)
	}

	// Analyze each holding
	analysis := &PortfolioAnalysis{
		Holdings:    []HoldingAnalysis{},
		Allocations: make(map[string]float64),
		TotalValue:  0,
	}

	sectorAllocations := make(map[string]float64)

	for _, holding := range holdings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		shares := holdingMap["shares"].(float64)

		// Mock current price (in real implementation, would call market data agent)
		currentPrice := p.getMockPrice(symbol)
		value := shares * currentPrice

		// Determine sector for diversification analysis
		sector := p.getSectorForSymbol(symbol)
		sectorAllocations[sector] += value

		holdingAnalysis := HoldingAnalysis{
			Symbol:       symbol,
			Shares:       shares,
			CurrentPrice: currentPrice,
			Value:        value,
			Performance:  p.calculateMockPerformance(symbol),
		}

		analysis.Holdings = append(analysis.Holdings, holdingAnalysis)
		analysis.TotalValue += value
	}

	// Calculate weights and risk contributions
	for i := range analysis.Holdings {
		analysis.Holdings[i].Weight = analysis.Holdings[i].Value / analysis.TotalValue * 100
		analysis.Holdings[i].RiskContribution = p.calculateRiskContribution(analysis.Holdings[i])
		analysis.Holdings[i].Recommendation = p.generateHoldingRecommendation(analysis.Holdings[i], riskTolerance)
	}

	// Calculate sector allocations as percentages
	for sector, value := range sectorAllocations {
		analysis.Allocations[sector] = value / analysis.TotalValue * 100
	}

	// Calculate portfolio metrics
	analysis.RiskScore = p.calculatePortfolioRisk(analysis.Holdings, sectorAllocations, analysis.TotalValue)
	analysis.ExpectedReturn = p.calculateExpectedReturn(analysis.Holdings)
	analysis.Diversification = p.calculateDiversificationScore(analysis.Allocations)
	analysis.RebalanceNeeded = p.isRebalanceNeeded(analysis.Holdings, riskTolerance)
	analysis.Recommendations = p.generatePortfolioRecommendations(analysis, riskTolerance)

	return map[string]interface{}{
		"portfolio_analysis":     analysis,
		"analysis_timestamp":     time.Now().UTC().Format(time.RFC3339),
		"risk_tolerance":         riskTolerance,
		"portfolio_health_score": p.calculateHealthScore(analysis),
	}, nil
}

// @capability: GenerateAllocationStrategy
// @description: Generate optimal asset allocation strategy based on goals and risk tolerance
// @input_types: investment-goals,risk-profile,time-horizon,current-portfolio
// @output_formats: allocation-strategy,target-weights,rebalance-plan
func (p *PortfolioAdvisorAgent) GenerateAllocationStrategy(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	p.Logger().Info("Generating allocation strategy", input)

	// Extract parameters
	riskTolerance := getStringWithDefault(input, "risk_tolerance", "moderate")
	timeHorizon := getStringWithDefault(input, "time_horizon", "long-term")    // short-term, medium-term, long-term
	investmentGoal := getStringWithDefault(input, "investment_goal", "growth") // growth, income, balanced

	// Generate target allocation based on parameters
	targetAllocation := p.generateTargetAllocation(riskTolerance, timeHorizon, investmentGoal)

	// Generate specific asset recommendations
	assetRecommendations := p.generateAssetRecommendations(targetAllocation, riskTolerance)

	// Calculate expected metrics
	expectedReturn := p.calculateExpectedReturnForAllocation(targetAllocation)
	expectedRisk := p.calculateExpectedRiskForAllocation(targetAllocation)

	return map[string]interface{}{
		"target_allocation":      targetAllocation,
		"asset_recommendations":  assetRecommendations,
		"expected_annual_return": expectedReturn,
		"expected_volatility":    expectedRisk,
		"rebalancing_frequency":  p.getRebalancingFrequency(riskTolerance),
		"implementation_plan":    p.generateImplementationPlan(targetAllocation),
		"risk_tolerance":         riskTolerance,
		"time_horizon":           timeHorizon,
		"investment_goal":        investmentGoal,
	}, nil
}

// @capability: OptimizePortfolio
// @description: Optimize portfolio for better risk-adjusted returns using modern portfolio theory concepts
// @input_types: current-portfolio,optimization-constraints,performance-targets
// @output_formats: optimized-portfolio,efficiency-metrics,trade-recommendations
func (p *PortfolioAdvisorAgent) OptimizePortfolio(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	p.Logger().Info("Optimizing portfolio", input)

	// Extract current portfolio
	currentHoldings, ok := input["current_holdings"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("current_holdings are required for optimization")
	}

	// Extract optimization parameters
	targetReturn := getFloatWithDefault(input, "target_return", 8.0) // 8% default target
	maxRisk := getFloatWithDefault(input, "max_risk", 15.0)          // 15% max volatility

	// Analyze current portfolio
	currentMetrics := p.calculateCurrentPortfolioMetrics(currentHoldings)

	// Generate optimized allocation
	optimizedAllocation := p.optimizeAllocation(currentHoldings, targetReturn, maxRisk)

	// Calculate improvement metrics
	improvement := p.calculateImprovement(currentMetrics, optimizedAllocation)

	// Generate specific trade recommendations
	tradeRecommendations := p.generateTradeRecommendations(currentHoldings, optimizedAllocation)

	return map[string]interface{}{
		"current_metrics":          currentMetrics,
		"optimized_allocation":     optimizedAllocation,
		"improvement_potential":    improvement,
		"trade_recommendations":    tradeRecommendations,
		"sharpe_ratio_improvement": improvement["sharpe_ratio"],
		"efficiency_score":         p.calculateEfficiencyScore(optimizedAllocation),
		"optimization_timestamp":   time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Helper functions for portfolio analysis and optimization

func (p *PortfolioAdvisorAgent) getMockPrice(symbol string) float64 {
	// Mock prices for demonstration
	prices := map[string]float64{
		"AAPL":  175.50,
		"GOOGL": 135.20,
		"MSFT":  378.85,
		"TSLA":  248.90,
		"AMZN":  153.75,
		"META":  487.65,
		"NVDA":  875.30,
		"SPY":   445.20,
		"QQQ":   385.50,
		"VTI":   242.80,
		"BND":   78.90,
		"GLD":   189.45,
	}

	if price, exists := prices[symbol]; exists {
		return price
	}
	return 100.0 // Default price
}

func (p *PortfolioAdvisorAgent) getSectorForSymbol(symbol string) string {
	sectors := map[string]string{
		"AAPL":  "Technology",
		"GOOGL": "Technology",
		"MSFT":  "Technology",
		"TSLA":  "Consumer Discretionary",
		"AMZN":  "Consumer Discretionary",
		"META":  "Communication Services",
		"NVDA":  "Technology",
		"SPY":   "Broad Market",
		"QQQ":   "Technology ETF",
		"VTI":   "Broad Market",
		"BND":   "Bonds",
		"GLD":   "Commodities",
	}

	if sector, exists := sectors[symbol]; exists {
		return sector
	}
	return "Other"
}

func (p *PortfolioAdvisorAgent) calculateMockPerformance(symbol string) float64 {
	// Mock performance data (annual return %)
	performances := map[string]float64{
		"AAPL":  12.5,
		"GOOGL": 8.2,
		"MSFT":  15.8,
		"TSLA":  -8.5,
		"AMZN":  6.7,
		"META":  18.9,
		"NVDA":  45.2,
		"SPY":   10.5,
		"QQQ":   12.8,
		"VTI":   9.8,
		"BND":   2.1,
		"GLD":   -2.3,
	}

	if performance, exists := performances[symbol]; exists {
		return performance
	}
	return 5.0 // Default performance
}

func (p *PortfolioAdvisorAgent) calculateRiskContribution(holding HoldingAnalysis) float64 {
	// Simplified risk contribution based on volatility and weight
	volatilities := map[string]float64{
		"AAPL":  22.5,
		"GOOGL": 25.8,
		"MSFT":  20.2,
		"TSLA":  55.8,
		"AMZN":  28.9,
		"META":  35.2,
		"NVDA":  45.8,
		"SPY":   16.5,
		"QQQ":   22.1,
		"VTI":   15.8,
		"BND":   4.2,
		"GLD":   18.5,
	}

	volatility := 25.0 // Default volatility
	if vol, exists := volatilities[holding.Symbol]; exists {
		volatility = vol
	}

	return (holding.Weight / 100) * volatility
}

func (p *PortfolioAdvisorAgent) generateHoldingRecommendation(holding HoldingAnalysis, riskTolerance string) string {
	weight := holding.Weight
	performance := holding.Performance

	if weight > 25 {
		return "REDUCE - Overweight position, consider trimming"
	} else if weight < 2 && performance > 10 {
		return "INCREASE - Underweight in strong performer"
	} else if performance < -5 {
		return "REVIEW - Poor performance, consider exit strategy"
	} else if riskTolerance == "conservative" && holding.RiskContribution > 20 {
		return "REDUCE - Too risky for conservative portfolio"
	}

	return "HOLD - Position sized appropriately"
}

func (p *PortfolioAdvisorAgent) calculatePortfolioRisk(holdings []HoldingAnalysis, sectorAllocations map[string]float64, totalValue float64) float64 {
	// Simplified portfolio risk calculation
	totalRisk := 0.0

	for _, holding := range holdings {
		totalRisk += holding.RiskContribution
	}

	// Add concentration risk penalty
	concentrationPenalty := 0.0
	for _, allocation := range sectorAllocations {
		sectorWeight := allocation
		if sectorWeight > 30 {
			concentrationPenalty += (sectorWeight - 30) * 0.1
		}
	}

	return math.Min(totalRisk+concentrationPenalty, 100.0)
}

func (p *PortfolioAdvisorAgent) calculateExpectedReturn(holdings []HoldingAnalysis) float64 {
	weightedReturn := 0.0

	for _, holding := range holdings {
		weightedReturn += (holding.Weight / 100) * holding.Performance
	}

	return weightedReturn
}

func (p *PortfolioAdvisorAgent) calculateDiversificationScore(allocations map[string]float64) float64 {
	// Higher score = better diversification
	if len(allocations) < 2 {
		return 0.0
	}

	// Calculate Herfindahl index (concentration measure)
	herfindahl := 0.0
	for _, allocation := range allocations {
		weight := allocation / 100
		herfindahl += weight * weight
	}

	// Convert to diversification score (1 - HHI, scaled to 100)
	diversificationScore := (1.0 - herfindahl) * 100
	return math.Max(0, diversificationScore)
}

func (p *PortfolioAdvisorAgent) isRebalanceNeeded(holdings []HoldingAnalysis, riskTolerance string) bool {
	// Check if any holding is significantly over/underweight
	for _, holding := range holdings {
		if holding.Weight > 30 || (holding.Weight > 20 && riskTolerance == "conservative") {
			return true
		}
	}
	return false
}

func (p *PortfolioAdvisorAgent) generatePortfolioRecommendations(analysis *PortfolioAnalysis, riskTolerance string) []string {
	recommendations := []string{}

	// Risk-based recommendations
	if analysis.RiskScore > 70 {
		recommendations = append(recommendations, "Portfolio risk is high - consider reducing volatile positions")
	} else if analysis.RiskScore < 30 && riskTolerance != "conservative" {
		recommendations = append(recommendations, "Portfolio may be too conservative - consider adding growth assets")
	}

	// Diversification recommendations
	if analysis.Diversification < 50 {
		recommendations = append(recommendations, "Improve diversification by adding different sectors/asset classes")
	}

	// Sector concentration check
	for sector, allocation := range analysis.Allocations {
		if allocation > 40 {
			recommendations = append(recommendations, fmt.Sprintf("Reduce concentration in %s sector (currently %.1f%%)", sector, allocation))
		}
	}

	// Rebalancing recommendation
	if analysis.RebalanceNeeded {
		recommendations = append(recommendations, "Portfolio rebalancing recommended to optimize risk-return profile")
	}

	// Performance-based recommendations
	if analysis.ExpectedReturn < 6 {
		recommendations = append(recommendations, "Expected return is low - consider adding growth-oriented assets")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Portfolio is well-balanced - maintain current allocation with periodic reviews")
	}

	return recommendations
}

func (p *PortfolioAdvisorAgent) calculateHealthScore(analysis *PortfolioAnalysis) float64 {
	// Composite health score (0-100)
	riskScore := math.Max(0, 100-analysis.RiskScore) // Lower risk = higher score
	diversificationScore := analysis.Diversification
	returnScore := math.Min(100, analysis.ExpectedReturn*10) // Cap at 100

	return (riskScore + diversificationScore + returnScore) / 3
}

func (p *PortfolioAdvisorAgent) generateTargetAllocation(riskTolerance, timeHorizon, investmentGoal string) map[string]float64 {
	allocation := make(map[string]float64)

	// Base allocations based on risk tolerance
	switch riskTolerance {
	case "conservative":
		allocation["Stocks"] = 30.0
		allocation["Bonds"] = 60.0
		allocation["Cash"] = 10.0
	case "moderate":
		allocation["Stocks"] = 60.0
		allocation["Bonds"] = 35.0
		allocation["Cash"] = 5.0
	case "aggressive":
		allocation["Stocks"] = 85.0
		allocation["Bonds"] = 10.0
		allocation["Cash"] = 5.0
	}

	// Adjust for time horizon
	if timeHorizon == "short-term" {
		allocation["Bonds"] += 10
		allocation["Stocks"] -= 10
	} else if timeHorizon == "long-term" && riskTolerance != "conservative" {
		allocation["Stocks"] += 5
		allocation["Bonds"] -= 5
	}

	// Adjust for investment goal
	if investmentGoal == "income" {
		allocation["Bonds"] += 15
		allocation["Stocks"] -= 15
	} else if investmentGoal == "growth" && riskTolerance != "conservative" {
		allocation["Stocks"] += 10
		allocation["Bonds"] -= 10
	}

	return allocation
}

func (p *PortfolioAdvisorAgent) generateAssetRecommendations(targetAllocation map[string]float64, riskTolerance string) map[string][]string {
	recommendations := make(map[string][]string)

	// Stock recommendations
	if allocation, exists := targetAllocation["Stocks"]; exists && allocation > 0 {
		if riskTolerance == "conservative" {
			recommendations["Stocks"] = []string{"VTI", "SPY", "SCHD", "VIG"}
		} else if riskTolerance == "moderate" {
			recommendations["Stocks"] = []string{"VTI", "VXUS", "VGT", "VHT", "QQQ"}
		} else {
			recommendations["Stocks"] = []string{"QQQ", "VGT", "ARKK", "VUG", "VTEB"}
		}
	}

	// Bond recommendations
	if allocation, exists := targetAllocation["Bonds"]; exists && allocation > 0 {
		recommendations["Bonds"] = []string{"BND", "VGIT", "TLT", "VTEB", "LQD"}
	}

	// Cash recommendations
	if allocation, exists := targetAllocation["Cash"]; exists && allocation > 0 {
		recommendations["Cash"] = []string{"High-yield savings", "Money market funds", "Short-term CDs"}
	}

	return recommendations
}

func (p *PortfolioAdvisorAgent) calculateExpectedReturnForAllocation(allocation map[string]float64) float64 {
	// Expected returns by asset class
	expectedReturns := map[string]float64{
		"Stocks": 10.0,
		"Bonds":  4.0,
		"Cash":   2.0,
	}

	weightedReturn := 0.0
	for assetClass, weight := range allocation {
		if expectedReturn, exists := expectedReturns[assetClass]; exists {
			weightedReturn += (weight / 100) * expectedReturn
		}
	}

	return weightedReturn
}

func (p *PortfolioAdvisorAgent) calculateExpectedRiskForAllocation(allocation map[string]float64) float64 {
	// Expected volatility by asset class
	expectedRisks := map[string]float64{
		"Stocks": 16.0,
		"Bonds":  4.0,
		"Cash":   0.5,
	}

	weightedRisk := 0.0
	for assetClass, weight := range allocation {
		if expectedRisk, exists := expectedRisks[assetClass]; exists {
			weightedRisk += (weight / 100) * expectedRisk
		}
	}

	return weightedRisk
}

func (p *PortfolioAdvisorAgent) getRebalancingFrequency(riskTolerance string) string {
	switch riskTolerance {
	case "conservative":
		return "Semi-annually"
	case "moderate":
		return "Quarterly"
	case "aggressive":
		return "Monthly"
	default:
		return "Quarterly"
	}
}

func (p *PortfolioAdvisorAgent) generateImplementationPlan(targetAllocation map[string]float64) []string {
	plan := []string{
		"1. Assess current portfolio allocation vs target",
		"2. Identify over/underweight positions",
		"3. Plan tax-efficient transitions",
		"4. Implement changes gradually over 3-6 months",
		"5. Set up automatic rebalancing triggers",
		"6. Monitor and adjust quarterly",
	}
	return plan
}

func (p *PortfolioAdvisorAgent) calculateCurrentPortfolioMetrics(holdings []interface{}) map[string]interface{} {
	// Simplified current portfolio metrics calculation
	totalValue := 0.0
	weightedReturn := 0.0
	weightedRisk := 0.0

	for _, holding := range holdings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		shares := holdingMap["shares"].(float64)

		value := shares * p.getMockPrice(symbol)
		totalValue += value
	}

	// Calculate weighted metrics (simplified)
	for _, holding := range holdings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		shares := holdingMap["shares"].(float64)

		value := shares * p.getMockPrice(symbol)
		weight := value / totalValue

		weightedReturn += weight * p.calculateMockPerformance(symbol)
		// Simplified risk calculation
		risk := 20.0 // Default risk
		weightedRisk += weight * risk
	}

	sharpeRatio := 0.0
	if weightedRisk > 0 {
		sharpeRatio = (weightedReturn - 2.0) / weightedRisk // Assuming 2% risk-free rate
	}

	return map[string]interface{}{
		"total_value":     totalValue,
		"expected_return": weightedReturn,
		"portfolio_risk":  weightedRisk,
		"sharpe_ratio":    sharpeRatio,
	}
}

func (p *PortfolioAdvisorAgent) optimizeAllocation(currentHoldings []interface{}, targetReturn, maxRisk float64) map[string]interface{} {
	// Simplified optimization algorithm
	// In a real implementation, this would use modern portfolio theory

	optimizedWeights := make(map[string]float64)
	symbols := []string{}

	// Extract symbols
	for _, holding := range currentHoldings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		symbols = append(symbols, symbol)
	}

	// Equal weight as starting point, then adjust
	equalWeight := 100.0 / float64(len(symbols))
	for _, symbol := range symbols {
		performance := p.calculateMockPerformance(symbol)

		// Adjust weight based on performance and target return
		if performance > targetReturn {
			optimizedWeights[symbol] = equalWeight * 1.2 // Overweight good performers
		} else {
			optimizedWeights[symbol] = equalWeight * 0.8 // Underweight poor performers
		}
	}

	// Normalize weights to sum to 100%
	totalWeight := 0.0
	for _, weight := range optimizedWeights {
		totalWeight += weight
	}

	for symbol := range optimizedWeights {
		optimizedWeights[symbol] = (optimizedWeights[symbol] / totalWeight) * 100
	}

	// Calculate optimized metrics
	optimizedReturn := 0.0
	optimizedRisk := 0.0

	for symbol, weight := range optimizedWeights {
		optimizedReturn += (weight / 100) * p.calculateMockPerformance(symbol)
		optimizedRisk += (weight / 100) * 20.0 // Simplified risk
	}

	sharpeRatio := 0.0
	if optimizedRisk > 0 {
		sharpeRatio = (optimizedReturn - 2.0) / optimizedRisk
	}

	return map[string]interface{}{
		"optimized_weights": optimizedWeights,
		"expected_return":   optimizedReturn,
		"expected_risk":     optimizedRisk,
		"sharpe_ratio":      sharpeRatio,
	}
}

func (p *PortfolioAdvisorAgent) calculateImprovement(currentMetrics map[string]interface{}, optimizedAllocation map[string]interface{}) map[string]interface{} {
	currentReturn := currentMetrics["expected_return"].(float64)
	currentRisk := currentMetrics["portfolio_risk"].(float64)
	currentSharpe := currentMetrics["sharpe_ratio"].(float64)

	optimizedReturn := optimizedAllocation["expected_return"].(float64)
	optimizedRisk := optimizedAllocation["expected_risk"].(float64)
	optimizedSharpe := optimizedAllocation["sharpe_ratio"].(float64)

	return map[string]interface{}{
		"return_improvement":  optimizedReturn - currentReturn,
		"risk_reduction":      currentRisk - optimizedRisk,
		"sharpe_ratio":        optimizedSharpe - currentSharpe,
		"overall_improvement": (optimizedSharpe - currentSharpe) / math.Abs(currentSharpe) * 100,
	}
}

func (p *PortfolioAdvisorAgent) generateTradeRecommendations(currentHoldings []interface{}, optimizedAllocation map[string]interface{}) []map[string]interface{} {
	recommendations := []map[string]interface{}{}
	optimizedWeights := optimizedAllocation["optimized_weights"].(map[string]float64)

	// Calculate current weights
	totalValue := 0.0
	currentWeights := make(map[string]float64)

	for _, holding := range currentHoldings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		shares := holdingMap["shares"].(float64)
		value := shares * p.getMockPrice(symbol)
		totalValue += value
	}

	for _, holding := range currentHoldings {
		holdingMap := holding.(map[string]interface{})
		symbol := holdingMap["symbol"].(string)
		shares := holdingMap["shares"].(float64)
		value := shares * p.getMockPrice(symbol)
		currentWeights[symbol] = (value / totalValue) * 100
	}

	// Generate trade recommendations
	for symbol, targetWeight := range optimizedWeights {
		currentWeight := currentWeights[symbol]
		difference := targetWeight - currentWeight

		if math.Abs(difference) > 2.0 { // Only recommend if difference > 2%
			action := "HOLD"
			if difference > 0 {
				action = "BUY"
			} else {
				action = "SELL"
			}

			recommendations = append(recommendations, map[string]interface{}{
				"symbol":         symbol,
				"action":         action,
				"current_weight": currentWeight,
				"target_weight":  targetWeight,
				"adjustment":     difference,
				"priority":       p.getPriority(math.Abs(difference)),
			})
		}
	}

	// Sort by priority
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i]["priority"].(string) < recommendations[j]["priority"].(string)
	})

	return recommendations
}

func (p *PortfolioAdvisorAgent) calculateEfficiencyScore(optimizedAllocation map[string]interface{}) float64 {
	// Calculate efficiency score based on risk-adjusted return
	expectedReturn := optimizedAllocation["expected_return"].(float64)
	expectedRisk := optimizedAllocation["expected_risk"].(float64)

	if expectedRisk == 0 {
		return 0
	}

	efficiency := (expectedReturn / expectedRisk) * 10 // Scale to 0-100
	return math.Min(100, efficiency)
}

func (p *PortfolioAdvisorAgent) getPriority(difference float64) string {
	if difference > 10 {
		return "HIGH"
	} else if difference > 5 {
		return "MEDIUM"
	}
	return "LOW"
}

// Utility functions
func getStringWithDefault(input map[string]interface{}, key, defaultValue string) string {
	if value, exists := input[key].(string); exists {
		return value
	}
	return defaultValue
}

func getFloatWithDefault(input map[string]interface{}, key string, defaultValue float64) float64 {
	if value, exists := input[key].(float64); exists {
		return value
	}
	return defaultValue
}

func main() {
	agent := &PortfolioAdvisorAgent{}

	// Start the agent with framework auto-configuration
	f, err := framework.NewFramework(
		framework.WithAgentName("portfolio-advisor-agent"),
		framework.WithPort(8085),
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

	fmt.Println("Starting Portfolio Advisor Agent on port 8085...")
	fmt.Println("Available capabilities:")
	fmt.Println("  - AnalyzePortfolio: Comprehensive portfolio analysis and recommendations")
	fmt.Println("  - GenerateAllocationStrategy: Optimal asset allocation based on goals")
	fmt.Println("  - OptimizePortfolio: Portfolio optimization for better risk-adjusted returns")

	// Start HTTP server
	if err := f.StartHTTPServer(ctx, agent); err != nil {
		fmt.Printf("Failed to start portfolio advisor agent: %v\n", err)
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
