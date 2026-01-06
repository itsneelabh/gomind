package core

import (
	"testing"
	"time"
)

func TestTaskStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusQueued, false},
		{TaskStatusRunning, false},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		{TaskStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.expected {
				t.Errorf("TaskStatus(%s).IsTerminal() = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestNewTask(t *testing.T) {
	id := "task-123"
	taskType := "orchestration"
	input := map[string]interface{}{
		"query": "test query",
	}

	task := NewTask(id, taskType, input)

	if task.ID != id {
		t.Errorf("NewTask().ID = %v, want %v", task.ID, id)
	}
	if task.Type != taskType {
		t.Errorf("NewTask().Type = %v, want %v", task.Type, taskType)
	}
	if task.Status != TaskStatusQueued {
		t.Errorf("NewTask().Status = %v, want %v", task.Status, TaskStatusQueued)
	}
	if task.Input["query"] != "test query" {
		t.Errorf("NewTask().Input[query] = %v, want %v", task.Input["query"], "test query")
	}
	if task.CreatedAt.IsZero() {
		t.Error("NewTask().CreatedAt should not be zero")
	}
}

func TestNewTaskWithTimeout(t *testing.T) {
	id := "task-456"
	taskType := "research"
	input := map[string]interface{}{"topic": "AI"}
	timeout := 10 * time.Minute

	task := NewTaskWithTimeout(id, taskType, input, timeout)

	if task.ID != id {
		t.Errorf("NewTaskWithTimeout().ID = %v, want %v", task.ID, id)
	}
	if task.Options.Timeout != timeout {
		t.Errorf("NewTaskWithTimeout().Options.Timeout = %v, want %v", task.Options.Timeout, timeout)
	}
}

func TestTask_SetTraceContext(t *testing.T) {
	task := NewTask("task-789", "test", nil)
	traceID := "0af7651916cd43dd8448eb211c80319c"
	spanID := "b7ad6b7169203331"

	task.SetTraceContext(traceID, spanID)

	if task.TraceID != traceID {
		t.Errorf("Task.TraceID = %v, want %v", task.TraceID, traceID)
	}
	if task.ParentSpanID != spanID {
		t.Errorf("Task.ParentSpanID = %v, want %v", task.ParentSpanID, spanID)
	}
}

func TestTaskError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *TaskError
		expected string
	}{
		{
			name: "with details",
			err: &TaskError{
				Code:    TaskErrorCodeTimeout,
				Message: "task exceeded timeout",
				Details: "timeout was 30m",
			},
			expected: "TASK_TIMEOUT: task exceeded timeout (timeout was 30m)",
		},
		{
			name: "without details",
			err: &TaskError{
				Code:    TaskErrorCodeHandlerError,
				Message: "handler failed",
			},
			expected: "HANDLER_ERROR: handler failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("TaskError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultAsyncTaskConfig(t *testing.T) {
	config := DefaultAsyncTaskConfig()

	if config.QueuePrefix != "gomind:tasks" {
		t.Errorf("QueuePrefix = %v, want gomind:tasks", config.QueuePrefix)
	}
	if config.WorkerCount != 5 {
		t.Errorf("WorkerCount = %v, want 5", config.WorkerCount)
	}
	if config.DequeueTimeout != 30*time.Second {
		t.Errorf("DequeueTimeout = %v, want 30s", config.DequeueTimeout)
	}
	if config.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", config.ShutdownTimeout)
	}
	if config.DefaultTimeout != 30*time.Minute {
		t.Errorf("DefaultTimeout = %v, want 30m", config.DefaultTimeout)
	}
	if config.ResultTTL != 24*time.Hour {
		t.Errorf("ResultTTL = %v, want 24h", config.ResultTTL)
	}
}

func TestTaskProgress(t *testing.T) {
	progress := &TaskProgress{
		CurrentStep: 2,
		TotalSteps:  5,
		StepName:    "Processing",
		Percentage:  40.0,
		Message:     "Analyzing data",
	}

	if progress.CurrentStep != 2 {
		t.Errorf("CurrentStep = %v, want 2", progress.CurrentStep)
	}
	if progress.TotalSteps != 5 {
		t.Errorf("TotalSteps = %v, want 5", progress.TotalSteps)
	}
	if progress.Percentage != 40.0 {
		t.Errorf("Percentage = %v, want 40.0", progress.Percentage)
	}
}

func TestTaskOptions(t *testing.T) {
	options := TaskOptions{
		Timeout: 5 * time.Minute,
	}

	if options.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", options.Timeout)
	}
}

func TestTask_FullLifecycle(t *testing.T) {
	// Create task
	task := NewTask("lifecycle-test", "integration", map[string]interface{}{
		"action": "test",
	})

	// Verify initial state
	if task.Status != TaskStatusQueued {
		t.Errorf("Initial status = %v, want queued", task.Status)
	}
	if task.StartedAt != nil {
		t.Error("StartedAt should be nil initially")
	}
	if task.CompletedAt != nil {
		t.Error("CompletedAt should be nil initially")
	}

	// Simulate starting
	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now

	if task.Status != TaskStatusRunning {
		t.Errorf("Status = %v, want running", task.Status)
	}
	if task.StartedAt == nil {
		t.Error("StartedAt should not be nil after starting")
	}

	// Simulate progress
	task.Progress = &TaskProgress{
		CurrentStep: 1,
		TotalSteps:  2,
		StepName:    "Step 1",
		Percentage:  50.0,
	}

	if task.Progress.CurrentStep != 1 {
		t.Errorf("Progress.CurrentStep = %v, want 1", task.Progress.CurrentStep)
	}

	// Simulate completion
	completedAt := time.Now()
	task.Status = TaskStatusCompleted
	task.CompletedAt = &completedAt
	task.Result = map[string]interface{}{"success": true}

	if task.Status != TaskStatusCompleted {
		t.Errorf("Status = %v, want completed", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should not be nil after completion")
	}
	if task.Result == nil {
		t.Error("Result should not be nil after completion")
	}
}

func TestTask_FailureScenario(t *testing.T) {
	task := NewTask("failure-test", "test", nil)

	// Simulate failure
	now := time.Now()
	task.Status = TaskStatusFailed
	task.CompletedAt = &now
	task.Error = &TaskError{
		Code:    TaskErrorCodeHandlerError,
		Message: "Something went wrong",
		Details: "Stack trace here",
	}

	if !task.Status.IsTerminal() {
		t.Error("Failed status should be terminal")
	}
	if task.Error == nil {
		t.Error("Error should not be nil for failed task")
	}
	if task.Error.Code != TaskErrorCodeHandlerError {
		t.Errorf("Error.Code = %v, want %v", task.Error.Code, TaskErrorCodeHandlerError)
	}
}

func TestTask_CancellationScenario(t *testing.T) {
	task := NewTask("cancel-test", "test", nil)

	// Start the task
	startedAt := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &startedAt

	// Cancel the task
	cancelledAt := time.Now()
	task.Status = TaskStatusCancelled
	task.CancelledAt = &cancelledAt
	task.Error = &TaskError{
		Code:    TaskErrorCodeCancelled,
		Message: "Task was cancelled by user",
	}

	if !task.Status.IsTerminal() {
		t.Error("Cancelled status should be terminal")
	}
	if task.CancelledAt == nil {
		t.Error("CancelledAt should not be nil for cancelled task")
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify error constants are defined
	if TaskErrorCodeTimeout != "TASK_TIMEOUT" {
		t.Errorf("TaskErrorCodeTimeout = %v, want TASK_TIMEOUT", TaskErrorCodeTimeout)
	}
	if TaskErrorCodeCancelled != "TASK_CANCELLED" {
		t.Errorf("TaskErrorCodeCancelled = %v, want TASK_CANCELLED", TaskErrorCodeCancelled)
	}
	if TaskErrorCodeHandlerError != "HANDLER_ERROR" {
		t.Errorf("TaskErrorCodeHandlerError = %v, want HANDLER_ERROR", TaskErrorCodeHandlerError)
	}
	if TaskErrorCodePanic != "HANDLER_PANIC" {
		t.Errorf("TaskErrorCodePanic = %v, want HANDLER_PANIC", TaskErrorCodePanic)
	}
	if TaskErrorCodeInvalidInput != "INVALID_INPUT" {
		t.Errorf("TaskErrorCodeInvalidInput = %v, want INVALID_INPUT", TaskErrorCodeInvalidInput)
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are defined
	if ErrTaskNotFound == nil {
		t.Error("ErrTaskNotFound should not be nil")
	}
	if ErrTaskNotCancellable == nil {
		t.Error("ErrTaskNotCancellable should not be nil")
	}
	if ErrTaskQueueEmpty == nil {
		t.Error("ErrTaskQueueEmpty should not be nil")
	}
	if ErrInvalidTaskStatus == nil {
		t.Error("ErrInvalidTaskStatus should not be nil")
	}

	// Verify error messages
	if ErrTaskNotFound.Error() != "task not found" {
		t.Errorf("ErrTaskNotFound.Error() = %v", ErrTaskNotFound.Error())
	}
	if ErrTaskNotCancellable.Error() != "task not cancellable" {
		t.Errorf("ErrTaskNotCancellable.Error() = %v", ErrTaskNotCancellable.Error())
	}
}
