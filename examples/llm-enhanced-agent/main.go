package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind"
)

// LLMEnhancedAgent demonstrates the new LLM-specific metadata fields
type LLMEnhancedAgent struct {
	*framework.BaseAgent
}

// @capability: stock_price
// @description: Provides real-time stock prices for publicly traded companies
// @llm_prompt: Ask me for current stock prices like 'What is AAPL trading at?' or 'Get Tesla price'
// @specialties: NYSE, NASDAQ, real-time quotes, after-hours trading, market data
// @domain: finance
// @complexity: low
// @latency: fast
// @cost: low
// @business_value: real-time-decisions, risk-assessment
func (l *LLMEnhancedAgent) GetStockPrice(symbol string) map[string]interface{} {
	// Simulate real-time stock data
	price := 100.0 + (rand.Float64()-0.5)*20 // Random price around $100
	change := (rand.Float64() - 0.5) * 5     // Random change ±$2.50

	return map[string]interface{}{
		"symbol":    symbol,
		"price":     fmt.Sprintf("%.2f", price),
		"change":    fmt.Sprintf("%.2f", change),
		"timestamp": time.Now().Format(time.RFC3339),
		"exchange":  "NASDAQ",
		"status":    "open",
	}
}

// @capability: market_analysis
// @description: Provides AI-powered market analysis and investment insights
// @llm_prompt: Ask me to analyze any stock or market trend like 'Analyze AAPL fundamentals' or 'What do you think about tech stocks?'
// @specialties: fundamental analysis, technical indicators, market sentiment, risk assessment
// @domain: finance
// @complexity: high
// @latency: medium
// @cost: medium
// @business_value: investment-strategy, portfolio-optimization, risk-management
func (l *LLMEnhancedAgent) AnalyzeMarket(query string) string {
	ctx := context.Background()

	// Use the LLM integration for sophisticated analysis
	prompt := fmt.Sprintf(`As a market analyst, analyze: %s
	
	Provide insights including:
	1. Key factors to consider
	2. Risk assessment
	3. Market sentiment
	4. Actionable recommendations
	
	Keep it concise but comprehensive.`, query)

	// Try to use AI for analysis via agent collaboration
	if response, err := l.ContactAgent(ctx, "ai-assistant", prompt); err == nil {
		return response
	}

	// Fallback to basic analysis if AI is not available
	return fmt.Sprintf("Basic analysis for '%s': Market conditions appear stable. "+
		"Please consult additional resources and set up AI integration (OPENAI_API_KEY) for detailed analysis.", query)
}

// @capability: agent_collaboration
// @description: Demonstrates agent-to-agent communication for complex workflows
// @llm_prompt: Ask me to coordinate with other agents like 'Get market data and news for AAPL' or 'Find financial analysts for portfolio review'
// @specialties: multi-agent workflows, data aggregation, intelligent routing, autonomous coordination
// @domain: coordination
// @complexity: high
// @latency: medium
// @cost: low
// @business_value: workflow-automation, decision-support, comprehensive-analysis
func (l *LLMEnhancedAgent) CoordinateAnalysis(request string) string {
	ctx := context.Background()

	// Generate agent catalog for LLM decision making
	catalog, err := l.GenerateAgentCatalog(ctx)
	if err != nil {
		return fmt.Sprintf("Failed to generate agent catalog: %v", err)
	}

	// Use LLM to decide which agents to contact
	response, err := l.ProcessLLMDecision(ctx, request, catalog)
	if err != nil {
		return fmt.Sprintf("Failed to process LLM decision: %v", err)
	}

	return response
}

// @capability: chat_interface
// @description: Provides conversational interface for financial queries
// @llm_prompt: Chat with me about financial markets, ask questions like 'Should I invest in tech stocks?' or 'Explain market volatility'
// @specialties: natural language processing, conversational AI, financial education, interactive guidance
// @domain: communication
// @complexity: medium
// @latency: fast
// @cost: low
// @business_value: user-engagement, financial-literacy, customer-support
func (l *LLMEnhancedAgent) Chat(message string) string {
	ctx := context.Background()

	// Enhanced chat with LLM integration
	prompt := fmt.Sprintf(`You are a helpful financial assistant. User says: "%s"
	
	Respond in a friendly, informative way. If they're asking about specific stocks or market data,
	let them know they can use more specific commands. Keep responses concise and helpful.`, message)

	if response, err := l.ContactAgent(ctx, "ai-assistant", prompt); err == nil {
		return response
	}

	// Fallback response
	return fmt.Sprintf("Thanks for your message: '%s'. I can help with stock prices, market analysis, and financial insights. "+
		"For more sophisticated responses, please configure AI integration.", message)
}

