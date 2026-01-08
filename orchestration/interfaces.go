package orchestration

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// RouterMode defines the routing strategy
// Note: This is currently only used for metrics and logging, not actual routing behavior
type RouterMode string

const (
	ModeAutonomous RouterMode = "autonomous" // AI-driven orchestration
	ModeWorkflow   RouterMode = "workflow"   // Workflow-based execution (separate system)
)

// RoutingStep represents a single step in a routing plan
type RoutingStep struct {
	StepID      string                 `json:"step_id"`
	AgentName   string                 `json:"agent_name"`
	Namespace   string                 `json:"namespace"`
	Instruction string                 `json:"instruction"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RoutingPlan represents a complete execution plan
type RoutingPlan struct {
	PlanID          string        `json:"plan_id"`
	OriginalRequest string        `json:"original_request"`
	Mode            RouterMode    `json:"mode"`
	Steps           []RoutingStep `json:"steps"`
	CreatedAt       time.Time     `json:"created_at"`
}

// Orchestrator coordinates multi-agent interactions
type Orchestrator interface {
	// ProcessRequest handles a natural language request by orchestrating multiple agents
	ProcessRequest(ctx context.Context, request string, metadata map[string]interface{}) (*OrchestratorResponse, error)

	// ExecutePlan executes a pre-defined routing plan
	ExecutePlan(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error)

	// GetExecutionHistory returns recent execution history
	GetExecutionHistory() []ExecutionRecord

	// GetMetrics returns orchestrator metrics
	GetMetrics() OrchestratorMetrics
}

// OrchestratorResponse represents the final synthesized response
type OrchestratorResponse struct {
	RequestID       string                 `json:"request_id"`
	OriginalRequest string                 `json:"original_request"`
	Response        string                 `json:"response"`
	RoutingMode     RouterMode             `json:"routing_mode"`
	ExecutionTime   time.Duration          `json:"execution_time"`
	AgentsInvolved  []string               `json:"agents_involved"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Errors          []string               `json:"errors,omitempty"`
	Confidence      float64                `json:"confidence"`
}

// StreamingOrchestratorResponse extends OrchestratorResponse for streaming scenarios
// It includes additional fields to track streaming-specific state and progress
type StreamingOrchestratorResponse struct {
	OrchestratorResponse

	// Streaming-specific fields
	ChunksDelivered int  `json:"chunks_delivered"` // Number of chunks successfully delivered
	StreamCompleted bool `json:"stream_completed"` // Whether streaming finished successfully
	PartialContent  bool `json:"partial_content"`  // True if response was truncated due to error/cancellation

	// Enhanced tracking fields
	StepResults  []StepResult     `json:"step_results,omitempty"`  // Detailed results from each execution step
	Usage        *core.TokenUsage `json:"usage,omitempty"`         // Token usage from AI synthesis
	FinishReason string           `json:"finish_reason,omitempty"` // Why streaming stopped (e.g., "stop", "length", "cancelled")
}

// Executor handles the execution of routing plans
type Executor interface {
	// Execute runs a routing plan and collects agent responses
	Execute(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error)

	// ExecuteStep executes a single routing step
	ExecuteStep(ctx context.Context, step RoutingStep) (*StepResult, error)

	// SetMaxConcurrency sets the maximum number of parallel executions
	SetMaxConcurrency(max int)
}

