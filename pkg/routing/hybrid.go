package routing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
)

// HybridRouter combines workflow and autonomous routing strategies
type HybridRouter struct {
	workflowRouter   *WorkflowRouter
	autonomousRouter *AutonomousRouter
	cache            RoutingCache
	stats            RouterStats
	mu               sync.RWMutex
	
	// Configuration
	preferWorkflow   bool
	enhanceWithLLM   bool
	fallbackEnabled  bool
	cacheEnabled     bool
	cacheTTL         time.Duration
	confidenceThreshold float64
}

// NewHybridRouter creates a new hybrid router
func NewHybridRouter(workflowPath string, aiClient ai.AIClient, options ...HybridOption) (*HybridRouter, error) {
	// Create workflow router
	workflowRouter, err := NewWorkflowRouter(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow router: %w", err)
	}
	
	// Create autonomous router
	autonomousRouter := NewAutonomousRouter(aiClient)
	
	r := &HybridRouter{
		workflowRouter:      workflowRouter,
		autonomousRouter:    autonomousRouter,
		preferWorkflow:      true,  // Default: prefer workflows
		enhanceWithLLM:      false, // Default: don't enhance
		fallbackEnabled:     true,  // Default: enable fallback
		cacheEnabled:        true,
		cacheTTL:            10 * time.Minute,
		confidenceThreshold: 0.7,   // Minimum confidence for autonomous routing
	}
	
	// Apply options
	for _, opt := range options {
		opt(r)
	}
	
	// Initialize cache if enabled
	if r.cacheEnabled && r.cache == nil {
		r.cache = NewLRUCache(500) // Smaller cache for hybrid router
	}
	
	return r, nil
}

// HybridOption configures the hybrid router
type HybridOption func(*HybridRouter)

// WithPreferWorkflow sets whether to prefer workflow routing
func WithPreferWorkflow(prefer bool) HybridOption {
	return func(r *HybridRouter) {
		r.preferWorkflow = prefer
	}
}

// WithEnhancement enables LLM enhancement of workflow plans
func WithEnhancement(enhance bool) HybridOption {
	return func(r *HybridRouter) {
		r.enhanceWithLLM = enhance
	}
}

// WithFallback enables/disables fallback to autonomous routing
func WithFallback(enabled bool) HybridOption {
	return func(r *HybridRouter) {
		r.fallbackEnabled = enabled
	}
}

// WithHybridCache sets a custom cache
func WithHybridCache(cache RoutingCache) HybridOption {
	return func(r *HybridRouter) {
		r.cache = cache
		r.cacheEnabled = cache != nil
	}
}

// WithConfidenceThreshold sets minimum confidence for autonomous routing
func WithConfidenceThreshold(threshold float64) HybridOption {
	return func(r *HybridRouter) {
		r.confidenceThreshold = threshold
	}
}

// Route analyzes the prompt and creates a routing plan using hybrid strategy
func (r *HybridRouter) Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	startTime := time.Now()
	
	// Update stats
	r.mu.Lock()
	r.stats.TotalRequests++
	r.mu.Unlock()
	
	// Check cache first
	if r.cacheEnabled && r.cache != nil {
		if cachedPlan, found := r.cache.Get(prompt); found {
			r.mu.Lock()
			r.stats.CacheHits++
			r.mu.Unlock()
			return cachedPlan, nil
		}
		r.mu.Lock()
		r.stats.CacheMisses++
		r.mu.Unlock()
	}
	
	var plan *RoutingPlan
	var routingErr error
	
	if r.preferWorkflow {
		// Try workflow routing first
		plan, routingErr = r.tryWorkflowRouting(ctx, prompt, metadata)
		
		// If no workflow matched and fallback is enabled, try autonomous
		if plan == nil && r.fallbackEnabled {
			plan, routingErr = r.tryAutonomousRouting(ctx, prompt, metadata)
		}
	} else {
		// Try autonomous routing first
		plan, routingErr = r.tryAutonomousRouting(ctx, prompt, metadata)
		
		// If confidence is low and fallback is enabled, try workflow
		if plan != nil && plan.Confidence < r.confidenceThreshold && r.fallbackEnabled {
			workflowPlan, _ := r.tryWorkflowRouting(ctx, prompt, metadata)
			if workflowPlan != nil {
				plan = workflowPlan
			}
		}
	}
	
	// If we have a plan and enhancement is enabled, enhance it
	if plan != nil && r.enhanceWithLLM && plan.Mode == ModeWorkflow {
		plan = r.enhancePlanWithLLM(ctx, plan, prompt, metadata)
	}
	
	if plan == nil {
		r.mu.Lock()
		r.stats.FailedRoutes++
		r.mu.Unlock()
		if routingErr != nil {
			return nil, routingErr
		}
		return nil, &RoutingError{
			Code:    ErrNoAgentsAvailable,
			Message: "No routing strategy could handle this request",
		}
	}
	
	// Mark as hybrid mode
	plan.Mode = ModeHybrid
	
	// Cache the plan if enabled
	if r.cacheEnabled && r.cache != nil {
		r.cache.Set(prompt, plan, r.cacheTTL)
	}
	
	// Update stats
	r.mu.Lock()
	r.stats.SuccessfulRoutes++
	r.stats.LastRoutingTime = time.Now()
	latency := time.Since(startTime)
	if r.stats.AverageLatency == 0 {
		r.stats.AverageLatency = latency
	} else {
		r.stats.AverageLatency = (r.stats.AverageLatency + latency) / 2
	}
	r.mu.Unlock()
	
	return plan, nil
}