func main() {
	// Create framework instance
	fw, err := framework.NewFramework(
		framework.WithPort(8081), // Different port to avoid conflicts
		framework.WithAgentName("llm-enhanced-agent"),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	// Create and register the LLM-enhanced agent
	agent := &LLMEnhancedAgent{}

	// Initialize and start the framework
	ctx := context.Background()
	if err := fw.InitializeAgent(ctx, agent); err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	// Add a simple web interface to test the agent
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <title>LLM-Enhanced Agent Demo</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .description { color: #666; margin: 5px 0; }
        .llm-prompt { background: #e3f2fd; padding: 10px; border-radius: 3px; font-style: italic; }
        .specialties { color: #2196f3; font-weight: bold; }
    </style>
</head>
<body>
    <h1>LLM-Enhanced Agent Demo</h1>
    <p>This agent demonstrates the new LLM-specific metadata fields for agentic communication.</p>
    
    <div class="endpoint">
        <h3>🏢 Stock Price (GET /stock_price?symbol=AAPL)</h3>
        <div class="description">Provides real-time stock prices for publicly traded companies</div>
        <div class="llm-prompt"><strong>LLM Prompt:</strong> Ask me for current stock prices like 'What is AAPL trading at?' or 'Get Tesla price'</div>
        <div class="specialties"><strong>Specialties:</strong> NYSE, NASDAQ, real-time quotes, after-hours trading, market data</div>
    </div>
    
    <div class="endpoint">
        <h3>📊 Market Analysis (POST /market_analysis)</h3>
        <div class="description">Provides AI-powered market analysis and investment insights</div>
        <div class="llm-prompt"><strong>LLM Prompt:</strong> Ask me to analyze any stock or market trend like 'Analyze AAPL fundamentals' or 'What do you think about tech stocks?'</div>
        <div class="specialties"><strong>Specialties:</strong> fundamental analysis, technical indicators, market sentiment, risk assessment</div>
    </div>
    
    <div class="endpoint">
        <h3>🤝 Agent Collaboration (POST /coordinate_analysis)</h3>
        <div class="description">Demonstrates agent-to-agent communication for complex workflows</div>
        <div class="llm-prompt"><strong>LLM Prompt:</strong> Ask me to coordinate with other agents like 'Get market data and news for AAPL' or 'Find financial analysts for portfolio review'</div>
        <div class="specialties"><strong>Specialties:</strong> multi-agent workflows, data aggregation, intelligent routing, autonomous coordination</div>
    </div>
    
    <div class="endpoint">
        <h3>💬 Chat Interface (POST /chat)</h3>
        <div class="description">Provides conversational interface for financial queries</div>
        <div class="llm-prompt"><strong>LLM Prompt:</strong> Chat with me about financial markets, ask questions like 'Should I invest in tech stocks?' or 'Explain market volatility'</div>
        <div class="specialties"><strong>Specialties:</strong> natural language processing, conversational AI, financial education, interactive guidance</div>
    </div>
    
    <h2>Try the Agent Catalog</h2>
    <p>Visit <a href="/agent_catalog">/agent_catalog</a> to see how this agent appears in LLM-friendly catalogs.</p>
    
    <h2>OpenAPI Documentation</h2>
    <p>Visit <a href="/openapi.json">/openapi.json</a> to see the auto-generated API documentation with LLM metadata.</p>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	// Add endpoint to demonstrate agent catalog generation
	http.HandleFunc("/agent_catalog", func(w http.ResponseWriter, r *http.Request) {
		catalog, err := agent.GenerateAgentCatalog(context.Background())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate catalog: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(catalog))
	})

	// Start the framework
	log.Printf("LLM-Enhanced Agent starting on port 8081...")
	log.Printf("Visit http://localhost:8081 to see the demo interface")
	log.Printf("Agent catalog: http://localhost:8081/agent_catalog")

	// Start the HTTP server
	if err := fw.StartHTTPServer(ctx, agent); err != nil {
		log.Fatalf("Framework error: %v", err)
	}
}
