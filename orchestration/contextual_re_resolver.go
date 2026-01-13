// Package orchestration provides intelligent parameter binding for multi-step workflows.
//
// This file implements Layer 4: Contextual Re-Resolution for semantic retry.
// When ErrorAnalyzer says "cannot fix" but source data exists, this component
// can compute derived values using the full execution context.
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// ExecutionContext captures all information needed for semantic retry.
// This is the "full trajectory" that enables intelligent re-resolution.
type ExecutionContext struct {
	// Original user intent (critical for understanding what to compute)
	UserQuery string

	// All source data from dependent steps (what MicroResolver had)
	SourceData map[string]interface{}

	// Step being executed
	StepID     string
	Capability *EnhancedCapability

	// What we tried (failed parameters)
	AttemptedParams map[string]interface{}

	// What went wrong
	ErrorResponse string
	HTTPStatus    int

	// Retry state (memory across attempts)
	RetryCount     int
	PreviousErrors []string
}

// ReResolutionResult is returned by ContextualReResolver
type ReResolutionResult struct {
	// Should we retry with corrected parameters?
	ShouldRetry bool `json:"should_retry"`

	// Corrected parameters to use for retry
	CorrectedParameters map[string]interface{} `json:"corrected_parameters"`

	// Explanation of what was fixed (for logging/debugging)
	Analysis string `json:"analysis"`
}

// ContextualReResolver combines error analysis with parameter re-resolution.
// Unlike ErrorAnalyzer (which only analyzes), this component can PRESCRIBE fixes
// because it has access to the full execution context including source data.
type ContextualReResolver struct {
	aiClient core.AIClient
	logger   core.Logger

	// LLM Debug Store for full payload visibility
	debugStore LLMDebugStore
	debugWg    sync.WaitGroup
	debugSeqID atomic.Uint64
}

// NewContextualReResolver creates a new contextual re-resolver
func NewContextualReResolver(aiClient core.AIClient, logger core.Logger) *ContextualReResolver {
	r := &ContextualReResolver{
		aiClient: aiClient,
		logger:   logger,
	}
	return r
}

