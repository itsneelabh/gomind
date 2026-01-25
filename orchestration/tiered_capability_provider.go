package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// TieredCapabilityProvider implements a two-phase capability resolution strategy
// that significantly reduces LLM token usage for large tool deployments.
//
// Research basis:
// - RAG-MCP (May 2025): 74.8% token reduction, 62.1% faster, 3.2x accuracy
// - Less is More (Nov 2024): Accuracy degrades beyond ~20 tools
// - Guided-Structured Templates (Sept 2025): 3-12% improvement with structured prompts
//
// Phase 1 (Tier 1): Send lightweight summaries to LLM for tool selection
// Phase 2 (Tier 2): Retrieve full schemas only for selected tools
//
// This approach reduces token usage by 50-75% for deployments with 20+ tools.
type TieredCapabilityProvider struct {
	catalog  *AgentCatalog
	aiClient core.AIClient

	// MinToolsForTiering is the minimum tool count to trigger tiered resolution.
	// Below this threshold, sends all tools directly (simpler, one LLM call).
	// Research shows degradation starts at ~20 tools (Less is More, Nov 2024)
	// Default: 20
	MinToolsForTiering int

	// Logger for observability
	logger core.Logger

	// Telemetry for metrics
	telemetry core.Telemetry

	// LLM Debug Store integration (per LLM_DEBUG_PAYLOAD_DESIGN.md)
	debugStore LLMDebugStore  // For recording LLM interactions
	debugWg    sync.WaitGroup // Tracks in-flight debug recordings for graceful shutdown
	debugSeqID atomic.Uint64  // For generating unique fallback IDs when TraceID is empty

	// Circuit breaker for sophisticated resilience (optional)
	circuitBreaker core.CircuitBreaker
}

// Environment variable constant for tiered resolution
const (
	// EnvTieredMinTools overrides the minimum tool count to trigger tiering.
	// Example: GOMIND_TIERED_MIN_TOOLS=15
	EnvTieredMinTools = "GOMIND_TIERED_MIN_TOOLS"
)

// NewTieredCapabilityProvider creates a provider with intelligent tiered resolution.
// Configuration precedence: Explicit config → GOMIND_TIERED_MIN_TOOLS → 20
// Both tiers use the AI client's default model for simplicity.
func NewTieredCapabilityProvider(
	catalog *AgentCatalog,
	aiClient core.AIClient,
	config *TieredCapabilityConfig,
) *TieredCapabilityProvider {
	if config == nil {
		config = &TieredCapabilityConfig{}
	}

	// Resolve MinToolsForTiering with environment variable fallback
	// Precedence: Explicit config → GOMIND_TIERED_MIN_TOOLS → 20
	minTools := config.MinToolsForTiering
	if minTools == 0 {
		if envVal := os.Getenv(EnvTieredMinTools); envVal != "" {
			if parsed, err := strconv.Atoi(envVal); err == nil && parsed > 0 {
				minTools = parsed
			}
		}
	}
	if minTools == 0 {
		minTools = 20 // Research-backed default
	}

	return &TieredCapabilityProvider{
		catalog:            catalog,
		aiClient:           aiClient,
		MinToolsForTiering: minTools,
	}
}

// SetLogger sets the logger for observability
func (t *TieredCapabilityProvider) SetLogger(logger core.Logger) {
	if logger != nil {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			t.logger = cal.WithComponent("framework/orchestration/tiered")
		} else {
			t.logger = logger
		}
	}
}

// SetTelemetry sets the telemetry provider for metrics
func (t *TieredCapabilityProvider) SetTelemetry(telemetry core.Telemetry) {
	t.telemetry = telemetry
}

// SetLLMDebugStore sets the debug store for recording LLM interactions.
// Per LLM_DEBUG_PAYLOAD_DESIGN.md, this enables recording of tiered_selection calls.
func (t *TieredCapabilityProvider) SetLLMDebugStore(store LLMDebugStore) {
	t.debugStore = store
}

// GetLLMDebugStore returns the debug store (for testing/inspection)
func (t *TieredCapabilityProvider) GetLLMDebugStore() LLMDebugStore {
	return t.debugStore
}

