package orchestration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// MockDiscovery implements core.Discovery for testing
type MockDiscovery struct {
	services map[string][]*core.ServiceRegistration
}

func NewMockDiscovery() *MockDiscovery {
	return &MockDiscovery{
		services: make(map[string][]*core.ServiceRegistration),
	}
}

func (m *MockDiscovery) Register(ctx context.Context, registration *core.ServiceRegistration) error {
	m.services[registration.Name] = append(m.services[registration.Name], registration)
	return nil
}

func (m *MockDiscovery) Unregister(ctx context.Context, serviceID string) error {
	return nil
}

func (m *MockDiscovery) FindService(ctx context.Context, serviceName string) ([]*core.ServiceRegistration, error) {
	return m.services[serviceName], nil
}

func (m *MockDiscovery) FindByCapability(ctx context.Context, capability string) ([]*core.ServiceRegistration, error) {
	var results []*core.ServiceRegistration
	for _, services := range m.services {
		for _, service := range services {
			for _, cap := range service.Capabilities {
				if cap.Name == capability {
					results = append(results, service)
					break
				}
			}
		}
	}
	return results, nil
}

func (m *MockDiscovery) UpdateHealth(ctx context.Context, serviceID string, status core.HealthStatus) error {
	return nil
}

func (m *MockDiscovery) Discover(ctx context.Context, filter core.DiscoveryFilter) ([]*core.ServiceInfo, error) {
	var results []*core.ServiceInfo
	
	// If searching by name
	if filter.Name != "" {
		if services, ok := m.services[filter.Name]; ok {
			for _, svc := range services {
				results = append(results, (*core.ServiceInfo)(svc))
			}
		}
		return results, nil
	}
	
	// If searching by capabilities
	if len(filter.Capabilities) > 0 {
		for _, services := range m.services {
			for _, service := range services {
				for _, cap := range service.Capabilities {
					for _, filterCap := range filter.Capabilities {
						if cap.Name == filterCap {
							results = append(results, (*core.ServiceInfo)(service))
							goto nextService
						}
					}
				}
				nextService:
			}
		}
		return results, nil
	}
	
	// Return all services
	for _, services := range m.services {
		for _, svc := range services {
			results = append(results, (*core.ServiceInfo)(svc))
		}
	}
	return results, nil
}

func TestAgentCatalog_Refresh(t *testing.T) {
	// Create mock discovery with test services
	discovery := NewMockDiscovery()

	// Register test agents
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "stock-1",
		Name:         "stock-analyzer",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "analyze_stock"}, {Name: "get_price"}},
		Health:       core.HealthHealthy,
	})

	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "news-1",
		Name:         "news-agent",
		Address:      "localhost",
		Port:         8081,
		Capabilities: []core.Capability{{Name: "get_news"}, {Name: "analyze_sentiment"}},
		Health:       core.HealthHealthy,
	})

	// Create catalog
	catalog := NewAgentCatalog(discovery)

	// Test initial state
	agents := catalog.GetAgents()
	if len(agents) != 0 {
		t.Errorf("Expected empty catalog, got %d agents", len(agents))
	}

	// Create mock HTTP server for capabilities endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capabilities := []EnhancedCapability{
			{
				Name:        "analyze_stock",
				Description: "Analyzes stock performance",
				Endpoint:    "/api/analyze",
				Parameters: []Parameter{
					{Name: "symbol", Type: "string", Required: true},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(capabilities)
	}))
	defer server.Close()

	// Note: In real test, we'd need to mock the HTTP calls properly
	// For now, test the basic refresh mechanism
	ctx := context.Background()
	err := catalog.Refresh(ctx)
	if err != nil {
		t.Errorf("Refresh failed: %v", err)
	}

	// Verify agents were loaded (even if capabilities fetch failed)
	agents = catalog.GetAgents()
	if len(agents) == 0 {
		t.Error("Expected agents to be loaded after refresh")
	}
}