// ExecutionResult contains the results from executing a routing plan
type ExecutionResult struct {
	PlanID        string                 `json:"plan_id"`
	Steps         []StepResult           `json:"steps"`
	Success       bool                   `json:"success"`
	TotalDuration time.Duration          `json:"total_duration"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// StepResult contains the result from executing a single step
type StepResult struct {
	StepID      string        `json:"step_id"`
	AgentName   string        `json:"agent_name"`
	Namespace   string        `json:"namespace"`
	Instruction string        `json:"instruction"`
	Response    string        `json:"response"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Attempts    int           `json:"attempts"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
}

// Synthesizer combines multiple agent responses into a coherent result
type Synthesizer interface {
	// Synthesize combines agent responses into a final response
	Synthesize(ctx context.Context, request string, results *ExecutionResult) (string, error)

	// SetStrategy sets the synthesis strategy
	SetStrategy(strategy SynthesisStrategy)
}

// SynthesisStrategy defines how responses are combined
type SynthesisStrategy string

const (
	// StrategyLLM uses an LLM to synthesize responses
	StrategyLLM SynthesisStrategy = "llm"

	// StrategyTemplate uses predefined templates
	StrategyTemplate SynthesisStrategy = "template"

	// StrategySimple concatenates responses
	StrategySimple SynthesisStrategy = "simple"

	// StrategyCustom uses a custom synthesis function
	StrategyCustom SynthesisStrategy = "custom"
)

// ExecutionRecord represents a historical execution
type ExecutionRecord struct {
	RequestID      string        `json:"request_id"`
	Timestamp      time.Time     `json:"timestamp"`
	Request        string        `json:"request"`
	Response       string        `json:"response"`
	RoutingMode    RouterMode    `json:"routing_mode"`
	AgentsInvolved []string      `json:"agents_involved"`
	ExecutionTime  time.Duration `json:"execution_time"`
	Success        bool          `json:"success"`
	Errors         []string      `json:"errors,omitempty"`
}

// OrchestratorMetrics contains performance metrics
type OrchestratorMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	MedianLatency      time.Duration `json:"median_latency"`
	P99Latency         time.Duration `json:"p99_latency"`
	AgentCallsTotal    int64         `json:"agent_calls_total"`
	AgentCallsFailed   int64         `json:"agent_calls_failed"`
	SynthesisCount     int64         `json:"synthesis_count"`
	SynthesisErrors    int64         `json:"synthesis_errors"`
	LastRequestTime    time.Time     `json:"last_request_time"`
	UptimeSeconds      int64         `json:"uptime_seconds"`
}

// StepCompleteCallback is called after each step in a routing plan completes.
// This enables real-time progress reporting for async workflows that use
// AI orchestration with multiple tool calls.
//
// The callback is invoked from within the executor goroutine after each step
// completes (success or failure). It should be lightweight or delegate to a
// channel for async processing to avoid blocking execution.
//
// Parameters:
//   - stepIndex: 0-based index of the completed step
//   - totalSteps: total number of steps in the plan
//   - step: the step that completed (contains AgentName, StepID, etc.)
//   - result: the step execution result (contains Success, Duration, Response, etc.)
//
// Usage with async tasks:
//
//	config.ExecutionOptions.OnStepComplete = func(stepIndex, totalSteps int, step RoutingStep, result StepResult) {
//	    reporter.Report(&core.TaskProgress{
//	        CurrentStep: stepIndex + 1,
//	        TotalSteps:  totalSteps,
//	        StepName:    step.AgentName,
//	        Percentage:  float64(stepIndex+1) / float64(totalSteps) * 100,
//	        Message:     fmt.Sprintf("Completed %s", step.AgentName),
//	    })
//	}
type StepCompleteCallback func(stepIndex, totalSteps int, step RoutingStep, result StepResult)

// stepCallbackKey is the context key for per-request step callbacks
type stepCallbackKey struct{}

// WithStepCallback returns a new context with the step callback attached.
// This allows per-request callbacks without modifying the orchestrator config.
func WithStepCallback(ctx context.Context, callback StepCompleteCallback) context.Context {
	return context.WithValue(ctx, stepCallbackKey{}, callback)
}

// GetStepCallback retrieves the step callback from context, if present.
func GetStepCallback(ctx context.Context) StepCompleteCallback {
	if cb, ok := ctx.Value(stepCallbackKey{}).(StepCompleteCallback); ok {
		return cb
	}
	return nil
}

// ExecutionOptions configures execution behavior
type ExecutionOptions struct {
	MaxConcurrency   int           `json:"max_concurrency"`
	StepTimeout      time.Duration `json:"step_timeout"`
	TotalTimeout     time.Duration `json:"total_timeout"`
	RetryAttempts    int           `json:"retry_attempts"`
	RetryDelay       time.Duration `json:"retry_delay"`
	CircuitBreaker   bool          `json:"circuit_breaker"`
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`

	// Layer 3: Validation Feedback configuration
	// When enabled, type errors trigger LLM-based parameter correction
	ValidationFeedbackEnabled bool `json:"validation_feedback_enabled"`
	MaxValidationRetries      int  `json:"max_validation_retries"` // Default: 2

	// Step completion callback for progress reporting (v1 addition)
	// Called after each step completes (success or failure).
	// Used by async task handlers to report per-tool progress.
	// See notes/ASYNC_TASK_DESIGN.md Phase 6 for details.
	OnStepComplete StepCompleteCallback `json:"-"` // Not serializable
}