// SetCircuitBreaker sets the circuit breaker for sophisticated resilience.
// When set, LLM calls are wrapped with circuit breaker protection.
func (t *TieredCapabilityProvider) SetCircuitBreaker(cb core.CircuitBreaker) {
	t.circuitBreaker = cb
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Uses WaitGroup to ensure graceful shutdown waits for pending recordings.
// Per LLM_DEBUG_PAYLOAD_DESIGN.md section 4.6 Lifecycle Management.
func (t *TieredCapabilityProvider) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
	if t.debugStore == nil {
		return
	}

	// Priority for requestID:
	// 1. Orchestrator's requestID from context value (for grouped debug records)
	// 2. Request ID from telemetry baggage (set by orchestrator in ProcessRequest)
	// 3. Trace ID from telemetry context
	// 4. Generated fallback ID
	requestID := GetRequestID(ctx)
	if requestID == "" {
		// Check telemetry baggage (orchestrator sets this in ProcessRequest)
		if baggage := telemetry.GetBaggage(ctx); baggage != nil {
			requestID = baggage["request_id"]
		}
	}
	if requestID == "" {
		// GetTraceContext is nil-safe, returns empty TraceID if no span
		tc := telemetry.GetTraceContext(ctx)
		requestID = tc.TraceID
	}
	if requestID == "" {
		// Generate unique fallback ID using atomic counter (collision-safe)
		seq := t.debugSeqID.Add(1)
		requestID = fmt.Sprintf("tiered-no-trace-%d-%d", time.Now().Unix(), seq)
	}

	t.debugWg.Add(1)
	go func() {
		defer t.debugWg.Done()

		// Use original context with timeout to preserve baggage (original_request_id).
		recordCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := t.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
			if t.logger != nil {
				t.logger.Warn("Failed to record tiered_selection debug interaction", map[string]interface{}{
					"operation":  "llm_debug_record",
					"request_id": requestID,
					"type":       interaction.Type,
					"error":      err.Error(),
				})
			}
		}
	}()
}

// Shutdown waits for pending debug recordings with a timeout.
// Should be called during graceful shutdown to ensure no data loss.
func (t *TieredCapabilityProvider) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		t.debugWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		if t.logger != nil {
			t.logger.Warn("TieredCapabilityProvider shutdown timeout: some debug recordings may be lost", map[string]interface{}{
				"operation": "tiered_provider_shutdown",
			})
		}
		return fmt.Errorf("tiered provider shutdown timeout: %w", ctx.Err())
	}
}

// GetCapabilities implements CapabilityProvider with tiered resolution.
// It automatically chooses between direct (all tools) and tiered based on tool count.
func (t *TieredCapabilityProvider) GetCapabilities(
	ctx context.Context,
	request string,
	metadata map[string]interface{},
) (string, error) {
	// Get all capability summaries
	summaries := t.catalog.GetCapabilitySummaries()

	// Check if tiering is beneficial
	if len(summaries) < t.MinToolsForTiering {
		// Below threshold - use direct approach (simpler, one LLM call)
		if t.logger != nil {
			t.logger.DebugWithContext(ctx, "Below tiering threshold, using direct approach", map[string]interface{}{
				"tool_count": len(summaries),
				"threshold":  t.MinToolsForTiering,
			})
		}
		return t.catalog.FormatForLLM(), nil
	}

	// Tier 1: Select relevant tools using lightweight summaries
	tier1Start := time.Now()
	selectedTools, err := t.selectRelevantTools(ctx, request, summaries)
	tier1Duration := time.Since(tier1Start)

	if err != nil {
		// Fallback to direct approach on selection failure
		if t.logger != nil {
			t.logger.WarnWithContext(ctx, "Tool selection failed, falling back to direct approach", map[string]interface{}{
				"operation":   "tiered_selection",
				"status":      "fallback",
				"error":       err.Error(),
				"duration_ms": tier1Duration.Milliseconds(),
			})
		}
		return t.catalog.FormatForLLM(), nil
	}

	if t.logger != nil {
		t.logger.InfoWithContext(ctx, "Tier 1 tool selection complete", map[string]interface{}{
			"operation":      "tiered_selection",
			"status":         "success",
			"total_tools":    len(summaries),
			"selected_tools": selectedTools,
			"reduction":      fmt.Sprintf("%.1f%%", (1-float64(len(selectedTools))/float64(len(summaries)))*100),
			"duration_ms":    tier1Duration.Milliseconds(),
		})
	}

	// Record metrics for observability (Phase 5)
	if t.telemetry != nil {
		t.telemetry.RecordMetric("orchestrator.tiered.tool_selection", 1, map[string]string{
			"total_tools":    strconv.Itoa(len(summaries)),
			"selected_tools": strconv.Itoa(len(selectedTools)),
		})

		// Record token savings estimate (~200 tokens per full schema)
		savedTokens := (len(summaries) - len(selectedTools)) * 200
		t.telemetry.RecordMetric("orchestrator.tiered.tokens_saved", float64(savedTokens), nil)
	}

	// Tier 2: Get full schemas for selected tools only
	return t.catalog.FormatToolsForLLM(selectedTools), nil
}

