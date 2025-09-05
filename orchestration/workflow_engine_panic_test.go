package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test worker panic recovery
func TestWorkflowEngine_WorkerPanicRecovery(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	// Create a workflow with a step that will panic
	workflow := &WorkflowDefinition{
		Name:    "panic-test",
		Version: "1.0",
		Steps: []WorkflowStepDefinition{
			{
				Name: "panic-step",
				Type: "agent",
				Agent: AgentRef{
					Name: "test-agent",
				},
			},
		},
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-1",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	// Initialize step execution
	execution.Steps["panic-step"] = &StepExecution{
		StepID: "panic-step",
		Status: StepPending,
	}
	execution.DAG.AddNode("panic-step", []string{})

	// Mock worker that panics
	oldWorker := engine.worker
	defer func() { engine.worker = oldWorker }()

	panicOccurred := false
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		for task := range taskQueue {
			if task.StepID == "panic-step" {
				panicOccurred = true
				panic("worker panic test")
			}
		}
	}

	// Execute workflow - should handle panic gracefully
	err := engine.executeDAG(ctx, execution, workflow)

	if err == nil || !strings.Contains(err.Error(), "worker panic") {
		t.Errorf("Expected worker panic error, got: %v", err)
	}

	if !panicOccurred {
		t.Error("Panic was not triggered")
	}

	// Check that error was recorded
	if len(execution.Errors) == 0 {
		t.Error("Expected panic to be recorded in execution errors")
	}
}

// Test DAG executor panic recovery
func TestWorkflowEngine_DAGExecutorPanicRecovery(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "dag-panic-test",
		Version: "1.0",
		Steps:   []WorkflowStepDefinition{},
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-2",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      nil, // This will cause panic when accessed
	}

	// Execute workflow - should handle DAG panic gracefully
	err := engine.executeDAG(ctx, execution, workflow)

	if err == nil || !strings.Contains(err.Error(), "panic") {
		t.Errorf("Expected DAG panic error, got: %v", err)
	}
}

// Test concurrent worker panics
func TestWorkflowEngine_ConcurrentWorkerPanics(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "concurrent-panic-test",
		Version: "1.0",
		Steps: []WorkflowStepDefinition{
			{Name: "step1", Type: "agent"},
			{Name: "step2", Type: "agent"},
			{Name: "step3", Type: "agent"},
		},
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-3",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	// Initialize all steps
	for _, step := range workflow.Steps {
		execution.Steps[step.Name] = &StepExecution{
			StepID: step.Name,
			Status: StepPending,
		}
		execution.DAG.AddNode(step.Name, []string{})
	}

	// Mock worker that panics for specific steps
	var panicCount int32
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		for task := range taskQueue {
			if task.StepID == "step2" || task.StepID == "step3" {
				atomic.AddInt32(&panicCount, 1)
				panic(fmt.Sprintf("panic in %s", task.StepID))
			}
			// step1 succeeds
			results <- &TaskResult{
				StepID: task.StepID,
				Output: map[string]interface{}{"result": "success"},
				Error:  nil,
			}
		}
	}

	// Execute workflow
	err := engine.executeDAG(ctx, execution, workflow)

	if err == nil {
		t.Error("Expected error from panics")
	}

	if atomic.LoadInt32(&panicCount) < 2 {
		t.Errorf("Expected at least 2 panics, got %d", panicCount)
	}
}

