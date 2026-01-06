// Package core provides async task interfaces and types for long-running operations.
//
// This file defines the interfaces and types for the async task system that enables
// long-running operations (minutes to hours) in GoMind. It solves the HTTP timeout
// problem by providing HTTP 202 + polling pattern with background worker execution.
//
// # Architecture Overview
//
// The async task system consists of:
//   - TaskQueue: Handles task submission and retrieval (Redis-backed by default)
//   - TaskStore: Persists task state and results (Redis-backed by default)
//   - TaskWorker: Processes tasks from the queue in the background
//   - ProgressReporter: Allows handlers to report progress updates
//
// # Distributed Tracing
//
// The Task struct includes TraceID and ParentSpanID fields to preserve distributed
// trace context across async boundaries. Workers restore this context using
// telemetry.StartLinkedSpan() to maintain full trace visibility in Jaeger.
//
// # Usage
//
// Submitting a task:
//
//	task := &core.Task{
//	    ID:           generateTaskID(),
//	    Type:         "orchestration",
//	    Status:       core.TaskStatusQueued,
//	    Input:        map[string]interface{}{"query": "weather in Tokyo"},
//	    TraceID:      tc.TraceID,      // From telemetry.GetTraceContext(ctx)
//	    ParentSpanID: tc.SpanID,
//	}
//	err := taskQueue.Enqueue(ctx, task)
//
// Processing a task (in worker):
//
//	func handleOrchestration(ctx context.Context, task *core.Task, reporter core.ProgressReporter) error {
//	    reporter.Report(&core.TaskProgress{CurrentStep: 1, TotalSteps: 3, StepName: "Planning"})
//	    // ... do work ...
//	    return nil
//	}
package core

import (
	"context"
	"errors"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// Errors
// ═══════════════════════════════════════════════════════════════════════════

// ErrTaskNotFound is returned when a task cannot be found
var ErrTaskNotFound = errors.New("task not found")

// ErrTaskNotCancellable is returned when a task cannot be cancelled
// (already completed, failed, or cancelled)
var ErrTaskNotCancellable = errors.New("task not cancellable")

// ErrTaskQueueEmpty is returned when Dequeue times out with no task available
var ErrTaskQueueEmpty = errors.New("task queue empty")

// ErrInvalidTaskStatus is returned when a task status transition is invalid
var ErrInvalidTaskStatus = errors.New("invalid task status transition")

// ═══════════════════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════════════════

// TaskStatus represents the state of a long-running task
type TaskStatus string

const (
	// TaskStatusQueued indicates the task is waiting in the queue
	TaskStatusQueued TaskStatus = "queued"

	// TaskStatusRunning indicates the task is currently being processed
	TaskStatusRunning TaskStatus = "running"

	// TaskStatusCompleted indicates the task finished successfully
	TaskStatusCompleted TaskStatus = "completed"

	// TaskStatusFailed indicates the task failed with an error
	TaskStatusFailed TaskStatus = "failed"

	// TaskStatusCancelled indicates the task was cancelled by request
	TaskStatusCancelled TaskStatus = "cancelled"
)

// IsTerminal returns true if the status is a terminal state (completed, failed, or cancelled)
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed || s == TaskStatusCancelled
}

