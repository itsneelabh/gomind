package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/pkg/capabilities"
	"github.com/itsneelabh/gomind/pkg/communication"
)

// Agent is the core interface that all agents must implement
// Framework provides zero-config auto-discovery and instrumentation
type Agent interface {
	// Optional: Initialize is called during agent startup
	// Framework provides auto-injection of dependencies via struct tags
	Initialize(ctx context.Context) error
}

// ProcessRequestAgent is an optional interface for agents that can process natural language requests
// Implementing this interface enables the agent to receive inter-agent communication
type ProcessRequestAgent interface {
	Agent
	// ProcessRequest handles natural language instructions from other agents
	// Returns a natural language response
	ProcessRequest(ctx context.Context, instruction string) (string, error)
}

// BaseAgent provides default implementation and framework integration
type BaseAgent struct {
	agentID      string
	capabilities []capabilities.CapabilityMetadata
	telemetry    AutoOTEL
	discovery    Discovery
	memory       Memory
	logger       Logger
	aiClient     AIClient // For LLM integration
	
	// Inter-agent communication
	communicator communication.AgentCommunicator
}

// GetAgentID returns the unique identifier for this agent
func (b *BaseAgent) GetAgentID() string {
	return b.agentID
}

// GetCapabilities returns the discovered capabilities for this agent
func (b *BaseAgent) GetCapabilities() []capabilities.CapabilityMetadata {
	return b.capabilities
}

// Initialize provides default initialization
// Override this method for custom initialization logic
func (b *BaseAgent) Initialize(ctx context.Context) error {
	return nil
}

// Memory returns the auto-configured memory interface
func (b *BaseAgent) Memory() Memory {
	return b.memory
}

// Logger returns the auto-configured logger
func (b *BaseAgent) Logger() Logger {
	return b.logger
}

// Telemetry returns the auto-configured OTEL telemetry
func (b *BaseAgent) Telemetry() AutoOTEL {
	return b.telemetry
}

// Discovery returns the service discovery interface
func (b *BaseAgent) Discovery() Discovery {
	return b.discovery
}

// AskAgent sends a natural language instruction to another agent and returns the response
// This is a convenience method for inter-agent communication
func (b *BaseAgent) AskAgent(agentIdentifier string, instruction string) string {
	ctx := context.Background()
	
	if b.communicator == nil {
		b.logger.Error("Agent communicator not initialized", map[string]interface{}{
			"agent": agentIdentifier,
		})
		return "Error: Agent communication not available"
	}
	
	response, err := b.communicator.CallAgent(ctx, agentIdentifier, instruction)
	if err != nil {
		b.logger.Error("Failed to call agent", map[string]interface{}{
			"target_agent": agentIdentifier,
			"error":        err.Error(),
		})
		return fmt.Sprintf("Error calling %s: %v", agentIdentifier, err)
	}
	
	return response
}

// AskAgentWithTimeout sends an instruction to another agent with a custom timeout
func (b *BaseAgent) AskAgentWithTimeout(agentIdentifier string, instruction string, timeout time.Duration) string {
	ctx := context.Background()
	
	if b.communicator == nil {
		b.logger.Error("Agent communicator not initialized", map[string]interface{}{
			"agent": agentIdentifier,
		})
		return "Error: Agent communication not available"
	}
	
	response, err := b.communicator.CallAgentWithTimeout(ctx, agentIdentifier, instruction, timeout)
	if err != nil {
		b.logger.Error("Failed to call agent with timeout", map[string]interface{}{
			"target_agent": agentIdentifier,
			"timeout":      timeout.String(),
			"error":        err.Error(),
		})
		return fmt.Sprintf("Error calling %s: %v", agentIdentifier, err)
	}
	
	return response
}

// SetCommunicator sets the agent communicator for inter-agent communication
func (b *BaseAgent) SetCommunicator(comm communication.AgentCommunicator) {
	b.communicator = comm
}

// GetCommunicator returns the agent communicator
func (b *BaseAgent) GetCommunicator() communication.AgentCommunicator {
	return b.communicator
}

