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

	// Observability (follows framework design principles)
	logger core.Logger // For structured logging
}

// AgentInfo contains complete information about an agent
type AgentInfo struct {
	Registration *core.ServiceInfo // Updated to use ServiceInfo
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

	// Internal marks capabilities that should not be exposed to LLM planning.
	// Internal capabilities are still callable via HTTP but are excluded from
	// the service catalog used for AI orchestration decisions.
	Internal bool `json:"internal,omitempty"`
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

// SetLogger sets the logger provider (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (c *AgentCatalog) SetLogger(logger core.Logger) {
	if logger == nil {
		c.logger = &core.NoOpLogger{}
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			c.logger = cal.WithComponent("framework/orchestration")
		} else {
			c.logger = logger
		}
	}
}

// Refresh updates the catalog from the discovery service.
// This method:
// 1. Queries discovery for all known agents
// 2. Fetches detailed capability information from each agent's /api/capabilities endpoint
// 3. Atomically updates the local catalog
// It should be called periodically to keep the catalog synchronized with the agent ecosystem.
func (c *AgentCatalog) Refresh(ctx context.Context) error {
	refreshStart := time.Now()

	c.mu.RLock()
	currentAgentCount := len(c.agents)
	c.mu.RUnlock()

	if c.logger != nil {
		c.logger.Info("Starting catalog refresh", map[string]interface{}{
			"operation":      "catalog_refresh_start",
			"current_agents": currentAgentCount,
		})
	}

	// Get all services from discovery
	// Note: We'll need to enhance discovery to support getting all services
	services, err := c.getAllServices(ctx)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("Failed to get services from discovery", map[string]interface{}{
				"operation":   "discovery_query",
				"error":       err.Error(),
				"duration_ms": time.Since(refreshStart).Milliseconds(),
			})
		}
		return fmt.Errorf("failed to get services: %w", err)
	}

	if c.logger != nil {
		c.logger.Debug("Services discovered successfully", map[string]interface{}{
			"operation":      "discovery_query",
			"services_found": len(services),
			"query_time_ms":  time.Since(refreshStart).Milliseconds(),
		})
	}

	newAgents := make(map[string]*AgentInfo)
	newIndex := make(map[string][]string)
	successfulFetches := 0
	failedFetches := 0

	// Fetch capabilities for each agent
	for _, service := range services {
		agentFetchStart := time.Now()

		if c.logger != nil {
			c.logger.Debug("Fetching agent capabilities", map[string]interface{}{
				"operation":    "fetch_agent_info",
				"service_id":   service.ID,
				"service_name": service.Name,
				"address":      service.Address,
			})
		}

		agentInfo, err := c.fetchAgentInfo(ctx, service)
		if err != nil {
			failedFetches++
			if c.logger != nil {
				c.logger.Warn("Failed to fetch agent capabilities", map[string]interface{}{
					"operation":     "fetch_agent_info",
					"service_id":    service.ID,
					"service_name":  service.Name,
					"error":         err.Error(),
					"fetch_time_ms": time.Since(agentFetchStart).Milliseconds(),
				})
			}
			// Log error but continue with other agents
			continue
		}

		successfulFetches++
		newAgents[service.ID] = agentInfo

		if c.logger != nil {
			c.logger.Debug("Agent capabilities fetched successfully", map[string]interface{}{
				"operation":          "fetch_agent_info",
				"service_id":         service.ID,
				"service_name":       service.Name,
				"capabilities_count": len(agentInfo.Capabilities),
				"fetch_time_ms":      time.Since(agentFetchStart).Milliseconds(),
			})
		}

		// Build capability index
		for _, cap := range agentInfo.Capabilities {
			newIndex[cap.Name] = append(newIndex[cap.Name], service.ID)
		}
	}

	if c.logger != nil {
		c.logger.Debug("Capability index built", map[string]interface{}{
			"operation":           "build_capability_index",
			"unique_capabilities": len(newIndex),
			"agents_indexed":      len(newAgents),
		})
	}

	// Atomic update - Use currentAgentCount to avoid race condition
	c.mu.Lock()
	c.agents = newAgents
	c.capabilityIndex = newIndex
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Info("Catalog refresh completed", map[string]interface{}{
			"operation":           "catalog_refresh_complete",
			"success":             true,
			"total_duration_ms":   time.Since(refreshStart).Milliseconds(),
			"successful_fetches":  successfulFetches,
			"failed_fetches":      failedFetches,
			"final_agent_count":   len(newAgents),
			"agent_count_change":  len(newAgents) - currentAgentCount,
		})
	}

	return nil
}

