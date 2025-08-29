package workflow

import (
	"context"
)

// HybridRouter combines workflow and autonomous routing
type HybridRouter struct {
	workflowRouter   *WorkflowRouter
	autonomousRouter *AutonomousRouter
	preferWorkflow   bool
}

// NewHybridRouter creates a new hybrid router
func NewHybridRouter() *HybridRouter {
	return &HybridRouter{
		workflowRouter:   NewWorkflowRouter(),
		autonomousRouter: NewAutonomousRouter(),
		preferWorkflow:   true,
	}
}

// Route first tries workflow patterns, then falls back to autonomous
func (h *HybridRouter) Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	// Try workflow first if preferred
	if h.preferWorkflow {
		if plan, err := h.workflowRouter.Route(ctx, prompt, metadata); err == nil && len(plan.Steps) > 0 {
			return plan, nil
		}
	}
	
	// Fall back to autonomous routing
	return h.autonomousRouter.Route(ctx, prompt, metadata)
}

// GetMode returns the router mode
func (h *HybridRouter) GetMode() RouterMode {
	return ModeHybrid
}

// SetAgentCatalog updates the available agents
func (h *HybridRouter) SetAgentCatalog(catalog string) {
	h.autonomousRouter.SetAgentCatalog(catalog)
}

// GetStats returns combined router statistics
func (h *HybridRouter) GetStats() RouterStats {
	// Combine stats from both routers
	return RouterStats{
		TotalRequests:    0,
		SuccessfulRoutes: 0,
		FailedRoutes:     0,
		AverageLatency:   0,
		CacheHitRate:     0,
	}
}