// Task represents a long-running async task
type Task struct {
	// ID is the unique identifier for this task
	ID string `json:"id"`

	// Type identifies the kind of task (e.g., "orchestration", "research")
	// Used to route tasks to the appropriate handler
	Type string `json:"type"`

	// Status is the current state of the task
	Status TaskStatus `json:"status"`

	// Input contains the task parameters
	Input map[string]interface{} `json:"input"`

	// Result contains the task output when completed
	Result interface{} `json:"result,omitempty"`

	// Error contains error information if the task failed
	Error *TaskError `json:"error,omitempty"`

	// Progress contains the current progress of the task
	Progress *TaskProgress `json:"progress,omitempty"`

	// Options configures task execution behavior
	Options TaskOptions `json:"options"`

	// CreatedAt is when the task was submitted
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is when the worker began processing (nil if queued)
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the task finished (nil if not complete)
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// CancelledAt is when the task was cancelled (nil if not cancelled)
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`

	// ═══════════════════════════════════════════════════════════════════════
	// Trace Context for Distributed Tracing
	// ═══════════════════════════════════════════════════════════════════════
	//
	// These fields preserve the trace chain across async boundaries.
	// When a task is submitted, extract trace context using:
	//   tc := telemetry.GetTraceContext(ctx)
	//   task.TraceID = tc.TraceID
	//   task.ParentSpanID = tc.SpanID
	//
	// When a worker processes the task, restore context using:
	//   ctx, endSpan := telemetry.StartLinkedSpan(ctx, "task.process",
	//       task.TraceID, task.ParentSpanID, attrs)
	//   defer endSpan()

	// TraceID is the W3C trace ID (32 hex chars) from the original request
	TraceID string `json:"trace_id,omitempty"`

	// ParentSpanID is the span ID (16 hex chars) of the submitting request
	ParentSpanID string `json:"parent_span_id,omitempty"`
}

// TaskProgress tracks execution progress
type TaskProgress struct {
	// CurrentStep is the current step number (1-indexed)
	CurrentStep int `json:"current_step"`

	// TotalSteps is the total number of steps
	TotalSteps int `json:"total_steps"`

	// StepName is a human-readable name for the current step
	StepName string `json:"step_name"`

	// Percentage is the overall completion percentage (0-100)
	Percentage float64 `json:"percentage"`

	// Message is an optional status message
	Message string `json:"message,omitempty"`
}

// TaskOptions configures task execution
type TaskOptions struct {
	// Timeout is the maximum duration for task execution
	// If zero, DefaultAsyncTaskConfig().DefaultTimeout is used
	Timeout time.Duration `json:"timeout"`
}

// TaskError contains error information
type TaskError struct {
	// Code is a machine-readable error code
	Code string `json:"code"`

	// Message is a human-readable error message
	Message string `json:"message"`

	// Details contains additional error details
	Details string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *TaskError) Error() string {
	if e.Details != "" {
		return e.Code + ": " + e.Message + " (" + e.Details + ")"
	}
	return e.Code + ": " + e.Message
}

// Common error codes for TaskError
const (
	// TaskErrorCodeTimeout indicates the task exceeded its timeout
	TaskErrorCodeTimeout = "TASK_TIMEOUT"

	// TaskErrorCodeCancelled indicates the task was cancelled
	TaskErrorCodeCancelled = "TASK_CANCELLED"

	// TaskErrorCodeHandlerError indicates the handler returned an error
	TaskErrorCodeHandlerError = "HANDLER_ERROR"

	// TaskErrorCodePanic indicates the handler panicked
	TaskErrorCodePanic = "HANDLER_PANIC"

	// TaskErrorCodeInvalidInput indicates invalid task input
	TaskErrorCodeInvalidInput = "INVALID_INPUT"
)

// ═══════════════════════════════════════════════════════════════════════════
// Interfaces (v1 MVP)
// ═══════════════════════════════════════════════════════════════════════════

// TaskQueue handles async task submission and retrieval.
// The default implementation uses Redis lists (LPUSH/BRPOP).
type TaskQueue interface {
	// Enqueue adds a task to the queue.
	// The task's Status should be TaskStatusQueued.
	Enqueue(ctx context.Context, task *Task) error

	// Dequeue retrieves the next task from the queue.
	// Blocks until a task is available or timeout expires.
	// Returns nil, nil if timeout expires with no task.
	// Returns ErrTaskQueueEmpty if queue is empty after timeout.
	Dequeue(ctx context.Context, timeout time.Duration) (*Task, error)

	// Acknowledge marks a task as successfully processed.
	// Called after the worker completes task processing.
	Acknowledge(ctx context.Context, taskID string) error

	// Reject returns a task to the queue for retry.
	// Called when processing fails but should be retried.
	Reject(ctx context.Context, taskID string, reason string) error
}

// TaskStore persists task state and results.
// The default implementation uses Redis hashes.
type TaskStore interface {
	// Create persists a new task.
	// Returns error if task with same ID already exists.
	Create(ctx context.Context, task *Task) error

	// Get retrieves a task by ID.
	// Returns ErrTaskNotFound if task doesn't exist.
	Get(ctx context.Context, taskID string) (*Task, error)

	// Update persists task changes (status, progress, result).
	// Returns ErrTaskNotFound if task doesn't exist.
	Update(ctx context.Context, task *Task) error

	// Delete removes a task.
	// Used for cleanup of old tasks.
	Delete(ctx context.Context, taskID string) error

	// Cancel marks a task as cancelled.
	// Returns ErrTaskNotFound if task doesn't exist.
	// Returns ErrTaskNotCancellable if task is already in a terminal state.
	Cancel(ctx context.Context, taskID string) error
}

// TaskWorker processes tasks from the queue.
type TaskWorker interface {
	// Start begins processing tasks.
	// Blocks until ctx is cancelled or Stop is called.
	Start(ctx context.Context) error

	// Stop gracefully stops the worker.
	// Waits for in-progress tasks to complete up to shutdown timeout.
	Stop(ctx context.Context) error

	// RegisterHandler registers a handler for a task type.
	// Must be called before Start.
	RegisterHandler(taskType string, handler TaskHandler) error
}

// TaskHandler processes a specific task type.
// The handler receives:
//   - ctx: Context with trace information (from StartLinkedSpan)
//   - task: The task to process
//   - reporter: For reporting progress updates
//
// The handler should:
//  1. Process the task using task.Input
//  2. Report progress periodically via reporter
//  3. Return nil on success (result should be set via task store)
//  4. Return error on failure
type TaskHandler func(ctx context.Context, task *Task, reporter ProgressReporter) error

// ProgressReporter allows handlers to report progress.
type ProgressReporter interface {
	// Report updates task progress.
	// Progress is persisted to the TaskStore.
	Report(progress *TaskProgress) error
}

// ═══════════════════════════════════════════════════════════════════════════
// Configuration
// ═══════════════════════════════════════════════════════════════════════════

// AsyncTaskConfig configures the async task system
type AsyncTaskConfig struct {
	// QueuePrefix is the Redis key prefix for queue keys
	QueuePrefix string `json:"queue_prefix"`

	// WorkerCount is the number of concurrent workers
	WorkerCount int `json:"worker_count"`

	// DequeueTimeout is how long to wait for a task in Dequeue
	DequeueTimeout time.Duration `json:"dequeue_timeout"`

	// ShutdownTimeout is how long to wait for workers to finish on shutdown
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// DefaultTimeout is the default task execution timeout
	DefaultTimeout time.Duration `json:"default_timeout"`

	// ResultTTL is how long to keep completed task results
	ResultTTL time.Duration `json:"result_ttl"`
}

// DefaultAsyncTaskConfig returns sensible defaults for the async task system.
// These values are suitable for most production deployments.
func DefaultAsyncTaskConfig() AsyncTaskConfig {
	return AsyncTaskConfig{
		QueuePrefix:     "gomind:tasks",
		WorkerCount:     5,
		DequeueTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		DefaultTimeout:  30 * time.Minute,
		ResultTTL:       24 * time.Hour,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════════════

// NewTask creates a new task with the given type and input.
// Sets CreatedAt to now and Status to TaskStatusQueued.
func NewTask(id, taskType string, input map[string]interface{}) *Task {
	return &Task{
		ID:        id,
		Type:      taskType,
		Status:    TaskStatusQueued,
		Input:     input,
		CreatedAt: time.Now(),
	}
}

// NewTaskWithTimeout creates a new task with a custom timeout.
func NewTaskWithTimeout(id, taskType string, input map[string]interface{}, timeout time.Duration) *Task {
	task := NewTask(id, taskType, input)
	task.Options.Timeout = timeout
	return task
}

// SetTraceContext sets the trace context fields on a task.
// Use with telemetry.GetTraceContext(ctx) to preserve trace chain.
func (t *Task) SetTraceContext(traceID, spanID string) {
	t.TraceID = traceID
	t.ParentSpanID = spanID
}
