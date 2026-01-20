package orchestration

import (
	"context"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// =============================================================================
// HITL (Human-in-the-Loop) Interfaces and Types
// =============================================================================
//
// This file defines the core interfaces and data types for the HITL system.
// The design follows Go best practices:
// - Small, focused interfaces (1-3 methods each)
// - Interface composition for larger behaviors
// - Constructors return concrete types, not interfaces
//
// See HUMAN_IN_THE_LOOP_PROPOSAL.md for full design documentation.
// =============================================================================

// -----------------------------------------------------------------------------
// Small, Focused Interfaces (Go Idiom: "Keep interfaces small!")
// -----------------------------------------------------------------------------

// PlanApprover determines if LLM-generated plans need human approval.
// Implement this interface for plan-level approval logic.
type PlanApprover interface {
	ShouldApprovePlan(ctx context.Context, plan *RoutingPlan) (*InterruptDecision, error)
}

// StepApprover determines if steps need approval before or after execution.
// Implement this interface for step-level approval logic.
type StepApprover interface {
	ShouldApproveBeforeStep(ctx context.Context, step RoutingStep, plan *RoutingPlan) (*InterruptDecision, error)
	ShouldApproveAfterStep(ctx context.Context, step RoutingStep, result *StepResult) (*InterruptDecision, error)
}

// ErrorEscalator determines if errors should escalate to humans.
// Implement this interface for error escalation logic.
type ErrorEscalator interface {
	ShouldEscalateError(ctx context.Context, step RoutingStep, err error, attempts int) (*InterruptDecision, error)
}

// -----------------------------------------------------------------------------
// Composed Interface
// -----------------------------------------------------------------------------

// InterruptPolicy composes all approval behaviors for convenience.
// Most implementations will embed the smaller interfaces or implement this directly.
// Implementations can be rule-based, ML-based, or external service calls.
type InterruptPolicy interface {
	PlanApprover
	StepApprover
	ErrorEscalator
}

// -----------------------------------------------------------------------------
// Checkpoint Storage Interface
// -----------------------------------------------------------------------------

// CheckpointStore persists workflow state for interrupt/resume.
// Implementations: Redis (default), PostgreSQL, S3, Memory (testing)
type CheckpointStore interface {
	// SaveCheckpoint persists execution state at an interrupt point
	SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error

	// LoadCheckpoint retrieves a checkpoint by ID
	LoadCheckpoint(ctx context.Context, checkpointID string) (*ExecutionCheckpoint, error)

	// UpdateCheckpointStatus updates the status of a pending checkpoint
	UpdateCheckpointStatus(ctx context.Context, checkpointID string, status CheckpointStatus) error

	// ListPendingCheckpoints returns checkpoints awaiting human response
	ListPendingCheckpoints(ctx context.Context, filter CheckpointFilter) ([]*ExecutionCheckpoint, error)

	// DeleteCheckpoint removes a checkpoint after completion
	DeleteCheckpoint(ctx context.Context, checkpointID string) error
}

// CheckpointFilter for querying checkpoints
type CheckpointFilter struct {
	Status    CheckpointStatus `json:"status,omitempty"`
	RequestID string           `json:"request_id,omitempty"`
	Limit     int              `json:"limit,omitempty"`
	Offset    int              `json:"offset,omitempty"`
}

// -----------------------------------------------------------------------------
// Interrupt Handler Interface
// -----------------------------------------------------------------------------

// InterruptHandler manages asynchronous human notification and response collection.
// This is the primary extension point for integrating with your notification infrastructure.
//
// The framework provides WebhookInterruptHandler as a reference implementation.
// Applications implement custom handlers for their specific infrastructure:
// - Slack/Discord bots
// - Email systems
// - Dashboard/UI integration
// - SMS gateways
// - Custom webhook processors
type InterruptHandler interface {
	// NotifyInterrupt sends notification about a pending interrupt.
	// The implementation decides HOW to notify (webhook, Slack, email, etc.)
	NotifyInterrupt(ctx context.Context, checkpoint *ExecutionCheckpoint) error

	// WaitForCommand blocks until human responds or timeout.
	// For async scenarios, use SubmitCommand instead.
	WaitForCommand(ctx context.Context, checkpointID string, timeout time.Duration) (*Command, error)

	// SubmitCommand processes a command submitted via external channel.
	// Called when human responds through your notification system.
	SubmitCommand(ctx context.Context, command *Command) error
}

// -----------------------------------------------------------------------------
// Command Store Interface
// -----------------------------------------------------------------------------

// CommandStore provides distributed command delivery for HITL.
// This enables cross-instance command submission in distributed deployments.
//
// The framework provides RedisCommandStore as the default implementation.
// Applications can implement custom stores (PostgreSQL, Kafka, etc.)
//
// Key format for Redis implementation:
//   - Command channel: {prefix}:command:{checkpoint_id}
//   - Pending commands: {prefix}:pending_commands (Redis List)
type CommandStore interface {
	// PublishCommand publishes a command for a waiting handler to receive.
	// Used by SubmitCommand to deliver commands across instances.
	PublishCommand(ctx context.Context, command *Command) error

	// SubscribeCommand subscribes to commands for a specific checkpoint.
	// Returns a channel that receives commands. Close the returned cancel func when done.
	SubscribeCommand(ctx context.Context, checkpointID string) (<-chan *Command, func(), error)

	// Close closes the command store and releases resources.
	Close() error
}

// -----------------------------------------------------------------------------
// Interrupt Controller Interface
// -----------------------------------------------------------------------------

// InterruptController coordinates all HITL functionality.
// This is the main entry point for HITL integration with the orchestrator.
//
// The controller delegates policy decisions to InterruptPolicy and notification
// to InterruptHandler. It owns the coordination logic between these components.
type InterruptController interface {
	// SetPolicy configures the interrupt policy (optional - can be set via constructor)
	SetPolicy(policy InterruptPolicy)

	// SetHandler configures the notification handler (optional - can be set via constructor)
	SetHandler(handler InterruptHandler)

	// SetCheckpointStore configures state persistence (optional - can be set via constructor)
	SetCheckpointStore(store CheckpointStore)

	// Check methods evaluate policy and handle interrupt if needed.
	// Returns nil if no interrupt, otherwise saves checkpoint and returns it.
	// These are called by the orchestrator/executor at the appropriate points.
	CheckPlanApproval(ctx context.Context, plan *RoutingPlan) (*ExecutionCheckpoint, error)
	CheckBeforeStep(ctx context.Context, step RoutingStep, plan *RoutingPlan) (*ExecutionCheckpoint, error)
	CheckAfterStep(ctx context.Context, step RoutingStep, result *StepResult) (*ExecutionCheckpoint, error)
	CheckOnError(ctx context.Context, step RoutingStep, err error, attempts int) (*ExecutionCheckpoint, error)

	// ProcessCommand handles a human command and updates checkpoint status.
	// Called when human responds through the InterruptHandler.
	ProcessCommand(ctx context.Context, command *Command) (*ResumeResult, error)

	// ResumeExecution continues workflow execution from a checkpoint.
	// Called after ProcessCommand returns ShouldResume=true.
	ResumeExecution(ctx context.Context, checkpointID string) (*ExecutionResult, error)

	// UpdateCheckpointProgress updates a checkpoint with completed steps.
	// Called by executor before returning ErrInterrupted for step-level interrupts.
	// This allows resumption to skip already-completed steps.
	UpdateCheckpointProgress(ctx context.Context, checkpointID string, completedSteps []StepResult) error
}

// =============================================================================
// Data Types
// =============================================================================

// -----------------------------------------------------------------------------
// Interrupt Decision
// -----------------------------------------------------------------------------

// InterruptDecision contains the decision and context for an interrupt
type InterruptDecision struct {
	ShouldInterrupt bool                   `json:"should_interrupt"`
	Reason          InterruptReason        `json:"reason"`
	Message         string                 `json:"message"`
	Priority        InterruptPriority      `json:"priority"`
	Timeout         time.Duration          `json:"timeout,omitempty"`        // Auto-approve after timeout
	DefaultAction   CommandType            `json:"default_action,omitempty"` // Action to take on timeout
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// InterruptReason categorizes why an interrupt was triggered
type InterruptReason string

const (
	ReasonPlanApproval       InterruptReason = "plan_approval"
	ReasonSensitiveOperation InterruptReason = "sensitive_operation"
	ReasonOutputValidation   InterruptReason = "output_validation"
	ReasonEscalation         InterruptReason = "escalation"
	ReasonContextGathering   InterruptReason = "context_gathering"
	ReasonCustom             InterruptReason = "custom"
)

// InterruptPriority indicates urgency of human response
type InterruptPriority string

const (
	PriorityLow      InterruptPriority = "low"      // Can wait, auto-timeout ok
	PriorityNormal   InterruptPriority = "normal"   // Standard review
	PriorityHigh     InterruptPriority = "high"     // Needs prompt attention
	PriorityCritical InterruptPriority = "critical" // Blocking, no timeout
)

// -----------------------------------------------------------------------------
// Execution Checkpoint
// -----------------------------------------------------------------------------

// ExecutionCheckpoint contains all state needed to resume execution
type ExecutionCheckpoint struct {
	CheckpointID string `json:"checkpoint_id"`
	RequestID    string `json:"request_id"`

	// Interrupt context
	InterruptPoint InterruptPoint     `json:"interrupt_point"`
	Decision       *InterruptDecision `json:"decision"`

	// Execution state
	Plan              *RoutingPlan           `json:"plan"`
	CompletedSteps    []StepResult           `json:"completed_steps"`
	CurrentStep       *RoutingStep           `json:"current_step,omitempty"`
	CurrentStepResult *StepResult            `json:"current_step_result,omitempty"`
	StepResults       map[string]*StepResult `json:"step_results"`

	// ResolvedParameters contains the actual parameter values after resolution.
	// For step-level HITL (Scenario 2), this shows users the real values
	// (e.g., amount: 15000) instead of templates (e.g., amount: "{{step-1.amount}}").
	ResolvedParameters map[string]interface{} `json:"resolved_parameters,omitempty"`

	// Metadata
	OriginalRequest string                 `json:"original_request"`
	UserContext     map[string]interface{} `json:"user_context,omitempty"`

	// Timing
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt time.Time        `json:"expires_at"`
	Status    CheckpointStatus `json:"status"`
}

// InterruptPoint identifies where in execution the interrupt occurred
type InterruptPoint string

const (
	InterruptPointPlanGenerated    InterruptPoint = "plan_generated"
	InterruptPointBeforeStep       InterruptPoint = "before_step"
	InterruptPointAfterStep        InterruptPoint = "after_step"
	InterruptPointOnError          InterruptPoint = "on_error"
	InterruptPointContextGathering InterruptPoint = "context_gathering"
)

// CheckpointStatus tracks the lifecycle of a checkpoint
type CheckpointStatus string

const (
	CheckpointStatusPending   CheckpointStatus = "pending"   // Awaiting human response
	CheckpointStatusApproved  CheckpointStatus = "approved"  // Human approved, ready to resume
	CheckpointStatusRejected  CheckpointStatus = "rejected"  // Human rejected
	CheckpointStatusEdited    CheckpointStatus = "edited"    // Human edited, ready to resume
	CheckpointStatusExpired   CheckpointStatus = "expired"   // Timeout reached
	CheckpointStatusCompleted CheckpointStatus = "completed" // Execution completed
	CheckpointStatusAborted   CheckpointStatus = "aborted"   // User aborted
)

// -----------------------------------------------------------------------------
// Command (Human Response)
// -----------------------------------------------------------------------------

// Command represents a human decision in response to an interrupt
type Command struct {
	CommandID    string      `json:"command_id"`
	CheckpointID string      `json:"checkpoint_id"`
	Type         CommandType `json:"type"`

	// Optional payload based on command type
	EditedPlan   *RoutingPlan           `json:"edited_plan,omitempty"`   // For plan edits
	EditedStep   *RoutingStep           `json:"edited_step,omitempty"`   // For step edits
	EditedParams map[string]interface{} `json:"edited_params,omitempty"` // For parameter edits
	Feedback     string                 `json:"feedback,omitempty"`      // Rejection reason
	Response     string                 `json:"response,omitempty"`      // Context gathering response

	// Audit
	UserID    string    `json:"user_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// CommandType defines the type of human decision
type CommandType string

const (
	CommandApprove CommandType = "approve" // Proceed as planned
	CommandEdit    CommandType = "edit"    // Proceed with modifications
	CommandReject  CommandType = "reject"  // Stop and provide feedback
	CommandSkip    CommandType = "skip"    // Skip current step, continue
	CommandAbort   CommandType = "abort"   // Stop entire workflow
	CommandRetry   CommandType = "retry"   // Retry with new parameters
	CommandRespond CommandType = "respond" // Provide requested information
)

// -----------------------------------------------------------------------------
// Resume Result
// -----------------------------------------------------------------------------

// ResumeResult contains the outcome of processing a command
type ResumeResult struct {
	CheckpointID string       `json:"checkpoint_id"`
	Action       CommandType  `json:"action"`
	ShouldResume bool         `json:"should_resume"`
	ModifiedPlan *RoutingPlan `json:"modified_plan,omitempty"`
	SkipStep     bool         `json:"skip_step,omitempty"`
	Abort        bool         `json:"abort,omitempty"`
	Feedback     string       `json:"feedback,omitempty"`
}

// -----------------------------------------------------------------------------
// HITL Configuration
// -----------------------------------------------------------------------------

// HITLConfig configures Human-in-the-Loop behavior
type HITLConfig struct {
	// Enable/disable HITL (default: false for backward compatibility)
	Enabled bool `json:"enabled"`

	// Policy configuration
	RequirePlanApproval   bool     `json:"require_plan_approval"`
	SensitiveCapabilities []string `json:"sensitive_capabilities"`
	SensitiveAgents       []string `json:"sensitive_agents"`
	EscalateAfterRetries  int      `json:"escalate_after_retries"`

	// Step-level HITL configuration (Scenario 2 only)
	// Capabilities listed here trigger step-level approval WITHOUT plan-level approval.
	// Use this when you want execution to start, but pause before specific sensitive steps.
	StepSensitiveCapabilities []string `json:"step_sensitive_capabilities"`
	StepSensitiveAgents       []string `json:"step_sensitive_agents"`

	// Timeout configuration
	DefaultTimeout time.Duration `json:"default_timeout"`
	DefaultAction  CommandType   `json:"default_action"`

	// Storage configuration
	CheckpointTTL  time.Duration `json:"checkpoint_ttl"`
	RedisKeyPrefix string        `json:"redis_key_prefix"`
}

// DefaultHITLConfig returns sensible defaults for HITL configuration
func DefaultHITLConfig() HITLConfig {
	return HITLConfig{
		Enabled:              false, // Opt-in for backward compatibility
		RequirePlanApproval:  false,
		EscalateAfterRetries: 3,
		DefaultTimeout:       5 * time.Minute,
		DefaultAction:        CommandApprove,
		CheckpointTTL:        24 * time.Hour,
		RedisKeyPrefix:       "gomind:hitl",
	}
}

// -----------------------------------------------------------------------------
// Workflow HITL Configuration (for YAML-based workflows)
// -----------------------------------------------------------------------------

// WorkflowHITLConfig configures workflow-level HITL behavior
type WorkflowHITLConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`
	DefaultAction  CommandType   `yaml:"default_action" json:"default_action"`
}

// StepApprovalConfig configures pre-step approval
type StepApprovalConfig struct {
	Reason   string            `yaml:"reason" json:"reason"`
	Priority InterruptPriority `yaml:"priority" json:"priority"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
}

// StepValidationConfig configures post-step validation
type StepValidationConfig struct {
	Reason   string            `yaml:"reason" json:"reason"`
	Priority InterruptPriority `yaml:"priority" json:"priority"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
}

// -----------------------------------------------------------------------------
// Option Functions (for dependency injection)
// -----------------------------------------------------------------------------

// InterruptControllerOption configures optional dependencies for DefaultInterruptController
type InterruptControllerOption func(*DefaultInterruptController)

// WithControllerLogger sets the logger for the interrupt controller
func WithControllerLogger(logger core.Logger) InterruptControllerOption {
	return func(c *DefaultInterruptController) {
		if logger == nil {
			return
		}
		// Use ComponentAwareLogger for component-based log segregation (per LOGGING_IMPLEMENTATION_GUIDE.md)
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			c.logger = cal.WithComponent("framework/orchestration")
		} else {
			c.logger = logger
		}
	}
}

// WithControllerTelemetry sets the telemetry provider for the interrupt controller
func WithControllerTelemetry(telemetry core.Telemetry) InterruptControllerOption {
	return func(c *DefaultInterruptController) {
		c.telemetry = telemetry
	}
}

// CheckpointStoreOption configures optional dependencies for RedisCheckpointStore
type CheckpointStoreOption func(*RedisCheckpointStore)

// WithCheckpointLogger sets the logger for the checkpoint store
func WithCheckpointLogger(logger core.Logger) CheckpointStoreOption {
	return func(s *RedisCheckpointStore) {
		if logger == nil {
			return
		}
		// Use ComponentAwareLogger for component-based log segregation (per LOGGING_IMPLEMENTATION_GUIDE.md)
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			s.logger = cal.WithComponent("framework/orchestration")
		} else {
			s.logger = logger
		}
	}
}

