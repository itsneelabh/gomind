// Package orchestration provides intelligent parameter binding for multi-step workflows.
// This file implements micro-resolution: focused LLM calls to extract parameters
// from source data when auto-wiring cannot find a match.
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

// FunctionCallingClient extends the basic AIClient with function calling support.
// Providers that support OpenAI-style function calling should implement this interface.
type FunctionCallingClient interface {
	core.AIClient

	// ChatWithFunctions sends a message with function definitions and returns
	// either a text response or a function call result
	ChatWithFunctions(ctx context.Context, messages []ChatMessage, functions []FunctionDef) (*FunctionCallResponse, error)
}

// ChatMessage represents a message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// FunctionDef defines a function for LLM function calling
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolCallResult is the result of a function call from the LLM
type ToolCallResult struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// FunctionCallResponse is the response from ChatWithFunctions
type FunctionCallResponse struct {
	Content  string          // Text response (if no function call)
	ToolCall *ToolCallResult // Function call result (if any)
}

// MicroResolver resolves parameters using focused LLM calls
type MicroResolver struct {
	// aiClient is the basic AI client for text-based resolution
	aiClient core.AIClient
	// functionClient is the optional function-calling client for typed resolution
	functionClient FunctionCallingClient
	// logger for debugging
	logger core.Logger

	// LLM Debug Store for full payload visibility
	debugStore LLMDebugStore
	debugWg    sync.WaitGroup
	debugSeqID atomic.Uint64
}

// NewMicroResolver creates a new micro-resolver
func NewMicroResolver(aiClient core.AIClient, logger core.Logger) *MicroResolver {
	mr := &MicroResolver{
		aiClient: aiClient,
		logger:   logger,
	}

	// Check if the client supports function calling
	if fc, ok := aiClient.(FunctionCallingClient); ok {
		mr.functionClient = fc
	}

	return mr
}

// ResolveParameters extracts parameters from source data for a target capability.
// If function calling is available, uses it for guaranteed type safety.
// Otherwise, falls back to text-based extraction with JSON parsing.
//
// The stepID parameter associates any LLM calls with the execution step for
// DAG visualization. Pass empty string if not step-specific.
func (m *MicroResolver) ResolveParameters(
	ctx context.Context,
	sourceData map[string]interface{},
	targetCapability *EnhancedCapability,
	hint string,
	stepID string,
) (map[string]interface{}, error) {
	if m.functionClient != nil {
		return m.resolveWithFunctions(ctx, sourceData, targetCapability, hint, stepID)
	}
	return m.resolveWithText(ctx, sourceData, targetCapability, hint, stepID)
}

// resolveWithFunctions uses LLM function calling for typed parameter extraction
func (m *MicroResolver) resolveWithFunctions(
	ctx context.Context,
	sourceData map[string]interface{},
	targetCapability *EnhancedCapability,
	hint string,
	stepID string,
) (map[string]interface{}, error) {
	// Build the JSON schema for the target parameters
	schema := m.buildParameterSchema(targetCapability)

	// Build the prompt
	sourceJSON, _ := json.MarshalIndent(sourceData, "", "  ")

	prompt := fmt.Sprintf(`Extract the parameters needed for the "%s" function.

Available data from previous step:
%s

%s

Return the extracted parameter values using the provided function.`,
		targetCapability.Name,
		string(sourceJSON),
		hint,
	)

	// Define the function for extraction
	function := FunctionDef{
		Name:        "provide_parameters",
		Description: fmt.Sprintf("Provide parameters for %s", targetCapability.Name),
		Parameters:  schema,
	}

	m.logDebug("Micro-resolution using function calling", map[string]interface{}{
		"capability":  targetCapability.Name,
		"source_keys": getMapKeys(sourceData),
	})

	// Make the LLM call
	resp, err := m.functionClient.ChatWithFunctions(ctx,
		[]ChatMessage{{Role: "user", Content: prompt}},
		[]FunctionDef{function},
	)
	if err != nil {
		return nil, fmt.Errorf("micro-resolution failed: %w", err)
	}

	if resp.ToolCall == nil {
		return nil, fmt.Errorf("LLM did not return a function call")
	}

	m.logInfo("Micro-resolution completed via function calling", map[string]interface{}{
		"capability":      targetCapability.Name,
		"resolved_params": resp.ToolCall.Arguments,
	})

	return resp.ToolCall.Arguments, nil
}

