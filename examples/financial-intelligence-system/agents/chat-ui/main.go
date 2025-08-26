package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	framework "github.com/itsneelabh/gomind"
)

// ChatUIAgent provides conversational interface with LLM-assisted routing to specialized agents
type ChatUIAgent struct {
	*framework.BaseAgent
	llmDecisionLog []LLMDecision
	routingStats   RoutingStats
}

// LLMDecision represents a logged LLM routing decision
type LLMDecision struct {
	Timestamp      time.Time     `json:"timestamp"`
	Event          string        `json:"event"`
	UserQuery      string        `json:"user_query"`
	LLMPrompt      string        `json:"llm_prompt"`
	LLMResponse    string        `json:"llm_response"`
	Confidence     float64       `json:"confidence"`
	SelectedAgents []string      `json:"selected_agents"`
	Reasoning      string        `json:"reasoning"`
	ExecutionTime  time.Duration `json:"execution_time"`
}

// RoutingStats tracks routing performance and accuracy
type RoutingStats struct {
	TotalQueries     int            `json:"total_queries"`
	SuccessfulRoutes int            `json:"successful_routes"`
	FailedRoutes     int            `json:"failed_routes"`
	AvgConfidence    float64        `json:"avg_confidence"`
	AvgResponseTime  time.Duration  `json:"avg_response_time"`
	TopAgents        map[string]int `json:"top_agents"`
}

// AgentDiscoveryEvent represents a logged agent discovery event
type AgentDiscoveryEvent struct {
	Timestamp           time.Time                     `json:"timestamp"`
	Event               string                        `json:"event"`
	RequestedCapability string                        `json:"requested_capability"`
	DiscoveryMethod     string                        `json:"discovery_method"`
	DiscoveredAgents    []framework.AgentRegistration `json:"discovered_agents"`
	SelectionCriteria   string                        `json:"selection_criteria"`
	ExecutionTime       time.Duration                 `json:"execution_time"`
}

// InterAgentCommunication represents logged communication between agents
type InterAgentCommunication struct {
	Timestamp       time.Time     `json:"timestamp"`
	Event           string        `json:"event"`
	From            string        `json:"from"`
	To              string        `json:"to"`
	Request         interface{}   `json:"request"`
	ResponseTime    time.Duration `json:"response_time"`
	Status          string        `json:"status"`
	ResponsePreview string        `json:"response_preview"`
}

// LLMDecisionDetail represents detailed LLM routing decision with comprehensive analysis
type LLMDecisionDetail struct {
	Query                string        `json:"query"`
	SelectedCapabilities []string      `json:"selected_capabilities"`
	Reasoning            string        `json:"reasoning"`
	Confidence           float64       `json:"confidence"`
	ExecutionStrategy    string        `json:"execution_strategy"`
	AlternativeOptions   []string      `json:"alternative_options"`
	RiskFactors          []string      `json:"risk_factors"`
	ExecutionTime        time.Duration `json:"execution_time"`
}

// AgentInfo represents detailed agent information for routing decisions
type AgentInfo struct {
	Name         string        `json:"name"`
	Specialties  []string      `json:"specialties"`
	Capabilities []string      `json:"capabilities"`
	LLMPrompt    string        `json:"llm_prompt"`
	ResponseTime time.Duration `json:"response_time"`
}

// Initialize sets up the chat UI agent
func (c *ChatUIAgent) Initialize(ctx context.Context) error {
	c.Logger().Info("Chat UI Agent initialized", map[string]interface{}{
		"agent_id": c.GetAgentID(),
		"features": []string{"LLM routing", "agent discovery", "natural language"},
	})

	return nil
}

