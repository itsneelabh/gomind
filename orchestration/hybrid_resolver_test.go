package orchestration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// mockAIClient implements core.AIClient for testing HybridResolver
type mockAIClient struct {
	generateFunc func(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error)
	calls        []string
}

func (m *mockAIClient) GenerateResponse(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
	m.calls = append(m.calls, prompt)
	if m.generateFunc != nil {
		return m.generateFunc(ctx, prompt, opts)
	}
	return &core.AIResponse{Content: "{}"}, nil
}

// Helper to create a mock AI client that returns specific JSON
func newMockAIClientWithResponse(response map[string]interface{}) *mockAIClient {
	return &mockAIClient{
		generateFunc: func(ctx context.Context, prompt string, opts *core.AIOptions) (*core.AIResponse, error) {
			jsonBytes, _ := json.Marshal(response)
			return &core.AIResponse{Content: string(jsonBytes)}, nil
		},
	}
}

// TestHybridResolver_AllParamsAutoWired tests the case where all parameters
// are successfully matched by auto-wiring, so no LLM call is needed.
func TestHybridResolver_AllParamsAutoWired(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	// Source data has exact name matches
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"lat": 48.85, "lon": 2.35}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have resolved both params
	if len(params) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(params), params)
	}

	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85, got %v", params["lat"])
	}

	if params["lon"].(float64) != 2.35 {
		t.Errorf("Expected lon=2.35, got %v", params["lon"])
	}

	// No LLM calls should have been made
	if len(aiClient.calls) != 0 {
		t.Errorf("Expected no AI calls (all auto-wired), got %d calls", len(aiClient.calls))
	}
}

// TestHybridResolver_OptionalParamsUnmapped tests that optional unmapped params
// don't trigger micro-resolution - only required params do.
func TestHybridResolver_OptionalParamsUnmapped(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"lat": 48.85, "lon": 2.35}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
			{Name: "unit", Type: "string", Required: false}, // Optional, not in source
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have resolved required params only
	if len(params) != 2 {
		t.Errorf("Expected 2 params (required only), got %d: %v", len(params), params)
	}

	// No LLM calls - optional params don't trigger micro-resolution
	if len(aiClient.calls) != 0 {
		t.Errorf("Expected no AI calls (optional params don't trigger LLM), got %d calls", len(aiClient.calls))
	}
}

// TestHybridResolver_RequiredParamsMissingTriggersMicroResolution tests that
// unmapped required params trigger the LLM micro-resolution fallback.
func TestHybridResolver_RequiredParamsMissingTriggersMicroResolution(t *testing.T) {
	// Mock AI client returns the missing params
	aiClient := newMockAIClientWithResponse(map[string]interface{}{
		"lat": 48.85,
		"lon": 2.35,
	})
	resolver := NewHybridResolver(aiClient, nil)

	// Source has different names - auto-wiring won't match
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"latitude": 48.85, "longitude": 2.35}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have resolved params via micro-resolution
	if len(params) != 2 {
		t.Errorf("Expected 2 params from micro-resolution, got %d: %v", len(params), params)
	}

	// LLM should have been called (micro-resolution)
	if len(aiClient.calls) == 0 {
		t.Error("Expected AI call for micro-resolution, got none")
	}
}

// TestHybridResolver_MicroResolutionDisabled tests that when micro-resolution
// is disabled, only auto-wired params are returned.
func TestHybridResolver_MicroResolutionDisabled(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil, WithMicroResolution(false))

	// Source has different names - auto-wiring won't match
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"latitude": 48.85, "longitude": 2.35}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return empty - nothing auto-wired, micro-resolution disabled
	if len(params) != 0 {
		t.Errorf("Expected 0 params (micro-resolution disabled), got %d: %v", len(params), params)
	}

	// No LLM calls since disabled
	if len(aiClient.calls) != 0 {
		t.Errorf("Expected no AI calls (micro-resolution disabled), got %d calls", len(aiClient.calls))
	}
}

// TestHybridResolver_EmptyDependencyResults tests handling of no dependency data.
func TestHybridResolver_EmptyDependencyResults(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return nil - no source data available
	if params != nil {
		t.Errorf("Expected nil params for empty dependencies, got %v", params)
	}
}

// TestHybridResolver_FailedStepsSkipped tests that failed dependency steps
// are not included in source data.
func TestHybridResolver_FailedStepsSkipped(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  false, // Failed step
			Response: `{"lat": 48.85, "lon": 2.35}`,
		},
		"step-2": {
			StepID:   "step-2",
			Success:  true,
			Response: `{"city": "Paris"}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "city", Type: "string", Required: false},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should only have city from step-2 (step-1 failed, so lat not available)
	if params["city"] != "Paris" {
		t.Errorf("Expected city=Paris from step-2, got %v", params["city"])
	}

	// lat should NOT be present (from failed step)
	if _, hasLat := params["lat"]; hasLat {
		t.Error("Expected lat to be missing (from failed step), but it was present")
	}
}

// TestHybridResolver_MultipleDependencies tests merging data from multiple steps.
func TestHybridResolver_MultipleDependencies(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{
		"geocoding": {
			StepID:   "geocoding",
			Success:  true,
			Response: `{"lat": 48.85, "lon": 2.35}`,
		},
		"country-info": {
			StepID:   "country-info",
			Success:  true,
			Response: `{"currency": "EUR", "population": 67000000}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "convert_currency",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "currency", Type: "string", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have merged data from both steps
	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85 from geocoding, got %v", params["lat"])
	}

	if params["currency"].(string) != "EUR" {
		t.Errorf("Expected currency=EUR from country-info, got %v", params["currency"])
	}
}