// tryWorkflowRouting attempts to route using workflows
func (r *HybridRouter) tryWorkflowRouting(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	plan, err := r.workflowRouter.Route(ctx, prompt, metadata)
	if err != nil {
		// Check if it's a "not found" error
		if routingErr, ok := err.(*RoutingError); ok && routingErr.Code == ErrWorkflowNotFound {
			return nil, nil // No workflow matched, this is okay
		}
		return nil, err // Actual error
	}
	return plan, nil
}

// tryAutonomousRouting attempts to route using LLM
func (r *HybridRouter) tryAutonomousRouting(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	plan, err := r.autonomousRouter.Route(ctx, prompt, metadata)
	if err != nil {
		return nil, err
	}
	
	// Check confidence threshold
	if plan != nil && plan.Confidence < r.confidenceThreshold {
		// Add metadata about low confidence
		if plan.Metadata == nil {
			plan.Metadata = make(map[string]interface{})
		}
		plan.Metadata["low_confidence_warning"] = fmt.Sprintf("Confidence %.2f is below threshold %.2f", 
			plan.Confidence, r.confidenceThreshold)
	}
	
	return plan, nil
}

// enhancePlanWithLLM uses LLM to enhance a workflow plan
func (r *HybridRouter) enhancePlanWithLLM(ctx context.Context, plan *RoutingPlan, prompt string, metadata map[string]interface{}) *RoutingPlan {
	// Create a new context with timeout for enhancement
	enhanceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	// Build enhancement prompt
	enhancePrompt := fmt.Sprintf(`Given this workflow plan for the user request, suggest any improvements or additional considerations:

User Request: %s

Current Plan:
`, prompt)
	
	for i, step := range plan.Steps {
		enhancePrompt += fmt.Sprintf("%d. Agent: %s, Instruction: %s\n", i+1, step.AgentName, step.Instruction)
	}
	
	enhancePrompt += `
Provide any improvements as a JSON object with fields:
- "additional_steps": array of new steps to add
- "modifications": array of step modifications
- "warnings": array of potential issues
- "optimization_hints": array of optimization suggestions`
	
	// Call LLM for enhancement
	response, err := r.autonomousRouter.aiClient.GenerateResponse(enhanceCtx, enhancePrompt, &ai.GenerationOptions{
		Model:       r.autonomousRouter.model,
		Temperature: 0.3,
		MaxTokens:   1000,
	})
	
	if err == nil && response != nil {
		// Add enhancement metadata
		if plan.Metadata == nil {
			plan.Metadata = make(map[string]interface{})
		}
		plan.Metadata["llm_enhanced"] = true
		plan.Metadata["enhancement_suggestions"] = response.Content
		
		// Potentially parse and apply enhancements (simplified for now)
		// In a full implementation, we would parse the JSON and modify the plan
	}
	
	return plan
}

// GetMode returns the router mode
func (r *HybridRouter) GetMode() RouterMode {
	return ModeHybrid
}

// SetAgentCatalog updates the agent catalog for autonomous routing
func (r *HybridRouter) SetAgentCatalog(catalog string) {
	r.autonomousRouter.SetAgentCatalog(catalog)
}

// GetStats returns routing statistics
func (r *HybridRouter) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Combine stats from both routers
	workflowStats := r.workflowRouter.GetStats()
	autonomousStats := r.autonomousRouter.GetStats()
	
	// Return hybrid stats with additional breakdown
	stats := r.stats
	stats.Metadata = map[string]interface{}{
		"workflow_stats":   workflowStats,
		"autonomous_stats": autonomousStats,
		"prefer_workflow":  r.preferWorkflow,
		"enhance_enabled":  r.enhanceWithLLM,
		"fallback_enabled": r.fallbackEnabled,
	}
	
	return stats
}

// ReloadWorkflows reloads workflow definitions
func (r *HybridRouter) ReloadWorkflows() error {
	return r.workflowRouter.ReloadWorkflows()
}

// AddWorkflow adds a new workflow definition
func (r *HybridRouter) AddWorkflow(workflow *WorkflowDefinition) error {
	return r.workflowRouter.AddWorkflow(workflow)
}

// RemoveWorkflow removes a workflow definition
func (r *HybridRouter) RemoveWorkflow(name string) {
	r.workflowRouter.RemoveWorkflow(name)
}

// ListWorkflows returns all loaded workflows
func (r *HybridRouter) ListWorkflows() []string {
	return r.workflowRouter.ListWorkflows()
}

// ClearCache clears the routing cache
func (r *HybridRouter) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
	}
	// Also clear sub-router caches
	r.autonomousRouter.ClearCache()
	if r.workflowRouter.cache != nil {
		r.workflowRouter.cache.Clear()
	}
}

// SetConfidenceThreshold updates the confidence threshold at runtime
func (r *HybridRouter) SetConfidenceThreshold(threshold float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.confidenceThreshold = threshold
}

// SetPreferWorkflow updates the workflow preference at runtime
func (r *HybridRouter) SetPreferWorkflow(prefer bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preferWorkflow = prefer
}

// SetEnhancement enables/disables LLM enhancement at runtime
func (r *HybridRouter) SetEnhancement(enhance bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enhanceWithLLM = enhance
}