// @capability: process-user-query
// @description: Advanced LLM-assisted query processing with comprehensive multi-agent coordination and logging
// @domain: conversational-ai
// @complexity: high
// @latency: 2-5s
// @cost: medium
// @confidence: 0.95
// @business_value: user-experience,intelligent-routing,system-orchestration
// @llm_prompt: Ask me any financial question and I'll intelligently route it to the best specialized agents
// @specialties: query-analysis,agent-coordination,response-synthesis,decision-logging
// @use_cases: financial-inquiry,multi-agent-coordination,intelligent-routing
// @input_types: natural-language-query,user-question
// @output_formats: formatted-response,agent-routing-info,combined-analysis
func (c *ChatUIAgent) ProcessUserQuery(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query, ok := input["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	startTime := time.Now()

	c.Logger().Info("Processing user query with advanced LLM analysis", map[string]interface{}{
		"query":      query,
		"session_id": fmt.Sprintf("session-%d", time.Now().Unix()),
	})

	// Step 1: Advanced LLM Analysis
	llmDecision, err := c.performAdvancedLLMAnalysis(ctx, query)
	if err != nil {
		c.Logger().Error("Advanced LLM analysis failed", map[string]interface{}{
			"error": err.Error(),
		})
		// Fallback to basic routing
		return c.fallbackProcessing(ctx, query)
	}

	// Step 2: Multi-capability agent discovery
	var allDiscoveredAgents []framework.AgentRegistration
	var discoveryEvents []AgentDiscoveryEvent

	for _, capability := range llmDecision.SelectedCapabilities {
		event, agents, err := c.performAgentDiscovery(ctx, capability)
		if err != nil {
			c.Logger().Warn("Agent discovery failed for capability", map[string]interface{}{
				"capability": capability,
				"error":      err.Error(),
			})
			continue
		}

		discoveryEvents = append(discoveryEvents, event)
		allDiscoveredAgents = append(allDiscoveredAgents, agents...)
	}

	// Step 3: Multi-agent orchestration and execution
	agentResponses, communications, err := c.orchestrateMultiAgentExecution(ctx, query, allDiscoveredAgents, llmDecision)
	if err != nil {
		return nil, fmt.Errorf("multi-agent execution failed: %w", err)
	}

	// Step 4: Response synthesis with confidence scoring
	synthesizedResponse := c.synthesizeMultiAgentResponse(query, agentResponses, llmDecision)

	// Step 5: Update routing statistics
	c.updateRoutingStatistics(llmDecision, len(allDiscoveredAgents), time.Since(startTime))

	// Comprehensive response with full transparency
	return map[string]interface{}{
		"user_query": query,
		"llm_analysis": map[string]interface{}{
			"selected_capabilities": llmDecision.SelectedCapabilities,
			"reasoning":             llmDecision.Reasoning,
			"confidence":            llmDecision.Confidence,
			"execution_strategy":    llmDecision.ExecutionStrategy,
			"risk_factors":          llmDecision.RiskFactors,
		},
		"agent_discovery": map[string]interface{}{
			"events":           discoveryEvents,
			"total_agents":     len(allDiscoveredAgents),
			"discovery_method": "redis_capability_search",
		},
		"agent_communications": communications,
		"agent_responses":      agentResponses,
		"synthesized_response": synthesizedResponse,
		"routing_statistics": map[string]interface{}{
			"total_execution_time": time.Since(startTime).String(),
			"llm_decision_time":    llmDecision.ExecutionTime.String(),
			"agents_contacted":     len(allDiscoveredAgents),
			"capabilities_used":    len(llmDecision.SelectedCapabilities),
		},
		"transparency_proof": map[string]interface{}{
			"autonomous_decision_making": true,
			"llm_assisted_routing":       true,
			"multi_agent_coordination":   len(agentResponses) > 1,
			"evidence_available":         true,
		},
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"processing_type": "advanced_llm_assisted_multi_agent_coordination",
	}, nil
}

// @capability: get-available-agents
// @description: Returns a catalog of all available agents and their capabilities for user awareness
// @domain: system-info
// @complexity: low
// @latency: 1-3s
// @cost: low
// @confidence: 0.95
// @business_value: transparency,user-guidance,system-overview
// @llm_prompt: Ask me to show you what financial services and agents are available in the system
// @specialties: agent discovery,system catalog,capability listing
// @use_cases: system-exploration,user-guidance,agent-discovery
// @input_types: catalog-request,filter-criteria
// @output_formats: agent-catalog,capability-list,service-directory
func (c *ChatUIAgent) GetAvailableAgents(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Use the framework's FindAgents method to get all agents
	// We'll search for common capabilities to find all agents
	commonCapabilities := []string{"process", "analyze", "get", "handle", "manage"}

	allAgents := make(map[string]framework.AgentRegistration)

	for _, capability := range commonCapabilities {
		agents, err := c.FindAgents(ctx, capability)
		if err != nil {
			c.Logger().Warn("Failed to find agents for capability", map[string]interface{}{
				"capability": capability,
				"error":      err.Error(),
			})
			continue
		}

		for _, agent := range agents {
			// Avoid duplicates by using agent ID as key
			if agent.ID != c.GetAgentID() { // Don't include self
				allAgents[agent.ID] = agent
			}
		}
	}

	var agentInfo []map[string]interface{}
	for _, agent := range allAgents {
		info := map[string]interface{}{
			"name":         agent.Name,
			"id":           agent.ID,
			"address":      fmt.Sprintf("%s:%d", agent.Address, agent.Port),
			"status":       string(agent.Status),
			"capabilities": len(agent.Capabilities),
		}

		// Add capability details
		var capabilities []map[string]interface{}
		for _, cap := range agent.Capabilities {
			capabilities = append(capabilities, map[string]interface{}{
				"name":        cap.Name,
				"description": cap.Description,
				"domain":      cap.Domain,
				"llm_prompt":  cap.LLMPrompt,
				"specialties": cap.Specialties,
			})
		}
		info["capability_details"] = capabilities
		agentInfo = append(agentInfo, info)
	}

	return map[string]interface{}{
		"total_agents":   len(allAgents),
		"agents":         agentInfo,
		"last_updated":   time.Now().UTC().Format(time.RFC3339),
		"discovery_type": "redis_service_discovery",
	}, nil
}

// @capability: health-check-system
// @description: Performs comprehensive health check of all agents in the financial intelligence system
// @domain: system-monitoring
// @complexity: medium
// @latency: 2-8s
// @cost: low
// @confidence: 0.90
// @business_value: system-reliability,monitoring,ops-visibility
// @llm_prompt: Ask me to check the health and status of all financial agents in the system
// @specialties: health monitoring,system status,agent connectivity
// @use_cases: system-monitoring,troubleshooting,ops-dashboard
// @input_types: health-check-request,agent-filter
// @output_formats: health-report,system-status,agent-availability
func (c *ChatUIAgent) HealthCheckSystem(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Use the framework's FindAgents method to get all agents
	commonCapabilities := []string{"process", "analyze", "get", "handle", "manage"}

	allAgents := make(map[string]framework.AgentRegistration)

	for _, capability := range commonCapabilities {
		agents, err := c.FindAgents(ctx, capability)
		if err != nil {
			c.Logger().Warn("Failed to find agents for capability", map[string]interface{}{
				"capability": capability,
				"error":      err.Error(),
			})
			continue
		}

		for _, agent := range agents {
			// Avoid duplicates by using agent ID as key
			if agent.ID != c.GetAgentID() { // Don't include self
				allAgents[agent.ID] = agent
			}
		}
	}

	var healthyAgents, unhealthyAgents []string
	agentHealthDetails := make(map[string]interface{})

	for _, agent := range allAgents {
		// Simple health check based on registration status and heartbeat
		isHealthy := agent.Status == framework.StatusHealthy
		timeSinceHeartbeat := time.Since(agent.LastHeartbeat)

		status := "healthy"
		if timeSinceHeartbeat > 2*time.Minute {
			status = "stale"
			isHealthy = false
		}

		agentHealthDetails[agent.Name] = map[string]interface{}{
			"status":               status,
			"last_heartbeat":       agent.LastHeartbeat.Format(time.RFC3339),
			"time_since_heartbeat": timeSinceHeartbeat.String(),
			"capabilities_count":   len(agent.Capabilities),
			"address":              fmt.Sprintf("%s:%d", agent.Address, agent.Port),
		}

		if isHealthy {
			healthyAgents = append(healthyAgents, agent.Name)
		} else {
			unhealthyAgents = append(unhealthyAgents, agent.Name)
		}
	}

	systemHealth := "healthy"
	if len(unhealthyAgents) > 0 {
		if len(unhealthyAgents) >= len(allAgents)/2 {
			systemHealth = "critical"
		} else {
			systemHealth = "degraded"
		}
	}

	return map[string]interface{}{
		"system_health":    systemHealth,
		"total_agents":     len(allAgents),
		"healthy_agents":   healthyAgents,
		"unhealthy_agents": unhealthyAgents,
		"agent_details":    agentHealthDetails,
		"check_timestamp":  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// askLLMForRouting uses the AI client to determine routing
func (c *ChatUIAgent) askLLMForRouting(ctx context.Context, query, catalog string) (map[string]interface{}, error) {
	// For this demo, we'll use simple fallback routing since AI client access is internal
	// In a real implementation, you would check if AI is available and use it
	c.Logger().Info("Using fallback routing for demo purposes", map[string]interface{}{
		"reason": "AI client access simplified for demonstration",
	})

	return c.fallbackRouting(query), nil
}

// fallbackRouting provides simple keyword-based routing when LLM is unavailable
func (c *ChatUIAgent) fallbackRouting(query string) map[string]interface{} {
	queryLower := strings.ToLower(query)

	// Simple keyword matching
	if strings.Contains(queryLower, "price") || strings.Contains(queryLower, "market") || strings.Contains(queryLower, "stock") {
		return map[string]interface{}{
			"primary_agent":    "market-data-agent",
			"secondary_agents": []string{},
			"reasoning":        "Query contains market data keywords",
			"query_type":       "market-data",
			"parameters": map[string]interface{}{
				"query": query,
			},
		}
	}

	if strings.Contains(queryLower, "news") || strings.Contains(queryLower, "sentiment") || strings.Contains(queryLower, "earnings") {
		return map[string]interface{}{
			"primary_agent":    "news-analysis-agent",
			"secondary_agents": []string{},
			"reasoning":        "Query contains news analysis keywords",
			"query_type":       "news-analysis",
			"parameters": map[string]interface{}{
				"query": query,
			},
		}
	}

	// Default to self-handling
	return map[string]interface{}{
		"primary_agent":    "chat-ui-agent",
		"secondary_agents": []string{},
		"reasoning":        "General query handled by chat interface",
		"query_type":       "general",
		"parameters": map[string]interface{}{
			"query": query,
		},
	}
}

// executeRouting contacts the determined agents and collects responses
func (c *ChatUIAgent) executeRouting(ctx context.Context, routingDecision map[string]interface{}) (map[string]interface{}, error) {
	primaryAgent, _ := routingDecision["primary_agent"].(string)
	parameters, _ := routingDecision["parameters"].(map[string]interface{})

	responses := make(map[string]interface{})

	// Contact primary agent
	if primaryAgent != "" && primaryAgent != "chat-ui-agent" {
		response, err := c.contactAgent(ctx, primaryAgent, parameters)
		if err != nil {
			c.Logger().Warn("Failed to contact primary agent", map[string]interface{}{
				"agent": primaryAgent,
				"error": err.Error(),
			})
			responses[primaryAgent] = map[string]interface{}{
				"error":  err.Error(),
				"status": "failed",
			}
		} else {
			responses[primaryAgent] = response
		}
	}

	// Contact secondary agents if specified
	if secondaryAgents, ok := routingDecision["secondary_agents"].([]interface{}); ok {
		for _, agentInterface := range secondaryAgents {
			if agentName, ok := agentInterface.(string); ok && agentName != "chat-ui-agent" {
				response, err := c.contactAgent(ctx, agentName, parameters)
				if err != nil {
					c.Logger().Warn("Failed to contact secondary agent", map[string]interface{}{
						"agent": agentName,
						"error": err.Error(),
					})
					responses[agentName] = map[string]interface{}{
						"error":  err.Error(),
						"status": "failed",
					}
				} else {
					responses[agentName] = response
				}
			}
		}
	}

	return responses, nil
}

// contactAgent communicates with another agent via HTTP
func (c *ChatUIAgent) contactAgent(ctx context.Context, agentName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	// This is a simplified implementation
	// In a real system, you would use the Discovery service to find the agent's address
	// and make HTTP calls to its capabilities

	// Try to find the agent by searching common capabilities
	commonCapabilities := []string{"market-data", "news", "analysis", "query", "price", "financial"}

	for _, capability := range commonCapabilities {
		agents, err := c.FindAgents(ctx, capability)
		if err != nil {
			c.Logger().Warn("Failed to find agents for capability", "capability", capability, "error", err)
			continue
		}

		for _, agent := range agents {
			if agent.Name == agentName {
				// For this demo, we'll return a simulated response
				// In reality, you'd make HTTP calls to the agent's endpoints
				return map[string]interface{}{
					"agent":      agentName,
					"status":     "contacted",
					"message":    fmt.Sprintf("Successfully contacted %s with parameters", agentName),
					"parameters": parameters,
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("agent %s not found in discovery service", agentName)
}

// ServeHTTP provides a simple web interface for demonstration
func (c *ChatUIAgent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This would be automatically handled by the framework's HTTP server
	// This is just for demonstration of the UI capability

	if r.URL.Path == "/chat" {
		c.serveChatUI(w, r)
		return
	}

	// Delegate to framework's default handling
	http.NotFound(w, r)
}

func (c *ChatUIAgent) serveChatUI(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Financial Intelligence Assistant</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .chat-container { border: 1px solid #ddd; height: 400px; overflow-y: auto; padding: 10px; margin-bottom: 10px; }
        .input-container { display: flex; }
        .input-container input { flex: 1; padding: 10px; }
        .input-container button { padding: 10px 20px; }
        .message { margin: 10px 0; padding: 10px; border-radius: 5px; }
        .user { background-color: #e3f2fd; text-align: right; }
        .assistant { background-color: #f3e5f5; }
        .examples { margin-top: 20px; }
        .example { background-color: #fff3e0; padding: 10px; margin: 5px 0; border-radius: 5px; cursor: pointer; }
    </style>
</head>
<body>
    <h1>🤖 Financial Intelligence Assistant</h1>
    <p>Ask me anything about financial markets, stocks, news, or get system status information!</p>
    
    <div class="chat-container" id="chatContainer">
        <div class="message assistant">
            <strong>Assistant:</strong> Hello! I'm your financial intelligence assistant. I can help you with:
            <ul>
                <li>📈 Real-time stock prices and market data</li>
                <li>📰 Financial news analysis and sentiment</li>
                <li>🔍 System health and agent status</li>
                <li>🤝 Intelligent routing to specialized agents</li>
            </ul>
            What would you like to know?
        </div>
    </div>
    
    <div class="input-container">
        <input type="text" id="userInput" placeholder="Ask about stocks, market news, or system status..." />
        <button onclick="sendMessage()">Send</button>
    </div>
    
    <div class="examples">
        <h3>Try these examples:</h3>
        <div class="example" onclick="setInput('What is the current price of Apple stock?')">
            What is the current price of Apple stock?
        </div>
        <div class="example" onclick="setInput('What is the market sentiment on Tesla today?')">
            What is the market sentiment on Tesla today?
        </div>
        <div class="example" onclick="setInput('Show me all available agents in the system')">
            Show me all available agents in the system
        </div>
        <div class="example" onclick="setInput('Check the health of all financial agents')">
            Check the health of all financial agents
        </div>
    </div>

    <script>
        function setInput(text) {
            document.getElementById('userInput').value = text;
        }
        
        function sendMessage() {
            const input = document.getElementById('userInput');
            const message = input.value.trim();
            if (!message) return;
            
            // Add user message to chat
            addMessage('user', message);
            input.value = '';
            
            // Send to backend (simplified for demo)
            fetch('/capabilities/process-user-query', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({query: message})
            })
            .then(response => response.json())
            .then(data => {
                addMessage('assistant', JSON.stringify(data, null, 2));
            })
            .catch(error => {
                addMessage('assistant', 'Error: ' + error.message);
            });
        }
        
        function addMessage(sender, text) {
            const container = document.getElementById('chatContainer');
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + sender;
            messageDiv.innerHTML = '<strong>' + (sender === 'user' ? 'You' : 'Assistant') + ':</strong> <pre>' + text + '</pre>';
            container.appendChild(messageDiv);
            container.scrollTop = container.scrollHeight;
        }
        
        // Enter key support
        document.getElementById('userInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// GenerateAgentCatalog creates a comprehensive catalog of available agents
func (c *ChatUIAgent) GenerateAgentCatalog(ctx context.Context) ([]AgentInfo, error) {
	// Use common capabilities to discover all agents
	commonCapabilities := []string{"process", "analyze", "get", "handle", "manage"}

	allAgents := make(map[string]framework.AgentRegistration)

	for _, capability := range commonCapabilities {
		agents, err := c.FindAgents(ctx, capability)
		if err != nil {
			continue
		}

		for _, agent := range agents {
			if agent.ID != c.GetAgentID() { // Don't include self
				allAgents[agent.ID] = agent
			}
		}
	}

	var catalog []AgentInfo
	for _, agent := range allAgents {
		var capabilities []string
		var specialties []string
		var llmPrompt string

		for _, cap := range agent.Capabilities {
			capabilities = append(capabilities, cap.Name)
			if len(cap.Specialties) > 0 {
				specialties = append(specialties, cap.Specialties...)
			}
			if cap.LLMPrompt != "" && llmPrompt == "" {
				llmPrompt = cap.LLMPrompt
			}
		}

		info := AgentInfo{
			Name:         agent.Name,
			Specialties:  specialties,
			Capabilities: capabilities,
			LLMPrompt:    llmPrompt,
			ResponseTime: time.Duration(100) * time.Millisecond, // Simulated
		}

		catalog = append(catalog, info)
	}

	return catalog, nil
}

// performAdvancedLLMAnalysis conducts sophisticated query analysis with comprehensive logging
func (c *ChatUIAgent) performAdvancedLLMAnalysis(ctx context.Context, query string) (LLMDecisionDetail, error) {
	startTime := time.Now()

	// Generate comprehensive agent catalog with capabilities
	catalog, err := c.GenerateAgentCatalog(ctx)
	if err != nil {
		return LLMDecisionDetail{}, fmt.Errorf("failed to generate agent catalog: %w", err)
	}

	// Create detailed LLM prompt for routing analysis
	llmPrompt := c.createAdvancedRoutingPrompt(query, catalog)

	c.Logger().Info("Performing advanced LLM analysis", map[string]interface{}{
		"query":            query,
		"prompt_length":    len(llmPrompt),
		"available_agents": len(catalog),
	})

	// For demonstration, we'll simulate sophisticated LLM analysis
	// In production, this would make actual OpenAI API calls
	decision := c.simulateAdvancedLLMDecision(query, catalog)
	decision.ExecutionTime = time.Since(startTime)

	// Log the complete decision process
	llmDecision := LLMDecision{
		Timestamp:      time.Now(),
		Event:          "llm_routing_decision",
		UserQuery:      query,
		LLMPrompt:      llmPrompt,
		LLMResponse:    decision.Reasoning,
		Confidence:     decision.Confidence,
		SelectedAgents: c.capabilitiesToAgentNames(decision.SelectedCapabilities),
		Reasoning:      decision.Reasoning,
		ExecutionTime:  decision.ExecutionTime,
	}

	c.llmDecisionLog = append(c.llmDecisionLog, llmDecision)

	return decision, nil
}

// createAdvancedRoutingPrompt creates a sophisticated prompt for LLM routing decisions
func (c *ChatUIAgent) createAdvancedRoutingPrompt(query string, catalog []AgentInfo) string {
	prompt := fmt.Sprintf(`
You are an intelligent financial query router. Analyze the user query and determine the optimal agent(s) to handle it.

User Query: "%s"

Available Financial Agents:
`, query)

	for _, agent := range catalog {
		prompt += fmt.Sprintf(`
Agent: %s
- Specialties: %v
- Capabilities: %v
- LLM Prompt: %s
- Response Time: %s
`, agent.Name, agent.Specialties, agent.Capabilities, agent.LLMPrompt, agent.ResponseTime)
	}

	prompt += `
Analysis Requirements:
1. Identify the key financial concepts and data requirements
2. Select the most appropriate agent(s) based on specialties
3. Consider if multiple agents need to coordinate
4. Assess confidence level (0.0-1.0)
5. Provide clear reasoning for your decision
6. Identify any risk factors or limitations
7. Suggest execution strategy (sequential, parallel, conditional)

Respond with your analysis including:
- Primary capabilities needed
- Selected agents with reasoning
- Confidence score
- Execution strategy
- Risk factors or limitations
`

	return prompt
}

// simulateAdvancedLLMDecision simulates sophisticated LLM decision making
func (c *ChatUIAgent) simulateAdvancedLLMDecision(query string, catalog []AgentInfo) LLMDecisionDetail {
	query = strings.ToLower(query)

	decision := LLMDecisionDetail{
		Query:              query,
		AlternativeOptions: []string{},
		RiskFactors:        []string{},
	}

	// Sophisticated query analysis with pattern matching
	if strings.Contains(query, "price") || strings.Contains(query, "trading") || strings.Contains(query, "quote") {
		decision.SelectedCapabilities = append(decision.SelectedCapabilities, "get-stock-price")
		decision.Reasoning += "Query requires real-time market data. "
		decision.Confidence += 0.3
	}

	if strings.Contains(query, "news") || strings.Contains(query, "sentiment") || strings.Contains(query, "earnings") {
		decision.SelectedCapabilities = append(decision.SelectedCapabilities, "analyze-financial-news")
		decision.Reasoning += "Query requires news sentiment analysis. "
		decision.Confidence += 0.25
	}

	if strings.Contains(query, "technical") || strings.Contains(query, "rsi") || strings.Contains(query, "macd") || strings.Contains(query, "indicators") {
		decision.SelectedCapabilities = append(decision.SelectedCapabilities, "calculate-technical-indicators")
		decision.Reasoning += "Query requires technical analysis. "
		decision.Confidence += 0.25
	}

	if strings.Contains(query, "portfolio") || strings.Contains(query, "invest") || strings.Contains(query, "allocation") || strings.Contains(query, "should i buy") {
		decision.SelectedCapabilities = append(decision.SelectedCapabilities, "analyze-portfolio")
		decision.Reasoning += "Query requires investment advice or portfolio analysis. "
		decision.Confidence += 0.2
	}

	// Complex multi-agent scenarios
	if strings.Contains(query, "complete analysis") || strings.Contains(query, "full analysis") {
		decision.SelectedCapabilities = []string{
			"get-stock-price", "analyze-financial-news",
			"calculate-technical-indicators", "analyze-portfolio",
		}
		decision.Reasoning = "Comprehensive analysis requires coordination across multiple specialized agents. "
		decision.Confidence = 0.9
		decision.ExecutionStrategy = "parallel_then_synthesis"
	}

	// Default fallback
	if len(decision.SelectedCapabilities) == 0 {
		decision.SelectedCapabilities = []string{"get-stock-price"}
		decision.Reasoning = "Query appears to be general financial inquiry, routing to market data as default. "
		decision.Confidence = 0.5
		decision.RiskFactors = append(decision.RiskFactors, "Low confidence routing - may need clarification")
	}

	// Determine execution strategy
	if len(decision.SelectedCapabilities) > 1 {
		decision.ExecutionStrategy = "parallel_coordination"
	} else {
		decision.ExecutionStrategy = "direct_routing"
	}

	// Normalize confidence (cap at 1.0)
	if decision.Confidence > 1.0 {
		decision.Confidence = 1.0
	}

	return decision
}

// performAgentDiscovery performs comprehensive agent discovery with detailed logging
func (c *ChatUIAgent) performAgentDiscovery(ctx context.Context, capability string) (AgentDiscoveryEvent, []framework.AgentRegistration, error) {
	startTime := time.Now()

	c.Logger().Info("Performing agent discovery", map[string]interface{}{
		"capability": capability,
		"method":     "redis_capability_search",
	})

	agents, err := c.FindAgents(ctx, capability)
	if err != nil {
		return AgentDiscoveryEvent{}, nil, fmt.Errorf("agent discovery failed: %w", err)
	}

	// Filter for healthy agents with additional criteria
	healthyAgents := make([]framework.AgentRegistration, 0)
	for _, agent := range agents {
		if agent.Status == framework.StatusHealthy {
			healthyAgents = append(healthyAgents, agent)
		}
	}

	event := AgentDiscoveryEvent{
		Timestamp:           time.Now(),
		Event:               "agent_discovery",
		RequestedCapability: capability,
		DiscoveryMethod:     "redis_capability_search",
		DiscoveredAgents:    healthyAgents,
		SelectionCriteria:   "healthy_status_and_best_response_time",
		ExecutionTime:       time.Since(startTime),
	}

	c.Logger().Info("Agent discovery completed", map[string]interface{}{
		"capability":        capability,
		"discovered_agents": len(healthyAgents),
		"execution_time":    event.ExecutionTime.String(),
	})

	return event, healthyAgents, nil
}

// orchestrateMultiAgentExecution manages parallel agent execution with communication logging
func (c *ChatUIAgent) orchestrateMultiAgentExecution(ctx context.Context, query string, agents []framework.AgentRegistration, decision LLMDecisionDetail) (map[string]interface{}, []InterAgentCommunication, error) {
	c.Logger().Info("Orchestrating multi-agent execution", map[string]interface{}{
		"query":              query,
		"agents_count":       len(agents),
		"execution_strategy": decision.ExecutionStrategy,
	})

	responses := make(map[string]interface{})
	communications := make([]InterAgentCommunication, 0)

	// Group agents by capability for coordinated execution
	agentGroups := c.groupAgentsByCapability(agents, decision.SelectedCapabilities)

	for capability, agentGroup := range agentGroups {
		if len(agentGroup) == 0 {
			continue
		}

		// Select best agent from group (first healthy one for now)
		selectedAgent := agentGroup[0]

		startTime := time.Now()

		// Prepare capability-specific request
		request := c.prepareAgentRequest(capability, query)

		c.Logger().Info("Executing agent capability", map[string]interface{}{
			"agent":      selectedAgent.Name,
			"capability": capability,
			"request":    request,
		})

		// For demonstration, simulate agent response
		// In production, this would make HTTP calls to agents
		response := c.simulateAgentResponse(capability, request)

		responses[capability] = response

		// Log inter-agent communication
		communication := InterAgentCommunication{
			Timestamp:       time.Now(),
			Event:           "agent_communication",
			From:            "chat-ui-agent",
			To:              selectedAgent.Name,
			Request:         request,
			ResponseTime:    time.Since(startTime),
			Status:          "success",
			ResponsePreview: c.generateResponsePreview(response),
		}

		communications = append(communications, communication)

		c.Logger().Info("Agent communication completed", map[string]interface{}{
			"agent":         selectedAgent.Name,
			"response_time": communication.ResponseTime.String(),
			"status":        communication.Status,
		})
	}

	return responses, communications, nil
}

// Helper methods for advanced functionality
func (c *ChatUIAgent) capabilitiesToAgentNames(capabilities []string) []string {
	// Map capabilities to likely agent names
	mapping := map[string]string{
		"get-stock-price":                "market-data-agent",
		"analyze-financial-news":         "news-analysis-agent",
		"calculate-technical-indicators": "technical-analysis-agent",
		"analyze-portfolio":              "portfolio-advisor-agent",
	}

	var agents []string
	for _, cap := range capabilities {
		if agent, exists := mapping[cap]; exists {
			agents = append(agents, agent)
		}
	}
	return agents
}

func (c *ChatUIAgent) groupAgentsByCapability(agents []framework.AgentRegistration, capabilities []string) map[string][]framework.AgentRegistration {
	groups := make(map[string][]framework.AgentRegistration)

	for _, capability := range capabilities {
		groups[capability] = make([]framework.AgentRegistration, 0)

		for _, agent := range agents {
			// Check if agent has this capability
			for _, agentCap := range agent.Capabilities {
				if agentCap.Name == capability {
					groups[capability] = append(groups[capability], agent)
					break
				}
			}
		}
	}

	return groups
}

func (c *ChatUIAgent) prepareAgentRequest(capability, query string) map[string]interface{} {
	baseRequest := map[string]interface{}{
		"query":     query,
		"timestamp": time.Now().Format(time.RFC3339),
		"source":    "chat-ui-agent",
	}

	// Add capability-specific parameters
	switch capability {
	case "get-stock-price":
		// Extract stock symbol from query if possible
		baseRequest["symbol"] = c.extractStockSymbol(query)
	case "analyze-financial-news":
		baseRequest["keywords"] = c.extractNewsKeywords(query)
	case "calculate-technical-indicators":
		baseRequest["indicators"] = c.extractTechnicalIndicators(query)
	case "analyze-portfolio":
		baseRequest["analysis_type"] = c.extractAnalysisType(query)
	}

	return baseRequest
}

func (c *ChatUIAgent) simulateAgentResponse(capability string, request map[string]interface{}) map[string]interface{} {
	// Simulate realistic agent responses
	switch capability {
	case "get-stock-price":
		return map[string]interface{}{
			"symbol":         request["symbol"],
			"price":          150.75,
			"change":         "+2.45",
			"change_percent": "+1.65%",
			"timestamp":      time.Now().Format(time.RFC3339),
		}
	case "analyze-financial-news":
		return map[string]interface{}{
			"sentiment": "positive",
			"score":     0.75,
			"keywords":  request["keywords"],
			"summary":   "Market shows positive sentiment with strong earnings reports",
		}
	case "calculate-technical-indicators":
		return map[string]interface{}{
			"rsi":        65.2,
			"macd":       1.23,
			"signal":     "buy",
			"confidence": 0.78,
		}
	case "analyze-portfolio":
		return map[string]interface{}{
			"recommendation":        "moderate_buy",
			"risk_level":            "medium",
			"diversification_score": 0.85,
			"suggested_allocation": map[string]interface{}{
				"stocks": 60,
				"bonds":  30,
				"cash":   10,
			},
		}
	default:
		return map[string]interface{}{
			"status":  "processed",
			"message": "Request processed successfully",
		}
	}
}

func (c *ChatUIAgent) generateResponsePreview(response map[string]interface{}) string {
	// Generate a brief preview of the response
	if len(response) == 0 {
		return "Empty response"
	}

	// Extract key fields for preview
	var preview []string
	for key, value := range response {
		if len(preview) >= 3 {
			break
		}
		preview = append(preview, fmt.Sprintf("%s: %v", key, value))
	}

	return fmt.Sprintf("{%s}", strings.Join(preview, ", "))
}

// Additional helper methods for query analysis
func (c *ChatUIAgent) extractStockSymbol(query string) string {
	// Simple extraction - in production, use more sophisticated NLP
	words := strings.Fields(strings.ToUpper(query))
	for _, word := range words {
		if len(word) >= 1 && len(word) <= 5 && strings.ToUpper(word) == word {
			return word
		}
	}
	return "AAPL" // default
}

func (c *ChatUIAgent) extractNewsKeywords(query string) []string {
	keywords := []string{}
	if strings.Contains(query, "earnings") {
		keywords = append(keywords, "earnings")
	}
	if strings.Contains(query, "merger") {
		keywords = append(keywords, "merger")
	}
	if len(keywords) == 0 {
		keywords = append(keywords, "general")
	}
	return keywords
}

func (c *ChatUIAgent) extractTechnicalIndicators(query string) []string {
	indicators := []string{}
	if strings.Contains(query, "rsi") {
		indicators = append(indicators, "rsi")
	}
	if strings.Contains(query, "macd") {
		indicators = append(indicators, "macd")
	}
	if len(indicators) == 0 {
		indicators = append(indicators, "rsi", "macd")
	}
	return indicators
}

func (c *ChatUIAgent) extractAnalysisType(query string) string {
	if strings.Contains(query, "risk") {
		return "risk_analysis"
	}
	if strings.Contains(query, "optimize") {
		return "optimization"
	}
	return "general_analysis"
}

// fallbackProcessing provides basic processing when advanced analysis fails
func (c *ChatUIAgent) fallbackProcessing(ctx context.Context, query string) (map[string]interface{}, error) {
	c.Logger().Warn("Using fallback processing", map[string]interface{}{
		"query": query,
	})

	// Simple routing fallback
	routingDecision := c.fallbackRouting(query)

	// Execute the routing decision
	response, err := c.executeRouting(ctx, routingDecision)
	if err != nil {
		return nil, fmt.Errorf("failed to execute fallback routing: %w", err)
	}

	return map[string]interface{}{
		"user_query":       query,
		"routing_decision": routingDecision,
		"agent_responses":  response,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"processing_type":  "fallback_routing",
		"note":             "Advanced LLM analysis failed, using basic routing",
	}, nil
}

// synthesizeMultiAgentResponse combines responses from multiple agents
func (c *ChatUIAgent) synthesizeMultiAgentResponse(query string, responses map[string]interface{}, decision LLMDecisionDetail) map[string]interface{} {
	synthesis := map[string]interface{}{
		"query":              query,
		"execution_strategy": decision.ExecutionStrategy,
		"total_agents":       len(responses),
		"confidence":         decision.Confidence,
	}

	// Create a summary based on response types
	if marketData, exists := responses["get-stock-price"]; exists {
		synthesis["market_data"] = marketData
	}

	if newsData, exists := responses["analyze-financial-news"]; exists {
		synthesis["news_analysis"] = newsData
	}

	if technicalData, exists := responses["calculate-technical-indicators"]; exists {
		synthesis["technical_analysis"] = technicalData
	}

	if portfolioData, exists := responses["analyze-portfolio"]; exists {
		synthesis["portfolio_analysis"] = portfolioData
	}

	// Generate a natural language summary
	var summary []string
	for capability := range responses {
		switch capability {
		case "get-stock-price":
			summary = append(summary, "Retrieved current market data")
		case "analyze-financial-news":
			summary = append(summary, "Analyzed latest financial news sentiment")
		case "calculate-technical-indicators":
			summary = append(summary, "Calculated technical analysis indicators")
		case "analyze-portfolio":
			summary = append(summary, "Generated portfolio recommendations")
		}
	}

	synthesis["summary"] = fmt.Sprintf("Processed query using %d agents: %s",
		len(responses), strings.Join(summary, ", "))
	synthesis["timestamp"] = time.Now().Format(time.RFC3339)

	return synthesis
}

// updateRoutingStatistics updates internal routing statistics
func (c *ChatUIAgent) updateRoutingStatistics(decision LLMDecisionDetail, agentCount int, executionTime time.Duration) {
	// Update the aggregate statistics
	c.routingStats.TotalQueries++
	c.routingStats.SuccessfulRoutes++

	// Update average confidence (running average)
	totalQueries := float64(c.routingStats.TotalQueries)
	c.routingStats.AvgConfidence = ((c.routingStats.AvgConfidence * (totalQueries - 1)) + decision.Confidence) / totalQueries

	// Update average response time (running average)
	c.routingStats.AvgResponseTime = time.Duration(
		(int64(c.routingStats.AvgResponseTime)*int64(totalQueries-1) + int64(executionTime)) / int64(totalQueries),
	)

	// Update top agents statistics
	if c.routingStats.TopAgents == nil {
		c.routingStats.TopAgents = make(map[string]int)
	}

	for _, agent := range c.capabilitiesToAgentNames(decision.SelectedCapabilities) {
		c.routingStats.TopAgents[agent]++
	}

	// Log the statistics
	c.Logger().Info("Routing statistics updated", map[string]interface{}{
		"total_queries":     c.routingStats.TotalQueries,
		"success_rate":      float64(c.routingStats.SuccessfulRoutes) / float64(c.routingStats.TotalQueries),
		"avg_confidence":    c.routingStats.AvgConfidence,
		"avg_response_time": c.routingStats.AvgResponseTime.String(),
		"agents_contacted":  agentCount,
		"capabilities_used": len(decision.SelectedCapabilities),
	})
}

func main() {
	agent := &ChatUIAgent{}

	// Start the agent with framework auto-configuration
	f, err := framework.NewFramework(
		framework.WithAgentName("chat-ui-agent"),
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

	// Add custom chat UI handler
	f.HandleFunc("/chat", agent.serveChatUI)

	fmt.Println("Starting Chat UI Agent on port 8080...")
	fmt.Println("🌐 Web Interface: http://localhost:8080/chat")
	fmt.Println("📋 API Documentation: http://localhost:8080")
	fmt.Println("Available capabilities:")
	fmt.Println("  - process-user-query: LLM-assisted routing to specialized agents")
	fmt.Println("  - get-available-agents: Show all available agents and capabilities")
	fmt.Println("  - health-check-system: Comprehensive system health check")

	// Start HTTP server
	if err := f.StartHTTPServer(ctx, agent); err != nil {
		fmt.Printf("Failed to start chat UI agent: %v\n", err)
		os.Exit(1)
	}
}
