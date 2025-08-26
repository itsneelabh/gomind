package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// AutonomousRouter uses LLM to intelligently route requests to agents
type AutonomousRouter struct {
	aiClient     ai.AIClient
	agentCatalog string
	cache        RoutingCache
	stats        RouterStats
	mu           sync.RWMutex
	logger       logger.Logger
	
	// Configuration
	model          string
	temperature    float64
	maxRetries     int
	cacheEnabled   bool
	cacheTTL       time.Duration
}

// NewAutonomousRouter creates a new LLM-based router
func NewAutonomousRouter(aiClient ai.AIClient, options ...AutonomousOption) *AutonomousRouter {
	r := &AutonomousRouter{
		aiClient:     aiClient,
		model:        "gpt-4", // Default model
		temperature:  0.3,      // Lower temperature for more deterministic routing
		maxRetries:   3,
		cacheEnabled: true,
		cacheTTL:     5 * time.Minute,
		logger:       logger.NewDefaultLogger(),
	}
	
	// Apply options
	for _, opt := range options {
		opt(r)
	}
	
	// Initialize cache if enabled
	if r.cacheEnabled && r.cache == nil {
		r.cache = NewSimpleCache()
	}
	
	return r
}

// AutonomousOption configures the autonomous router
type AutonomousOption func(*AutonomousRouter)

// WithModel sets the LLM model to use
func WithModel(model string) AutonomousOption {
	return func(r *AutonomousRouter) {
		r.model = model
	}
}

// WithTemperature sets the LLM temperature
func WithTemperature(temp float64) AutonomousOption {
	return func(r *AutonomousRouter) {
		r.temperature = temp
	}
}

// WithCache sets a custom cache implementation
func WithCache(cache RoutingCache) AutonomousOption {
	return func(r *AutonomousRouter) {
		r.cache = cache
		r.cacheEnabled = cache != nil
	}
}

// WithCacheTTL sets the cache TTL
func WithCacheTTL(ttl time.Duration) AutonomousOption {
	return func(r *AutonomousRouter) {
		r.cacheTTL = ttl
	}
}

// WithLogger sets a custom logger
func WithLogger(log logger.Logger) AutonomousOption {
	return func(r *AutonomousRouter) {
		r.logger = log
	}
}

// Route analyzes the prompt and creates a routing plan using LLM
func (r *AutonomousRouter) Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	tracer := otel.Tracer("gomind.routing")
	ctx, span := tracer.Start(ctx, "AutonomousRouter.Route",
		trace.WithAttributes(
			attribute.String("prompt.preview", r.truncateString(prompt, 100)),
			attribute.String("model", r.model),
			attribute.Float64("temperature", r.temperature),
		),
	)
	defer span.End()
	
	startTime := time.Now()
	planID := uuid.New().String()
	
	r.logger.Info("Starting autonomous routing", map[string]interface{}{
		"plan_id":     planID,
		"prompt_len":  len(prompt),
		"model":       r.model,
		"trace_id":    trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
	})
	
	// Update stats
	r.mu.Lock()
	r.stats.TotalRequests++
	r.mu.Unlock()
	
	// Check cache first
	if r.cacheEnabled && r.cache != nil {
		if cachedPlan, found := r.cache.Get(prompt); found {
			r.logger.Debug("Cache hit for routing plan", map[string]interface{}{
				"plan_id":    cachedPlan.ID,
				"prompt_len": len(prompt),
			})
			span.AddEvent("Cache hit")
			r.mu.Lock()
			r.stats.CacheHits++
			r.mu.Unlock()
			return cachedPlan, nil
		}
		r.mu.Lock()
		r.stats.CacheMisses++
		r.mu.Unlock()
	}
	
	// Generate routing plan using LLM
	plan, err := r.generateRoutingPlan(ctx, prompt, metadata)
	if err != nil {
		r.mu.Lock()
		r.stats.FailedRoutes++
		r.mu.Unlock()
		return nil, err
	}
	
	// Cache the plan if enabled
	if r.cacheEnabled && r.cache != nil && plan != nil {
		r.cache.Set(prompt, plan, r.cacheTTL)
	}
	
	// Update stats
	r.mu.Lock()
	r.stats.SuccessfulRoutes++
	r.stats.LastRoutingTime = time.Now()
	latency := time.Since(startTime)
	// Simple moving average for latency
	if r.stats.AverageLatency == 0 {
		r.stats.AverageLatency = latency
	} else {
		r.stats.AverageLatency = (r.stats.AverageLatency + latency) / 2
	}
	r.mu.Unlock()
	
	return plan, nil
}

