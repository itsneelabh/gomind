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

// MockAIClient implements core.AIClient for testing
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
		Capabilities: []core.Capability{{Name: "analyze_stock"}},
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
				Capabilities: []core.Capability{{Name: "analyze_stock"}},
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
		Capabilities: []core.Capability{{Name: "capability1"}},
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

// ============================================================================
// Layer 3: extractJSON and requestParameterCorrection Tests
// ============================================================================

// TestExtractJSON tests the extractJSON helper function for Layer 3
func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON object",
			input:    `{"lat": 35.6897, "lon": 139.6917}`,
			expected: `{"lat": 35.6897, "lon": 139.6917}`,
		},
		{
			name:     "JSON with whitespace",
			input:    `   {"lat": 35.6897}   `,
			expected: `{"lat": 35.6897}`,
		},
		{
			name: "JSON wrapped in markdown json code block",
			input: "```json\n{\"lat\": 35.6897}\n```",
			expected: `{"lat": 35.6897}`,
		},
		{
			name: "JSON wrapped in plain markdown code block",
			input: "```\n{\"lat\": 35.6897}\n```",
			expected: `{"lat": 35.6897}`,
		},
		{
			name: "JSON with markdown and extra whitespace",
			input: "```json\n  {\"lat\": 35.6897, \"lon\": 139.6917}  \n```",
			expected: `{"lat": 35.6897, "lon": 139.6917}`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name: "multiline JSON in code block",
			input: "```json\n{\n  \"lat\": 35.6897,\n  \"lon\": 139.6917\n}\n```",
			expected: "{\n  \"lat\": 35.6897,\n  \"lon\": 139.6917\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestRequestParameterCorrection tests the requestParameterCorrection method
func TestRequestParameterCorrection(t *testing.T) {
	// Create a mock AI client
	mockAIClient := &mockAIClientForCorrection{
		response: `{"lat": 35.6897, "lon": 139.6917}`,
	}

	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	config := DefaultConfig()

	orchestrator := NewAIOrchestrator(config, nil, mockAIClient)
	orchestrator.catalog = catalog

	step := RoutingStep{
		StepID:    "test-step",
		AgentName: "weather-tool",
		Metadata: map[string]interface{}{
			"capability": "get_weather",
		},
	}

	originalParams := map[string]interface{}{
		"lat": "35.6897",  // String - incorrect
		"lon": "139.6917", // String - incorrect
	}

	schema := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "float64", Required: true},
			{Name: "lon", Type: "float64", Required: true},
		},
	}

	ctx := context.Background()
	corrected, err := orchestrator.requestParameterCorrection(ctx, step, originalParams, "type error", schema)

	if err != nil {
		t.Fatalf("requestParameterCorrection failed: %v", err)
	}

	// Verify corrected params have proper types
	if lat, ok := corrected["lat"].(float64); !ok {
		t.Errorf("Expected lat to be float64, got %T", corrected["lat"])
	} else if lat != 35.6897 {
		t.Errorf("Expected lat=35.6897, got %v", lat)
	}

	// Verify the AI client was called with appropriate prompt
	if !mockAIClient.called {
		t.Error("AI client should have been called")
	}

	if !strings.Contains(mockAIClient.lastPrompt, "type error") {
		t.Error("Prompt should contain error message")
	}

	if !strings.Contains(mockAIClient.lastPrompt, "lat") {
		t.Error("Prompt should contain parameter names")
	}
}

// TestRequestParameterCorrectionNoAIClient tests error handling when AI client is nil
func TestRequestParameterCorrectionNoAIClient(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	config := DefaultConfig()
	config.ExecutionOptions.ValidationFeedbackEnabled = false // Don't wire up callback

	orchestrator := NewAIOrchestrator(config, nil, nil) // nil AI client
	orchestrator.catalog = catalog

	step := RoutingStep{StepID: "test"}
	params := map[string]interface{}{"lat": "35.6897"}
	schema := &EnhancedCapability{}

	_, err := orchestrator.requestParameterCorrection(context.Background(), step, params, "error", schema)

	if err == nil {
		t.Error("Expected error when AI client is nil")
	}
	if !strings.Contains(err.Error(), "AI client not available") {
		t.Errorf("Expected 'AI client not available' error, got: %v", err)
	}
}

// TestRequestParameterCorrectionInvalidJSON tests error handling for invalid LLM response
func TestRequestParameterCorrectionInvalidJSON(t *testing.T) {
	mockAIClient := &mockAIClientForCorrection{
		response: "this is not valid JSON",
	}

	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	config := DefaultConfig()

	orchestrator := NewAIOrchestrator(config, nil, mockAIClient)
	orchestrator.catalog = catalog

	step := RoutingStep{StepID: "test", Metadata: map[string]interface{}{"capability": "test"}}
	params := map[string]interface{}{"lat": "35.6897"}
	schema := &EnhancedCapability{}

	_, err := orchestrator.requestParameterCorrection(context.Background(), step, params, "error", schema)

	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Expected 'failed to parse' error, got: %v", err)
	}
}

// mockAIClientForCorrection is a mock AI client for testing Layer 3 correction
type mockAIClientForCorrection struct {
	response   string
	called     bool
	lastPrompt string
	shouldFail bool
	failError  error
}

func (m *mockAIClientForCorrection) GenerateResponse(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
	m.called = true
	m.lastPrompt = prompt
	if m.shouldFail {
		return nil, m.failError
	}
	return &core.AIResponse{Content: m.response}, nil
}

// =============================================================================
// Plan Parse Retry Tests
// =============================================================================

