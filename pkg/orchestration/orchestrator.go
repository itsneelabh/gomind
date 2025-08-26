package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/communication"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/routing"
	"github.com/google/uuid"
)

// StandardOrchestrator is the default implementation of Orchestrator
type StandardOrchestrator struct {
	config         *OrchestratorConfig
	router         routing.Router
	executor       Executor
	synthesizer    Synthesizer
	communicator   communication.AgentCommunicator
	aiClient       ai.AIClient
	logger         logger.Logger
	
	// Metrics and history
	metrics        *OrchestratorMetrics
	history        []ExecutionRecord
	historyMutex   sync.RWMutex
	metricsMutex   sync.RWMutex
	
	// Response cache
	cache          map[string]*cachedResponse
	cacheMutex     sync.RWMutex
	
	// Circuit breaker state
	circuitBreaker *CircuitBreaker
	
	startTime      time.Time
}

type cachedResponse struct {
	response  *OrchestratorResponse
	expiresAt time.Time
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(
	router routing.Router,
	communicator communication.AgentCommunicator,
	aiClient ai.AIClient,
	logger logger.Logger,
	config *OrchestratorConfig,
) *StandardOrchestrator {
	if config == nil {
		config = DefaultConfig()
	}
	
	o := &StandardOrchestrator{
		config:       config,
		router:       router,
		communicator: communicator,
		aiClient:     aiClient,
		logger:       logger,
		metrics:      &OrchestratorMetrics{},
		history:      make([]ExecutionRecord, 0, config.HistorySize),
		cache:        make(map[string]*cachedResponse),
		startTime:    time.Now(),
	}
	
	// Initialize executor
	o.executor = NewPlanExecutor(communicator, logger, &config.ExecutionOptions)
	
	// Initialize synthesizer
	o.synthesizer = NewResponseSynthesizer(aiClient, logger, config.SynthesisStrategy)
	
	// Initialize circuit breaker if enabled
	if config.ExecutionOptions.CircuitBreaker {
		o.circuitBreaker = NewCircuitBreaker(
			config.ExecutionOptions.FailureThreshold,
			config.ExecutionOptions.RecoveryTimeout,
		)
	}
	
	// Start cache cleanup routine
	if config.CacheEnabled {
		go o.cleanupCache()
	}
	
	return o
}

// ProcessRequest handles a natural language request by orchestrating multiple agents
func (o *StandardOrchestrator) ProcessRequest(
	ctx context.Context,
	request string,
	metadata map[string]interface{},
) (*OrchestratorResponse, error) {
	startTime := time.Now()
	requestID := uuid.New().String()
	
	o.logger.Info("Processing orchestrator request", map[string]interface{}{
		"request_id": requestID,
		"request":    request,
		"metadata":   metadata,
	})
	
	// Update metrics
	o.incrementRequestCount()
	
	// Check cache if enabled
	if o.config.CacheEnabled {
		if cached := o.checkCache(request); cached != nil {
			o.logger.Debug("Returning cached response", map[string]interface{}{
				"request_id": requestID,
			})
			return cached, nil
		}
	}
	
	// Check circuit breaker
	if o.circuitBreaker != nil && !o.circuitBreaker.CanExecute() {
		o.incrementFailureCount()
		return nil, &ExecutionError{
			Code:    ErrCircuitOpen,
			Message: "Circuit breaker is open due to high failure rate",
		}
	}
	
	// Create routing plan
	plan, err := o.router.Route(ctx, request, metadata)
	if err != nil {
		o.logger.Error("Failed to create routing plan", map[string]interface{}{
			"error":      err.Error(),
			"request_id": requestID,
		})
		o.incrementFailureCount()
		o.recordFailure()
		return nil, &ExecutionError{
			Code:    ErrRoutingFailure,
			Message: fmt.Sprintf("Failed to create routing plan: %v", err),
		}
	}
	
	o.logger.Info("Created routing plan", map[string]interface{}{
		"request_id":  requestID,
		"plan_id":     plan.ID,
		"mode":        plan.Mode,
		"steps_count": len(plan.Steps),
		"confidence":  plan.Confidence,
	})
	
	// Execute the plan
	result, err := o.ExecutePlan(ctx, plan)
	if err != nil {
		o.logger.Error("Failed to execute plan", map[string]interface{}{
			"error":      err.Error(),
			"request_id": requestID,
			"plan_id":    plan.ID,
		})
		o.incrementFailureCount()
		o.recordFailure()
		// Don't return error immediately - try to synthesize partial results
	}
	
	// Synthesize response
	synthesizedResponse, synthErr := o.synthesizer.Synthesize(ctx, request, result)
	if synthErr != nil {
		o.logger.Error("Failed to synthesize response", map[string]interface{}{
			"error":      synthErr.Error(),
			"request_id": requestID,
		})
		o.incrementSynthesisError()
		
		// If both execution and synthesis failed, return error
		if err != nil {
			return nil, &ExecutionError{
				Code:    ErrSynthesisFailure,
				Message: fmt.Sprintf("Execution and synthesis failed: %v, %v", err, synthErr),
			}
		}
		
		// Use simple concatenation as fallback
		synthesizedResponse = o.fallbackSynthesis(result)
	}
	
	// Collect agent names
	agentsInvolved := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		agentsInvolved = append(agentsInvolved, fmt.Sprintf("%s.%s", step.AgentName, step.Namespace))
	}
	
	// Collect errors
	var errors []string
	for _, step := range result.Steps {
		if !step.Success && step.Error != "" {
			errors = append(errors, fmt.Sprintf("%s: %s", step.AgentName, step.Error))
		}
	}
	
	// Create response
	response := &OrchestratorResponse{
		RequestID:       requestID,
		OriginalRequest: request,
		Response:        synthesizedResponse,
		RoutingMode:     plan.Mode,
		ExecutionTime:   time.Since(startTime),
		AgentsInvolved:  agentsInvolved,
		Confidence:      plan.Confidence,
		Metadata:        metadata,
		Errors:          errors,
	}
	
	// Cache response if enabled
	if o.config.CacheEnabled && result.Success {
		o.cacheResponse(request, response)
	}
	
	// Record in history
	o.recordExecution(requestID, request, response, result.Success)
	
	// Update metrics
	if result.Success {
		o.incrementSuccessCount()
		o.recordSuccess()
	} else {
		o.incrementFailureCount()
		o.recordFailure()
	}
	o.updateLatencyMetrics(time.Since(startTime))
	
	o.logger.Info("Completed orchestrator request", map[string]interface{}{
		"request_id":     requestID,
		"execution_time": time.Since(startTime),
		"success":        result.Success,
		"agents_count":   len(agentsInvolved),
	})
	
	return response, nil
}

