package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	framework "github.com/itsneelabh/gomind"
)

// NewsAnalysisAgent provides financial news analysis and sentiment scoring
type NewsAnalysisAgent struct {
	framework.BaseAgent
	newsAPIKey string
	client     *http.Client
}

// Initialize sets up the news analysis agent with API credentials
func (n *NewsAnalysisAgent) Initialize(ctx context.Context) error {
	n.newsAPIKey = os.Getenv("NEWS_API_KEY")
	if n.newsAPIKey == "" {
		return fmt.Errorf("NEWS_API_KEY environment variable is required")
	}

	n.client = &http.Client{
		Timeout: 15 * time.Second,
	}

	n.Logger().Info("News Analysis Agent initialized", map[string]interface{}{
		"agent_id":       n.GetAgentID(),
		"api_configured": true,
	})

	return nil
}

// @capability: analyze-financial-news
// @description: Analyzes financial news articles and provides sentiment scoring for market impact
// @domain: financial-analysis
// @complexity: high
// @latency: 3-8s
// @cost: moderate
// @confidence: 0.82
// @business_value: market-sentiment,risk-assessment,investment-timing
// @llm_prompt: Ask me to analyze financial news like 'What is the market sentiment on Tesla?' or 'Find news affecting Apple stock'
// @specialties: sentiment analysis,market news,earnings reports,economic indicators,breaking news
// @use_cases: market-sentiment-tracking,investment-timing,risk-assessment,news-alerts
// @input_types: company-name,stock-symbol,news-keywords
// @output_formats: sentiment-scores,news-summaries,market-impact-analysis
func (n *NewsAnalysisAgent) AnalyzeFinancialNews(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query, ok := input["query"].(string)
	if !ok {
		// Try to extract from symbol or company name
		if symbol, exists := input["symbol"]; exists {
			query = symbol.(string)
		} else if company, exists := input["company"]; exists {
			query = company.(string)
		} else {
			return nil, fmt.Errorf("query, symbol, or company parameter is required")
		}
	}

	// Search for financial news
	newsData, err := n.fetchFinancialNews(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}

	// Analyze sentiment of news articles
	sentiment := n.analyzeSentiment(newsData)

	return map[string]interface{}{
		"query":              query,
		"news_articles":      newsData,
		"sentiment_analysis": sentiment,
		"market_impact":      n.assessMarketImpact(sentiment),
		"last_updated":       time.Now().UTC().Format(time.RFC3339),
		"source":             "News API",
	}, nil
}

// @capability: get-market-headlines
// @description: Retrieves latest financial market headlines and top business news
// @domain: financial-news
// @complexity: low
// @latency: 2-5s
// @cost: low
// @confidence: 0.90
// @business_value: market-awareness,daily-briefings,trend-identification
// @llm_prompt: Ask me for market headlines like 'What are the top financial news today?' or 'Give me the latest market headlines'
// @specialties: breaking news,market headlines,business news,economic reports
// @use_cases: daily-briefings,market-monitoring,news-alerts,trend-identification
// @input_types: news-category,country-code,date-range
// @output_formats: headlines-list,news-summaries,publication-info
func (n *NewsAnalysisAgent) GetMarketHeadlines(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	category := "business"
	if cat, exists := input["category"]; exists {
		category = cat.(string)
	}

	country := "us"
	if ctry, exists := input["country"]; exists {
		country = ctry.(string)
	}

	// Fetch top business headlines
	headlines, err := n.fetchTopHeadlines(category, country)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch headlines: %w", err)
	}

	return map[string]interface{}{
		"category":      category,
		"country":       country,
		"headlines":     headlines,
		"total_results": len(headlines),
		"last_updated":  time.Now().UTC().Format(time.RFC3339),
		"source":        "News API",
	}, nil
}