// fetchAgentInfo fetches detailed capability information from an agent.
// It calls the agent's /api/capabilities endpoint to get enhanced metadata
// about what the agent can do, including parameter schemas and examples.
// If the endpoint is unavailable, it falls back to basic capability names from registration.
func (c *AgentCatalog) fetchAgentInfo(ctx context.Context, service *core.ServiceInfo) (*AgentInfo, error) {
	fetchStart := time.Now()
	// Call the agent's /api/capabilities endpoint
	url := fmt.Sprintf("http://%s:%d/api/capabilities", service.Address, service.Port)

	if c.logger != nil {
		c.logger.Debug("Making HTTP request for capabilities", map[string]interface{}{
			"operation":    "http_request_start",
			"service_id":   service.ID,
			"service_name": service.Name,
			"url":          url,
		})
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var capabilities []EnhancedCapability

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("HTTP request failed, using fallback capabilities", map[string]interface{}{
				"operation":       "http_request_fallback",
				"service_id":      service.ID,
				"error":           err.Error(),
				"request_time_ms": time.Since(fetchStart).Milliseconds(),
			})
		}
		// HTTP call failed, fallback to basic capabilities from registration
		capabilities = c.convertBasicCapabilities(service.Capabilities)
	} else {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				// Log error but don't fail the operation
				if c.logger != nil {
					c.logger.Warn("Error closing response body", map[string]interface{}{
						"operation":  "http_response_cleanup",
						"service_id": service.ID,
						"error":      closeErr.Error(),
					})
				}
			}
		}()

		if c.logger != nil {
			c.logger.Debug("HTTP response received", map[string]interface{}{
				"operation":       "http_response",
				"service_id":      service.ID,
				"status_code":     resp.StatusCode,
				"content_length":  resp.ContentLength,
				"request_time_ms": time.Since(fetchStart).Milliseconds(),
			})
		}

		if err := json.NewDecoder(resp.Body).Decode(&capabilities); err != nil {
			if c.logger != nil {
				c.logger.Warn("JSON decode failed, using fallback capabilities", map[string]interface{}{
					"operation":  "json_decode_fallback",
					"service_id": service.ID,
					"error":      err.Error(),
				})
			}
			// JSON decode failed, fallback to basic capabilities from registration
			capabilities = c.convertBasicCapabilities(service.Capabilities)
		} else {
			// HTTP succeeded - enrich capabilities with InputSummary from registration
			// This ensures LLM receives type information even when /api/capabilities
			// returns empty parameters (Phase 2 field hints from registration take precedence)
			capabilities = c.enrichCapabilitiesWithInputSummary(capabilities, service.Capabilities)
		}
	}

	return &AgentInfo{
		Registration: service,
		Capabilities: capabilities,
		LastUpdated:  time.Now(),
	}, nil
}

// convertBasicCapabilities converts simple capability names to enhanced format
// This function extracts Phase 2 field hints (InputSummary) and converts them
// to Parameters so the LLM receives proper type information for payload generation.
func (c *AgentCatalog) convertBasicCapabilities(caps []core.Capability) []EnhancedCapability {
	enhanced := make([]EnhancedCapability, len(caps))
	for i, cap := range caps {
		enhanced[i] = EnhancedCapability{
			Name:        cap.Name,
			Description: cap.Description,
			Endpoint:    cap.Endpoint,
			Internal:    cap.Internal, // Preserve internal flag for LLM filtering
		}
		// Use defaults if not set
		if enhanced[i].Endpoint == "" {
			enhanced[i].Endpoint = fmt.Sprintf("/api/%s", cap.Name)
		}
		if enhanced[i].Description == "" {
			enhanced[i].Description = fmt.Sprintf("Capability: %s", cap.Name)
		}

		// Convert Phase 2 field hints (InputSummary) to Parameters
		// This ensures the LLM receives type information for accurate payload generation
		if cap.InputSummary != nil {
			params := make([]Parameter, 0)

			// Add required fields
			for _, field := range cap.InputSummary.RequiredFields {
				params = append(params, Parameter{
					Name:        field.Name,
					Type:        field.Type,
					Required:    true,
					Description: field.Description,
					Default:     field.Example, // Use example as default hint
				})
			}

			// Add optional fields
			for _, field := range cap.InputSummary.OptionalFields {
				params = append(params, Parameter{
					Name:        field.Name,
					Type:        field.Type,
					Required:    false,
					Description: field.Description,
					Default:     field.Example,
				})
			}

			enhanced[i].Parameters = params
		}
	}
	return enhanced
}