// TestHybridResolver_TypeCoercion tests that type coercion works during auto-wiring.
func TestHybridResolver_TypeCoercion(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	// Source has string values
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"lat": "48.85", "lon": "2.35"}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have coerced strings to numbers
	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85 (coerced), got %v", params["lat"])
	}

	if params["lon"].(float64) != 2.35 {
		t.Errorf("Expected lon=2.35 (coerced), got %v", params["lon"])
	}

	// No LLM calls needed - auto-wiring with coercion
	if len(aiClient.calls) != 0 {
		t.Errorf("Expected no AI calls (auto-wiring with coercion), got %d calls", len(aiClient.calls))
	}
}

// TestHybridResolver_NestedObjectExtraction tests extraction from nested objects.
func TestHybridResolver_NestedObjectExtraction(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	// Source has nested currency object
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"currency": {"code": "EUR", "name": "Euro"}}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "convert",
		Parameters: []Parameter{
			{Name: "currency", Type: "string", Required: true}, // Expects string, not object
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should extract "code" from nested object
	if params["currency"].(string) != "EUR" {
		t.Errorf("Expected currency=EUR (extracted from nested), got %v", params["currency"])
	}
}

// TestHybridResolver_CaseInsensitiveMatch tests case-insensitive name matching.
func TestHybridResolver_CaseInsensitiveMatch(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"LAT": 48.85, "LON": 2.35}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "lon", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85 (case-insensitive), got %v", params["lat"])
	}

	// No LLM calls - case-insensitive is handled by auto-wiring
	if len(aiClient.calls) != 0 {
		t.Errorf("Expected no AI calls (case-insensitive auto-wire), got %d calls", len(aiClient.calls))
	}
}

// TestHybridResolver_AutoWiredPriorityOverMicroResolution tests that auto-wired
// values are not overwritten by micro-resolution results.
func TestHybridResolver_AutoWiredPriorityOverMicroResolution(t *testing.T) {
	// Mock returns different values than source
	aiClient := newMockAIClientWithResponse(map[string]interface{}{
		"lat":  99.99, // Different from source
		"city": "Tokyo",
	})
	resolver := NewHybridResolver(aiClient, nil)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"lat": 48.85}`, // Has lat but not city
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
			{Name: "city", Type: "string", Required: true}, // Not in source
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// lat should be from auto-wiring (48.85), not micro-resolution (99.99)
	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85 (auto-wired priority), got %v", params["lat"])
	}

	// city should be from micro-resolution
	if params["city"].(string) != "Tokyo" {
		t.Errorf("Expected city=Tokyo (from micro-resolution), got %v", params["city"])
	}
}

// TestHybridResolver_NilAIClient tests behavior when no AI client is provided.
func TestHybridResolver_NilAIClient(t *testing.T) {
	// Create resolver without AI client
	resolver := NewHybridResolver(nil, nil)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"latitude": 48.85}`, // Won't auto-wire to "lat"
		},
	}

	targetCap := &EnhancedCapability{
		Name: "get_weather",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return empty - can't auto-wire, no AI client for micro-resolution
	if len(params) != 0 {
		t.Errorf("Expected 0 params (no AI client), got %d: %v", len(params), params)
	}
}

// TestHybridResolver_SetLogger tests logger propagation to sub-components.
func TestHybridResolver_SetLogger(t *testing.T) {
	aiClient := &mockAIClient{}
	resolver := NewHybridResolver(aiClient, nil)

	logger := &mockLogger{}
	resolver.SetLogger(logger)

	// Trigger some logging via resolution
	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `{"lat": 48.85}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "test",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	_, _ = resolver.ResolveParameters(ctx, depResults, targetCap)

	// Should have logged something
	if len(logger.messages) == 0 {
		t.Error("Expected logger to receive messages, got none")
	}
}

// TestHybridResolver_InvalidJSONResponse tests handling of invalid JSON in step response.
func TestHybridResolver_InvalidJSONResponse(t *testing.T) {
	aiClient := &mockAIClient{}
	logger := &mockLogger{}
	resolver := NewHybridResolver(aiClient, logger)

	depResults := map[string]*StepResult{
		"step-1": {
			StepID:   "step-1",
			Success:  true,
			Response: `not valid json`,
		},
		"step-2": {
			StepID:   "step-2",
			Success:  true,
			Response: `{"lat": 48.85}`,
		},
	}

	targetCap := &EnhancedCapability{
		Name: "test",
		Parameters: []Parameter{
			{Name: "lat", Type: "number", Required: true},
		},
	}

	ctx := context.Background()
	params, err := resolver.ResolveParameters(ctx, depResults, targetCap)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still get lat from step-2
	if params["lat"].(float64) != 48.85 {
		t.Errorf("Expected lat=48.85 from step-2, got %v", params["lat"])
	}

	// Should have logged a warning about invalid JSON
	hasWarning := false
	for _, msg := range logger.messages {
		if msg == "Failed to parse step response for parameter resolution" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("Expected warning about invalid JSON, got none")
	}
}
