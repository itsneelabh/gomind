package routing

import (
	"context"
	"time"
)

// RouterMode defines the routing strategy
type RouterMode string

const (
	// ModeAutonomous uses LLM to decide routing
	ModeAutonomous RouterMode = "autonomous"
	// ModeWorkflow uses predefined workflow patterns
	ModeWorkflow RouterMode = "workflow"
	// ModeHybrid tries workflow first, then falls back to autonomous
	ModeHybrid RouterMode = "hybrid"
)

// Router is the main interface for all routing implementations
type Router interface {
	// Route analyzes the prompt and creates a routing plan
	Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error)
	
	// GetMode returns the router mode
	GetMode() RouterMode
	
	// SetAgentCatalog updates the available agents (for autonomous routing)
	SetAgentCatalog(catalog string)
	
	// GetStats returns routing statistics
	GetStats() RouterStats
}

// RoutingPlan represents a complete plan for handling a user request
type RoutingPlan struct {
	// ID is a unique identifier for this plan
	ID string `json:"id"`
	
	// Mode indicates which router created this plan
	Mode RouterMode `json:"mode"`
	
	// OriginalPrompt is the user's original request
	OriginalPrompt string `json:"original_prompt"`
	
	// Steps are the ordered execution steps
	Steps []RoutingStep `json:"steps"`
	
	// EstimatedDuration is the expected completion time
	EstimatedDuration time.Duration `json:"estimated_duration,omitempty"`
	
	// Confidence is the router's confidence in this plan (0-1)
	Confidence float64 `json:"confidence"`
	
	// Metadata contains additional routing information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// CreatedAt timestamp
	CreatedAt time.Time `json:"created_at"`
}

// RoutingStep represents a single step in the routing plan
type RoutingStep struct {
	// Order is the execution order (1-based)
	Order int `json:"order"`
	
	// StepID is a unique identifier for this step
	StepID string `json:"step_id"`
	
	// AgentName is the target agent's name
	AgentName string `json:"agent_name"`
	
	// Namespace is the agent's namespace
	Namespace string `json:"namespace"`
	
	// Instruction is what to ask the agent
	Instruction string `json:"instruction"`
	
	// DependsOn lists step orders this depends on
	DependsOn []int `json:"depends_on,omitempty"`
	
	// Parallel indicates if this can run in parallel with same-order steps
	Parallel bool `json:"parallel"`
	
	// Timeout for this specific step
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// Required indicates if this step must succeed
	Required bool `json:"required"`
	
	// ExpectedOutputType hints at what kind of response to expect
	ExpectedOutputType string `json:"expected_output_type,omitempty"`
	
	// RetryPolicy defines retry behavior
	RetryPolicy *RetryPolicy `json:"retry_policy,omitempty"`
}

// RetryPolicy defines how to handle step failures
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Delay       time.Duration `json:"delay"`
	BackoffType string        `json:"backoff_type"` // "fixed", "exponential"
}

// RouterStats provides metrics about routing performance
type RouterStats struct {
	TotalRequests      int64                  `json:"total_requests"`
	SuccessfulRoutes   int64                  `json:"successful_routes"`
	FailedRoutes       int64                  `json:"failed_routes"`
	AverageLatency     time.Duration          `json:"average_latency"`
	CacheHits          int64                  `json:"cache_hits"`
	CacheMisses        int64                  `json:"cache_misses"`
	LastRoutingTime    time.Time              `json:"last_routing_time"`
	ActivePlans        int                    `json:"active_plans"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// RoutingCache provides caching for routing decisions
type RoutingCache interface {
	// Get retrieves a cached routing plan
	Get(prompt string) (*RoutingPlan, bool)
	
	// Set stores a routing plan in cache
	Set(prompt string, plan *RoutingPlan, ttl time.Duration)
	
	// Clear removes all cached plans
	Clear()
	
	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Size        int           `json:"size"`
	Hits        int64         `json:"hits"`
	Misses      int64         `json:"misses"`
	Evictions   int64         `json:"evictions"`
	HitRate     float64       `json:"hit_rate"`
	MemoryUsage int64         `json:"memory_bytes"`
}

// WorkflowDefinition represents a predefined workflow
type WorkflowDefinition struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Triggers    WorkflowTriggers  `yaml:"triggers" json:"triggers"`
	Steps       []WorkflowStep    `yaml:"steps" json:"steps"`
	Variables   map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
	OnError     string            `yaml:"on_error,omitempty" json:"on_error,omitempty"`
}

// WorkflowTriggers defines what activates a workflow
type WorkflowTriggers struct {
	Patterns []string `yaml:"patterns,omitempty" json:"patterns,omitempty"`
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Intents  []string `yaml:"intents,omitempty" json:"intents,omitempty"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	Name        string                 `yaml:"name" json:"name"`
	Agent       string                 `yaml:"agent" json:"agent"`
	Namespace   string                 `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Instruction string                 `yaml:"instruction" json:"instruction"`
	DependsOn   []string               `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Parallel    bool                   `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	Required    bool                   `yaml:"required,omitempty" json:"required,omitempty"`
	Timeout     string                 `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Variables   map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// RoutingError represents an error during routing
type RoutingError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Step    string `json:"step,omitempty"`
}

func (e *RoutingError) Error() string {
	if e.Step != "" {
		return e.Code + " at step " + e.Step + ": " + e.Message
	}
	return e.Code + ": " + e.Message
}

// Common error codes
const (
	ErrNoAgentsAvailable = "NO_AGENTS_AVAILABLE"
	ErrLLMFailure        = "LLM_FAILURE"
	ErrInvalidPrompt     = "INVALID_PROMPT"
	ErrWorkflowNotFound  = "WORKFLOW_NOT_FOUND"
	ErrPlanGeneration    = "PLAN_GENERATION_FAILED"
	ErrCacheFailure      = "CACHE_FAILURE"
)