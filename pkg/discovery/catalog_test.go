package discovery

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// MockRedisDiscovery creates a mock RedisDiscovery for testing
func createMockRedisDiscovery() *RedisDiscovery {
	rd := &RedisDiscovery{
		namespace:        "test",
		fullCatalog:      make(map[string]*AgentRegistration),
		catalogSyncTime:  30 * time.Second,
		catalogSyncErrors: 0,
		cache: &DiscoveryCache{
			capabilities: make(map[string][]AgentRegistration),
			agents:       make(map[string]AgentRegistration),
		},
		cbThreshold:      5,
		cbCooldown:       2 * time.Minute,
		persistEnabled:   false,
	}
	return rd
}

// TestCatalogStorage tests catalog storage and retrieval
func TestCatalogStorage(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Create test agents
	agent1 := &AgentRegistration{
		ID:              "agent-1",
		Name:            "calculator",
		Namespace:       "math",
		ServiceName:     "calculator-service",
		ServiceEndpoint: "calculator-service.math.svc.cluster.local:8080",
		Description:     "Performs mathematical calculations",
		Examples:        []string{"Add 5 and 3", "Calculate square root of 16"},
		LLMHints:        "Use for arithmetic operations",
		Status:          StatusHealthy,
		LastHeartbeat:   time.Now(),
		Capabilities: []CapabilityMetadata{
			{
				Name:        "add",
				Description: "Addition operation",
				LLMPrompt:   "Add two numbers",
			},
			{
				Name:        "multiply",
				Description: "Multiplication operation",
				LLMPrompt:   "Multiply two numbers",
			},
		},
	}

	agent2 := &AgentRegistration{
		ID:              "agent-2",
		Name:            "translator",
		Namespace:       "nlp",
		ServiceName:     "translator-service",
		ServiceEndpoint: "translator-service.nlp.svc.cluster.local:8080",
		Description:     "Translates text between languages",
		Examples:        []string{"Translate 'hello' to Spanish", "What is 'goodbye' in French?"},
		LLMHints:        "Use for language translation",
		Status:          StatusHealthy,
		LastHeartbeat:   time.Now(),
		Capabilities: []CapabilityMetadata{
			{
				Name:        "translate",
				Description: "Text translation",
				LLMPrompt:   "Translate text between languages",
			},
		},
	}

	// Store agents in catalog
	rd.catalogMutex.Lock()
	rd.fullCatalog[agent1.ID] = agent1
	rd.fullCatalog[agent2.ID] = agent2
	rd.lastCatalogSync = time.Now()
	rd.catalogMutex.Unlock()

	// Test GetFullCatalog
	catalog := rd.GetFullCatalog()
	if len(catalog) != 2 {
		t.Errorf("Expected 2 agents in catalog, got %d", len(catalog))
	}

	// Verify agent1 is in catalog
	if retrieved, ok := catalog["agent-1"]; !ok {
		t.Error("Agent-1 not found in catalog")
	} else {
		if retrieved.Name != "calculator" {
			t.Errorf("Expected agent name 'calculator', got '%s'", retrieved.Name)
		}
		if retrieved.Namespace != "math" {
			t.Errorf("Expected namespace 'math', got '%s'", retrieved.Namespace)
		}
		if len(retrieved.Capabilities) != 2 {
			t.Errorf("Expected 2 capabilities, got %d", len(retrieved.Capabilities))
		}
	}

	// Verify agent2 is in catalog
	if retrieved, ok := catalog["agent-2"]; !ok {
		t.Error("Agent-2 not found in catalog")
	} else {
		if retrieved.Name != "translator" {
			t.Errorf("Expected agent name 'translator', got '%s'", retrieved.Name)
		}
	}
}

