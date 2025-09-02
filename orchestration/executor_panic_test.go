package orchestration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// Test basic panic recovery in step execution
func TestSmartExecutor_PanicRecovery(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	// Add a test agent that will cause panic
	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "test-agent",
			Name:    "panic-agent",
			Address: "localhost",
			Port:    8080,
		},
		Capabilities: []core.Capability{
			{Name: "panic-capability", Endpoint: "/api/panic"},
		},
	})

	executor := NewSmartExecutor(catalog)

	// Override executeStep to simulate panic
	originalExecuteStep := executor.executeStep
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		if step.AgentName == "panic-agent" {
			panic("simulated step panic")
		}
		return originalExecuteStep(ctx, step)
	}

	plan := &RoutingPlan{
		PlanID: "test-plan",
		Steps: []RoutingStep{
			{
				StepID:    "step1",
				AgentName: "panic-agent",
				Namespace: "test",
				Metadata: map[string]interface{}{
					"capability": "panic-capability",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	// Should not error, but result should show failure
	if err != nil {
		t.Fatalf("Expected no error from Execute, got: %v", err)
	}

	if result.Success {
		t.Error("Expected execution to fail due to panic")
	}

	if len(result.Steps) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(result.Steps))
	}

	stepResult := result.Steps[0]
	if stepResult.Success {
		t.Error("Expected step to fail due to panic")
	}

	if !strings.Contains(stepResult.Error, "panic") {
		t.Errorf("Expected error to mention panic, got: %s", stepResult.Error)
	}
}

// Test concurrent panics in multiple steps
func TestSmartExecutor_ConcurrentPanics(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	// Add multiple test agents
	for i := 0; i < 5; i++ {
		catalog.RegisterAgent(&AgentInfo{
			Registration: &core.ServiceRegistration{
				ID:      fmt.Sprintf("agent-%d", i),
				Name:    fmt.Sprintf("agent-%d", i),
				Address: "localhost",
				Port:    8080 + i,
			},
			Capabilities: []core.Capability{
				{Name: "test", Endpoint: "/api/test"},
			},
		})
	}

	executor := NewSmartExecutor(catalog)
	panicCount := int32(0)

	// Override executeStep to simulate random panics
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		// Agents 1 and 3 will panic
		if step.AgentName == "agent-1" || step.AgentName == "agent-3" {
			atomic.AddInt32(&panicCount, 1)
			panic(fmt.Sprintf("panic from %s", step.AgentName))
		}
		return StepResult{
			StepID:    step.StepID,
			AgentName: step.AgentName,
			Success:   true,
			Response:  "ok",
		}
	}

	// Create plan with independent steps (can run concurrently)
	var steps []RoutingStep
	for i := 0; i < 5; i++ {
		steps = append(steps, RoutingStep{
			StepID:    fmt.Sprintf("step-%d", i),
			AgentName: fmt.Sprintf("agent-%d", i),
			Namespace: "test",
			Metadata: map[string]interface{}{
				"capability": "test",
			},
		})
	}

	plan := &RoutingPlan{
		PlanID: "concurrent-test",
		Steps:  steps,
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	if err != nil {
		t.Fatalf("Expected no error from Execute, got: %v", err)
	}

	if result.Success {
		t.Error("Expected execution to fail due to panics")
	}

	// Check that we got all step results
	if len(result.Steps) != 5 {
		t.Errorf("Expected 5 step results, got %d", len(result.Steps))
	}

	// Verify panic count
	if atomic.LoadInt32(&panicCount) != 2 {
		t.Errorf("Expected 2 panics, got %d", atomic.LoadInt32(&panicCount))
	}

	// Check specific steps failed
	failedSteps := 0
	for _, step := range result.Steps {
		if !step.Success {
			failedSteps++
			if !strings.Contains(step.Error, "panic") {
				t.Errorf("Failed step should mention panic: %s", step.Error)
			}
		}
	}

	if failedSteps != 2 {
		t.Errorf("Expected 2 failed steps, got %d", failedSteps)
	}
}

// Test panic with dependencies
func TestSmartExecutor_PanicWithDependencies(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "agent1",
			Name:    "agent1",
			Address: "localhost",
			Port:    8080,
		},
		Capabilities: []core.Capability{
			{Name: "test", Endpoint: "/api/test"},
		},
	})

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "agent2",
			Name:    "agent2",
			Address: "localhost",
			Port:    8081,
		},
		Capabilities: []core.Capability{
			{Name: "test", Endpoint: "/api/test"},
		},
	})

	executor := NewSmartExecutor(catalog)

	// First step will panic
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		if step.StepID == "step1" {
			panic("first step panic")
		}
		return StepResult{
			StepID:    step.StepID,
			AgentName: step.AgentName,
			Success:   true,
			Response:  "ok",
		}
	}

	plan := &RoutingPlan{
		PlanID: "dependency-test",
		Steps: []RoutingStep{
			{
				StepID:    "step1",
				AgentName: "agent1",
				Namespace: "test",
				DependsOn: []string{}, // No dependencies
			},
			{
				StepID:    "step2",
				AgentName: "agent2",
				Namespace: "test",
				DependsOn: []string{"step1"}, // Depends on panicking step
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	if err != nil {
		t.Fatalf("Expected no error from Execute, got: %v", err)
	}

	// step2 should not execute because step1 failed
	executedSteps := 0
	for _, step := range result.Steps {
		if step.StepID != "" {
			executedSteps++
		}
	}

	// Only step1 should have been executed (and failed)
	if executedSteps != 1 {
		t.Errorf("Expected only 1 step to execute, got %d", executedSteps)
	}

	if result.Steps[0].StepID != "step1" {
		t.Error("Expected step1 to be executed")
	}

	if result.Steps[0].Success {
		t.Error("Expected step1 to fail")
	}
}

// Test panic in findReadySteps
func TestSmartExecutor_PanicInFindReadySteps(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	executor := NewSmartExecutor(catalog)

	// Create a plan with nil step that could cause panic
	plan := &RoutingPlan{
		PlanID: "test",
		Steps: []RoutingStep{
			{}, // Empty step that might cause issues
		},
	}

	ctx := context.Background()
	
	// This should handle the situation gracefully
	result, err := executor.Execute(ctx, plan)
	
	// Should complete without panic
	if err == nil && result != nil {
		// The execution might fail but shouldn't panic
		t.Log("Execution handled gracefully")
	}
}

// Test panic during HTTP call
func TestSmartExecutor_PanicDuringHTTPCall(t *testing.T) {
	// Create a test server that causes client-side issues
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write invalid response that might cause panic during decode
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json")) // Invalid JSON
	}))
	defer server.Close()

	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	// Parse server URL to get port
	serverURL := strings.TrimPrefix(server.URL, "http://")
	parts := strings.Split(serverURL, ":")
	port := 80
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &port)
	}

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "http-agent",
			Name:    "http-agent",
			Address: parts[0],
			Port:    port,
		},
		Capabilities: []core.Capability{
			{Name: "test", Endpoint: "/"},
		},
	})

	executor := NewSmartExecutor(catalog)

	plan := &RoutingPlan{
		PlanID: "http-test",
		Steps: []RoutingStep{
			{
				StepID:    "http-step",
				AgentName: "http-agent",
				Namespace: "test",
				Metadata: map[string]interface{}{
					"capability": "test",
					"parameters": map[string]interface{}{"test": "value"},
				},
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	// Should handle the HTTP error gracefully
	if err != nil {
		t.Fatalf("Expected no error from Execute, got: %v", err)
	}

	if result.Success {
		t.Error("Expected execution to fail due to invalid response")
	}

	if len(result.Steps) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(result.Steps))
	}

	if result.Steps[0].Success {
		t.Error("Expected step to fail")
	}

	if !strings.Contains(result.Steps[0].Error, "decode") {
		t.Logf("Error might not mention decode explicitly: %s", result.Steps[0].Error)
	}
}

