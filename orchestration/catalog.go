package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// AgentCatalog maintains a local cache of all agents and their capabilities.
// It periodically refreshes from the Redis discovery service to stay up-to-date
// with the current agent ecosystem. The catalog is thread-safe for concurrent access.
type AgentCatalog struct {
	// agents maps agent IDs to their complete information including capabilities
	agents map[string]*AgentInfo
	// capabilityIndex provides fast lookup of agents by capability name
	capabilityIndex map[string][]string // capability -> [agent_ids]
	// mu protects concurrent access to the catalog
	mu sync.RWMutex
	// discovery is the service discovery interface (typically Redis)
	discovery core.Discovery
	// httpClient is used to fetch capability details from agents
	httpClient *http.Client
}

// AgentInfo contains complete information about an agent
type AgentInfo struct {
	Registration *core.ServiceRegistration
	Capabilities []EnhancedCapability
	LastUpdated  time.Time
}

// EnhancedCapability extends the basic capability with detailed metadata
type EnhancedCapability struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Endpoint    string      `json:"endpoint"`
	Parameters  []Parameter `json:"parameters"`
	Returns     ReturnType  `json:"returns"`
	Tags        []string    `json:"tags"`
	Examples    []Example   `json:"examples,omitempty"`
}

// Parameter describes an input parameter
type Parameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

// ReturnType describes what the capability returns
type ReturnType struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Schema      string `json:"schema,omitempty"`
}

// Example shows how to use a capability
type Example struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output,omitempty"`
}

// NewAgentCatalog creates a new agent catalog
func NewAgentCatalog(discovery core.Discovery) *AgentCatalog {
	return &AgentCatalog{
		agents:          make(map[string]*AgentInfo),
		capabilityIndex: make(map[string][]string),
		discovery:       discovery,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Refresh updates the catalog from the discovery service.
// This method:
// 1. Queries discovery for all known agents
// 2. Fetches detailed capability information from each agent's /api/capabilities endpoint
// 3. Atomically updates the local catalog
// It should be called periodically to keep the catalog synchronized with the agent ecosystem.
func (c *AgentCatalog) Refresh(ctx context.Context) error {
	// Get all services from discovery
	// Note: We'll need to enhance discovery to support getting all services
	services, err := c.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	newAgents := make(map[string]*AgentInfo)
	newIndex := make(map[string][]string)

	// Fetch capabilities for each agent
	for _, service := range services {
		agentInfo, err := c.fetchAgentInfo(ctx, service)
		if err != nil {
			// Log error but continue with other agents
			continue
		}

		newAgents[service.ID] = agentInfo

		// Build capability index
		for _, cap := range agentInfo.Capabilities {
			newIndex[cap.Name] = append(newIndex[cap.Name], service.ID)
		}
	}

	// Atomic update
	c.mu.Lock()
	c.agents = newAgents
	c.capabilityIndex = newIndex
	c.mu.Unlock()

	return nil
}

// fetchAgentInfo fetches detailed capability information from an agent.
// It calls the agent's /api/capabilities endpoint to get enhanced metadata
// about what the agent can do, including parameter schemas and examples.
// If the endpoint is unavailable, it falls back to basic capability names from registration.
func (c *AgentCatalog) fetchAgentInfo(ctx context.Context, service *core.ServiceRegistration) (*AgentInfo, error) {
	// Call the agent's /api/capabilities endpoint
	url := fmt.Sprintf("http://%s:%d/api/capabilities", service.Address, service.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("Error closing response body: %v\n", closeErr)
		}
	}()

	var capabilities []EnhancedCapability
	if err := json.NewDecoder(resp.Body).Decode(&capabilities); err != nil {
		// Fallback to basic capabilities from registration
		capabilities = c.convertBasicCapabilities(service.Capabilities)
	}

	return &AgentInfo{
		Registration: service,
		Capabilities: capabilities,
		LastUpdated:  time.Now(),
	}, nil
}

// convertBasicCapabilities converts simple capability names to enhanced format
func (c *AgentCatalog) convertBasicCapabilities(caps []string) []EnhancedCapability {
	enhanced := make([]EnhancedCapability, len(caps))
	for i, cap := range caps {
		enhanced[i] = EnhancedCapability{
			Name:        cap,
			Description: fmt.Sprintf("Capability: %s", cap),
			Endpoint:    fmt.Sprintf("/api/%s", cap),
		}
	}
	return enhanced
}

// getAllServices gets all services from discovery
// This is a workaround since core.Discovery doesn't have a GetAll method
func (c *AgentCatalog) getAllServices(ctx context.Context) ([]*core.ServiceRegistration, error) {
	// For MVP, we'll use a known list of agent names
	// In production, this should be enhanced in core.Discovery
	knownAgents := []string{
		"orchestrator",
		"stock-analyzer",
		"market-data",
		"news-agent",
		"portfolio-advisor",
		"technical-analyst",
	}

	var allServices []*core.ServiceRegistration
	for _, name := range knownAgents {
		services, err := c.discovery.FindService(ctx, name)
		if err != nil {
			continue
		}
		allServices = append(allServices, services...)
	}

	return allServices, nil
}

// GetAgents returns all agents in the catalog
func (c *AgentCatalog) GetAgents() map[string]*AgentInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	agents := make(map[string]*AgentInfo)
	for k, v := range c.agents {
		agents[k] = v
	}
	return agents
}

// GetAgent returns a specific agent by ID
func (c *AgentCatalog) GetAgent(agentID string) *AgentInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agents[agentID]
}

// FindByCapability returns agents that have a specific capability
func (c *AgentCatalog) FindByCapability(capability string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilityIndex[capability]
}

// FormatForLLM formats the catalog for LLM consumption.
// This creates a human-readable text representation of all agents and their capabilities
// that can be included in prompts to LLMs for intelligent orchestration decisions.
// The format includes agent names, endpoints, capability descriptions, parameters, and return types.
func (c *AgentCatalog) FormatForLLM() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var output string
	output = "Available Agents and Capabilities:\n\n"

	for id, agent := range c.agents {
		output += fmt.Sprintf("Agent: %s (ID: %s)\n", agent.Registration.Name, id)
		output += fmt.Sprintf("  Address: http://%s:%d\n", agent.Registration.Address, agent.Registration.Port)

		for _, cap := range agent.Capabilities {
			output += fmt.Sprintf("  - Capability: %s\n", cap.Name)
			output += fmt.Sprintf("    Description: %s\n", cap.Description)

			if len(cap.Parameters) > 0 {
				output += "    Parameters:\n"
				for _, param := range cap.Parameters {
					required := ""
					if param.Required {
						required = " (required)"
					}
					output += fmt.Sprintf("      - %s: %s%s - %s\n",
						param.Name, param.Type, required, param.Description)
				}
			}

			if cap.Returns.Type != "" {
				output += fmt.Sprintf("    Returns: %s - %s\n",
					cap.Returns.Type, cap.Returns.Description)
			}
		}
		output += "\n"
	}

	return output
}
