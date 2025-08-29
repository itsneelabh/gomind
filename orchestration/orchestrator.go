package orchestration

import (
	"context"
	"sync"
	"time"
)

// StandardOrchestrator is the default implementation of Orchestrator
type StandardOrchestrator struct {
	config         *OrchestratorConfig
	
	// Metrics and history
	metrics        *OrchestratorMetrics
	history        []ExecutionRecord
	historyMutex   sync.RWMutex
	metricsMutex   sync.RWMutex
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config *OrchestratorConfig) *StandardOrchestrator {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &StandardOrchestrator{
		config:  config,
		metrics: &OrchestratorMetrics{},
		history: make([]ExecutionRecord, 0, config.HistorySize),
	}
}

// ProcessRequest handles a natural language request by orchestrating multiple agents
func (o *StandardOrchestrator) ProcessRequest(ctx context.Context, request string, metadata map[string]interface{}) (*OrchestratorResponse, error) {
	startTime := time.Now()
	
	// For now, return a simple response
	// In a full implementation, this would route to agents and synthesize responses
	response := &OrchestratorResponse{
		RequestID:       generateID(),
		OriginalRequest: request,
		Response:        "Orchestration module is being refactored",
		RoutingMode:     o.config.RoutingMode,
		ExecutionTime:   time.Since(startTime),
		AgentsInvolved:  []string{},
		Metadata:        metadata,
		Confidence:      1.0,
	}
	
	// Update metrics
	o.updateMetrics(response.ExecutionTime, true)
	
	// Add to history
	o.addToHistory(response)
	
	return response, nil
}

// ExecutePlan executes a pre-defined routing plan
func (o *StandardOrchestrator) ExecutePlan(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error) {
	// Simplified implementation
	return &ExecutionResult{
		PlanID:        plan.PlanID,
		Steps:         []StepResult{},
		Success:       true,
		TotalDuration: 0,
		Metadata:      make(map[string]interface{}),
	}, nil
}

// GetExecutionHistory returns recent execution history
func (o *StandardOrchestrator) GetExecutionHistory() []ExecutionRecord {
	o.historyMutex.RLock()
	defer o.historyMutex.RUnlock()
	
	// Return a copy of the history
	historyCopy := make([]ExecutionRecord, len(o.history))
	copy(historyCopy, o.history)
	return historyCopy
}

// GetMetrics returns orchestrator metrics
func (o *StandardOrchestrator) GetMetrics() OrchestratorMetrics {
	o.metricsMutex.RLock()
	defer o.metricsMutex.RUnlock()
	
	return *o.metrics
}

// Helper functions

func (o *StandardOrchestrator) updateMetrics(duration time.Duration, success bool) {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	
	o.metrics.TotalRequests++
	if success {
		o.metrics.SuccessfulRequests++
	} else {
		o.metrics.FailedRequests++
	}
	
	// Update latency metrics (simplified)
	if o.metrics.AverageLatency == 0 {
		o.metrics.AverageLatency = duration
	} else {
		// Simple moving average
		o.metrics.AverageLatency = (o.metrics.AverageLatency + duration) / 2
	}
	
	o.metrics.LastRequestTime = time.Now()
}

func (o *StandardOrchestrator) addToHistory(response *OrchestratorResponse) {
	o.historyMutex.Lock()
	defer o.historyMutex.Unlock()
	
	record := ExecutionRecord{
		RequestID:       response.RequestID,
		Timestamp:       time.Now(),
		Request:         response.OriginalRequest,
		Response:        response.Response,
		RoutingMode:     response.RoutingMode,
		AgentsInvolved:  response.AgentsInvolved,
		ExecutionTime:   response.ExecutionTime,
		Success:         len(response.Errors) == 0,
		Errors:          response.Errors,
	}
	
	// Add to history with size limit
	if len(o.history) >= o.config.HistorySize {
		o.history = o.history[1:]
	}
	o.history = append(o.history, record)
}

func generateID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}