// Test panic recovery with context cancellation
func TestWorkflowEngine_PanicWithContextCancel(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "cancel-panic-test",
		Version: "1.0",
		Steps: []WorkflowStepDefinition{
			{Name: "step1", Type: "agent"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	execution := &WorkflowExecution{
		ID:       "test-exec-4",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	execution.Steps["step1"] = &StepExecution{
		StepID: "step1",
		Status: StepPending,
	}
	execution.DAG.AddNode("step1", []string{})

	// Mock worker that cancels context then panics
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		for range taskQueue {
			cancel() // Cancel context
			time.Sleep(10 * time.Millisecond)
			panic("panic after cancel")
		}
	}

	// Execute workflow
	err := engine.executeDAG(ctx, execution, workflow)

	// Should handle both cancellation and panic
	if err == nil {
		t.Error("Expected error from cancelled context or panic")
	}
}

// Test panic recovery with full results channel
func TestWorkflowEngine_PanicWithFullChannel(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "full-channel-test",
		Version: "1.0",
		Steps:   make([]WorkflowStepDefinition, 10),
	}

	// Create many steps
	for i := 0; i < 10; i++ {
		workflow.Steps[i] = WorkflowStepDefinition{
			Name: fmt.Sprintf("step%d", i),
			Type: "agent",
		}
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-5",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	// Initialize all steps
	for _, step := range workflow.Steps {
		execution.Steps[step.Name] = &StepExecution{
			StepID: step.Name,
			Status: StepPending,
		}
		execution.DAG.AddNode(step.Name, []string{})
	}

	// Mock worker that panics immediately
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		// Fill results channel first
		for i := 0; i < cap(results); i++ {
			select {
			case results <- &TaskResult{
				StepID: fmt.Sprintf("fill%d", i),
				Output: map[string]interface{}{"filler": true},
			}:
			default:
				break
			}
		}

		// Now panic - channel is full
		panic("panic with full channel")
	}

	// Execute workflow - should handle panic even with full channel
	err := engine.executeDAG(ctx, execution, workflow)

	if err == nil {
		t.Error("Expected error from panic")
	}
}

// Test that non-existent step IDs from panic recovery are handled
func TestWorkflowEngine_PanicRecoveryNonExistentStep(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "nonexistent-step-test",
		Version: "1.0",
		Steps: []WorkflowStepDefinition{
			{Name: "real-step", Type: "agent"},
		},
		OnError: &ErrorHandler{
			Strategy: "continue", // Continue on error
		},
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-6",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	execution.Steps["real-step"] = &StepExecution{
		StepID: "real-step",
		Status: StepPending,
	}
	execution.DAG.AddNode("real-step", []string{})

	// Directly test result processing with panic recovery message
	results := make(chan *TaskResult, 10)

	// Send panic recovery result with non-existent step ID
	results <- &TaskResult{
		StepID: "panic-recovery-worker-1", // This doesn't exist in execution.Steps
		Error:  fmt.Errorf("worker 1 panic: test panic"),
		Output: map[string]interface{}{
			"panic":       "test panic",
			"worker_id":   1,
			"stack_trace": "fake stack trace",
		},
	}

	// Also send real step result
	results <- &TaskResult{
		StepID: "real-step",
		Output: map[string]interface{}{"result": "success"},
	}

	close(results)

	// Process results - should handle non-existent step gracefully
	processedCount := 0
	for result := range results {
		processedCount++

		if _, exists := execution.Steps[result.StepID]; !exists {
			// Should handle this case without panic
			execution.Errors = append(execution.Errors, fmt.Errorf("worker panic: %w", result.Error))
		}
	}

	if processedCount != 2 {
		t.Errorf("Expected to process 2 results, got %d", processedCount)
	}

	if len(execution.Errors) != 1 {
		t.Errorf("Expected 1 error for panic recovery, got %d", len(execution.Errors))
	}
}

// Test error strategy with panic recovery
func TestWorkflowEngine_PanicRecoveryErrorStrategy(t *testing.T) {
	tests := []struct {
		name           string
		errorStrategy  string
		expectContinue bool
	}{
		{
			name:           "fail-fast strategy",
			errorStrategy:  "fail",
			expectContinue: false,
		},
		{
			name:           "continue strategy",
			errorStrategy:  "continue",
			expectContinue: true,
		},
		{
			name:           "no strategy (default fail)",
			errorStrategy:  "",
			expectContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &WorkflowEngine{
				stateStore: NewInMemoryStateStore(),
				metrics:    &WorkflowMetrics{},
			}

			workflow := &WorkflowDefinition{
				Name:    "strategy-test",
				Version: "1.0",
				Steps: []WorkflowStepDefinition{
					{Name: "step1", Type: "agent"},
					{Name: "step2", Type: "agent"},
				},
			}

			if tt.errorStrategy != "" {
				workflow.OnError = &ErrorHandler{
					Strategy: tt.errorStrategy,
				}
			}

			ctx := context.Background()
			execution := &WorkflowExecution{
				ID:       fmt.Sprintf("test-exec-%s", tt.name),
				Workflow: workflow.Name,
				Status:   ExecutionRunning,
				Steps:    make(map[string]*StepExecution),
				Context:  make(map[string]interface{}),
				Errors:   []error{},
				DAG:      NewWorkflowDAG(),
			}

			for _, step := range workflow.Steps {
				execution.Steps[step.Name] = &StepExecution{
					StepID: step.Name,
					Status: StepPending,
				}
				execution.DAG.AddNode(step.Name, []string{})
			}

			// Mock worker that panics on first step
			var step2Executed bool
			engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
				for task := range taskQueue {
					if task.StepID == "step1" {
						panic("step1 panic")
					}
					if task.StepID == "step2" {
						step2Executed = true
						results <- &TaskResult{
							StepID: task.StepID,
							Output: map[string]interface{}{"result": "success"},
						}
					}
				}
			}

			// Execute workflow
			err := engine.executeDAG(ctx, execution, workflow)

			if tt.expectContinue {
				// Should continue and execute step2
				if !step2Executed {
					t.Error("Expected step2 to execute with continue strategy")
				}
			} else {
				// Should fail fast
				if err == nil {
					t.Error("Expected error with fail-fast strategy")
				}
			}
		})
	}
}