// resolveWithText uses text-based LLM extraction as fallback
func (m *MicroResolver) resolveWithText(
	ctx context.Context,
	sourceData map[string]interface{},
	targetCapability *EnhancedCapability,
	hint string,
	stepID string,
) (map[string]interface{}, error) {
	sourceJSON, _ := json.MarshalIndent(sourceData, "", "  ")

	// Build parameter descriptions
	var paramDescs []string
	for _, p := range targetCapability.Parameters {
		paramDescs = append(paramDescs, fmt.Sprintf("- %s (%s): %s", p.Name, p.Type, p.Description))
	}

	prompt := fmt.Sprintf(`You are a parameter extraction assistant. Extract values from the source data to fill the required parameters.

SOURCE DATA:
%s

REQUIRED PARAMETERS for "%s":
%s

%s

INSTRUCTIONS:
1. Find values in the source data that match each required parameter
2. Convert types as needed (e.g., string "48.85" to number 48.85)
3. Return ONLY a valid JSON object with the extracted parameters
4. Do not include any explanation, only the JSON

RESPONSE FORMAT (JSON only):
{
  "paramName1": value1,
  "paramName2": value2
}`,
		string(sourceJSON),
		targetCapability.Name,
		strings.Join(paramDescs, "\n"),
		hint,
	)

	m.logDebug("Micro-resolution using text extraction", map[string]interface{}{
		"capability":  targetCapability.Name,
		"source_keys": getMapKeys(sourceData),
	})

	// Telemetry: Record LLM prompt for micro-resolution
	telemetry.AddSpanEvent(ctx, "llm.micro_resolution.request",
		attribute.String("capability", targetCapability.Name),
		attribute.String("prompt", truncateString(prompt, 1500)),
		attribute.Int("prompt_length", len(prompt)),
		attribute.String("hint", hint),
	)

	// Get request ID from context baggage for debug correlation
	requestID := ""
	if baggage := telemetry.GetBaggage(ctx); baggage != nil {
		requestID = baggage["request_id"]
	}
	if requestID == "" {
		requestID = m.generateFallbackRequestID()
	}

	// Make the LLM call
	llmStartTime := time.Now()
	resp, err := m.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.0, // Deterministic for extraction
		MaxTokens:   500,
	})
	llmDuration := time.Since(llmStartTime)

	if err != nil {
		telemetry.AddSpanEvent(ctx, "llm.micro_resolution.error",
			attribute.String("capability", targetCapability.Name),
			attribute.String("error", err.Error()),
			attribute.Int64("duration_ms", llmDuration.Milliseconds()),
		)

		// LLM Debug: Record failed micro-resolution attempt
		m.recordDebugInteraction(ctx, requestID, LLMInteraction{
			Type:        "micro_resolution",
			Timestamp:   llmStartTime,
			DurationMs:  llmDuration.Milliseconds(),
			Prompt:      prompt,
			Temperature: 0.0,
			MaxTokens:   500,
			Success:     false,
			Error:       err.Error(),
			Attempt:     1,
			StepID:      stepID,
		})

		return nil, fmt.Errorf("micro-resolution text call failed: %w", err)
	}

	// Telemetry: Record LLM response for micro-resolution
	telemetry.AddSpanEvent(ctx, "llm.micro_resolution.response",
		attribute.String("capability", targetCapability.Name),
		attribute.String("response", truncateString(resp.Content, 1000)),
		attribute.Int("response_length", len(resp.Content)),
		attribute.Int64("duration_ms", llmDuration.Milliseconds()),
	)

	// LLM Debug: Record successful micro-resolution
	m.recordDebugInteraction(ctx, requestID, LLMInteraction{
		Type:             "micro_resolution",
		Timestamp:        llmStartTime,
		DurationMs:       llmDuration.Milliseconds(),
		Prompt:           prompt,
		Temperature:      0.0,
		MaxTokens:        500,
		Model:            resp.Model,
		Provider:         resp.Provider,
		Response:         resp.Content,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		Success:          true,
		Attempt:          1,
		StepID:           stepID,
	})

	// Parse the JSON response
	content := strings.TrimSpace(resp.Content)
	// Remove markdown code blocks if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		m.logWarn("Failed to parse micro-resolution response", map[string]interface{}{
			"error":    err.Error(),
			"response": content,
		})
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	// Apply type coercion to match target types
	for _, param := range targetCapability.Parameters {
		if val, ok := result[param.Name]; ok {
			result[param.Name] = coerceType(val, param.Type)
		}
	}

	m.logInfo("Micro-resolution completed via text extraction", map[string]interface{}{
		"capability":      targetCapability.Name,
		"resolved_params": result,
	})

	return result, nil
}