// Test panic with context cancellation
func TestSmartExecutor_PanicWithContextCancellation(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "slow-agent",
			Name:    "slow-agent",
			Address: "localhost",
			Port:    8080,
		},
		Capabilities: []core.Capability{
			{Name: "test", Endpoint: "/api/test"},
		},
	})

	executor := NewSmartExecutor(catalog)

	// Override to simulate slow operation that panics
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		select {
		case <-time.After(100 * time.Millisecond):
			panic("panic after delay")
		case <-ctx.Done():
			return StepResult{
				StepID:  step.StepID,
				Success: false,
				Error:   "context cancelled",
			}
		}
	}

	plan := &RoutingPlan{
		PlanID: "cancel-test",
		Steps: []RoutingStep{
			{
				StepID:    "step1",
				AgentName: "slow-agent",
				Namespace: "test",
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := executor.Execute(ctx, plan)

	// Should get context cancellation error
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if result != nil {
		t.Log("Got partial result despite cancellation")
	}
}

// Test start time preservation in panic
func TestSmartExecutor_StartTimePreservationInPanic(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "test-agent",
			Name:    "test-agent",
			Address: "localhost",
			Port:    8080,
		},
	})

	executor := NewSmartExecutor(catalog)

	// Override to panic after delay
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		time.Sleep(100 * time.Millisecond) // Simulate work
		panic("panic after work")
	}

	plan := &RoutingPlan{
		PlanID: "timing-test",
		Steps: []RoutingStep{
			{
				StepID:    "step1",
				AgentName: "test-agent",
				Namespace: "test",
			},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, plan)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(result.Steps))
	}

	stepResult := result.Steps[0]
	
	// Duration should be at least 100ms
	if stepResult.Duration < 100*time.Millisecond {
		t.Errorf("Duration should be at least 100ms, got %v", stepResult.Duration)
	}

	// StartTime should be before now minus duration
	expectedStartTime := time.Now().Add(-stepResult.Duration)
	if stepResult.StartTime.After(expectedStartTime.Add(50 * time.Millisecond)) {
		t.Error("StartTime not preserved correctly in panic handler")
	}
}