// TestGetCatalogForLLM tests LLM-friendly formatting
func TestGetCatalogForLLM(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Test empty catalog
	output := rd.GetCatalogForLLM()
	if output != "No agents currently available in the catalog." {
		t.Errorf("Expected empty catalog message, got: %s", output)
	}

	// Add test agents
	agent1 := &AgentRegistration{
		ID:              "calc-1",
		Name:            "calculator",
		Namespace:       "math",
		ServiceName:     "calculator-service",
		ServiceEndpoint: "calculator-service.math.svc.cluster.local:8080",
		Description:     "Mathematical calculation service",
		Examples:        []string{"Add 10 and 20", "Multiply 5 by 6"},
		LLMHints:        "Best for arithmetic and mathematical operations",
		Status:          StatusHealthy,
		LastHeartbeat:   time.Now(),
		Capabilities: []CapabilityMetadata{
			{
				Name:        "calculate",
				Description: "Perform calculations",
				LLMPrompt:   "Use this for any math problem",
			},
		},
	}

	rd.catalogMutex.Lock()
	rd.fullCatalog[agent1.ID] = agent1
	rd.lastCatalogSync = time.Now()
	rd.catalogMutex.Unlock()

	// Get LLM formatted output
	output = rd.GetCatalogForLLM()

	// Verify output contains expected elements
	if !strings.Contains(output, "=== AVAILABLE AGENTS CATALOG ===") {
		t.Error("Missing catalog header")
	}

	if !strings.Contains(output, "NAMESPACE: math") {
		t.Error("Missing namespace section")
	}

	if !strings.Contains(output, "AGENT: calculator") {
		t.Error("Missing agent name")
	}

	if !strings.Contains(output, "Description: Mathematical calculation service") {
		t.Error("Missing agent description")
	}

	if !strings.Contains(output, "Endpoint: calculator-service.math.svc.cluster.local:8080") {
		t.Error("Missing service endpoint")
	}

	if !strings.Contains(output, "Status: healthy") {
		t.Error("Missing status")
	}

	if !strings.Contains(output, "• calculate - Perform calculations") {
		t.Error("Missing capability")
	}

	if !strings.Contains(output, "Example requests:") {
		t.Error("Missing examples section")
	}

	if !strings.Contains(output, "Routing hints: Best for arithmetic and mathematical operations") {
		t.Error("Missing LLM hints")
	}

	if !strings.Contains(output, "Health: ✓ Active") {
		t.Error("Missing health status")
	}

	if !strings.Contains(output, "SUMMARY: 1 agents across 1 namespaces") {
		t.Error("Missing summary")
	}
}

// TestCatalogWithMultipleNamespaces tests catalog organization by namespace
func TestCatalogWithMultipleNamespaces(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Add agents in different namespaces
	agents := []*AgentRegistration{
		{
			ID:        "agent-1",
			Name:      "service-a",
			Namespace: "namespace-1",
			Status:    StatusHealthy,
		},
		{
			ID:        "agent-2",
			Name:      "service-b",
			Namespace: "namespace-1",
			Status:    StatusHealthy,
		},
		{
			ID:        "agent-3",
			Name:      "service-c",
			Namespace: "namespace-2",
			Status:    StatusHealthy,
		},
		{
			ID:        "agent-4",
			Name:      "service-d",
			Namespace: "", // Should default to "default"
			Status:    StatusHealthy,
		},
	}

	rd.catalogMutex.Lock()
	for _, agent := range agents {
		agent.LastHeartbeat = time.Now()
		rd.fullCatalog[agent.ID] = agent
	}
	rd.lastCatalogSync = time.Now()
	rd.catalogMutex.Unlock()

	// Get LLM formatted output
	output := rd.GetCatalogForLLM()

	// Check that all namespaces are present
	if !strings.Contains(output, "NAMESPACE: namespace-1") {
		t.Error("Missing namespace-1")
	}
	if !strings.Contains(output, "NAMESPACE: namespace-2") {
		t.Error("Missing namespace-2")
	}
	if !strings.Contains(output, "NAMESPACE: default") {
		t.Error("Missing default namespace")
	}

	// Check summary
	if !strings.Contains(output, "4 agents across 3 namespaces") {
		t.Error("Incorrect summary count")
	}
}

// TestCatalogHealthIndicators tests health status display
func TestCatalogHealthIndicators(t *testing.T) {
	rd := createMockRedisDiscovery()

	now := time.Now()

	agents := []*AgentRegistration{
		{
			ID:            "agent-recent",
			Name:          "recent-agent",
			Namespace:     "test",
			Status:        StatusHealthy,
			LastHeartbeat: now.Add(-30 * time.Second), // Recent
		},
		{
			ID:            "agent-warning",
			Name:          "warning-agent",
			Namespace:     "test",
			Status:        StatusHealthy,
			LastHeartbeat: now.Add(-3 * time.Minute), // Warning
		},
		{
			ID:            "agent-inactive",
			Name:          "inactive-agent",
			Namespace:     "test",
			Status:        StatusUnhealthy,
			LastHeartbeat: now.Add(-10 * time.Minute), // Inactive
		},
	}

	rd.catalogMutex.Lock()
	for _, agent := range agents {
		rd.fullCatalog[agent.ID] = agent
	}
	rd.lastCatalogSync = now
	rd.catalogMutex.Unlock()

	output := rd.GetCatalogForLLM()

	// Check health indicators
	if !strings.Contains(output, "Health: ✓ Active") {
		t.Error("Missing active health indicator")
	}
	if !strings.Contains(output, "Health: ⚠ Last seen") {
		t.Error("Missing warning health indicator")
	}
	if !strings.Contains(output, "Health: ✗ Inactive") {
		t.Error("Missing inactive health indicator")
	}
}