// ExecutePlan executes a pre-defined routing plan
func (o *StandardOrchestrator) ExecutePlan(
	ctx context.Context,
	plan *routing.RoutingPlan,
) (*ExecutionResult, error) {
	// Apply total timeout if configured
	if o.config.ExecutionOptions.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.ExecutionOptions.TotalTimeout)
		defer cancel()
	}
	
	return o.executor.Execute(ctx, plan)
}

// GetExecutionHistory returns recent execution history
func (o *StandardOrchestrator) GetExecutionHistory() []ExecutionRecord {
	o.historyMutex.RLock()
	defer o.historyMutex.RUnlock()
	
	// Return a copy to prevent external modification
	historyCopy := make([]ExecutionRecord, len(o.history))
	copy(historyCopy, o.history)
	return historyCopy
}

// GetMetrics returns orchestrator metrics
func (o *StandardOrchestrator) GetMetrics() OrchestratorMetrics {
	o.metricsMutex.RLock()
	defer o.metricsMutex.RUnlock()
	
	metrics := *o.metrics
	metrics.UptimeSeconds = int64(time.Since(o.startTime).Seconds())
	return metrics
}

// fallbackSynthesis creates a simple concatenated response
func (o *StandardOrchestrator) fallbackSynthesis(result *ExecutionResult) string {
	var response string
	response = "Based on the information gathered:\n\n"
	
	for _, step := range result.Steps {
		if step.Success && step.Response != "" {
			response += fmt.Sprintf("**%s**: %s\n\n", step.AgentName, step.Response)
		}
	}
	
	if response == "Based on the information gathered:\n\n" {
		response = "Unable to gather information from the requested agents."
	}
	
	return response
}

