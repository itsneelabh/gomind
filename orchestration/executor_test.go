package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
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