// Test no deadlock with mutex in panic
func TestSmartExecutor_NoDeadlockInPanic(t *testing.T) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	// Add multiple agents
	for i := 0; i < 10; i++ {
		catalog.RegisterAgent(&AgentInfo{
			Registration: &core.ServiceRegistration{
				ID:      fmt.Sprintf("agent-%d", i),
				Name:    fmt.Sprintf("agent-%d", i),
				Address: "localhost",
				Port:    8080 + i,
			},
		})
	}

	executor := NewSmartExecutor(catalog)

	// All steps will panic
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		panic(fmt.Sprintf("panic from %s", step.StepID))
	}

	// Create many concurrent steps
	var steps []RoutingStep
	for i := 0; i < 10; i++ {
		steps = append(steps, RoutingStep{
			StepID:    fmt.Sprintf("step-%d", i),
			AgentName: fmt.Sprintf("agent-%d", i),
			Namespace: "test",
		})
	}

	plan := &RoutingPlan{
		PlanID: "deadlock-test",
		Steps:  steps,
	}

	ctx := context.Background()
	
	// Should complete without deadlock
	done := make(chan bool)
	go func() {
		result, err := executor.Execute(ctx, plan)
		if err != nil {
			t.Logf("Execution error: %v", err)
		}
		if result != nil && len(result.Steps) == 10 {
			t.Log("All steps processed despite panics")
		}
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Execution deadlocked")
	}
}

// Benchmark panic recovery overhead
func BenchmarkSmartExecutor_PanicRecovery(b *testing.B) {
	catalog := &AgentCatalog{
		agents: make(map[string]*AgentInfo),
		mu:     sync.RWMutex{},
	}

	catalog.RegisterAgent(&AgentInfo{
		Registration: &core.ServiceRegistration{
			ID:      "bench-agent",
			Name:    "bench-agent",
			Address: "localhost",
			Port:    8080,
		},
	})

	executor := NewSmartExecutor(catalog)

	// Alternate between panic and success
	counter := 0
	executor.executeStep = func(ctx context.Context, step RoutingStep) StepResult {
		counter++
		if counter%2 == 0 {
			panic("benchmark panic")
		}
		return StepResult{
			StepID:  step.StepID,
			Success: true,
		}
	}

	plan := &RoutingPlan{
		PlanID: "bench",
		Steps: []RoutingStep{
			{
				StepID:    "step1",
				AgentName: "bench-agent",
				Namespace: "test",
			},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, plan)
	}
}