func TestAgentCatalog_FindByCapability(t *testing.T) {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	// Manually populate catalog for testing
	catalog.agents = map[string]*AgentInfo{
		"stock-1": {
			Registration: &core.ServiceRegistration{
				ID:   "stock-1",
				Name: "stock-analyzer",
			},
			Capabilities: []EnhancedCapability{
				{Name: "analyze_stock"},
				{Name: "get_price"},
			},
		},
		"news-1": {
			Registration: &core.ServiceRegistration{
				ID:   "news-1",
				Name: "news-agent",
			},
			Capabilities: []EnhancedCapability{
				{Name: "get_news"},
			},
		},
	}

	// Rebuild capability index
	catalog.capabilityIndex = make(map[string][]string)
	for id, agent := range catalog.agents {
		for _, cap := range agent.Capabilities {
			catalog.capabilityIndex[cap.Name] = append(catalog.capabilityIndex[cap.Name], id)
		}
	}

	// Test finding by capability
	agents := catalog.FindByCapability("analyze_stock")
	if len(agents) != 1 || agents[0] != "stock-1" {
		t.Errorf("Expected to find stock-1 for analyze_stock, got %v", agents)
	}

	agents = catalog.FindByCapability("get_news")
	if len(agents) != 1 || agents[0] != "news-1" {
		t.Errorf("Expected to find news-1 for get_news, got %v", agents)
	}

	agents = catalog.FindByCapability("nonexistent")
	if len(agents) != 0 {
		t.Errorf("Expected no agents for nonexistent capability, got %v", agents)
	}
}

func TestAgentCatalog_FormatForLLM(t *testing.T) {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	// Setup test data
	catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:      "test-1",
				Name:    "test-agent",
				Address: "localhost",
				Port:    8080,
			},
			Capabilities: []EnhancedCapability{
				{
					Name:        "test_capability",
					Description: "Test capability description",
					Parameters: []Parameter{
						{
							Name:        "param1",
							Type:        "string",
							Required:    true,
							Description: "Test parameter",
						},
					},
					Returns: ReturnType{
						Type:        "object",
						Description: "Test result",
					},
				},
			},
		},
	}

	output := catalog.FormatForLLM()

	// Verify output contains expected information
	if output == "" {
		t.Error("Expected non-empty LLM format output")
	}

	expectedStrings := []string{
		"Available Agents",
		"test-agent",
		"test_capability",
		"Test capability description",
		"param1: string (required)",
		"Returns: object",
	}

	for _, expected := range expectedStrings {
		if !contains(output, expected) {
			t.Errorf("Expected output to contain '%s'", expected)
		}
	}
}

func TestAgentCatalog_GetAgent(t *testing.T) {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	testAgent := &AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:   "test-1",
			Name: "test-agent",
		},
		LastUpdated: time.Now(),
	}

	catalog.agents = map[string]*AgentInfo{
		"test-1": testAgent,
	}

	// Test getting existing agent
	agent := catalog.GetAgent("test-1")
	if agent == nil {
		t.Fatal("Expected to get agent, got nil")
	}
	if agent.Registration.Name != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got %s", agent.Registration.Name)
	}

	// Test getting non-existent agent
	agent = catalog.GetAgent("nonexistent")
	if agent != nil {
		t.Error("Expected nil for non-existent agent")
	}
}

func TestAgentCatalog_ConvertBasicCapabilities(t *testing.T) {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	basic := []core.Capability{
		{Name: "capability1"},
		{Name: "capability2"},
	}
	enhanced := catalog.convertBasicCapabilities(basic)

	if len(enhanced) != 2 {
		t.Errorf("Expected 2 enhanced capabilities, got %d", len(enhanced))
	}

	if enhanced[0].Name != "capability1" {
		t.Errorf("Expected capability name 'capability1', got %s", enhanced[0].Name)
	}

	if enhanced[0].Endpoint != "/api/capability1" {
		t.Errorf("Expected endpoint '/api/capability1', got %s", enhanced[0].Endpoint)
	}
}

// Helper function
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}