// Cache management
func (o *StandardOrchestrator) checkCache(request string) *OrchestratorResponse {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()
	
	if cached, found := o.cache[request]; found {
		if time.Now().Before(cached.expiresAt) {
			return cached.response
		}
	}
	return nil
}

func (o *StandardOrchestrator) cacheResponse(request string, response *OrchestratorResponse) {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()
	
	o.cache[request] = &cachedResponse{
		response:  response,
		expiresAt: time.Now().Add(o.config.CacheTTL),
	}
}

func (o *StandardOrchestrator) cleanupCache() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		o.cacheMutex.Lock()
		now := time.Now()
		for key, cached := range o.cache {
			if now.After(cached.expiresAt) {
				delete(o.cache, key)
			}
		}
		o.cacheMutex.Unlock()
	}
}

// History management
func (o *StandardOrchestrator) recordExecution(
	requestID string,
	request string,
	response *OrchestratorResponse,
	success bool,
) {
	o.historyMutex.Lock()
	defer o.historyMutex.Unlock()
	
	record := ExecutionRecord{
		RequestID:      requestID,
		Timestamp:      time.Now(),
		Request:        request,
		Response:       response.Response,
		RoutingMode:    response.RoutingMode,
		AgentsInvolved: response.AgentsInvolved,
		ExecutionTime:  response.ExecutionTime,
		Success:        success,
		Errors:         response.Errors,
	}
	
	// Add to history, maintaining size limit
	o.history = append(o.history, record)
	if len(o.history) > o.config.HistorySize {
		o.history = o.history[1:]
	}
}

// Metrics management
func (o *StandardOrchestrator) incrementRequestCount() {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	o.metrics.TotalRequests++
	o.metrics.LastRequestTime = time.Now()
}

func (o *StandardOrchestrator) incrementSuccessCount() {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	o.metrics.SuccessfulRequests++
}

func (o *StandardOrchestrator) incrementFailureCount() {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	o.metrics.FailedRequests++
}

func (o *StandardOrchestrator) incrementSynthesisError() {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	o.metrics.SynthesisErrors++
}

func (o *StandardOrchestrator) updateLatencyMetrics(duration time.Duration) {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	
	// Simple moving average for now
	if o.metrics.AverageLatency == 0 {
		o.metrics.AverageLatency = duration
	} else {
		o.metrics.AverageLatency = (o.metrics.AverageLatency + duration) / 2
	}
	
	// Update median and P99 (simplified - would need proper percentile tracking)
	o.metrics.MedianLatency = o.metrics.AverageLatency
	o.metrics.P99Latency = o.metrics.AverageLatency * 2
}

func (o *StandardOrchestrator) recordSuccess() {
	if o.circuitBreaker != nil {
		o.circuitBreaker.RecordSuccess()
	}
}

func (o *StandardOrchestrator) recordFailure() {
	if o.circuitBreaker != nil {
		o.circuitBreaker.RecordFailure()
	}
}

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	failureThreshold int
	recoveryTimeout  time.Duration
	failureCount     int
	lastFailureTime  time.Time
	state            string
	mutex            sync.RWMutex
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: threshold,
		recoveryTimeout:  timeout,
		state:            "closed",
	}
}

func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	if cb.state == "open" {
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			return true // Allow half-open state
		}
		return false
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if cb.state == "open" && time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
		cb.state = "closed"
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	
	if cb.failureCount >= cb.failureThreshold {
		cb.state = "open"
	}
}