// ReResolve attempts to resolve parameters after an execution failure.
// It uses the full execution context to compute corrected parameters.
func (r *ContextualReResolver) ReResolve(
	ctx context.Context,
	execCtx *ExecutionContext,
) (*ReResolutionResult, error) {
	if execCtx == nil {
		return nil, fmt.Errorf("execution context is required")
	}

	if r.aiClient == nil {
		return &ReResolutionResult{
			ShouldRetry: false,
			Analysis:    "AI client not configured for semantic retry",
		}, nil
	}

	// Check context before expensive LLM operation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Build comprehensive prompt with ALL context
	prompt := r.buildReResolutionPrompt(execCtx)

	// Telemetry: Track re-resolution attempt
	telemetry.AddSpanEvent(ctx, "contextual_re_resolution.start",
		attribute.String("step_id", execCtx.StepID),
		attribute.String("capability", execCtx.Capability.Name),
		attribute.Int("retry_count", execCtx.RetryCount),
		attribute.Int("http_status", execCtx.HTTPStatus),
		attribute.Int("source_data_keys", len(execCtx.SourceData)),
	)

	r.logInfo("Starting contextual re-resolution", map[string]interface{}{
		"step_id":          execCtx.StepID,
		"capability":       execCtx.Capability.Name,
		"http_status":      execCtx.HTTPStatus,
		"source_data_keys": getMapKeys(execCtx.SourceData),
	})

	// Get request ID from context baggage for debug correlation
	requestID := ""
	if baggage := telemetry.GetBaggage(ctx); baggage != nil {
		requestID = baggage["request_id"]
	}
	if requestID == "" {
		requestID = r.generateFallbackRequestID()
	}

	startTime := time.Now()

	// LLM generates corrected parameters with reasoning
	response, err := r.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.0,  // Deterministic for parameter extraction
		MaxTokens:   1000, // Allow space for reasoning and computation
	})

	duration := time.Since(startTime)

	// Record LLM latency
	telemetry.Histogram("orchestration.semantic_retry.llm_latency_ms",
		float64(duration.Milliseconds()),
		"capability", execCtx.Capability.Name,
		"module", telemetry.ModuleOrchestration,
	)

	if err != nil {
		telemetry.AddSpanEvent(ctx, "contextual_re_resolution.error",
			attribute.String("error", err.Error()),
			attribute.Int64("duration_ms", duration.Milliseconds()),
		)
		telemetry.Counter("orchestration.semantic_retry.llm_errors",
			"capability", execCtx.Capability.Name,
			"module", telemetry.ModuleOrchestration,
		)

		// LLM Debug: Record failed semantic retry attempt
		r.recordDebugInteraction(ctx, requestID, LLMInteraction{
			Type:        "semantic_retry",
			Timestamp:   startTime,
			DurationMs:  duration.Milliseconds(),
			Prompt:      prompt,
			Temperature: 0.0,
			MaxTokens:   1000,
			Success:     false,
			Error:       err.Error(),
			Attempt:     execCtx.RetryCount + 1,
		})

		r.logWarn("Re-resolution LLM call failed", map[string]interface{}{
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
		})
		return nil, fmt.Errorf("re-resolution LLM call failed: %w", err)
	}

	// LLM Debug: Record successful semantic retry
	r.recordDebugInteraction(ctx, requestID, LLMInteraction{
		Type:             "semantic_retry",
		Timestamp:        startTime,
		DurationMs:       duration.Milliseconds(),
		Prompt:           prompt,
		Temperature:      0.0,
		MaxTokens:        1000,
		Model:            response.Model,
		Provider:         response.Provider,
		Response:         response.Content,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		Success:          true,
		Attempt:          execCtx.RetryCount + 1,
	})

	// Parse structured response
	result, parseErr := r.parseReResolutionResponse(response.Content)
	if parseErr != nil {
		telemetry.AddSpanEvent(ctx, "contextual_re_resolution.parse_error",
			attribute.String("error", parseErr.Error()),
			attribute.String("response", truncateString(response.Content, 200)),
		)
		r.logWarn("Failed to parse re-resolution response", map[string]interface{}{
			"error":    parseErr.Error(),
			"response": truncateString(response.Content, 200),
		})
		return nil, fmt.Errorf("failed to parse re-resolution response: %w", parseErr)
	}

	// Telemetry: Track result
	telemetry.AddSpanEvent(ctx, "contextual_re_resolution.complete",
		attribute.Bool("should_retry", result.ShouldRetry),
		attribute.String("analysis", truncateString(result.Analysis, 200)),
		attribute.Int("corrected_params_count", len(result.CorrectedParameters)),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	if result.ShouldRetry {
		telemetry.Counter("orchestration.semantic_retry.success",
			"capability", execCtx.Capability.Name,
			"module", telemetry.ModuleOrchestration,
		)
	} else {
		telemetry.Counter("orchestration.semantic_retry.cannot_fix",
			"capability", execCtx.Capability.Name,
			"module", telemetry.ModuleOrchestration,
		)
	}

	r.logInfo("Contextual re-resolution completed", map[string]interface{}{
		"step_id":          execCtx.StepID,
		"capability":       execCtx.Capability.Name,
		"should_retry":     result.ShouldRetry,
		"analysis":         result.Analysis,
		"corrected_params": result.CorrectedParameters,
		"duration_ms":      duration.Milliseconds(),
	})

	return result, nil
}