// ServiceCapabilityConfig holds configuration for the service capability provider
type ServiceCapabilityConfig struct {
	// Required configuration
	Endpoint  string        `json:"endpoint"`
	TopK      int           `json:"top_k"`     // Default: 20
	Threshold float64       `json:"threshold"` // Default: 0.7
	Timeout   time.Duration `json:"timeout"`   // Default: 30s

	// Optional dependencies (not serializable, injected by application)
	CircuitBreaker   core.CircuitBreaker `json:"-"` // Optional: sophisticated resilience
	Logger           core.Logger         `json:"-"` // Optional: observability
	Telemetry        core.Telemetry      `json:"-"` // Optional: metrics
	FallbackProvider CapabilityProvider  `json:"-"` // Optional: graceful degradation
}

// OrchestratorConfig configures the orchestrator
type OrchestratorConfig struct {
	RoutingMode       RouterMode        `json:"routing_mode"`
	ExecutionOptions  ExecutionOptions  `json:"execution_options"`
	SynthesisStrategy SynthesisStrategy `json:"synthesis_strategy"`
	HistorySize       int               `json:"history_size"`
	MetricsEnabled    bool              `json:"metrics_enabled"`
	CacheEnabled      bool              `json:"cache_enabled"`
	CacheTTL          time.Duration     `json:"cache_ttl"`

	// CapabilityProvider configuration
	CapabilityProviderType string                  `json:"capability_provider_type"` // "default" or "service"
	CapabilityService      ServiceCapabilityConfig `json:"capability_service"`       // Service provider config
	EnableFallback         bool                    `json:"enable_fallback"`          // Graceful degradation

	// PromptBuilder configuration (extensible prompt customization)
	// Use omitempty to maintain backwards compatibility with existing JSON consumers
	PromptConfig PromptConfig `json:"prompt_config,omitempty"`

	// Telemetry configuration (uses framework telemetry)
	EnableTelemetry bool `json:"enable_telemetry"`

	// Hybrid Parameter Resolution (auto-wiring + micro-resolution)
	// When enabled, uses schema-based auto-wiring for parameter binding between steps,
	// with LLM-based micro-resolution as fallback for complex cases.
	// This provides more reliable parameter binding than template substitution alone.
	EnableHybridResolution bool `json:"enable_hybrid_resolution"`

	// Plan Parse Retry configuration
	// When enabled, retries LLM plan generation if JSON parsing fails.
	// This handles cases where the LLM produces invalid JSON (arithmetic expressions,
	// malformed syntax) that cannot be fixed by the cleanup functions.
	PlanParseRetryEnabled bool `json:"plan_parse_retry_enabled"`
	PlanParseMaxRetries   int  `json:"plan_parse_max_retries"` // Default: 2

	// Layer 4: Semantic Retry Configuration
	// When enabled, uses ContextualReResolver to fix errors that require computation
	SemanticRetry SemanticRetryConfig `json:"semantic_retry,omitempty"`
}

// SemanticRetryConfig configures Layer 4 contextual re-resolution
type SemanticRetryConfig struct {
	// Enable contextual re-resolution on validation errors (default: true)
	Enabled bool `json:"enabled"`

	// Maximum semantic retry attempts per step (default: 2)
	MaxAttempts int `json:"max_attempts"`

	// HTTP status codes that trigger semantic retry in addition to ErrorAnalyzer
	// Default: [400, 422] - validation errors that might be fixable with different params
	TriggerStatusCodes []int `json:"trigger_status_codes,omitempty"`

	// EnableForIndependentSteps controls whether Layer 4 runs for steps without
	// dependencies (no DependsOn entries). When true, semantic retry activates
	// even when source data from previous steps is empty.
	// Default: true | Env: GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS
	EnableForIndependentSteps bool `json:"enable_for_independent_steps"`
}