// buildParameterSchema creates a JSON Schema from capability parameters
func (m *MicroResolver) buildParameterSchema(cap *EnhancedCapability) json.RawMessage {
	properties := make(map[string]interface{})
	required := []string{}

	for _, param := range cap.Parameters {
		prop := map[string]interface{}{
			"type":        mapToJSONSchemaType(param.Type),
			"description": param.Description,
		}
		properties[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false, // Required for strict mode
	}

	schemaJSON, _ := json.Marshal(schema)
	return schemaJSON
}

// mapToJSONSchemaType converts Go type names to JSON Schema types
func mapToJSONSchemaType(goType string) string {
	switch strings.ToLower(goType) {
	case "number", "float", "float64", "double":
		return "number"
	case "integer", "int", "int64", "int32":
		return "integer"
	case "boolean", "bool":
		return "boolean"
	case "array", "[]string", "[]int":
		return "array"
	case "object", "map":
		return "object"
	default:
		return "string"
	}
}

// SetLogger sets the logger for the micro-resolver
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (m *MicroResolver) SetLogger(logger core.Logger) {
	if logger == nil {
		m.logger = nil
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			m.logger = cal.WithComponent("framework/orchestration")
		} else {
			m.logger = logger
		}
	}
}

// Logging helpers
func (m *MicroResolver) logDebug(msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.Debug(msg, fields)
	}
}

func (m *MicroResolver) logInfo(msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.Info(msg, fields)
	}
}

func (m *MicroResolver) logWarn(msg string, fields map[string]interface{}) {
	if m.logger != nil {
		m.logger.Warn(msg, fields)
	}
}

// SetLLMDebugStore sets the LLM debug store for full payload visibility.
func (m *MicroResolver) SetLLMDebugStore(store LLMDebugStore) {
	m.debugStore = store
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Uses WaitGroup to track in-flight recordings for graceful shutdown.
func (m *MicroResolver) recordDebugInteraction(ctx context.Context, requestID string, interaction LLMInteraction) {
	if m.debugStore == nil {
		return
	}

	// Extract baggage BEFORE spawning goroutine to preserve correlation data.
	bag := telemetry.GetBaggage(ctx)

	m.debugWg.Add(1)
	go func() {
		defer m.debugWg.Done()

		// Use background context with timeout to avoid inheriting request cancellation.
		recordCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Re-inject baggage for HITL correlation (original_request_id).
		if bag != nil {
			pairs := make([]string, 0, len(bag)*2)
			for k, v := range bag {
				pairs = append(pairs, k, v)
			}
			recordCtx = telemetry.WithBaggage(recordCtx, pairs...)
		}

		if err := m.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
			m.logWarn("Failed to record LLM debug interaction", map[string]interface{}{
				"request_id": requestID,
				"type":       interaction.Type,
				"error":      err.Error(),
			})
		}
	}()
}

// generateFallbackRequestID generates a request ID when TraceID is not available.
func (m *MicroResolver) generateFallbackRequestID() string {
	seq := m.debugSeqID.Add(1)
	return fmt.Sprintf("micro-%d-%d", time.Now().UnixNano(), seq)
}

// Shutdown waits for pending debug recordings to complete.
func (m *MicroResolver) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.debugWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