// buildReResolutionPrompt creates the domain-agnostic prompt for re-resolution.
// The framework provides ALL available context and lets the LLM figure out
// what computation (if any) is needed.
func (r *ContextualReResolver) buildReResolutionPrompt(execCtx *ExecutionContext) string {
	sourceJSON, _ := json.MarshalIndent(execCtx.SourceData, "", "  ")
	failedJSON, _ := json.MarshalIndent(execCtx.AttemptedParams, "", "  ")

	// Build parameter schema description
	var paramDescs []string
	for _, p := range execCtx.Capability.Parameters {
		required := ""
		if p.Required {
			required = " (required)"
		}
		paramDescs = append(paramDescs, fmt.Sprintf("- %s (%s%s): %s",
			p.Name, p.Type, required, p.Description))
	}

	// Include previous errors if this is a retry of a retry
	previousContext := ""
	if len(execCtx.PreviousErrors) > 0 {
		previousContext = fmt.Sprintf("\nPREVIOUS FAILED ATTEMPTS:\n%s\n",
			strings.Join(execCtx.PreviousErrors, "\n---\n"))
	}

	return fmt.Sprintf(`TASK: Re-resolve parameters after execution failure

USER REQUEST:
"%s"

SOURCE DATA FROM PREVIOUS STEPS:
%s

FAILED ATTEMPT:
- Capability: %s
- Parameters sent: %s
- Error received: "%s"
- HTTP Status: %d
%s
TARGET CAPABILITY SCHEMA:
%s

INSTRUCTIONS:
1. Analyze the error message to understand what went wrong
2. Look at the USER REQUEST to understand the intent
3. Look at the SOURCE DATA to find values that can fix the error
4. If the fix requires deriving a value (calculation, combination, transformation),
   perform that computation and provide the result
5. Return ONLY valid JSON with:
   - should_retry: true/false - can this be fixed with corrected parameters?
   - analysis: brief explanation of what was wrong and how you fixed it
   - corrected_parameters: the complete corrected parameters object

RESPONSE FORMAT (JSON only):
{
  "should_retry": true,
  "analysis": "Brief explanation of the fix",
  "corrected_parameters": {
    "param1": "value1",
    "param2": 123
  }
}

If the error CANNOT be fixed with different parameters, respond with:
{
  "should_retry": false,
  "analysis": "Explanation of why this cannot be fixed",
  "corrected_parameters": {}
}`,
		execCtx.UserQuery,
		string(sourceJSON),
		execCtx.Capability.Name,
		string(failedJSON),
		execCtx.ErrorResponse,
		execCtx.HTTPStatus,
		previousContext,
		strings.Join(paramDescs, "\n"),
	)
}

// parseReResolutionResponse parses the LLM's JSON response.
// Uses the same pattern as ErrorAnalyzer.parseAnalysisResponse.
func (r *ContextualReResolver) parseReResolutionResponse(content string) (*ReResolutionResult, error) {
	// Clean up the response (handle markdown, extra text, etc.)
	content = strings.TrimSpace(content)

	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Find JSON object (same logic as error_analyzer.go:328-337)
	jsonStart := strings.Index(content, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonEnd := findJSONEndSimple(content, jsonStart) // Reuse from error_analyzer.go
	if jsonEnd == -1 {
		return nil, fmt.Errorf("invalid JSON structure in response")
	}

	jsonStr := content[jsonStart:jsonEnd]

	var result ReResolutionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Initialize empty map if nil
	if result.CorrectedParameters == nil {
		result.CorrectedParameters = make(map[string]interface{})
	}

	return &result, nil
}

// SetLogger sets the logger for the contextual re-resolver.
// The component is always set to "framework/orchestration" for proper attribution.
func (r *ContextualReResolver) SetLogger(logger core.Logger) {
	if logger == nil {
		r.logger = nil
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			r.logger = cal.WithComponent("framework/orchestration")
		} else {
			r.logger = logger
		}
	}
}

// Logging helpers
func (r *ContextualReResolver) logInfo(msg string, fields map[string]interface{}) {
	if r.logger != nil {
		r.logger.Info(msg, fields)
	}
}

func (r *ContextualReResolver) logWarn(msg string, fields map[string]interface{}) {
	if r.logger != nil {
		r.logger.Warn(msg, fields)
	}
}

// SetLLMDebugStore sets the LLM debug store for full payload visibility.
func (r *ContextualReResolver) SetLLMDebugStore(store LLMDebugStore) {
	r.debugStore = store
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Uses WaitGroup to track in-flight recordings for graceful shutdown.
func (r *ContextualReResolver) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
	if r.debugStore == nil {
		return
	}

	r.debugWg.Add(1)
	go func() {
		defer r.debugWg.Done()

		recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := r.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
			r.logWarn("Failed to record LLM debug interaction", map[string]interface{}{
				"request_id": requestID,
				"type":       interaction.Type,
				"error":      err.Error(),
			})
		}
	}()
}

// generateFallbackRequestID generates a request ID when TraceID is not available.
func (r *ContextualReResolver) generateFallbackRequestID() string {
	seq := r.debugSeqID.Add(1)
	return fmt.Sprintf("reresolver-%d-%d", time.Now().UnixNano(), seq)
}

// Shutdown waits for pending debug recordings to complete.
func (r *ContextualReResolver) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.debugWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