// @capability: analyze-earnings-sentiment
// @description: Specialized analysis of earnings reports and their market sentiment impact
// @domain: earnings-analysis
// @complexity: high
// @latency: 5-12s
// @cost: moderate
// @confidence: 0.78
// @business_value: earnings-predictions,investment-timing,quarterly-analysis
// @llm_prompt: Ask me about earnings sentiment like 'How did the market react to Apple earnings?' or 'Analyze Tesla earnings sentiment'
// @specialties: earnings reports,quarterly results,guidance analysis,analyst reactions
// @use_cases: earnings-season-tracking,investment-decisions,analyst-consensus
// @input_types: company-symbol,earnings-date,quarter
// @output_formats: earnings-sentiment,analyst-reactions,market-response
func (n *NewsAnalysisAgent) AnalyzeEarningsSentiment(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	symbol, ok := input["symbol"].(string)
	if !ok {
		return nil, fmt.Errorf("symbol is required for earnings analysis")
	}

	// Search for earnings-related news
	earningsQuery := fmt.Sprintf("%s earnings quarterly results", strings.ToUpper(symbol))
	newsData, err := n.fetchFinancialNews(earningsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch earnings news: %w", err)
	}

	// Filter for earnings-specific content
	earningsNews := n.filterEarningsNews(newsData)
	sentiment := n.analyzeSentiment(earningsNews)

	return map[string]interface{}{
		"symbol":            symbol,
		"earnings_news":     earningsNews,
		"sentiment_score":   sentiment["overall_score"],
		"analyst_sentiment": sentiment["analyst_reactions"],
		"market_reaction":   n.assessMarketImpact(sentiment),
		"confidence_level":  sentiment["confidence"],
		"last_updated":      time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// fetchFinancialNews retrieves news articles from News API
func (n *NewsAnalysisAgent) fetchFinancialNews(query string) ([]map[string]interface{}, error) {
	// Add financial keywords to improve relevance
	searchQuery := fmt.Sprintf("%s AND (stock OR financial OR market OR earnings OR trading)", query)

	url := fmt.Sprintf("https://newsapi.org/v2/everything?q=%s&language=en&sortBy=relevancy&pageSize=20&apiKey=%s",
		strings.ReplaceAll(searchQuery, " ", "%20"), n.newsAPIKey)

	resp, err := n.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	articles, ok := result["articles"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	var newsData []map[string]interface{}
	for _, article := range articles {
		if articleMap, ok := article.(map[string]interface{}); ok {
			newsData = append(newsData, articleMap)
		}
	}

	return newsData, nil
}

// fetchTopHeadlines retrieves top headlines from News API
func (n *NewsAnalysisAgent) fetchTopHeadlines(category, country string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://newsapi.org/v2/top-headlines?category=%s&country=%s&pageSize=20&apiKey=%s",
		category, country, n.newsAPIKey)

	resp, err := n.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	articles, ok := result["articles"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	var headlines []map[string]interface{}
	for _, article := range articles {
		if articleMap, ok := article.(map[string]interface{}); ok {
			headlines = append(headlines, articleMap)
		}
	}

	return headlines, nil
}

// analyzeSentiment performs basic sentiment analysis on news articles
func (n *NewsAnalysisAgent) analyzeSentiment(articles []map[string]interface{}) map[string]interface{} {
	if len(articles) == 0 {
		return map[string]interface{}{
			"overall_score": 0.0,
			"confidence":    0.0,
			"article_count": 0,
		}
	}

	// Simple keyword-based sentiment analysis
	positiveKeywords := []string{"gain", "rise", "up", "positive", "strong", "growth", "profit", "beat", "exceed", "bullish"}
	negativeKeywords := []string{"fall", "drop", "down", "negative", "weak", "loss", "miss", "below", "decline", "bearish"}

	var totalScore float64
	var sentimentDetails []map[string]interface{}

	for _, article := range articles {
		title := strings.ToLower(article["title"].(string))
		description := ""
		if desc, ok := article["description"].(string); ok {
			description = strings.ToLower(desc)
		}

		text := title + " " + description

		positiveCount := 0
		negativeCount := 0

		for _, keyword := range positiveKeywords {
			if strings.Contains(text, keyword) {
				positiveCount++
			}
		}

		for _, keyword := range negativeKeywords {
			if strings.Contains(text, keyword) {
				negativeCount++
			}
		}

		// Calculate article sentiment score (-1 to 1)
		var articleScore float64
		if positiveCount+negativeCount > 0 {
			articleScore = float64(positiveCount-negativeCount) / float64(positiveCount+negativeCount)
		}

		totalScore += articleScore

		sentimentDetails = append(sentimentDetails, map[string]interface{}{
			"title":            article["title"],
			"sentiment_score":  articleScore,
			"positive_signals": positiveCount,
			"negative_signals": negativeCount,
		})
	}

	overallScore := totalScore / float64(len(articles))

	return map[string]interface{}{
		"overall_score":     overallScore,
		"confidence":        n.calculateConfidence(len(articles), totalScore),
		"article_count":     len(articles),
		"sentiment_details": sentimentDetails,
		"analyst_reactions": n.extractAnalystReactions(articles),
	}
}

// assessMarketImpact determines potential market impact based on sentiment
func (n *NewsAnalysisAgent) assessMarketImpact(sentiment map[string]interface{}) map[string]interface{} {
	score, _ := sentiment["overall_score"].(float64)
	confidence, _ := sentiment["confidence"].(float64)

	var impact, direction string
	var magnitude float64

	if score > 0.3 {
		direction = "positive"
		impact = "bullish"
		magnitude = score * confidence
	} else if score < -0.3 {
		direction = "negative"
		impact = "bearish"
		magnitude = -score * confidence
	} else {
		direction = "neutral"
		impact = "mixed"
		magnitude = 0.0
	}

	return map[string]interface{}{
		"direction":        direction,
		"impact_type":      impact,
		"magnitude":        magnitude,
		"confidence_level": confidence,
		"recommendation":   n.generateRecommendation(direction, magnitude),
	}
}

// Helper functions
func (n *NewsAnalysisAgent) filterEarningsNews(articles []map[string]interface{}) []map[string]interface{} {
	var filtered []map[string]interface{}
	earningsKeywords := []string{"earnings", "quarterly", "results", "revenue", "guidance", "analyst", "eps"}

	for _, article := range articles {
		title := strings.ToLower(article["title"].(string))
		for _, keyword := range earningsKeywords {
			if strings.Contains(title, keyword) {
				filtered = append(filtered, article)
				break
			}
		}
	}

	return filtered
}

func (n *NewsAnalysisAgent) calculateConfidence(articleCount int, totalScore float64) float64 {
	// Confidence increases with article count and consistency of sentiment
	if articleCount == 0 {
		return 0.0
	}

	articleFactor := float64(articleCount) / 20.0 // Max confidence boost from 20 articles
	if articleFactor > 1.0 {
		articleFactor = 1.0
	}

	// Consistency factor based on how extreme the sentiment is
	consistencyFactor := 1.0
	if totalScore != 0 {
		avgScore := totalScore / float64(articleCount)
		consistencyFactor = (avgScore * avgScore) // Square to emphasize strong sentiment
	}

	return articleFactor * consistencyFactor * 0.9 // Max 90% confidence
}

func (n *NewsAnalysisAgent) extractAnalystReactions(articles []map[string]interface{}) []string {
	reactions := []string{}
	analystKeywords := []string{"analyst", "rating", "upgrade", "downgrade", "target", "recommendation"}

	for _, article := range articles {
		title := strings.ToLower(article["title"].(string))
		for _, keyword := range analystKeywords {
			if strings.Contains(title, keyword) {
				reactions = append(reactions, article["title"].(string))
				break
			}
		}
	}

	return reactions
}

func (n *NewsAnalysisAgent) generateRecommendation(direction string, magnitude float64) string {
	switch direction {
	case "positive":
		if magnitude > 0.7 {
			return "Strong positive sentiment - Consider bullish positions"
		}
		return "Positive sentiment detected - Monitor for entry opportunities"
	case "negative":
		if magnitude > 0.7 {
			return "Strong negative sentiment - Consider defensive positions"
		}
		return "Negative sentiment detected - Exercise caution"
	default:
		return "Mixed sentiment - Wait for clearer signals"
	}
}

func main() {
	agent := &NewsAnalysisAgent{}

	// Start the agent with framework auto-configuration
	f, err := framework.NewFramework(
		framework.WithAgentName("news-analysis-agent"),
		framework.WithPort(8081),
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

	fmt.Println("Starting News Analysis Agent on port 8081...")
	fmt.Println("Available capabilities:")
	fmt.Println("  - analyze-financial-news: Sentiment analysis of financial news")
	fmt.Println("  - get-market-headlines: Latest market headlines")
	fmt.Println("  - analyze-earnings-sentiment: Earnings reports analysis")

	// Start HTTP server
	if err := f.StartHTTPServer(ctx, agent); err != nil {
		fmt.Printf("Failed to start news analysis agent: %v\n", err)
		os.Exit(1)
	}
}
