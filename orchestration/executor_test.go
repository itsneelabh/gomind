package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

func TestSmartExecutor_Execute(t *testing.T) {
	// Create mock catalog with test agents
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "test-agent",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "capability1",
						Endpoint: "/api/capability1",
					},
				},
			},
		},
	}

	// Create executor with mock HTTP client
	executor := NewSmartExecutor(catalog)

	// Replace HTTP client with mock
	mockRT := NewMockRoundTripper()
	mockRT.SetResponse("http://localhost:8080/api/capability1", http.StatusOK, `{"status": "success", "data": "test response"}`)
	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	// Create test plan with dependencies
	plan := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "test-agent",
				Metadata: map[string]interface{}{
					"capability": "capability1",
					"parameters": map[string]interface{}{
						"param1": "value1",
					},
				},
			},
			{
				StepID:    "step-2",
				AgentName: "test-agent",
				DependsOn: []string{"step-1"},
				Metadata: map[string]interface{}{
					"capability": "capability1",
					"parameters": map[string]interface{}{
						"param2": "value2",
					},
				},
			},
		},
	}

	ctx := context.Background()

	// Execute plan
	result, err := executor.Execute(ctx, plan)

	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if !result.Success {
		t.Error("Expected successful execution")
	}

	if len(result.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(result.Steps))
	}

	// Verify steps were executed in order
	if result.Steps[0].StepID != "step-1" {
		t.Error("Expected step-1 to be executed first")
	}

	if result.Steps[1].StepID != "step-2" {
		t.Error("Expected step-2 to be executed second")
	}

	// Verify both steps succeeded
	for _, step := range result.Steps {
		if !step.Success {
			t.Errorf("Step %s failed: %s", step.StepID, step.Error)
		}
	}
}

func TestSmartExecutor_FindReadySteps(t *testing.T) {
	executor := &SmartExecutor{}

	plan := &RoutingPlan{
		Steps: []RoutingStep{
			{StepID: "step-1", DependsOn: []string{}},
			{StepID: "step-2", DependsOn: []string{"step-1"}},
			{StepID: "step-3", DependsOn: []string{"step-1", "step-2"}},
			{StepID: "step-4", DependsOn: []string{}},
		},
	}

	executed := make(map[string]bool)
	results := make(map[string]*StepResult)

	// Initially, step-1 and step-4 should be ready (no dependencies)
	ready := executor.findReadySteps(plan, executed, results)
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready steps initially, got %d", len(ready))
	}

	// Verify the ready steps are the ones without dependencies
	readyIDs := make(map[string]bool)
	for _, step := range ready {
		readyIDs[step.StepID] = true
	}
	if !readyIDs["step-1"] || !readyIDs["step-4"] {
		t.Error("Expected step-1 and step-4 to be ready initially")
	}

	// Mark step-1 as executed and successful
	executed["step-1"] = true
	results["step-1"] = &StepResult{Success: true}

	// Mark step-4 as executed
	executed["step-4"] = true
	results["step-4"] = &StepResult{Success: true}

	// Now only step-2 should be ready
	ready = executor.findReadySteps(plan, executed, results)
	if len(ready) != 1 || ready[0].StepID != "step-2" {
		t.Error("Expected only step-2 to be ready after step-1 completes")
	}

	// Mark step-2 as executed but failed
	executed["step-2"] = true
	results["step-2"] = &StepResult{Success: false}

	// Step-3 should not be ready (dependency failed)
	ready = executor.findReadySteps(plan, executed, results)
	if len(ready) != 0 {
		t.Error("Expected no steps ready when dependency failed")
	}
}

func TestSmartExecutor_CircularDependency(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	executor := NewSmartExecutor(catalog)

	// Create plan with circular dependency
	plan := &RoutingPlan{
		PlanID: "circular-plan",
		Steps: []RoutingStep{
			{StepID: "step-1", DependsOn: []string{"step-2"}},
			{StepID: "step-2", DependsOn: []string{"step-1"}},
		},
	}

	ctx := context.Background()
	_, err := executor.Execute(ctx, plan)

	if err == nil {
		t.Error("Expected error for circular dependency")
	}

	if !containsString(err.Error(), "circular") {
		t.Errorf("Expected error message to mention circular dependency, got: %v", err)
	}
}