// TestAIOrchestrator_BuildPlanningPromptWithParseError tests the error feedback prompt
func TestAIOrchestrator_BuildPlanningPromptWithParseError(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()
	config := DefaultConfig()

	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Set up a default prompt builder to avoid nil pointer
	builder, _ := NewDefaultPromptBuilder(nil)
	orchestrator.SetPromptBuilder(builder)

	// Create a sample parse error
	parseErr := fmt.Errorf("invalid character '*' after object key:value pair")

	prompt, err := orchestrator.buildPlanningPromptWithParseError(context.Background(), "test request", parseErr)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the prompt contains error feedback
	if !strings.Contains(prompt, "IMPORTANT: Your previous response could not be parsed as valid JSON") {
		t.Error("Expected error feedback header in prompt")
	}

	if !strings.Contains(prompt, "invalid character '*'") {
		t.Error("Expected parse error message in prompt")
	}

	if !strings.Contains(prompt, "NO arithmetic expressions") {
		t.Error("Expected arithmetic expression warning in prompt")
	}

	if !strings.Contains(prompt, "NO markdown formatting") {
		t.Error("Expected markdown formatting warning in prompt")
	}

	if !strings.Contains(prompt, "NO trailing commas") {
		t.Error("Expected trailing comma warning in prompt")
	}
}

// TestAIOrchestrator_GenerateExecutionPlan_RetryDisabled tests behavior when retry is disabled
func TestAIOrchestrator_GenerateExecutionPlan_RetryDisabled(t *testing.T) {
	discovery := NewMockDiscovery()

	// Mock AI client that returns invalid JSON
	mockAI := &mockAIClientForRetry{
		responses: []string{
			"this is not valid JSON",
		},
	}

	config := DefaultConfig()
	config.PlanParseRetryEnabled = false // Disable retry

	orchestrator := NewAIOrchestrator(config, discovery, mockAI)

	// Set up a default prompt builder
	builder, _ := NewDefaultPromptBuilder(nil)
	orchestrator.SetPromptBuilder(builder)

	_, err := orchestrator.generateExecutionPlan(context.Background(), "test request", "test-123")

	if err == nil {
		t.Fatal("Expected error when parsing invalid JSON")
	}

	// Should have only made 1 call (no retry)
	if mockAI.callCount != 1 {
		t.Errorf("Expected 1 call (no retry), got %d", mockAI.callCount)
	}
}

// TestAIOrchestrator_GenerateExecutionPlan_RetrySuccess tests successful retry
func TestAIOrchestrator_GenerateExecutionPlan_RetrySuccess(t *testing.T) {
	discovery := NewMockDiscovery()

	// Mock AI client that fails first, then succeeds
	validJSON := `{
		"plan_id": "test-123",
		"original_request": "test request",
		"mode": "autonomous",
		"steps": [
			{
				"step_id": "step-1",
				"agent_name": "test-agent",
				"namespace": "default",
				"instruction": "do something"
			}
		]
	}`

	mockAI := &mockAIClientForRetry{
		responses: []string{
			"invalid JSON with * arithmetic",
			validJSON,
		},
	}

	config := DefaultConfig()
	config.PlanParseRetryEnabled = true
	config.PlanParseMaxRetries = 2

	orchestrator := NewAIOrchestrator(config, discovery, mockAI)

	// Set up a default prompt builder
	builder, _ := NewDefaultPromptBuilder(nil)
	orchestrator.SetPromptBuilder(builder)

	plan, err := orchestrator.generateExecutionPlan(context.Background(), "test request", "test-123")

	if err != nil {
		t.Fatalf("Expected successful retry, got error: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected plan to be returned")
	}

	if plan.PlanID != "test-123" {
		t.Errorf("Expected plan_id 'test-123', got %s", plan.PlanID)
	}

	// Should have made 2 calls (initial + 1 retry)
	if mockAI.callCount != 2 {
		t.Errorf("Expected 2 calls (initial + 1 retry), got %d", mockAI.callCount)
	}
}

// TestAIOrchestrator_GenerateExecutionPlan_AllRetriesExhausted tests when all retries fail
func TestAIOrchestrator_GenerateExecutionPlan_AllRetriesExhausted(t *testing.T) {
	discovery := NewMockDiscovery()

	// Mock AI client that always returns invalid JSON
	mockAI := &mockAIClientForRetry{
		responses: []string{
			"invalid JSON 1",
			"invalid JSON 2",
			"invalid JSON 3",
		},
	}

	config := DefaultConfig()
	config.PlanParseRetryEnabled = true
	config.PlanParseMaxRetries = 2

	orchestrator := NewAIOrchestrator(config, discovery, mockAI)

	// Set up a default prompt builder
	builder, _ := NewDefaultPromptBuilder(nil)
	orchestrator.SetPromptBuilder(builder)

	_, err := orchestrator.generateExecutionPlan(context.Background(), "test request", "test-123")

	if err == nil {
		t.Fatal("Expected error when all retries exhausted")
	}

	// Should have made 3 calls (initial + 2 retries)
	if mockAI.callCount != 3 {
		t.Errorf("Expected 3 calls (initial + 2 retries), got %d", mockAI.callCount)
	}
}

// mockAIClientForRetry is a mock AI client that returns different responses on each call
type mockAIClientForRetry struct {
	responses []string
	callCount int
}

func (m *mockAIClientForRetry) GenerateResponse(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
	idx := m.callCount
	m.callCount++

	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}

	return &core.AIResponse{
		Content: m.responses[idx],
		Usage: core.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}