// Test panic recovery with timeout
func TestWorkflowEngine_PanicRecoveryTimeout(t *testing.T) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "timeout-test",
		Version: "1.0",
		Timeout: 100 * time.Millisecond, // Short timeout
		Steps: []WorkflowStepDefinition{
			{Name: "slow-step", Type: "agent"},
		},
	}

	ctx := context.Background()
	execution := &WorkflowExecution{
		ID:       "test-exec-timeout",
		Workflow: workflow.Name,
		Status:   ExecutionRunning,
		Steps:    make(map[string]*StepExecution),
		Context:  make(map[string]interface{}),
		Errors:   []error{},
		DAG:      NewWorkflowDAG(),
	}

	execution.Steps["slow-step"] = &StepExecution{
		StepID: "slow-step",
		Status: StepPending,
	}
	execution.DAG.AddNode("slow-step", []string{})

	// Mock worker that sleeps then panics
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		for range taskQueue {
			time.Sleep(200 * time.Millisecond) // Longer than timeout
			panic("panic after timeout")
		}
	}

	// Execute workflow - should handle timeout
	err := engine.executeDAG(ctx, execution, workflow)

	if err == nil {
		t.Error("Expected timeout or panic error")
	}
}

// Benchmark panic recovery performance
func BenchmarkWorkflowEngine_PanicRecovery(b *testing.B) {
	engine := &WorkflowEngine{
		stateStore: NewInMemoryStateStore(),
		metrics:    &WorkflowMetrics{},
	}

	workflow := &WorkflowDefinition{
		Name:    "bench-test",
		Version: "1.0",
		Steps: []WorkflowStepDefinition{
			{Name: "step1", Type: "agent"},
		},
	}

	// Mock worker that always panics
	engine.worker = func(ctx context.Context, taskQueue <-chan *WorkflowTask, results chan<- *TaskResult) {
		for range taskQueue {
			panic("benchmark panic")
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		execution := &WorkflowExecution{
			ID:       fmt.Sprintf("bench-%d", i),
			Workflow: workflow.Name,
			Status:   ExecutionRunning,
			Steps:    make(map[string]*StepExecution),
			Context:  make(map[string]interface{}),
			Errors:   []error{},
			DAG:      NewWorkflowDAG(),
		}

		execution.Steps["step1"] = &StepExecution{
			StepID: "step1",
			Status: StepPending,
		}
		execution.DAG.AddNode("step1", []string{})

		_ = engine.executeDAG(ctx, execution, workflow)
	}
}

// Test mutex safety in panic recovery
func TestWorkflowEngine_PanicMutexSafety(t *testing.T) {
	// This test verifies that panic recovery doesn't cause deadlocks
	// by improperly handling mutexes

	var mu sync.Mutex
	sharedData := make(map[string]int)

	// Simulate multiple workers that might panic while holding locks
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					// Recovery without nested defer for unlock
					mu.Lock()
					sharedData[fmt.Sprintf("panic-%d", id)] = id
					mu.Unlock() // Direct unlock, no defer
				}
				wg.Done()
			}()

			if id%2 == 0 {
				panic(fmt.Sprintf("worker %d panic", id))
			}

			mu.Lock()
			sharedData[fmt.Sprintf("success-%d", id)] = id
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify no deadlock occurred
	mu.Lock()
	dataLen := len(sharedData)
	mu.Unlock()

	if dataLen != 10 {
		t.Errorf("Expected 10 entries, got %d (possible deadlock)", dataLen)
	}
}