// WithCheckpointTelemetry sets the telemetry provider for the checkpoint store
func WithCheckpointTelemetry(telemetry core.Telemetry) CheckpointStoreOption {
	return func(s *RedisCheckpointStore) {
		s.telemetry = telemetry
	}
}

// WebhookHandlerOption configures optional dependencies for WebhookInterruptHandler
type WebhookHandlerOption func(*WebhookInterruptHandler)

// WithHandlerCircuitBreaker sets the circuit breaker for the webhook handler
func WithHandlerCircuitBreaker(cb core.CircuitBreaker) WebhookHandlerOption {
	return func(h *WebhookInterruptHandler) {
		h.circuitBreaker = cb
	}
}

// WithHandlerLogger sets the logger for the webhook handler
func WithHandlerLogger(logger core.Logger) WebhookHandlerOption {
	return func(h *WebhookInterruptHandler) {
		if logger == nil {
			return
		}
		// Use ComponentAwareLogger for component-based log segregation (per LOGGING_IMPLEMENTATION_GUIDE.md)
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			h.logger = cal.WithComponent("framework/orchestration")
		} else {
			h.logger = logger
		}
	}
}

// WithHandlerTelemetry sets the telemetry provider for the webhook handler
func WithHandlerTelemetry(telemetry core.Telemetry) WebhookHandlerOption {
	return func(h *WebhookInterruptHandler) {
		h.telemetry = telemetry
	}
}

// PolicyOption configures optional dependencies for RuleBasedPolicy
type PolicyOption func(*RuleBasedPolicy)

// WithPolicyLogger sets the logger for the policy
func WithPolicyLogger(logger core.Logger) PolicyOption {
	return func(p *RuleBasedPolicy) {
		if logger == nil {
			return
		}
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			p.logger = cal.WithComponent("framework/orchestration")
		} else {
			p.logger = logger
		}
	}
}