// GetAvailableAgents returns a list of available agents from the discovery service
func (b *BaseAgent) GetAvailableAgents() []communication.AgentInfo {
	if b.communicator == nil {
		b.logger.Warn("Agent communicator not initialized", nil)
		return []communication.AgentInfo{}
	}
	
	ctx := context.Background()
	agents, err := b.communicator.GetAvailableAgents(ctx)
	if err != nil {
		b.logger.Error("Failed to get available agents", map[string]interface{}{
			"error": err.Error(),
		})
		return []communication.AgentInfo{}
	}
	
	return agents
}

// CapabilityMetadata is now defined in the capabilities package
// We use type alias for backward compatibility
type CapabilityMetadata = capabilities.CapabilityMetadata

// Legacy struct definition commented out for reference
/*
type CapabilityMetadata struct {
	// Core Capability Identity
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Domain      string `json:"domain" yaml:"domain"`

	// Performance Characteristics
	Complexity      string  `json:"complexity" yaml:"complexity"`
	Latency         string  `json:"latency" yaml:"latency"`
	Cost            string  `json:"cost" yaml:"cost"`
	ConfidenceLevel float64 `json:"confidence_level" yaml:"confidence_level"`

	// Business Context (Enhanced for Autonomous AI)
	BusinessValue  []string        `json:"business_value" yaml:"business_value"`
	BusinessImpact *BusinessImpact `json:"business_impact,omitempty" yaml:"business_impact,omitempty"`
	UseCases       []string        `json:"use_cases" yaml:"use_cases"`
	QualityMetrics *QualityMetrics `json:"quality_metrics,omitempty" yaml:"quality_metrics,omitempty"`

	// LLM-Friendly Fields for Agentic Communication
	LLMPrompt   string   `json:"llm_prompt,omitempty" yaml:"llm_prompt,omitempty"`
	Specialties []string `json:"specialties,omitempty" yaml:"specialties,omitempty"`

	// Technical Requirements
	Prerequisites []string              `json:"prerequisites" yaml:"prerequisites"`
	Dependencies  []string              `json:"dependencies" yaml:"dependencies"`
	InputTypes    []string              `json:"input_types" yaml:"input_types"`
	OutputFormats []string              `json:"output_formats" yaml:"output_formats"`
	ResourceReqs  *ResourceRequirements `json:"resource_requirements,omitempty" yaml:"resource_requirements,omitempty"`

	// Capability Relationships
	ComplementaryTo []string `json:"complementary_to" yaml:"complementary_to"`
	AlternativeTo   []string `json:"alternative_to" yaml:"alternative_to"`

	// Autonomous Operation Support
	AutomationLevel string       `json:"automation_level" yaml:"automation_level"` // "manual", "semi-auto", "autonomous"
	RiskProfile     *RiskProfile `json:"risk_profile,omitempty" yaml:"risk_profile,omitempty"`

	// Metadata Source Tracking
	Source *MetadataSource `json:"source,omitempty" yaml:"source,omitempty"`

	// Framework Integration (existing)
	Method reflect.Method    `json:"-" yaml:"-"`
	Tags   map[string]string `json:"tags" yaml:"tags"`
}
*/

// Type aliases for supporting types from capabilities package
type BusinessImpact = capabilities.BusinessImpact
type QualityMetrics = capabilities.QualityMetrics
type ResourceRequirements = capabilities.ResourceRequirements  
type RiskProfile = capabilities.RiskProfile
type MetadataSource = capabilities.MetadataSource