// generateRoutingPlan creates a routing plan using LLM
func (r *AutonomousRouter) generateRoutingPlan(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	tracer := otel.Tracer("gomind.routing")
	ctx, span := tracer.Start(ctx, "GenerateRoutingPlan")
	defer span.End()
	
	planID := uuid.New().String()
	
	if r.agentCatalog == "" {
		r.logger.Error("No agent catalog available", map[string]interface{}{
			"plan_id": planID,
		})
		span.SetStatus(codes.Error, "No catalog")
		return nil, &RoutingError{
			Code:    ErrNoAgentsAvailable,
			Message: "No agent catalog available for routing",
		}
	}
	
	// Build the LLM prompt
	llmPrompt := r.buildLLMPrompt(prompt, metadata)
	
	r.logger.Debug("Calling LLM for routing", map[string]interface{}{
		"plan_id":     planID,
		"prompt_size": len(llmPrompt),
		"model":       r.model,
	})
	
	// Call LLM with retries
	var response *ai.AIResponse
	var err error
	
	for attempt := 0; attempt < r.maxRetries; attempt++ {
		if attempt > 0 {
			r.logger.Debug("Retrying LLM call", map[string]interface{}{
				"plan_id": planID,
				"attempt": attempt + 1,
			})
		}
		
		response, err = r.aiClient.GenerateResponse(ctx, llmPrompt, &ai.GenerationOptions{
			Model:       r.model,
			Temperature: r.temperature,
			MaxTokens:   2000,
			SystemPrompt: `You are an intelligent routing system for a multi-agent framework. 
Your task is to analyze user requests and create routing plans that determine which agents to contact and in what order.
Always respond with valid JSON that matches the specified schema.
Be precise and efficient in your routing decisions.`,
		})
		
		if err == nil {
			r.logger.Debug("LLM call successful", map[string]interface{}{
				"plan_id":       planID,
				"response_size": len(response.Content),
			})
			break
		}
		
		r.logger.Warn("LLM call failed", map[string]interface{}{
			"plan_id": planID,
			"attempt": attempt + 1,
			"error":   err.Error(),
		})
		
		if attempt < r.maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	
	if err != nil {
		r.logger.Error("Failed to generate routing plan", map[string]interface{}{
			"plan_id": planID,
			"error":   err.Error(),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, "LLM failure")
		return nil, &RoutingError{
			Code:    ErrLLMFailure,
			Message: "Failed to generate routing plan",
			Details: err.Error(),
		}
	}
	
	// Parse LLM response into routing plan
	plan, err := r.parseLLMResponse(response.Content, prompt)
	if err != nil {
		r.logger.Error("Failed to parse LLM response", map[string]interface{}{
			"plan_id": planID,
			"error":   err.Error(),
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, "Parse failure")
		return nil, &RoutingError{
			Code:    ErrPlanGeneration,
			Message: "Failed to parse LLM routing decision",
			Details: err.Error(),
		}
	}
	
	plan.ID = planID
	
	r.logger.Info("Routing plan generated", map[string]interface{}{
		"plan_id":     plan.ID,
		"steps_count": len(plan.Steps),
		"confidence":  plan.Confidence,
	})
	span.SetAttributes(
		attribute.String("plan.id", plan.ID),
		attribute.Int("plan.steps", len(plan.Steps)),
	)
	span.SetStatus(codes.Ok, "Plan generated")
	
	return plan, nil
}

// truncateString truncates a string to a maximum length
func (r *AutonomousRouter) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// buildLLMPrompt constructs the prompt for the LLM
func (r *AutonomousRouter) buildLLMPrompt(userRequest string, metadata map[string]interface{}) string {
	var prompt strings.Builder
	
	prompt.WriteString("AVAILABLE AGENTS:\n")
	prompt.WriteString(r.agentCatalog)
	prompt.WriteString("\n\n")
	
	prompt.WriteString("USER REQUEST:\n")
	prompt.WriteString(userRequest)
	prompt.WriteString("\n\n")
	
	// Add metadata if available
	if len(metadata) > 0 {
		prompt.WriteString("ADDITIONAL CONTEXT:\n")
		for key, value := range metadata {
			prompt.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
		prompt.WriteString("\n")
	}
	
	prompt.WriteString(`TASK:
Analyze the user request and create a routing plan to fulfill it using the available agents.

IMPORTANT GUIDELINES:
1. Only use agents that are listed as available
2. Break complex requests into logical steps
3. Identify dependencies between steps
4. Mark steps as parallel when they don't depend on each other
5. Provide clear, specific instructions for each agent
6. Consider agent capabilities and specializations
7. Prefer agents with "healthy" status

RESPONSE FORMAT:
You must respond with a JSON object in this exact format:
{
  "analysis": "Brief analysis of what the user is asking for",
  "selected_agents": ["agent1", "agent2"],
  "confidence": 0.95,
  "steps": [
    {
      "order": 1,
      "agent_name": "agent-name",
      "namespace": "namespace",
      "instruction": "Specific instruction for this agent",
      "depends_on": [],
      "parallel": false,
      "required": true,
      "reason": "Why this agent was chosen"
    }
  ],
  "expected_outcome": "What the final result should be"
}

Ensure your response is valid JSON that can be parsed.`)
	
	return prompt.String()
}

// parseLLMResponse converts the LLM response into a RoutingPlan
func (r *AutonomousRouter) parseLLMResponse(llmResponse string, originalPrompt string) (*RoutingPlan, error) {
	// Extract JSON from response (LLM might include explanation text)
	jsonStart := strings.Index(llmResponse, "{")
	jsonEnd := strings.LastIndex(llmResponse, "}")
	
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("no valid JSON found in LLM response")
	}
	
	jsonStr := llmResponse[jsonStart : jsonEnd+1]
	
	// Parse the JSON response
	var llmPlan struct {
		Analysis       string  `json:"analysis"`
		SelectedAgents []string `json:"selected_agents"`
		Confidence     float64 `json:"confidence"`
		Steps          []struct {
			Order       int      `json:"order"`
			AgentName   string   `json:"agent_name"`
			Namespace   string   `json:"namespace"`
			Instruction string   `json:"instruction"`
			DependsOn   []int    `json:"depends_on"`
			Parallel    bool     `json:"parallel"`
			Required    bool     `json:"required"`
			Reason      string   `json:"reason"`
		} `json:"steps"`
		ExpectedOutcome string `json:"expected_outcome"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &llmPlan); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Convert to RoutingPlan
	plan := &RoutingPlan{
		ID:             uuid.New().String(),
		Mode:           ModeAutonomous,
		OriginalPrompt: originalPrompt,
		Confidence:     llmPlan.Confidence,
		CreatedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"analysis":         llmPlan.Analysis,
			"selected_agents":  llmPlan.SelectedAgents,
			"expected_outcome": llmPlan.ExpectedOutcome,
		},
	}
	
	// Convert steps
	for _, llmStep := range llmPlan.Steps {
		step := RoutingStep{
			Order:       llmStep.Order,
			StepID:      fmt.Sprintf("step-%d-%s", llmStep.Order, uuid.New().String()[:8]),
			AgentName:   llmStep.AgentName,
			Namespace:   llmStep.Namespace,
			Instruction: llmStep.Instruction,
			DependsOn:   llmStep.DependsOn,
			Parallel:    llmStep.Parallel,
			Required:    llmStep.Required,
			Timeout:     30 * time.Second, // Default timeout
		}
		
		// Add retry policy for required steps
		if step.Required {
			step.RetryPolicy = &RetryPolicy{
				MaxAttempts: 3,
				Delay:       2 * time.Second,
				BackoffType: "exponential",
			}
		}
		
		plan.Steps = append(plan.Steps, step)
	}
	
	// Estimate duration based on steps
	plan.EstimatedDuration = time.Duration(len(plan.Steps)) * 5 * time.Second
	
	return plan, nil
}

// GetMode returns the router mode
func (r *AutonomousRouter) GetMode() RouterMode {
	return ModeAutonomous
}

// SetAgentCatalog updates the available agents catalog
func (r *AutonomousRouter) SetAgentCatalog(catalog string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agentCatalog = catalog
}

// GetStats returns routing statistics
func (r *AutonomousRouter) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// ClearCache clears the routing cache
func (r *AutonomousRouter) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
	}
}