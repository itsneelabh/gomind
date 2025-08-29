package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// MockAIClient implements ai.AIClient for testing
type MockAIClient struct {
	responses map[string]string
	calls     []string
}

func NewMockAIClient() *MockAIClient {
	return &MockAIClient{
		responses: make(map[string]string),
		calls:     []string{},
	}
}

func (m *MockAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	m.calls = append(m.calls, prompt)

	// Return predefined responses based on prompt content
	if strings.Contains(prompt, "Create an execution plan") {
		return &core.AIResponse{
			Content: m.getPlanResponse(),
			Usage: core.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}, nil
	}

	if strings.Contains(prompt, "Synthesize") {
		return &core.AIResponse{
			Content: "This is a synthesized response combining all agent outputs.",
		}, nil
	}

	return &core.AIResponse{
		Content: "Default response",
	}, nil
}

func (m *MockAIClient) getPlanResponse() string {
	plan := map[string]interface{}{
		"plan_id":          "test-plan-1",
		"original_request": "test request",
		"mode":             "autonomous",
		"steps": []map[string]interface{}{
			{
				"step_id":     "step-1",
				"agent_name":  "stock-analyzer",
				"namespace":   "default",
				"instruction": "Analyze stock",
				"depends_on":  []string{},
				"metadata": map[string]interface{}{
					"capability": "analyze_stock",
					"parameters": map[string]interface{}{
						"symbol": "AAPL",
					},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(plan)
	return string(jsonBytes)
}

func TestAIOrchestrator_ProcessRequest(t *testing.T) {
	// Setup mocks
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "stock-1",
		Name:         "stock-analyzer",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []string{"analyze_stock"},
	})

	// Create orchestrator
	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog with test data
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"stock-1": {
			Registration: &core.ServiceRegistration{
				ID:           "stock-1",
				Name:         "stock-analyzer",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []string{"analyze_stock"},
			},
			Capabilities: []EnhancedCapability{
				{Name: "analyze_stock", Description: "Analyzes stocks"},
			},
		},
	}

	// Replace executor with properly initialized one
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	ctx := context.Background()

	// Test request processing
	response, err := orchestrator.ProcessRequest(ctx, "Analyze Apple stock", nil)

	if err != nil {
		t.Errorf("ProcessRequest failed: %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Response == "" {
		t.Error("Expected non-empty response")
	}

	// Verify AI client was called
	if len(aiClient.calls) < 2 {
		t.Error("Expected at least 2 AI calls (planning + synthesis)")
	}
}

func TestAIOrchestrator_ValidatePlan(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()

	// Register test agents
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "agent-1",
		Name:         "test-agent",
		Capabilities: []string{"capability1"},
	})

	orchestrator := NewAIOrchestrator(DefaultConfig(), discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"agent-1": {
			Registration: &core.ServiceRegistration{
				ID:   "agent-1",
				Name: "test-agent",
			},
			Capabilities: []EnhancedCapability{
				{Name: "capability1"},
			},
		},
	}

	// Test valid plan
	validPlan := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "test-agent",
				Metadata: map[string]interface{}{
					"capability": "capability1",
				},
			},
		},
	}

	err := orchestrator.validatePlan(validPlan)
	if err != nil {
		t.Errorf("Valid plan validation failed: %v", err)
	}

	// Test invalid plan - non-existent agent
	invalidPlan1 := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "non-existent-agent",
			},
		},
	}

	err = orchestrator.validatePlan(invalidPlan1)
	if err == nil {
		t.Error("Expected validation to fail for non-existent agent")
	}

	// Test invalid plan - non-existent capability
	invalidPlan2 := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "test-agent",
				Metadata: map[string]interface{}{
					"capability": "non-existent-capability",
				},
			},
		},
	}

	err = orchestrator.validatePlan(invalidPlan2)
	if err == nil {
		t.Error("Expected validation to fail for non-existent capability")
	}

	// Test invalid plan - missing dependency
	invalidPlan3 := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "test-agent",
				DependsOn: []string{"non-existent-step"},
			},
		},
	}

	err = orchestrator.validatePlan(invalidPlan3)
	if err == nil {
		t.Error("Expected validation to fail for missing dependency")
	}
}