// selectRelevantTools uses an LLM call to identify which tools are needed.
// Uses structured prompting (Guided-Structured Templates, Sept 2025) and validates
// results to filter hallucinated tools (RAG-MCP, May 2025).
// Records LLM interaction to debug store per LLM_DEBUG_PAYLOAD_DESIGN.md.
// Uses AI client's default model - cost savings come from reduced token counts.
func (t *TieredCapabilityProvider) selectRelevantTools(
	ctx context.Context,
	request string,
	summaries []CapabilitySummary,
) ([]string, error) {
	// Check context before expensive LLM call (per Component Lifecycle Rules)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Build the selection prompt using structured template
	prompt := t.buildSelectionPrompt(summaries, request)

	// Use deterministic settings for tool selection
	options := &core.AIOptions{
		Temperature: 0.0, // Deterministic selection
		MaxTokens:   500, // Small output (just a list)
	}
	// Uses AI client's default model - no override needed

	// Capture timing for LLM debug recording
	llmStartTime := time.Now()

	// Get requestID for span events (same logic as recordDebugInteraction)
	requestID := GetRequestID(ctx)
	if requestID == "" {
		// Check telemetry baggage (orchestrator sets this in ProcessRequest)
		if baggage := telemetry.GetBaggage(ctx); baggage != nil {
			requestID = baggage["request_id"]
		}
	}
	if requestID == "" {
		tc := telemetry.GetTraceContext(ctx)
		requestID = tc.TraceID
	}

	// Telemetry: Record LLM request for visibility in distributed traces
	telemetry.AddSpanEvent(ctx, "llm.tiered_selection.request",
		attribute.String("request_id", requestID),
		attribute.String("user_request", truncateRequest(request, 200)),
		attribute.Int("prompt_length", len(prompt)),
		attribute.Int("tool_count", len(summaries)),
		attribute.Float64("temperature", 0.0),
		attribute.Int("max_tokens", 500),
		attribute.Int("attempt", 1),
	)

	// Make the LLM call, optionally wrapped with circuit breaker
	var response *core.AIResponse
	var err error

	if t.circuitBreaker != nil {
		err = t.circuitBreaker.Execute(ctx, func() error {
			var cbErr error
			response, cbErr = t.aiClient.GenerateResponse(ctx, prompt, options)
			return cbErr
		})
	} else {
		response, err = t.aiClient.GenerateResponse(ctx, prompt, options)
	}
	llmDuration := time.Since(llmStartTime)

	// LLM Debug: Record interaction (success or failure)
	// Per LLM_DEBUG_PAYLOAD_DESIGN.md - this is the 7th recording site: "tiered_selection"
	if err != nil {
		// Telemetry: Record error for visibility in distributed traces
		telemetry.AddSpanEvent(ctx, "llm.tiered_selection.error",
			attribute.String("request_id", requestID),
			attribute.String("error", err.Error()),
			attribute.Int64("duration_ms", llmDuration.Milliseconds()),
			attribute.Int("attempt", 1),
		)

		// Record failed selection attempt
		t.recordDebugInteraction(ctx, LLMInteraction{
			Type:        "tiered_selection",
			Timestamp:   llmStartTime,
			DurationMs:  llmDuration.Milliseconds(),
			Prompt:      prompt,
			Temperature: 0.0,
			MaxTokens:   500,
			Success:     false,
			Error:       err.Error(),
			Attempt:     1,
		})
		return nil, fmt.Errorf("tiered tool selection failed for request %q: %w",
			truncateRequest(request, 50), err)
	}

	// Telemetry: Record LLM response for visibility in distributed traces
	telemetry.AddSpanEvent(ctx, "llm.tiered_selection.response",
		attribute.String("request_id", requestID),
		attribute.String("response", truncateRequest(response.Content, 500)),
		attribute.Int("response_length", len(response.Content)),
		attribute.Int("prompt_tokens", response.Usage.PromptTokens),
		attribute.Int("completion_tokens", response.Usage.CompletionTokens),
		attribute.Int("total_tokens", response.Usage.TotalTokens),
		attribute.Int64("duration_ms", llmDuration.Milliseconds()),
		attribute.Int("attempt", 1),
	)

	// Record successful selection
	t.recordDebugInteraction(ctx, LLMInteraction{
		Type:             "tiered_selection",
		Timestamp:        llmStartTime,
		DurationMs:       llmDuration.Milliseconds(),
		Prompt:           prompt,
		Temperature:      0.0,
		MaxTokens:        500,
		Model:            response.Model,
		Provider:         response.Provider,
		Response:         response.Content,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		Success:          true,
		Attempt:          1,
	})

	// Parse the response
	selectedTools, err := t.parseToolSelection(response.Content)
	if err != nil {
		return nil, err
	}

	// Validate and filter to prevent hallucinated tool names
	// RAG-MCP research: "model often picks the wrong one or makes up fake tools"
	validatedTools := t.validateAndFilterTools(ctx, selectedTools, summaries)

	if len(validatedTools) == 0 {
		return nil, fmt.Errorf("no valid tools after filtering (all selections were hallucinated)")
	}

	return validatedTools, nil
}