// enrichCapabilitiesWithInputSummary merges InputSummary from service registration
// into capabilities fetched from HTTP endpoint when parameters are empty.
// This ensures the LLM receives type information even when /api/capabilities
// returns capabilities without parameter details.
func (c *AgentCatalog) enrichCapabilitiesWithInputSummary(httpCaps []EnhancedCapability, registrationCaps []core.Capability) []EnhancedCapability {
	// Build a lookup map from registration capabilities by name
	regCapMap := make(map[string]*core.Capability)
	for i := range registrationCaps {
		regCapMap[registrationCaps[i].Name] = &registrationCaps[i]
	}

	// Enrich each HTTP capability with InputSummary and Internal flag if available
	for i := range httpCaps {
		// Look up corresponding registration capability
		if regCap, ok := regCapMap[httpCaps[i].Name]; ok {
			// Propagate Internal flag from registration to HTTP capability
			// This ensures capabilities marked as internal are filtered in FormatForLLM
			if regCap.Internal {
				httpCaps[i].Internal = true
			}

			// Add parameters from InputSummary if not present in HTTP capability
			if len(httpCaps[i].Parameters) == 0 && regCap.InputSummary != nil {
				params := make([]Parameter, 0)

				// Add required fields
				for _, field := range regCap.InputSummary.RequiredFields {
					params = append(params, Parameter{
						Name:        field.Name,
						Type:        field.Type,
						Required:    true,
						Description: field.Description,
						Default:     field.Example,
					})
				}

				// Add optional fields
				for _, field := range regCap.InputSummary.OptionalFields {
					params = append(params, Parameter{
						Name:        field.Name,
						Type:        field.Type,
						Required:    false,
						Description: field.Description,
						Default:     field.Example,
					})
				}

				httpCaps[i].Parameters = params

				if c.logger != nil {
					c.logger.Debug("Enriched capability with InputSummary from registration", map[string]interface{}{
						"capability":       httpCaps[i].Name,
						"parameters_added": len(params),
					})
				}
			}
		}
	}

	return httpCaps
}

// getAllServices gets all services from discovery using the new Discover API
func (c *AgentCatalog) getAllServices(ctx context.Context) ([]*core.ServiceInfo, error) {
	// Use the new Discover method with empty filter to get all services
	return c.discovery.Discover(ctx, core.DiscoveryFilter{})
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
// Note: Internal capabilities (marked with Internal: true) are filtered out to prevent
// self-referential orchestration bugs where the LLM calls the orchestrator recursively.
func (c *AgentCatalog) FormatForLLM() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var output string
	output = "Available Agents and Capabilities:\n\n"

	for id, agent := range c.agents {
		// Filter out internal capabilities - only include public ones for LLM planning
		publicCaps := make([]EnhancedCapability, 0, len(agent.Capabilities))
		for _, cap := range agent.Capabilities {
			if !cap.Internal {
				publicCaps = append(publicCaps, cap)
			}
		}

		// Skip agents that have no public capabilities
		if len(publicCaps) == 0 {
			continue
		}

		output += fmt.Sprintf("Agent: %s (ID: %s)\n", agent.Registration.Name, id)
		output += fmt.Sprintf("  Address: http://%s:%d\n", agent.Registration.Address, agent.Registration.Port)

		for _, cap := range publicCaps {
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