// DefaultConfig returns default orchestrator configuration with intelligent defaults
func DefaultConfig() *OrchestratorConfig {
	config := &OrchestratorConfig{
		RoutingMode:       ModeAutonomous, // Default to AI-driven orchestration
		SynthesisStrategy: StrategyLLM,
		HistorySize:       100,
		MetricsEnabled:    true,
		CacheEnabled:      true,
		CacheTTL:          5 * time.Minute,
		ExecutionOptions: ExecutionOptions{
			MaxConcurrency:   5,
			StepTimeout:      30 * time.Second,
			TotalTimeout:     2 * time.Minute,
			RetryAttempts:    2,
			RetryDelay:       2 * time.Second,
			CircuitBreaker:   true,
			FailureThreshold: 5,
			RecoveryTimeout:  30 * time.Second,
			// Layer 3: Validation Feedback defaults
			ValidationFeedbackEnabled: true, // Enable by default for production reliability
			MaxValidationRetries:      2,    // Up to 2 correction attempts
		},
		// CapabilityProvider defaults
		CapabilityProviderType: "default", // Quick start default
		EnableTelemetry:        true,      // Production-first
		EnableFallback:         true,      // Graceful degradation

		// Hybrid Parameter Resolution defaults
		EnableHybridResolution: true, // Enable by default for reliable parameter binding

		// Plan Parse Retry defaults
		PlanParseRetryEnabled: true, // Enable by default for production reliability
		PlanParseMaxRetries:   2,    // Up to 2 retry attempts after initial failure
	}

	// Auto-configure based on environment (intelligent configuration)
	if serviceURL := os.Getenv("GOMIND_CAPABILITY_SERVICE_URL"); serviceURL != "" {
		// User intent is clear - auto-configure for service provider
		config.CapabilityProviderType = "service"
		config.CapabilityService.Endpoint = serviceURL
	}

	// Plan Parse Retry configuration from environment
	if retryEnabled := os.Getenv("GOMIND_PLAN_RETRY_ENABLED"); retryEnabled != "" {
		config.PlanParseRetryEnabled = strings.ToLower(retryEnabled) == "true"
	}
	if maxRetries := os.Getenv("GOMIND_PLAN_RETRY_MAX"); maxRetries != "" {
		if val, err := strconv.Atoi(maxRetries); err == nil && val >= 0 {
			config.PlanParseMaxRetries = val
		}
	}

	// Layer 4: Semantic Retry defaults
	config.SemanticRetry = SemanticRetryConfig{
		Enabled:                   true,
		MaxAttempts:               2,
		TriggerStatusCodes:        []int{400, 422},
		EnableForIndependentSteps: true, // Default: enabled for steps without dependencies
	}

	// Semantic Retry configuration from environment
	if enabled := os.Getenv("GOMIND_SEMANTIC_RETRY_ENABLED"); enabled != "" {
		config.SemanticRetry.Enabled = strings.ToLower(enabled) == "true"
	}
	if maxAttempts := os.Getenv("GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS"); maxAttempts != "" {
		if val, err := strconv.Atoi(maxAttempts); err == nil && val >= 0 {
			config.SemanticRetry.MaxAttempts = val
		}
	}
	if independentSteps := os.Getenv("GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS"); independentSteps != "" {
		config.SemanticRetry.EnableForIndependentSteps = strings.ToLower(independentSteps) == "true"
	}

	return config
}

// ExecutionError represents an error during execution
type ExecutionError struct {
	StepID  string `json:"step_id"`
	Agent   string `json:"agent"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Retries int    `json:"retries"`
}

func (e *ExecutionError) Error() string {
	return e.Code + " at " + e.Agent + ": " + e.Message
}

// Common error codes
const (
	ErrAgentTimeout      = "AGENT_TIMEOUT"
	ErrAgentUnavailable  = "AGENT_UNAVAILABLE"
	ErrAgentError        = "AGENT_ERROR"
	ErrSynthesisFailure  = "SYNTHESIS_FAILURE"
	ErrRoutingFailure    = "ROUTING_FAILURE"
	ErrCircuitOpen       = "CIRCUIT_BREAKER_OPEN"
	ErrMaxRetriesReached = "MAX_RETRIES_REACHED"
)