// TestGetCatalogStats tests catalog statistics
func TestGetCatalogStats(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Initially empty
	count, lastSync, errors := rd.GetCatalogStats()
	if count != 0 {
		t.Errorf("Expected 0 agents, got %d", count)
	}
	if !lastSync.IsZero() {
		t.Error("Expected zero time for lastSync")
	}
	if errors != 0 {
		t.Errorf("Expected 0 errors, got %d", errors)
	}

	// Add agents and update stats
	rd.catalogMutex.Lock()
	rd.fullCatalog["agent-1"] = &AgentRegistration{ID: "agent-1"}
	rd.fullCatalog["agent-2"] = &AgentRegistration{ID: "agent-2"}
	rd.lastCatalogSync = time.Now()
	rd.catalogSyncErrors = 2
	rd.catalogMutex.Unlock()

	count, lastSync, errors = rd.GetCatalogStats()
	if count != 2 {
		t.Errorf("Expected 2 agents, got %d", count)
	}
	if lastSync.IsZero() {
		t.Error("Expected non-zero lastSync time")
	}
	if errors != 2 {
		t.Errorf("Expected 2 errors, got %d", errors)
	}
}

// TestSetCatalogSyncInterval tests setting sync interval
func TestSetCatalogSyncInterval(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Initial interval
	if rd.catalogSyncTime != 30*time.Second {
		t.Errorf("Expected initial sync time 30s, got %v", rd.catalogSyncTime)
	}

	// Update interval
	rd.SetCatalogSyncInterval(60 * time.Second)
	if rd.catalogSyncTime != 60*time.Second {
		t.Errorf("Expected sync time 60s, got %v", rd.catalogSyncTime)
	}
}

// TestCatalogSyncContext tests catalog sync with context cancellation
func TestCatalogSyncContext(t *testing.T) {
	rd := createMockRedisDiscovery()

	ctx, cancel := context.WithCancel(context.Background())
	
	// Start catalog sync
	rd.StartCatalogSync(ctx, 100*time.Millisecond)
	
	// Let it run briefly
	time.Sleep(50 * time.Millisecond)
	
	// Cancel context
	cancel()
	
	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)
	
	// Verify it stopped (this is hard to test directly, but we can check
	// that a new sync can be started without issues)
	ctx2 := context.Background()
	rd.StartCatalogSync(ctx2, 200*time.Millisecond)
	
	// If we got here without panic, the test passes
}

// TestCatalogThreadSafety tests concurrent access to catalog
func TestCatalogThreadSafety(t *testing.T) {
	rd := createMockRedisDiscovery()

	// Populate initial catalog
	for i := 0; i < 10; i++ {
		agent := &AgentRegistration{
			ID:   fmt.Sprintf("agent-%d", i),
			Name: fmt.Sprintf("service-%d", i),
		}
		rd.catalogMutex.Lock()
		rd.fullCatalog[agent.ID] = agent
		rd.catalogMutex.Unlock()
	}

	// Run concurrent operations
	done := make(chan bool)

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = rd.GetFullCatalog()
				_ = rd.GetCatalogForLLM()
				_, _, _ = rd.GetCatalogStats()
			}
			done <- true
		}()
	}

	// Writer goroutine
	go func() {
		for j := 0; j < 100; j++ {
			rd.catalogMutex.Lock()
			rd.fullCatalog[fmt.Sprintf("new-%d", j)] = &AgentRegistration{
				ID: fmt.Sprintf("new-%d", j),
			}
			rd.catalogMutex.Unlock()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 6; i++ {
		<-done
	}

	// Verify catalog integrity
	catalog := rd.GetFullCatalog()
	if len(catalog) != 110 { // 10 initial + 100 added
		t.Errorf("Expected 110 agents, got %d", len(catalog))
	}
}

// TestK8sEndpointGeneration tests K8s service endpoint generation
func TestK8sEndpointGeneration(t *testing.T) {
	rd := createMockRedisDiscovery()

	agents := []*AgentRegistration{
		{
			ID:              "agent-1",
			Name:            "service-with-endpoint",
			ServiceEndpoint: "custom.endpoint.local:9090",
			LastHeartbeat:   time.Now(),
		},
		{
			ID:          "agent-2",
			Name:        "service-with-names",
			ServiceName: "my-service",
			Namespace:   "my-namespace",
			LastHeartbeat: time.Now(),
		},
		{
			ID:      "agent-3",
			Name:    "service-with-address",
			Address: "10.0.0.1",
			Port:    8080,
			LastHeartbeat: time.Now(),
		},
	}

	rd.catalogMutex.Lock()
	for _, agent := range agents {
		rd.fullCatalog[agent.ID] = agent
	}
	rd.catalogMutex.Unlock()

	output := rd.GetCatalogForLLM()

	// Check different endpoint formats
	if !strings.Contains(output, "Endpoint: custom.endpoint.local:9090") {
		t.Error("Missing custom endpoint")
	}
	if !strings.Contains(output, "Endpoint: my-service.my-namespace.svc.cluster.local:8080") {
		t.Error("Missing K8s generated endpoint")
	}
	if !strings.Contains(output, "Endpoint: 10.0.0.1:8080") {
		t.Error("Missing address:port endpoint")
	}
}