// buildSelectionPrompt creates the Tier 1 prompt with tool summaries.
// Uses structured template approach based on "Guided-Structured Templates" research (Sept 2025)
// which shows 3-12% accuracy improvement over free-form prompts.
func (t *TieredCapabilityProvider) buildSelectionPrompt(
	summaries []CapabilitySummary,
	request string,
) string {
	var sb strings.Builder

	// Structured template following research recommendations:
	// identification → relevancy decision → dependency analysis → selection
	sb.WriteString(`You are a tool selector. Follow this structured process to select tools.

## STEP 1: TASK IDENTIFICATION
Analyze what the user wants to accomplish. Break down the request into discrete sub-tasks.

## STEP 2: AVAILABLE TOOLS
`)

	// Format each tool summary (compact format)
	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf("- %s/%s: %s\n", s.AgentName, s.CapabilityName, s.Summary))
	}

	sb.WriteString(fmt.Sprintf(`
## STEP 3: USER REQUEST
%s

## STEP 4: STRUCTURED SELECTION PROCESS

Think through these questions (but only output the final JSON):

A. PRIMARY TOOLS: Which tools directly address the user's explicit requests?
   - What information is the user explicitly asking for?
   - Which tools provide that information?

B. DATA DEPENDENCY TOOLS: What intermediate data is needed?
   - Do any selected tools require input from other tools?
   - Example: Weather tools often need coordinates → need geocoding tool
   - Example: Currency conversion needs currency codes → need country-info tool

C. COMPLETENESS CHECK: Review each part of the request
   - Is every aspect of the user's request covered?
   - Are all data dependencies satisfied?

## OUTPUT FORMAT
Return ONLY a JSON array of tool identifiers. Format: "agent_name/capability_name"
Example: ["stock-service/stock_quote", "country-info-tool/get_country_info", "currency-tool/convert_currency"]

JSON array (no explanation):
`, request))

	return sb.String()
}

// parseToolSelection extracts tool names from the LLM response.
func (t *TieredCapabilityProvider) parseToolSelection(response string) ([]string, error) {
	// Clean up response (handle markdown wrapping)
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON array
	var tools []string
	if err := json.Unmarshal([]byte(response), &tools); err != nil {
		return nil, fmt.Errorf("failed to parse tool selection: %w (response: %s)", err, response)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools selected")
	}

	return tools, nil
}

// validateAndFilterTools verifies selected tools exist in the catalog.
// Returns only valid tools and logs warnings for invalid selections.
// This prevents hallucinated tool names (a known issue per RAG-MCP research).
func (t *TieredCapabilityProvider) validateAndFilterTools(
	ctx context.Context,
	selectedTools []string,
	summaries []CapabilitySummary,
) []string {
	// Build lookup set of valid tool IDs
	validTools := make(map[string]bool)
	for _, s := range summaries {
		toolID := fmt.Sprintf("%s/%s", s.AgentName, s.CapabilityName)
		validTools[toolID] = true
	}

	// Filter to only valid tools
	var filtered []string
	var invalid []string
	for _, tool := range selectedTools {
		if validTools[tool] {
			filtered = append(filtered, tool)
		} else {
			invalid = append(invalid, tool)
		}
	}

	// Log any hallucinated tools (research shows this is common with many tools)
	if len(invalid) > 0 && t.logger != nil {
		t.logger.WarnWithContext(ctx, "LLM selected non-existent tools (hallucination)", map[string]interface{}{
			"invalid_tools": invalid,
			"valid_count":   len(filtered),
		})
	}

	return filtered
}

// truncateRequest truncates a request string for error messages.
// Helps keep error messages readable while providing context.
func truncateRequest(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