func TestAIOrchestrator_ParsePlan(t *testing.T) {
	orchestrator := &AIOrchestrator{}

	// Test valid JSON
	validJSON := `{
		"plan_id": "test-123",
		"original_request": "test request",
		"mode": "autonomous",
		"steps": [
			{
				"step_id": "step-1",
				"agent_name": "agent1",
				"namespace": "default",
				"instruction": "do something"
			}
		]
	}`

	plan, err := orchestrator.parsePlan(validJSON)
	if err != nil {
		t.Errorf("Failed to parse valid JSON: %v", err)
	}

	if plan.PlanID != "test-123" {
		t.Errorf("Expected plan_id 'test-123', got %s", plan.PlanID)
	}

	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}

	// Test JSON with markdown
	jsonWithMarkdown := fmt.Sprintf("```json\n%s\n```", validJSON)
	_, err = orchestrator.parsePlan(jsonWithMarkdown)
	if err != nil {
		t.Errorf("Failed to parse JSON with markdown: %v", err)
	}

	// Test invalid JSON
	invalidJSON := "not json at all"
	_, err = orchestrator.parsePlan(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestAIOrchestrator_Metrics(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()
	orchestrator := NewAIOrchestrator(DefaultConfig(), discovery, aiClient)

	// Initial metrics should be zero
	metrics := orchestrator.GetMetrics()
	if metrics.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests, got %d", metrics.TotalRequests)
	}

	// Update metrics
	orchestrator.updateMetrics(100*time.Millisecond, true)
	orchestrator.updateMetrics(200*time.Millisecond, false)

	metrics = orchestrator.GetMetrics()
	if metrics.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", metrics.TotalRequests)
	}
	if metrics.SuccessfulRequests != 1 {
		t.Errorf("Expected 1 successful request, got %d", metrics.SuccessfulRequests)
	}
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
}

func TestAIOrchestrator_History(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()

	config := DefaultConfig()
	config.HistorySize = 2 // Small size for testing

	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Add history entries
	for i := 0; i < 3; i++ {
		response := &OrchestratorResponse{
			RequestID:       fmt.Sprintf("req-%d", i),
			OriginalRequest: fmt.Sprintf("request %d", i),
			Response:        fmt.Sprintf("response %d", i),
			ExecutionTime:   time.Duration(i) * time.Second,
		}
		orchestrator.addToHistory(response)
	}

	// Check history size is limited
	history := orchestrator.GetExecutionHistory()
	if len(history) != 2 {
		t.Errorf("Expected history size 2, got %d", len(history))
	}

	// Verify oldest entry was removed
	if history[0].Request != "request 1" {
		t.Errorf("Expected oldest entry to be 'request 1', got %s", history[0].Request)
	}
}

func TestAIOrchestrator_ExtractAgentsFromPlan(t *testing.T) {
	orchestrator := &AIOrchestrator{}

	plan := &RoutingPlan{
		Steps: []RoutingStep{
			{AgentName: "agent1"},
			{AgentName: "agent2"},
			{AgentName: "agent1"}, // Duplicate
			{AgentName: "agent3"},
		},
	}

	agents := orchestrator.extractAgentsFromPlan(plan)

	// Should have 3 unique agents
	if len(agents) != 3 {
		t.Errorf("Expected 3 unique agents, got %d", len(agents))
	}

	// Check all agents are present
	agentMap := make(map[string]bool)
	for _, agent := range agents {
		agentMap[agent] = true
	}

	for _, expected := range []string{"agent1", "agent2", "agent3"} {
		if !agentMap[expected] {
			t.Errorf("Expected agent %s not found", expected)
		}
	}
}

func TestFindJSONFunctions(t *testing.T) {
	// Test findJSONStart
	cases := []struct {
		input    string
		expected int
	}{
		{"{}", 0},
		{"text before {}", 12},
		{"no json here", -1},
		{"   {  }", 3},
	}

	for _, tc := range cases {
		result := findJSONStart(tc.input)
		if result != tc.expected {
			t.Errorf("findJSONStart(%q) = %d, expected %d", tc.input, result, tc.expected)
		}
	}

	// Test findJSONEnd
	endCases := []struct {
		input    string
		start    int
		expected int
	}{
		{"{}", 0, 2},
		{`{"nested": {}}`, 0, 14},
		{`{"a": 1, "b": 2}`, 0, 16},
		{`{"incomplete": `, 0, -1},
	}

	for _, tc := range endCases {
		result := findJSONEnd(tc.input, tc.start)
		if result != tc.expected {
			t.Errorf("findJSONEnd(%q, %d) = %d, expected %d", tc.input, tc.start, result, tc.expected)
		}
	}
}