// Legacy struct definitions commented out for reference
/*
// BusinessImpact represents the business impact assessment
type BusinessImpact struct {
	Criticality    string   `json:"criticality" yaml:"criticality"`       // "low", "medium", "high", "critical"
	RevenueImpact  string   `json:"revenue_impact" yaml:"revenue_impact"` // "positive", "neutral", "negative"
	CustomerFacing bool     `json:"customer_facing" yaml:"customer_facing"`
	Stakeholders   []string `json:"stakeholders" yaml:"stakeholders"`
}

// QualityMetrics represents quality and reliability metrics
type QualityMetrics struct {
	Accuracy     float64 `json:"accuracy" yaml:"accuracy"`           // 0.0 - 1.0
	Reliability  float64 `json:"reliability" yaml:"reliability"`     // 0.0 - 1.0
	ResponseTime string  `json:"response_time" yaml:"response_time"` // "fast", "medium", "slow"
	Throughput   string  `json:"throughput" yaml:"throughput"`       // "low", "medium", "high"
	ErrorRate    float64 `json:"error_rate" yaml:"error_rate"`       // 0.0 - 1.0
}

// ResourceRequirements represents computational resource needs
type ResourceRequirements struct {
	CPU    string `json:"cpu" yaml:"cpu"`       // "low", "medium", "high"
	Memory string `json:"memory" yaml:"memory"` // "low", "medium", "high"
	IO     string `json:"io" yaml:"io"`         // "low", "medium", "high"
}

// RiskProfile represents operational risk assessment
type RiskProfile struct {
	DataSensitivity string   `json:"data_sensitivity" yaml:"data_sensitivity"` // "public", "internal", "confidential", "restricted"
	OperationalRisk string   `json:"operational_risk" yaml:"operational_risk"` // "low", "medium", "high"
	ComplianceReqs  []string `json:"compliance_requirements" yaml:"compliance_requirements"`
}

// MetadataSource tracks where metadata originated
type MetadataSource struct {
	Type        string `json:"type" yaml:"type"` // "comment", "yaml", "merged"
	File        string `json:"file" yaml:"file"`
	Line        int    `json:"line" yaml:"line"`
	LastUpdated string `json:"last_updated" yaml:"last_updated"`
}
*/

// ConversationalAgent interface for agents that handle conversations
type ConversationalAgent interface {
	Agent
	HandleConversation(ctx context.Context, message Message) (Response, error)
}

// StreamingConversationalAgent interface for real-time conversation streaming
type StreamingConversationalAgent interface {
	Agent
	HandleConversationStream(ctx context.Context, message Message) (<-chan StreamResponse, error)
}