func TestSmartExecutor_ExecuteStep(t *testing.T) {
	// Create mock catalog
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "test-agent",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "test_cap",
						Endpoint: "/api/test",
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Use mock HTTP client
	mockRT := NewMockRoundTripper()
	mockRT.SetResponse("http://localhost:8080/api/test", http.StatusOK, `{"result": "success"}`)
	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	step := RoutingStep{
		StepID:    "test-step",
		AgentName: "test-agent",
		Metadata: map[string]interface{}{
			"capability": "test_cap",
			"parameters": map[string]interface{}{},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	// Test agent not found
	stepNotFound := RoutingStep{
		StepID:    "test-step",
		AgentName: "non-existent-agent",
	}

	result = executor.executeStep(ctx, stepNotFound)
	if result.Success {
		t.Error("Expected failure for non-existent agent")
	}

	if !containsString(result.Error, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Error)
	}
}

func TestSmartExecutor_Retry(t *testing.T) {
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "test-agent",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "test_cap",
						Endpoint: "/api/test",
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Create mock round tripper that fails twice then succeeds
	mockRT := NewMockRoundTripper()
	mockRT.SetRetryResponses("http://localhost:8080/api/test", []struct {
		StatusCode int
		Body       string
	}{
		{StatusCode: http.StatusInternalServerError, Body: "error"},
		{StatusCode: http.StatusInternalServerError, Body: "error"},
		{StatusCode: http.StatusOK, Body: `{"result": "success"}`},
	})

	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	step := RoutingStep{
		StepID:    "test-step",
		AgentName: "test-agent",
		Metadata: map[string]interface{}{
			"capability": "test_cap",
			"parameters": map[string]interface{}{},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	if !result.Success {
		t.Errorf("Expected successful execution after retries, got: %s", result.Error)
	}

	if result.Attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", result.Attempts)
	}
}

func TestSmartExecutor_SetMaxConcurrency(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	executor := NewSmartExecutor(catalog)

	// Test default concurrency
	if executor.maxConcurrency != 5 {
		t.Errorf("Expected default max concurrency 5, got %d", executor.maxConcurrency)
	}

	// Set new concurrency
	executor.SetMaxConcurrency(10)
	if executor.maxConcurrency != 10 {
		t.Errorf("Expected max concurrency 10, got %d", executor.maxConcurrency)
	}

	// Verify semaphore was recreated
	if cap(executor.semaphore) != 10 {
		t.Errorf("Expected semaphore capacity 10, got %d", cap(executor.semaphore))
	}
}

func TestSmartExecutor_ContextCancellation(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	executor := NewSmartExecutor(catalog)

	plan := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{StepID: "step-1"},
		},
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executor.Execute(ctx, plan)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestSmartExecutor_ParallelExecution(t *testing.T) {
	// Create catalog with multiple agents
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "agent-1",
					Address: "localhost",
					Port:    8081,
				},
				Capabilities: []EnhancedCapability{
					{Name: "cap1", Endpoint: "/api/cap1"},
				},
			},
			"agent-2": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-2",
					Name:    "agent-2",
					Address: "localhost",
					Port:    8082,
				},
				Capabilities: []EnhancedCapability{
					{Name: "cap2", Endpoint: "/api/cap2"},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Mock HTTP client
	mockRT := NewMockRoundTripper()
	mockRT.SetResponse("http://localhost:8081/api/cap1", http.StatusOK, `{"result": "result1"}`)
	mockRT.SetResponse("http://localhost:8082/api/cap2", http.StatusOK, `{"result": "result2"}`)
	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	// Create plan with parallel steps
	plan := &RoutingPlan{
		PlanID: "parallel-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "agent-1",
				Metadata: map[string]interface{}{
					"capability": "cap1",
					"parameters": map[string]interface{}{},
				},
			},
			{
				StepID:    "step-2",
				AgentName: "agent-2",
				Metadata: map[string]interface{}{
					"capability": "cap2",
					"parameters": map[string]interface{}{},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	if err != nil {
		t.Errorf("Parallel execution failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful parallel execution")
	}

	if len(result.Steps) != 2 {
		t.Errorf("Expected 2 steps executed, got %d", len(result.Steps))
	}

	// Verify both steps succeeded
	for _, step := range result.Steps {
		if !step.Success {
			t.Errorf("Step %s failed in parallel execution", step.StepID)
		}
	}
}

func TestSmartExecutor_FailedDependency(t *testing.T) {
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "test-agent",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{Name: "cap1", Endpoint: "/api/cap1"},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Mock HTTP client that returns error
	mockRT := NewMockRoundTripper()
	mockRT.SetError("http://localhost:8080/api/cap1", fmt.Errorf("service unavailable"))
	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	// Plan where step-2 depends on step-1
	plan := &RoutingPlan{
		PlanID: "dependency-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step-1",
				AgentName: "test-agent",
				Metadata: map[string]interface{}{
					"capability": "cap1",
					"parameters": map[string]interface{}{},
				},
			},
			{
				StepID:    "step-2",
				AgentName: "test-agent",
				DependsOn: []string{"step-1"},
				Metadata: map[string]interface{}{
					"capability": "cap1",
					"parameters": map[string]interface{}{},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	// If there's an error, that's ok for this test
	if err != nil {
		// Some steps may not be executable due to failed dependencies
		return
	}

	// If no error, result should exist
	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// But result should indicate failure
	if result.Success {
		t.Error("Expected execution to fail when dependency fails")
	}

	// Step 1 should have been attempted
	if len(result.Steps) == 0 {
		t.Error("Expected at least step-1 to be attempted")
	}

	// Verify step-1 failed
	step1Found := false
	for _, step := range result.Steps {
		if step.StepID == "step-1" {
			step1Found = true
			if step.Success {
				t.Error("Expected step-1 to fail")
			}
		}
	}

	if !step1Found {
		t.Error("Step-1 was not executed")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Helper to verify response parsing
func TestSmartExecutor_ResponseParsing(t *testing.T) {
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"agent-1": {
				Registration: &core.ServiceRegistration{
					ID:      "agent-1",
					Name:    "test-agent",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{Name: "test", Endpoint: "/api/test"},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Test with complex JSON response
	mockRT := NewMockRoundTripper()
	complexResponse := `{"status": "success", "data": {"value": 123, "items": ["a", "b", "c"]}}`
	mockRT.SetResponse("http://localhost:8080/api/test", http.StatusOK, complexResponse)
	executor.httpClient = &http.Client{
		Transport: mockRT,
	}

	step := RoutingStep{
		StepID:    "test-step",
		AgentName: "test-agent",
		Metadata: map[string]interface{}{
			"capability": "test",
			"parameters": map[string]interface{}{},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	if !result.Success {
		t.Errorf("Failed to parse complex response: %s", result.Error)
	}

	// Verify response can be unmarshaled back
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result.Response), &parsed); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if parsed["status"] != "success" {
		t.Error("Response parsing lost data")
	}
}

// TestCoerceValue tests the coerceValue function for Layer 2 type coercion
func TestCoerceValue(t *testing.T) {
	tests := []struct {
		name         string
		value        interface{}
		expectedType string
		want         interface{}
		wantCoerced  bool
	}{
		// Float coercion
		{
			name:         "string to float64",
			value:        "35.6897",
			expectedType: "float64",
			want:         35.6897,
			wantCoerced:  true,
		},
		{
			name:         "string to number",
			value:        "139.6917",
			expectedType: "number",
			want:         139.6917,
			wantCoerced:  true,
		},
		{
			name:         "string to float",
			value:        "-12.5",
			expectedType: "float",
			want:         -12.5,
			wantCoerced:  true,
		},
		{
			name:         "string to double",
			value:        "3.14159",
			expectedType: "double",
			want:         3.14159,
			wantCoerced:  true,
		},
		// Integer coercion
		{
			name:         "string to integer",
			value:        "42",
			expectedType: "integer",
			want:         int64(42),
			wantCoerced:  true,
		},
		{
			name:         "string to int",
			value:        "-100",
			expectedType: "int",
			want:         int64(-100),
			wantCoerced:  true,
		},
		{
			name:         "string to int64",
			value:        "9999999999",
			expectedType: "int64",
			want:         int64(9999999999),
			wantCoerced:  true,
		},
		// Boolean coercion
		{
			name:         "string true to boolean",
			value:        "true",
			expectedType: "boolean",
			want:         true,
			wantCoerced:  true,
		},
		{
			name:         "string false to bool",
			value:        "false",
			expectedType: "bool",
			want:         false,
			wantCoerced:  true,
		},
		{
			name:         "string 1 to boolean",
			value:        "1",
			expectedType: "boolean",
			want:         true,
			wantCoerced:  true,
		},
		{
			name:         "string 0 to boolean",
			value:        "0",
			expectedType: "boolean",
			want:         false,
			wantCoerced:  true,
		},
		// No coercion needed
		{
			name:         "already float64 stays unchanged",
			value:        48.8566,
			expectedType: "float64",
			want:         48.8566,
			wantCoerced:  false,
		},
		{
			name:         "already int stays unchanged",
			value:        int64(10),
			expectedType: "integer",
			want:         int64(10),
			wantCoerced:  false,
		},
		{
			name:         "already bool stays unchanged",
			value:        true,
			expectedType: "boolean",
			want:         true,
			wantCoerced:  false,
		},
		{
			name:         "string to string no coercion",
			value:        "Tokyo",
			expectedType: "string",
			want:         "Tokyo",
			wantCoerced:  false,
		},
		// Invalid coercion returns original
		{
			name:         "invalid float coercion returns original",
			value:        "not-a-number",
			expectedType: "float64",
			want:         "not-a-number",
			wantCoerced:  false,
		},
		{
			name:         "invalid int coercion returns original",
			value:        "12.5",
			expectedType: "integer",
			want:         "12.5",
			wantCoerced:  false,
		},
		{
			name:         "invalid bool coercion returns original",
			value:        "yes",
			expectedType: "boolean",
			want:         "yes",
			wantCoerced:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotCoerced := coerceValue(tt.value, tt.expectedType)
			if got != tt.want {
				t.Errorf("coerceValue() got = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
			if gotCoerced != tt.wantCoerced {
				t.Errorf("coerceValue() coerced = %v, want %v", gotCoerced, tt.wantCoerced)
			}
		})
	}
}

// TestCoerceParameterTypes tests the coerceParameterTypes function for Layer 2 type coercion
func TestCoerceParameterTypes(t *testing.T) {
	schema := []Parameter{
		{Name: "lat", Type: "float64"},
		{Name: "lon", Type: "float64"},
		{Name: "count", Type: "integer"},
		{Name: "enabled", Type: "boolean"},
		{Name: "city", Type: "string"},
	}

	tests := []struct {
		name            string
		params          map[string]interface{}
		schema          []Parameter
		expectedParams  map[string]interface{}
		expectedLogLen  int
	}{
		{
			name: "coerce string numbers to float64",
			params: map[string]interface{}{
				"lat": "35.6897",
				"lon": "139.6917",
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"lat": 35.6897,
				"lon": 139.6917,
			},
			expectedLogLen: 2,
		},
		{
			name: "coerce string to integer",
			params: map[string]interface{}{
				"count": "42",
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"count": int64(42),
			},
			expectedLogLen: 1,
		},
		{
			name: "coerce string to boolean",
			params: map[string]interface{}{
				"enabled": "true",
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"enabled": true,
			},
			expectedLogLen: 1,
		},
		{
			name: "already correct types unchanged",
			params: map[string]interface{}{
				"lat":     48.8566,
				"count":   int64(10),
				"enabled": false,
				"city":    "Paris",
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"lat":     48.8566,
				"count":   int64(10),
				"enabled": false,
				"city":    "Paris",
			},
			expectedLogLen: 0,
		},
		{
			name: "mixed coercion and unchanged",
			params: map[string]interface{}{
				"lat":     "35.6897", // needs coercion
				"lon":     139.6917,  // already correct
				"city":    "Tokyo",   // string stays string
				"enabled": "true",    // needs coercion
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"lat":     35.6897,
				"lon":     139.6917,
				"city":    "Tokyo",
				"enabled": true,
			},
			expectedLogLen: 2, // lat and enabled coerced
		},
		{
			name: "parameter not in schema passes through unchanged",
			params: map[string]interface{}{
				"lat":         "35.6897",
				"unknown_key": "some value",
			},
			schema: schema,
			expectedParams: map[string]interface{}{
				"lat":         35.6897,
				"unknown_key": "some value",
			},
			expectedLogLen: 1, // only lat coerced
		},
		{
			name:           "nil params returns nil",
			params:         nil,
			schema:         schema,
			expectedParams: nil,
			expectedLogLen: 0,
		},
		{
			name: "empty schema returns original",
			params: map[string]interface{}{
				"lat": "35.6897",
			},
			schema: []Parameter{},
			expectedParams: map[string]interface{}{
				"lat": "35.6897",
			},
			expectedLogLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotLog := coerceParameterTypes(tt.params, tt.schema)

			// Check log length
			if len(gotLog) != tt.expectedLogLen {
				t.Errorf("coerceParameterTypes() log length = %d, want %d", len(gotLog), tt.expectedLogLen)
			}

			// Check nil case
			if tt.expectedParams == nil {
				if got != nil {
					t.Errorf("coerceParameterTypes() got = %v, want nil", got)
				}
				return
			}

			// Check each expected parameter
			for key, expectedVal := range tt.expectedParams {
				gotVal, exists := got[key]
				if !exists {
					t.Errorf("coerceParameterTypes() missing key %s", key)
					continue
				}
				if gotVal != expectedVal {
					t.Errorf("coerceParameterTypes() got[%s] = %v (%T), want %v (%T)",
						key, gotVal, gotVal, expectedVal, expectedVal)
				}
			}
		})
	}
}

// TestFindCapabilitySchema tests the findCapabilitySchema helper function
func TestFindCapabilitySchema(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
	}
	executor := NewSmartExecutor(catalog)

	agentInfo := &AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:   "test-agent",
			Name: "test-agent",
		},
		Capabilities: []EnhancedCapability{
			{
				Name:     "get_weather",
				Endpoint: "/api/weather",
				Parameters: []Parameter{
					{Name: "lat", Type: "float64", Required: true},
					{Name: "lon", Type: "float64", Required: true},
				},
			},
			{
				Name:     "get_stock",
				Endpoint: "/api/stock",
				Parameters: []Parameter{
					{Name: "symbol", Type: "string", Required: true},
				},
			},
		},
	}

	tests := []struct {
		name       string
		agentInfo  *AgentInfo
		capability string
		wantNil    bool
		wantName   string
	}{
		{
			name:       "find existing capability",
			agentInfo:  agentInfo,
			capability: "get_weather",
			wantNil:    false,
			wantName:   "get_weather",
		},
		{
			name:       "find another capability",
			agentInfo:  agentInfo,
			capability: "get_stock",
			wantNil:    false,
			wantName:   "get_stock",
		},
		{
			name:       "capability not found",
			agentInfo:  agentInfo,
			capability: "non_existent",
			wantNil:    true,
		},
		{
			name:       "nil agent info",
			agentInfo:  nil,
			capability: "get_weather",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.findCapabilitySchema(tt.agentInfo, tt.capability)

			if tt.wantNil {
				if got != nil {
					t.Errorf("findCapabilitySchema() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("findCapabilitySchema() = nil, want non-nil")
			}

			if got.Name != tt.wantName {
				t.Errorf("findCapabilitySchema().Name = %s, want %s", got.Name, tt.wantName)
			}
		})
	}
}

// TestSmartExecutor_TypeCoercionIntegration tests end-to-end type coercion in executeStep
func TestSmartExecutor_TypeCoercionIntegration(t *testing.T) {
	// Create catalog with agent that has typed parameters
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"weather-tool": {
				Registration: &core.ServiceRegistration{
					ID:      "weather-tool",
					Name:    "weather-tool",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "get_weather",
						Endpoint: "/api/weather",
						Parameters: []Parameter{
							{Name: "lat", Type: "float64", Required: true},
							{Name: "lon", Type: "float64", Required: true},
						},
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Track what parameters were actually sent
	var sentParams map[string]interface{}
	mockRT := &trackingRoundTripper{
		onRequest: func(req *http.Request) {
			var params map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&params); err == nil {
				sentParams = params
			}
		},
		response: `{"temperature": 25, "condition": "sunny"}`,
	}
	executor.httpClient = &http.Client{Transport: mockRT}

	// Create step with STRING parameters (as LLM would generate)
	step := RoutingStep{
		StepID:    "weather-step",
		AgentName: "weather-tool",
		Metadata: map[string]interface{}{
			"capability": "get_weather",
			"parameters": map[string]interface{}{
				"lat": "35.6897",  // STRING - should be coerced to float64
				"lon": "139.6917", // STRING - should be coerced to float64
			},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	if !result.Success {
		t.Fatalf("Step execution failed: %s", result.Error)
	}

	// Verify the parameters were coerced before being sent
	if sentParams == nil {
		t.Fatal("No parameters were sent")
	}

	// Check lat was coerced from string to float64
	lat, ok := sentParams["lat"].(float64)
	if !ok {
		t.Errorf("lat should be float64 after coercion, got %T", sentParams["lat"])
	} else if lat != 35.6897 {
		t.Errorf("lat = %v, want 35.6897", lat)
	}

	// Check lon was coerced from string to float64
	lon, ok := sentParams["lon"].(float64)
	if !ok {
		t.Errorf("lon should be float64 after coercion, got %T", sentParams["lon"])
	} else if lon != 139.6917 {
		t.Errorf("lon = %v, want 139.6917", lon)
	}
}

// trackingRoundTripper is a mock HTTP transport that tracks request parameters
type trackingRoundTripper struct {
	onRequest func(req *http.Request)
	response  string
}

func (t *trackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.onRequest != nil {
		t.onRequest(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(t.response)),
		Header:     make(http.Header),
	}, nil
}

// ============================================================================
// Layer 3: Validation Feedback Tests
// ============================================================================

// TestIsTypeRelatedError tests the isTypeRelatedError function for Layer 3
func TestIsTypeRelatedError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		responseBody string
		want         bool
	}{
		// Positive cases - should be detected as type errors
		{
			name:         "json unmarshal string into float64",
			err:          fmt.Errorf("json: cannot unmarshal string into Go struct field WeatherRequest.lat of type float64"),
			responseBody: "",
			want:         true,
		},
		{
			name:         "json unmarshal number into string",
			err:          fmt.Errorf("json: cannot unmarshal number into Go struct field .name of type string"),
			responseBody: "",
			want:         true,
		},
		{
			name:         "json unmarshal bool into int",
			err:          fmt.Errorf("json: cannot unmarshal bool into Go struct field .count of type int"),
			responseBody: "",
			want:         true,
		},
		{
			name:         "type mismatch in error",
			err:          fmt.Errorf("type mismatch: expected number, got string"),
			responseBody: "",
			want:         true,
		},
		{
			name:         "invalid type in error",
			err:          fmt.Errorf("invalid type for field lat"),
			responseBody: "",
			want:         true,
		},
		{
			name:         "expected number in response body",
			err:          fmt.Errorf("validation failed"),
			responseBody: `{"error": "expected number for field latitude"}`,
			want:         true,
		},
		{
			name:         "expected string in response body",
			err:          fmt.Errorf("validation failed"),
			responseBody: `{"error": "expected string for field name"}`,
			want:         true,
		},
		{
			name:         "expected boolean in response body",
			err:          fmt.Errorf("validation failed"),
			responseBody: `{"error": "expected boolean for field enabled"}`,
			want:         true,
		},
		{
			name:         "invalid value in error",
			err:          fmt.Errorf("invalid value for field count"),
			responseBody: "",
			want:         true,
		},
		// Negative cases - should NOT be detected as type errors
		{
			name:         "connection refused",
			err:          fmt.Errorf("connection refused"),
			responseBody: "",
			want:         false,
		},
		{
			name:         "timeout error",
			err:          fmt.Errorf("request timeout"),
			responseBody: "",
			want:         false,
		},
		{
			name:         "not found error",
			err:          fmt.Errorf("agent returned status 404"),
			responseBody: `{"error": "capability not found"}`,
			want:         false,
		},
		{
			name:         "authorization error",
			err:          fmt.Errorf("unauthorized"),
			responseBody: "",
			want:         false,
		},
		{
			name:         "generic server error",
			err:          fmt.Errorf("internal server error"),
			responseBody: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTypeRelatedError(tt.err, tt.responseBody)
			if got != tt.want {
				t.Errorf("isTypeRelatedError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSmartExecutor_ValidationFeedback tests Layer 3 validation feedback integration
func TestSmartExecutor_ValidationFeedback(t *testing.T) {
	// Create catalog with agent but NO schema (to bypass Layer 2 coercion)
	// This tests Layer 3 as the PRIMARY defense mechanism
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"weather-tool": {
				Registration: &core.ServiceRegistration{
					ID:      "weather-tool",
					Name:    "weather-tool",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "get_weather",
						Endpoint: "/api/weather",
						// NO Parameters defined - Layer 2 won't coerce
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Track calls and parameters
	callCount := 0
	var sentParams []map[string]interface{}

	// Mock transport that fails on first call with type error, succeeds on second
	mockRT := &validationFeedbackRoundTripper{
		onRequest: func(req *http.Request) map[string]interface{} {
			var params map[string]interface{}
			json.NewDecoder(req.Body).Decode(&params)
			sentParams = append(sentParams, params)
			return params
		},
		getResponse: func(callNum int, params map[string]interface{}) (int, string) {
			callCount++
			if callCount == 1 {
				// First call: simulate type error
				return http.StatusBadRequest, `{"error": "json: cannot unmarshal string into Go struct field .lat of type float64"}`
			}
			// Second call: success
			return http.StatusOK, `{"temperature": 25, "condition": "sunny"}`
		},
	}
	executor.httpClient = &http.Client{Transport: mockRT}

	// Track what the callback receives
	var callbackParams map[string]interface{}
	var callbackErrMsg string
	correctionCalled := false

	executor.SetCorrectionCallback(func(ctx context.Context, step RoutingStep, params map[string]interface{}, errMsg string, schema *EnhancedCapability) (map[string]interface{}, error) {
		correctionCalled = true
		callbackParams = params
		callbackErrMsg = errMsg
		// Return corrected parameters with proper types
		return map[string]interface{}{
			"lat": 35.6897,  // Fixed: now a float64
			"lon": 139.6917, // Fixed: now a float64
		}, nil
	})
	executor.SetValidationFeedback(true, 2)

	// Create step with STRING parameters (as LLM would generate incorrectly)
	step := RoutingStep{
		StepID:    "weather-step",
		AgentName: "weather-tool",
		Metadata: map[string]interface{}{
			"capability": "get_weather",
			"parameters": map[string]interface{}{
				"lat": "35.6897",  // STRING - will cause type error
				"lon": "139.6917", // STRING - will cause type error
			},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	// Verify correction was called
	if !correctionCalled {
		t.Fatal("Correction callback was not called")
	}

	// Verify callback received the original (incorrect) parameters
	if lat, ok := callbackParams["lat"].(string); !ok || lat != "35.6897" {
		t.Errorf("Callback should receive original string param, got lat=%v (%T)", callbackParams["lat"], callbackParams["lat"])
	}

	// Verify callback received error message containing type error
	if !strings.Contains(callbackErrMsg, "cannot unmarshal string") {
		t.Errorf("Callback should receive type error message, got: %s", callbackErrMsg)
	}

	// Verify step succeeded after correction
	if !result.Success {
		t.Fatalf("Step execution failed: %s", result.Error)
	}

	// Verify two HTTP calls were made (first failed, second succeeded)
	if callCount != 2 {
		t.Errorf("Expected 2 HTTP calls, got %d", callCount)
	}

	// Verify the SECOND call used corrected parameters (float64, not string)
	if len(sentParams) < 2 {
		t.Fatalf("Expected at least 2 parameter sets, got %d", len(sentParams))
	}

	// First call should have string params (original)
	if lat, ok := sentParams[0]["lat"].(string); !ok || lat != "35.6897" {
		t.Errorf("First call should have string lat, got %v (%T)", sentParams[0]["lat"], sentParams[0]["lat"])
	}

	// Second call should have float64 params (corrected by callback)
	if lat, ok := sentParams[1]["lat"].(float64); !ok || lat != 35.6897 {
		t.Errorf("Second call should have float64 lat=35.6897, got %v (%T)", sentParams[1]["lat"], sentParams[1]["lat"])
	}
}

// TestSmartExecutor_ValidationFeedbackWithLayer2 tests Layer 3 when Layer 2 coercion exists
// This simulates an edge case where the tool still fails despite Layer 2 coercion
func TestSmartExecutor_ValidationFeedbackWithLayer2(t *testing.T) {
	// Create catalog with schema (Layer 2 will coerce)
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"weather-tool": {
				Registration: &core.ServiceRegistration{
					ID:      "weather-tool",
					Name:    "weather-tool",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "get_weather",
						Endpoint: "/api/weather",
						Parameters: []Parameter{
							{Name: "lat", Type: "float64", Required: true},
							{Name: "lon", Type: "float64", Required: true},
						},
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	callCount := 0
	var sentParams []map[string]interface{}

	mockRT := &validationFeedbackRoundTripper{
		onRequest: func(req *http.Request) map[string]interface{} {
			var params map[string]interface{}
			json.NewDecoder(req.Body).Decode(&params)
			sentParams = append(sentParams, params)
			return params
		},
		getResponse: func(callNum int, params map[string]interface{}) (int, string) {
			callCount++
			if callCount == 1 {
				// Simulate a tool that validates more strictly (e.g., range check fails)
				return http.StatusBadRequest, `{"error": "invalid value for latitude: expected number in range -90 to 90"}`
			}
			return http.StatusOK, `{"temperature": 25}`
		},
	}
	executor.httpClient = &http.Client{Transport: mockRT}

	correctionCalled := false
	executor.SetCorrectionCallback(func(ctx context.Context, step RoutingStep, params map[string]interface{}, errMsg string, schema *EnhancedCapability) (map[string]interface{}, error) {
		correctionCalled = true
		// LLM correction - maybe it was using wrong coordinates
		return map[string]interface{}{
			"lat": 35.6897,
			"lon": 139.6917,
		}, nil
	})
	executor.SetValidationFeedback(true, 2)

	step := RoutingStep{
		StepID:    "weather-step",
		AgentName: "weather-tool",
		Metadata: map[string]interface{}{
			"capability": "get_weather",
			"parameters": map[string]interface{}{
				"lat": "35.6897",
				"lon": "139.6917",
			},
		},
	}

	result := executor.executeStep(context.Background(), step)

	// Layer 2 should have coerced the params, so first call should have float64
	if len(sentParams) > 0 {
		if lat, ok := sentParams[0]["lat"].(float64); !ok {
			t.Errorf("After Layer 2, first call should have float64 lat, got %T", sentParams[0]["lat"])
		} else if lat != 35.6897 {
			t.Errorf("Layer 2 coercion should produce 35.6897, got %v", lat)
		}
	}

	// Layer 3 should still be triggered by "invalid value" error
	if !correctionCalled {
		t.Error("Layer 3 correction should be called even after Layer 2 coercion (for edge cases)")
	}

	if !result.Success {
		t.Errorf("Step should succeed after correction: %s", result.Error)
	}
}

// validationFeedbackRoundTripper is a mock transport for validation feedback tests
type validationFeedbackRoundTripper struct {
	onRequest   func(req *http.Request) map[string]interface{}
	getResponse func(callNum int, params map[string]interface{}) (int, string)
	callNum     int
}

func (v *validationFeedbackRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	v.callNum++
	var params map[string]interface{}
	if v.onRequest != nil {
		params = v.onRequest(req)
	}
	statusCode, body := v.getResponse(v.callNum, params)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

// TestSmartExecutor_ValidationFeedbackDisabled tests that validation feedback can be disabled
func TestSmartExecutor_ValidationFeedbackDisabled(t *testing.T) {
	catalog := &AgentCatalog{
		agents: map[string]*AgentInfo{
			"weather-tool": {
				Registration: &core.ServiceRegistration{
					ID:      "weather-tool",
					Name:    "weather-tool",
					Address: "localhost",
					Port:    8080,
				},
				Capabilities: []EnhancedCapability{
					{
						Name:     "get_weather",
						Endpoint: "/api/weather",
						Parameters: []Parameter{
							{Name: "lat", Type: "float64", Required: true},
						},
					},
				},
			},
		},
	}

	executor := NewSmartExecutor(catalog)

	// Mock transport that always fails with type error
	callCount := 0
	mockRT := &validationFeedbackRoundTripper{
		getResponse: func(callNum int, params map[string]interface{}) (int, string) {
			callCount++
			return http.StatusBadRequest, `{"error": "json: cannot unmarshal string into float64"}`
		},
	}
	executor.httpClient = &http.Client{Transport: mockRT}

	// Disable validation feedback
	executor.SetValidationFeedback(false, 0)

	// Set callback that should NOT be called
	callbackCalled := false
	executor.SetCorrectionCallback(func(ctx context.Context, step RoutingStep, params map[string]interface{}, errMsg string, schema *EnhancedCapability) (map[string]interface{}, error) {
		callbackCalled = true
		return params, nil
	})

	step := RoutingStep{
		StepID:    "weather-step",
		AgentName: "weather-tool",
		Metadata: map[string]interface{}{
			"capability": "get_weather",
			"parameters": map[string]interface{}{"lat": "35.6897"},
		},
	}

	ctx := context.Background()
	result := executor.executeStep(ctx, step)

	// Callback should NOT have been called
	if callbackCalled {
		t.Error("Correction callback was called when validation feedback was disabled")
	}

	// Step should fail (no correction attempted)
	if result.Success {
		t.Error("Step should have failed when validation feedback is disabled")
	}
}