// Message represents an incoming conversational message
type Message struct {
	Text      string                 `json:"text"`
	SessionID string                 `json:"session_id"`
	UserID    string                 `json:"user_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	Media     []MediaAttachment      `json:"media,omitempty"`
}

// Response represents a conversational response
type Response struct {
	Text         string                 `json:"text"`
	Type         ResponseType           `json:"type"`
	QuickReplies []string               `json:"quick_replies,omitempty"`
	Actions      []Action               `json:"actions,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// StreamResponse represents a streaming conversational response
type StreamResponse struct {
	Text     string                 `json:"text"`
	Type     ResponseType           `json:"type"`
	Delta    string                 `json:"delta,omitempty"`
	Complete bool                   `json:"complete"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ResponseType defines the type of conversational response
type ResponseType string

const (
	ResponseTypeText     ResponseType = "text"
	ResponseTypeTyping   ResponseType = "typing"
	ResponseTypeProgress ResponseType = "progress"
	ResponseTypeComplete ResponseType = "complete"
	ResponseTypeError    ResponseType = "error"
)

// =============================================================================
// Agent Communication Helper Methods
// These methods enable simple agent-to-agent communication using Redis discovery
// =============================================================================

// FindAgents finds other agents that provide a specific capability
// Returns a list of healthy agents that can handle the requested capability
func (b *BaseAgent) FindAgents(ctx context.Context, capability string) ([]AgentRegistration, error) {
	// Query Redis discovery for agents with this capability
	agents, err := b.discovery.FindCapability(ctx, capability)
	if err != nil {
		b.logger.Error("Failed to find agents", "capability", capability, "error", err)
		return nil, err
	}

	// Filter for healthy agents only
	var healthy []AgentRegistration
	for _, agent := range agents {
		if agent.Status == StatusHealthy {
			healthy = append(healthy, agent)
		}
	}

	b.logger.Debug("Found agents for capability", "capability", capability, "count", len(healthy))
	return healthy, nil
}

// SendMessage sends a natural language message to another agent via HTTP
// Returns the agent's response as a string
func (b *BaseAgent) SendMessage(ctx context.Context, target AgentRegistration, message string) (string, error) {
	// Build the target URL from discovery information
	url := fmt.Sprintf("http://%s:%d/api/message", target.Address, target.Port)

	// Create message request
	request := map[string]interface{}{
		"from":    b.agentID,
		"message": message,
		"type":    "text",
	}

	// Send HTTP POST request (implementation would use actual HTTP client)
	// This is a simplified version - real implementation would handle HTTP details
	response, err := b.sendHTTPRequest(ctx, url, request)
	if err != nil {
		b.logger.Error("Failed to send message to agent",
			"target", target.ID,
			"url", url,
			"error", err)
		return "", err
	}

	b.logger.Debug("Sent message to agent",
		"target", target.ID,
		"message", message,
		"response", response)

	return response, nil
}

// AskForHelp is a high-level helper that finds an agent and sends a message
// This combines discovery and communication in one convenient method
func (b *BaseAgent) AskForHelp(ctx context.Context, capability, question string) (string, error) {
	// Find agents with the required capability
	agents, err := b.FindAgents(ctx, capability)
	if err != nil {
		return "", err
	}

	if len(agents) == 0 {
		return "", fmt.Errorf("no agents found with capability: %s", capability)
	}

	// Use the first available agent (could be enhanced with selection logic)
	selectedAgent := agents[0]

	// Send the question to the selected agent
	response, err := b.SendMessage(ctx, selectedAgent, question)
	if err != nil {
		return "", err
	}

	return response, nil
}

// =============================================================================
// AI-AGENTIC COMMUNICATION IMPLEMENTATION
// =============================================================================

// askLLM provides LLM integration for autonomous agent decision-making
func (b *BaseAgent) askLLM(ctx context.Context, prompt string) (string, error) {
	if b.aiClient == nil {
		return "", fmt.Errorf("AI client not configured - set OPENAI_API_KEY or ANTHROPIC_API_KEY")
	}

	options := &GenerationOptions{
		Model:       "gpt-4",
		Temperature: 0.3, // Lower temperature for more deterministic agent decisions
		MaxTokens:   1000,
		SystemPrompt: fmt.Sprintf(`You are an AI assistant helping agent "%s" make decisions about contacting other agents.
Always respond with clear, actionable information that helps autonomous agent coordination.
Focus on practical agent communication and capability matching.`, b.agentID),
	}

	response, err := b.aiClient.GenerateResponse(ctx, prompt, options)
	if err != nil {
		b.logger.Error("LLM request failed", "error", err, "agent_id", b.agentID)
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	b.logger.Debug("LLM response received", "agent_id", b.agentID, "response_length", len(response.Content))
	return response.Content, nil
}

// GenerateAgentCatalog creates an LLM-friendly directory of all available agents
func (b *BaseAgent) GenerateAgentCatalog(ctx context.Context) (string, error) {
	if b.discovery == nil {
		return "", fmt.Errorf("discovery service not available")
	}

	// For autonomous mode, we discover ALL agents by querying multiple capabilities
	// This is a simplified approach - in production, we might maintain a capability index
	commonCapabilities := []string{
		"process", "analyze", "calculate", "generate", "manage", "monitor",
		"financial-analysis", "data-processing", "chat", "trading", "risk-assessment",
	}

	allAgents := make(map[string]*AgentRegistration)

	// Query each capability to build comprehensive agent catalog
	for _, capability := range commonCapabilities {
		agents, err := b.discovery.FindCapability(ctx, capability)
		if err != nil {
			b.logger.Warn("Failed to query capability during catalog generation",
				"capability", capability, "error", err)
			continue
		}

		for _, agent := range agents {
			// Avoid duplicates by using agent ID as key
			if agent.ID != b.agentID { // Don't include self
				allAgents[agent.ID] = &agent
			}
		}
	}

	if len(allAgents) == 0 {
		return "No other agents currently available in the network.", nil
	}

	// Build human-readable catalog for LLM consumption
	catalog := "Available Agents in Your Network:\n\n"

	index := 1
	for _, agent := range allAgents {
		catalog += fmt.Sprintf("%d. %s (%s)\n", index, agent.Name, agent.ID)

		// Add capabilities in human-readable format
		if len(agent.Capabilities) > 0 {
			var capabilityNames []string
			var specialties []string
			var businessValue []string
			var llmPrompts []string

			for _, cap := range agent.Capabilities {
				capabilityNames = append(capabilityNames, cap.Name)

				// Use LLM-specific Specialties field if available, fallback to Description
				if len(cap.Specialties) > 0 {
					specialties = append(specialties, strings.Join(cap.Specialties, ", "))
				} else if cap.Description != "" {
					specialties = append(specialties, cap.Description)
				}

				if len(cap.BusinessValue) > 0 {
					businessValue = append(businessValue, strings.Join(cap.BusinessValue, ", "))
				}

				// Add LLM prompts for better agent interaction
				if cap.LLMPrompt != "" {
					llmPrompts = append(llmPrompts, cap.LLMPrompt)
				}
			}

			catalog += fmt.Sprintf("   - Capabilities: %s\n", strings.Join(capabilityNames, ", "))

			// Use LLM-optimized specialties
			if len(specialties) > 0 {
				catalog += fmt.Sprintf("   - Specializes in: %s\n", strings.Join(specialties, "; "))
			}

			// Add LLM interaction guidance
			if len(llmPrompts) > 0 {
				catalog += fmt.Sprintf("   - How to interact: %s\n", strings.Join(llmPrompts, "; "))
			}

			if len(businessValue) > 0 {
				catalog += fmt.Sprintf("   - Business Value: %s\n", strings.Join(businessValue, ", "))
			}

			// Add performance indicators if available
			for _, cap := range agent.Capabilities {
				// Use available fields from discovery.CapabilityMetadata
				catalog += fmt.Sprintf("   - Response Time: %s | Cost: %s | Confidence: %.2f\n",
					cap.Latency,
					cap.Cost,
					cap.ConfidenceLevel)
				break // Only show once per agent
			}
		}

		catalog += fmt.Sprintf("   - Address: %s:%d\n\n", agent.Address, agent.Port)
		index++
	}

	return catalog, nil
}

// buildCollaboratorCatalog creates a catalog for pre-defined agent workflows
func (b *BaseAgent) buildCollaboratorCatalog(ctx context.Context, agentNames []string) (string, error) {
	if b.discovery == nil {
		return "", fmt.Errorf("discovery service not available")
	}

	catalog := "Your Available Collaborator Agents:\n\n"
	found := 0

	for _, agentName := range agentNames {
		// Try to find agent by name or ID
		agent, err := b.discovery.FindAgent(ctx, agentName)
		if err != nil {
			b.logger.Warn("Could not find collaborator agent", "agent_name", agentName, "error", err)
			continue
		}

		found++
		catalog += fmt.Sprintf("%d. %s (%s)\n", found, agent.Name, agent.ID)

		// Add capabilities
		if len(agent.Capabilities) > 0 {
			var capabilityNames []string
			for _, cap := range agent.Capabilities {
				capabilityNames = append(capabilityNames, cap.Name)
			}
			catalog += fmt.Sprintf("   - Capabilities: %s\n", strings.Join(capabilityNames, ", "))
		}

		catalog += fmt.Sprintf("   - Address: %s:%d\n\n", agent.Address, agent.Port)
	}

	if found == 0 {
		return "None of your specified collaborator agents are currently available.", nil
	}

	return catalog, nil
}

// ContactAgent provides intelligent agent resolution and communication
func (b *BaseAgent) ContactAgent(ctx context.Context, agentName, query string) (string, error) {
	if b.discovery == nil {
		return "", fmt.Errorf("discovery service not available")
	}

	// Step 1: Resolve agent name to service address with fuzzy matching
	agent, err := b.resolveAgentName(ctx, agentName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve agent '%s': %w", agentName, err)
	}

	// Step 2: Send natural language query to resolved agent
	response, err := b.SendMessage(ctx, *agent, query)
	if err != nil {
		return "", fmt.Errorf("failed to contact agent '%s': %w", agentName, err)
	}

	b.logger.Info("Successfully contacted agent",
		"agent_name", agentName,
		"resolved_id", agent.ID,
		"response_length", len(response))

	return response, nil
}

// resolveAgentName performs intelligent agent name resolution with fuzzy matching
func (b *BaseAgent) resolveAgentName(ctx context.Context, agentName string) (*AgentRegistration, error) {
	// Try exact match first by ID
	if agent, err := b.discovery.FindAgent(ctx, agentName); err == nil {
		return agent, nil
	}

	// Try fuzzy matching against names and IDs
	// For simplicity, we'll query common capabilities and match names
	commonCapabilities := []string{"process", "analyze", "manage", "generate"}

	for _, capability := range commonCapabilities {
		agents, err := b.discovery.FindCapability(ctx, capability)
		if err != nil {
			continue
		}

		for _, agent := range agents {
			// Case-insensitive partial matching
			agentNameLower := strings.ToLower(agentName)
			agentIDLower := strings.ToLower(agent.ID)
			agentActualNameLower := strings.ToLower(agent.Name)

			if strings.Contains(agentIDLower, agentNameLower) ||
				strings.Contains(agentActualNameLower, agentNameLower) ||
				strings.Contains(agentNameLower, agentIDLower) {
				return &agent, nil
			}
		}
	}

	return nil, fmt.Errorf("no agent found matching name '%s'", agentName)
}

// ProcessLLMDecision is a helper method for processing LLM decisions in agent workflows
func (b *BaseAgent) ProcessLLMDecision(ctx context.Context, userRequest, agentCatalog string) (string, error) {
	// Create comprehensive prompt for LLM decision making
	prompt := fmt.Sprintf(`
User Request: "%s"

Available Agents:
%s

Based on the user request and available agents, determine:
1. Which agent(s) would be most helpful
2. What specific question(s) to ask them

Respond in this format:
AGENT: <agent_name>
QUERY: <specific_question_to_ask>

If multiple agents are needed, use multiple AGENT/QUERY pairs.
If you can answer directly without other agents, respond with:
RESPONSE: <your_direct_response>
`, userRequest, agentCatalog)

	// Get LLM decision
	decision, err := b.askLLM(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to get LLM decision: %w", err)
	}

	// Parse LLM decision and execute
	return b.executeLLMDecision(ctx, decision, userRequest)
}

// executeLLMDecision parses and executes the LLM's agent contact decision
func (b *BaseAgent) executeLLMDecision(ctx context.Context, decision, originalRequest string) (string, error) {
	lines := strings.Split(decision, "\n")

	var responses []string
	var directResponse string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for direct response
		if strings.HasPrefix(line, "RESPONSE:") {
			directResponse = strings.TrimSpace(strings.TrimPrefix(line, "RESPONSE:"))
			continue
		}

		// Check for agent contact
		if strings.HasPrefix(line, "AGENT:") {
			agentName := strings.TrimSpace(strings.TrimPrefix(line, "AGENT:"))

			// Look for corresponding QUERY line
			var query string
			for i, nextLine := range lines {
				if strings.TrimSpace(nextLine) == line {
					// Check if next line has QUERY
					if i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "QUERY:") {
						query = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i+1]), "QUERY:"))
						break
					}
				}
			}

			if query == "" {
				query = originalRequest // Fallback to original request
			}

			// Contact the agent
			response, err := b.ContactAgent(ctx, agentName, query)
			if err != nil {
				b.logger.Warn("Failed to contact agent from LLM decision",
					"agent", agentName, "error", err)
				responses = append(responses, fmt.Sprintf("Failed to contact %s: %s", agentName, err.Error()))
			} else {
				responses = append(responses, fmt.Sprintf("Response from %s: %s", agentName, response))
			}
		}
	}

	// Return direct response if provided
	if directResponse != "" {
		return directResponse, nil
	}

	// Return collected responses
	if len(responses) > 0 {
		return strings.Join(responses, "\n\n"), nil
	}

	return "No clear action determined from LLM decision.", nil
}

// =============================================================================

// sendHTTPRequest is a placeholder for actual HTTP communication
// In real implementation, this would use the framework's HTTP client
func (b *BaseAgent) sendHTTPRequest(ctx context.Context, url string, request map[string]interface{}) (string, error) {
	// Placeholder implementation
	// Real version would marshal request to JSON, send HTTP POST, parse response
	return "Mock response from agent", nil
}

// =============================================================================

// Action represents an action that can be taken in conversation
type Action struct {
	Type    string                 `json:"type"`
	Label   string                 `json:"label"`
	Payload map[string]interface{} `json:"payload"`
}

// MediaAttachment represents media content in messages
type MediaAttachment struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}
