package orchestration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// TieredTestAIClient is a mock AI client for tiered provider tests
type TieredTestAIClient struct {
	response     string
	err          error
	calls        []string
	model        string
	provider     string
	promptTokens int
	compTokens   int
}

func NewTieredTestAIClient() *TieredTestAIClient {
	return &TieredTestAIClient{
		model:        "test-model",
		provider:     "test-provider",
		promptTokens: 100,
		compTokens:   50,
	}
}

func (m *TieredTestAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	m.calls = append(m.calls, prompt)
	if m.err != nil {
		return nil, m.err
	}
	return &core.AIResponse{
		Content:  m.response,
		Model:    m.model,
		Provider: m.provider,
		Usage: core.TokenUsage{
			PromptTokens:     m.promptTokens,
			CompletionTokens: m.compTokens,
			TotalTokens:      m.promptTokens + m.compTokens,
		},
	}, nil
}

func (m *TieredTestAIClient) SetResponse(response string) {
	m.response = response
}

func (m *TieredTestAIClient) SetError(err error) {
	m.err = err
}

func (m *TieredTestAIClient) GetCalls() []string {
	return m.calls
}

// setupTestCatalog creates a catalog with the specified number of tools
func setupTestCatalog(toolCount int) *AgentCatalog {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	// Create capabilities
	caps := make([]core.Capability, toolCount)
	for i := 0; i < toolCount; i++ {
		caps[i] = core.Capability{
			Name:        fmt.Sprintf("capability_%d", i),
			Description: fmt.Sprintf("Test capability %d. This is a description.", i),
		}
	}

	// Register a test agent with all capabilities
	registration := &core.ServiceRegistration{
		ID:           "test-agent",
		Name:         "test-agent",
		Type:         core.ComponentTypeAgent,
		Description:  "Test agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: caps,
		Health:       core.HealthHealthy,
	}
	discovery.Register(context.Background(), registration)
	catalog.Refresh(context.Background())

	return catalog
}

// setupMultiAgentCatalog creates a catalog with multiple agents
func setupMultiAgentCatalog(agentCount, capsPerAgent int) *AgentCatalog {
	discovery := NewMockDiscovery()
	catalog := NewAgentCatalog(discovery)

	for a := 0; a < agentCount; a++ {
		caps := make([]core.Capability, capsPerAgent)
		for i := 0; i < capsPerAgent; i++ {
			caps[i] = core.Capability{
				Name:        fmt.Sprintf("capability_%d", i),
				Description: fmt.Sprintf("Capability %d of agent %d. This is useful.", i, a),
			}
		}

		registration := &core.ServiceRegistration{
			ID:           fmt.Sprintf("agent-%d", a),
			Name:         fmt.Sprintf("agent-%d", a),
			Type:         core.ComponentTypeAgent,
			Description:  fmt.Sprintf("Agent %d", a),
			Address:      "localhost",
			Port:         8080 + a,
			Capabilities: caps,
			Health:       core.HealthHealthy,
		}
		discovery.Register(context.Background(), registration)
	}
	catalog.Refresh(context.Background())

	return catalog
}

func TestTieredCapabilityProvider_BelowThreshold(t *testing.T) {
	// Setup: Create catalog with 15 tools (below default threshold of 20)
	catalog := setupTestCatalog(15)
	aiClient := NewTieredTestAIClient()

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify no LLM call was made (below threshold)
	if len(aiClient.GetCalls()) > 0 {
		t.Error("Expected no LLM calls below threshold, but calls were made")
	}

	// Verify full catalog returned
	if !strings.Contains(capabilities, "test-agent") {
		t.Error("Expected capabilities to contain test-agent")
	}
}

func TestTieredCapabilityProvider_AboveThreshold(t *testing.T) {
	// Setup: Create catalog with 30 tools (above default threshold of 20)
	catalog := setupTestCatalog(30)
	aiClient := NewTieredTestAIClient()

	// Mock AI client returns a selection of tools
	aiClient.SetResponse(`["test-agent/capability_0", "test-agent/capability_5"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify LLM call was made
	if len(aiClient.GetCalls()) == 0 {
		t.Error("Expected LLM call above threshold, but none was made")
	}

	// Verify only selected tools are in output
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected capabilities to contain capability_0")
	}
	if !strings.Contains(capabilities, "capability_5") {
		t.Error("Expected capabilities to contain capability_5")
	}
	// Verify non-selected tools are NOT in output
	if strings.Contains(capabilities, "capability_10") {
		t.Error("Expected capabilities to NOT contain capability_10 (not selected)")
	}
}

func TestTieredCapabilityProvider_HallucinationFiltering(t *testing.T) {
	// Setup: Create catalog with 25 tools
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Mock AI client returns both real and fake tools
	aiClient.SetResponse(`["test-agent/capability_0", "fake-tool/fake_cap", "test-agent/capability_1"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify only valid tools returned
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected capabilities to contain capability_0")
	}
	if !strings.Contains(capabilities, "capability_1") {
		t.Error("Expected capabilities to contain capability_1")
	}
	// Verify fake tool is not included
	if strings.Contains(capabilities, "fake_cap") {
		t.Error("Expected fake tool to be filtered out")
	}
}

func TestTieredCapabilityProvider_FallbackOnError(t *testing.T) {
	// Setup: Create catalog with 25 tools
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Mock AI client returns an error
	aiClient.SetError(errors.New("LLM service unavailable"))

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities - should fall back to FormatForLLM
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful degradation, got error: %v", err)
	}

	// Verify full catalog returned as fallback
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
	if !strings.Contains(capabilities, "capability_10") {
		t.Error("Expected fallback to include all tools")
	}
}

func TestTieredCapabilityProvider_EmptySelection(t *testing.T) {
	// Setup: Create catalog with 25 tools
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Mock AI client returns empty array
	aiClient.SetResponse(`[]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities - should fall back since no valid tools
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful degradation on empty selection, got error: %v", err)
	}

	// Verify full catalog returned as fallback
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
}

func TestTieredCapabilityProvider_AllHallucinations(t *testing.T) {
	// Setup: Create catalog with 25 tools
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Mock AI client returns only fake tools
	aiClient.SetResponse(`["fake-tool/fake1", "fake-tool/fake2"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities - should fall back since all selections are hallucinated
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful degradation when all selections hallucinated, got error: %v", err)
	}

	// Verify full catalog returned as fallback
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
}

func TestTieredCapabilityProvider_CustomThreshold(t *testing.T) {
	// Setup: Create catalog with 15 tools
	catalog := setupTestCatalog(15)
	aiClient := NewTieredTestAIClient()

	// Set threshold to 10 so 15 tools will trigger tiering
	config := &TieredCapabilityConfig{
		MinToolsForTiering: 10,
	}

	aiClient.SetResponse(`["test-agent/capability_0"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, config)

	// Get capabilities
	_, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify LLM call was made (custom threshold exceeded)
	if len(aiClient.GetCalls()) == 0 {
		t.Error("Expected LLM call with custom threshold, but none was made")
	}
}

func TestTieredCapabilityProvider_EnvVarThreshold(t *testing.T) {
	// Set environment variable
	os.Setenv("GOMIND_TIERED_MIN_TOOLS", "10")
	defer os.Unsetenv("GOMIND_TIERED_MIN_TOOLS")

	// Setup: Create catalog with 15 tools
	catalog := setupTestCatalog(15)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	// Create provider without explicit config - should use env var
	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Verify threshold was set from env var
	if provider.MinToolsForTiering != 10 {
		t.Errorf("Expected MinToolsForTiering=10 from env var, got %d", provider.MinToolsForTiering)
	}

	// Get capabilities
	_, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify LLM call was made (env var threshold exceeded)
	if len(aiClient.GetCalls()) == 0 {
		t.Error("Expected LLM call with env var threshold, but none was made")
	}
}

func TestTieredCapabilityProvider_ConfigPrecedence(t *testing.T) {
	// Set environment variable
	os.Setenv("GOMIND_TIERED_MIN_TOOLS", "10")
	defer os.Unsetenv("GOMIND_TIERED_MIN_TOOLS")

	// Create provider with explicit config that should override env var
	config := &TieredCapabilityConfig{
		MinToolsForTiering: 25,
	}

	catalog := setupTestCatalog(1)
	aiClient := NewTieredTestAIClient()

	provider := NewTieredCapabilityProvider(catalog, aiClient, config)

	// Verify explicit config takes precedence over env var
	if provider.MinToolsForTiering != 25 {
		t.Errorf("Expected explicit config (25) to override env var (10), got %d", provider.MinToolsForTiering)
	}
}

func TestTieredCapabilityProvider_StructuredPrompt(t *testing.T) {
	// Setup: Create catalog with 25 tools
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities
	_, err := provider.GetCapabilities(context.Background(), "Get weather for NYC", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify prompt structure follows Guided-Structured Templates
	calls := aiClient.GetCalls()
	if len(calls) == 0 {
		t.Fatal("Expected LLM call to be made")
	}

	prompt := calls[0]

	// Check for structured sections
	if !strings.Contains(prompt, "STEP 1: TASK IDENTIFICATION") {
		t.Error("Expected prompt to contain STEP 1: TASK IDENTIFICATION")
	}
	if !strings.Contains(prompt, "STEP 2: AVAILABLE TOOLS") {
		t.Error("Expected prompt to contain STEP 2: AVAILABLE TOOLS")
	}
	if !strings.Contains(prompt, "STEP 3: USER REQUEST") {
		t.Error("Expected prompt to contain STEP 3: USER REQUEST")
	}
	if !strings.Contains(prompt, "STEP 4: STRUCTURED SELECTION PROCESS") {
		t.Error("Expected prompt to contain STEP 4: STRUCTURED SELECTION PROCESS")
	}
	if !strings.Contains(prompt, "Get weather for NYC") {
		t.Error("Expected prompt to contain user request")
	}
}

func TestTieredCapabilityProvider_MultiAgent(t *testing.T) {
	// Setup: Create catalog with 3 agents, 10 capabilities each (30 total)
	catalog := setupMultiAgentCatalog(3, 10)
	aiClient := NewTieredTestAIClient()

	// Select tools from different agents
	aiClient.SetResponse(`["agent-0/capability_0", "agent-1/capability_5", "agent-2/capability_9"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to get capabilities: %v", err)
	}

	// Verify tools from all selected agents are present
	if !strings.Contains(capabilities, "agent-0") {
		t.Error("Expected capabilities to contain agent-0")
	}
	if !strings.Contains(capabilities, "agent-1") {
		t.Error("Expected capabilities to contain agent-1")
	}
	if !strings.Contains(capabilities, "agent-2") {
		t.Error("Expected capabilities to contain agent-2")
	}
}

func TestTieredCapabilityProvider_ParseToolSelection_MarkdownWrapped(t *testing.T) {
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Test markdown-wrapped JSON response
	aiClient.SetResponse("```json\n[\"test-agent/capability_0\"]\n```")

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Failed to parse markdown-wrapped response: %v", err)
	}

	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected capabilities to contain capability_0")
	}
}

func TestTieredCapabilityProvider_InvalidJSON(t *testing.T) {
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Test invalid JSON response
	aiClient.SetResponse("not valid json")

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Should fall back gracefully
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful fallback on invalid JSON, got error: %v", err)
	}

	// Verify full catalog returned as fallback
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
}

func TestTieredCapabilityProvider_SetLogger(t *testing.T) {
	catalog := setupTestCatalog(5)
	aiClient := NewTieredTestAIClient()

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Should not panic when setting logger
	provider.SetLogger(&core.NoOpLogger{})
}

func TestTieredCapabilityProvider_SetTelemetry(t *testing.T) {
	catalog := setupTestCatalog(5)
	aiClient := NewTieredTestAIClient()

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Should not panic when setting telemetry
	provider.SetTelemetry(&core.NoOpTelemetry{})
}

func TestTieredCapabilityProvider_Shutdown(t *testing.T) {
	catalog := setupTestCatalog(5)
	aiClient := NewTieredTestAIClient()

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Shutdown should complete without error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected clean shutdown, got error: %v", err)
	}
}

func TestTieredCapabilityProvider_ShutdownTimeout(t *testing.T) {
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	// Create mock debug store that takes a long time
	mockStore := &mockSlowDebugStore{delay: 2 * time.Second}

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)
	provider.SetLLMDebugStore(mockStore)

	// Make a call to trigger debug recording
	_, _ = provider.GetCapabilities(context.Background(), "test", nil)

	// Shutdown with short timeout should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := provider.Shutdown(ctx)
	if err == nil {
		t.Error("Expected shutdown timeout error")
	}
}

// mockSlowDebugStore simulates a slow debug store for testing shutdown
type mockSlowDebugStore struct {
	delay time.Duration
}

func (m *mockSlowDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
	time.Sleep(m.delay)
	return nil
}

func (m *mockSlowDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
	return nil, nil
}

func (m *mockSlowDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
	return nil
}

func (m *mockSlowDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
	return nil
}

func (m *mockSlowDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
	return nil, nil
}

// Test helper function for extractFirstSentences
func TestExtractFirstSentences(t *testing.T) {
	tests := []struct {
		text     string
		n        int
		expected string
	}{
		{"First. Second. Third.", 2, "First. Second."},
		{"Only one sentence", 2, "Only one sentence"},
		{"First! Second? Third.", 2, "First! Second?"},
		{"", 2, ""},
		{"First sentence. Second sentence. Third sentence. Fourth.", 3, "First sentence. Second sentence. Third sentence."},
		{"No periods here", 1, "No periods here"},
		{"  Spaces around.  ", 1, "Spaces around."},
	}

	for _, tc := range tests {
		result := extractFirstSentences(tc.text, tc.n)
		if result != tc.expected {
			t.Errorf("extractFirstSentences(%q, %d) = %q, expected %q",
				tc.text, tc.n, result, tc.expected)
		}
	}
}

// Test catalog extension methods
func TestAgentCatalog_GetCapabilitySummaries(t *testing.T) {
	catalog := setupTestCatalog(5)

	summaries := catalog.GetCapabilitySummaries()

	if len(summaries) != 5 {
		t.Errorf("Expected 5 summaries, got %d", len(summaries))
	}

	// Verify structure
	for _, s := range summaries {
		if s.AgentName == "" {
			t.Error("Expected AgentName to be set")
		}
		if s.CapabilityName == "" {
			t.Error("Expected CapabilityName to be set")
		}
		if s.Summary == "" {
			t.Error("Expected Summary to be generated")
		}
	}
}

func TestAgentCatalog_GetToolCount(t *testing.T) {
	catalog := setupTestCatalog(15)

	count := catalog.GetToolCount()
	if count != 15 {
		t.Errorf("Expected tool count 15, got %d", count)
	}
}

func TestAgentCatalog_FormatToolsForLLM(t *testing.T) {
	catalog := setupMultiAgentCatalog(3, 10) // 30 total tools

	// Select specific tools
	toolIDs := []string{"agent-0/capability_0", "agent-1/capability_5"}

	formatted := catalog.FormatToolsForLLM(toolIDs)

	// Verify selected tools are present
	if !strings.Contains(formatted, "capability_0") {
		t.Error("Expected formatted output to contain capability_0")
	}
	if !strings.Contains(formatted, "capability_5") {
		t.Error("Expected formatted output to contain capability_5")
	}

	// Verify non-selected tools are NOT present
	if strings.Contains(formatted, "capability_9") {
		t.Error("Expected formatted output to NOT contain capability_9")
	}
}

func TestAgentCatalog_FormatToolsForLLM_UnknownTools(t *testing.T) {
	catalog := setupTestCatalog(5)

	// Include an unknown tool
	toolIDs := []string{"test-agent/capability_0", "unknown-agent/unknown_cap"}

	formatted := catalog.FormatToolsForLLM(toolIDs)

	// Should include valid tool
	if !strings.Contains(formatted, "capability_0") {
		t.Error("Expected formatted output to contain capability_0")
	}

	// Should NOT include unknown tool (silently ignored)
	if strings.Contains(formatted, "unknown_cap") {
		t.Error("Expected unknown tool to be silently ignored")
	}
}

func TestEnhancedCapability_GetSummary(t *testing.T) {
	// Test with explicit Summary
	cap1 := EnhancedCapability{
		Name:        "test",
		Description: "This is a long description. With multiple sentences. And more text.",
		Summary:     "Custom summary",
	}
	if cap1.GetSummary() != "Custom summary" {
		t.Errorf("Expected custom summary, got %s", cap1.GetSummary())
	}

	// Test auto-generation from Description
	cap2 := EnhancedCapability{
		Name:        "test",
		Description: "First sentence. Second sentence. Third sentence.",
	}
	expected := "First sentence. Second sentence."
	if cap2.GetSummary() != expected {
		t.Errorf("Expected auto-generated summary %q, got %q", expected, cap2.GetSummary())
	}
}

// =============================================================================
// Circuit Breaker Integration Tests
// =============================================================================

// tieredTestCircuitBreaker implements core.CircuitBreaker for tiered provider tests
// (separate from mockCircuitBreaker in test_mocks.go to track additional fields)
type tieredTestCircuitBreaker struct {
	shouldOpen    bool // If true, Execute returns error
	executeCalled bool // Track if Execute was called
	callCount     int  // Number of Execute calls
}

func newTieredTestCircuitBreaker() *tieredTestCircuitBreaker {
	return &tieredTestCircuitBreaker{}
}

func (m *tieredTestCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	m.executeCalled = true
	m.callCount++

	if m.shouldOpen {
		return errors.New("circuit breaker is open")
	}

	// Execute the wrapped function
	return fn()
}

func (m *tieredTestCircuitBreaker) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	return m.Execute(ctx, fn)
}

func (m *tieredTestCircuitBreaker) GetState() string {
	if m.shouldOpen {
		return "open"
	}
	return "closed"
}

func (m *tieredTestCircuitBreaker) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"call_count": m.callCount,
	}
}

func (m *tieredTestCircuitBreaker) Reset() {
	m.callCount = 0
	m.shouldOpen = false
}

func (m *tieredTestCircuitBreaker) CanExecute() bool {
	return !m.shouldOpen
}

func TestTieredCapabilityProvider_CircuitBreakerIntegration(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0", "test-agent/capability_1"]`)

	cb := newTieredTestCircuitBreaker()
	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)
	provider.SetCircuitBreaker(cb)

	// Get capabilities
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify circuit breaker was used
	if !cb.executeCalled {
		t.Error("Expected circuit breaker Execute to be called")
	}
	if cb.callCount != 1 {
		t.Errorf("Expected 1 circuit breaker call, got %d", cb.callCount)
	}

	// Verify capabilities were returned correctly
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected capabilities to contain capability_0")
	}
}

func TestTieredCapabilityProvider_CircuitBreakerOpen(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	cb := newTieredTestCircuitBreaker()
	cb.shouldOpen = true // Circuit is open - should reject

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)
	provider.SetCircuitBreaker(cb)

	// Get capabilities - should fall back gracefully when circuit is open
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful fallback, got error: %v", err)
	}

	// Verify fallback to FormatForLLM (all tools returned)
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include capability_0")
	}
	if !strings.Contains(capabilities, "capability_10") {
		t.Error("Expected fallback to include capability_10 (all tools)")
	}
	if !strings.Contains(capabilities, "capability_20") {
		t.Error("Expected fallback to include capability_20 (all tools)")
	}
}

func TestTieredCapabilityProvider_WithoutCircuitBreaker(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	// Create provider WITHOUT circuit breaker
	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)
	// Note: No SetCircuitBreaker call

	// Get capabilities - should work without circuit breaker
	capabilities, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected success without circuit breaker, got error: %v", err)
	}

	// Verify LLM call was made directly
	if len(aiClient.GetCalls()) == 0 {
		t.Error("Expected LLM call to be made")
	}

	// Verify capabilities were returned
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected capabilities to contain capability_0")
	}
}

func TestTieredCapabilityProvider_CircuitBreakerErrorRecording(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	cb := newTieredTestCircuitBreaker()
	mockStore := &tieredTestDebugStore{interactions: make([]LLMInteraction, 0)}

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)
	provider.SetCircuitBreaker(cb)
	provider.SetLLMDebugStore(mockStore)

	// Get capabilities
	_, err := provider.GetCapabilities(context.Background(), "test request", nil)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Use Shutdown() to wait for async debug recording (uses internal WaitGroup)
	// This is cleaner than time.Sleep and ensures deterministic behavior
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Verify debug interaction was recorded
	if len(mockStore.interactions) == 0 {
		t.Error("Expected debug interaction to be recorded")
	}
	if len(mockStore.interactions) > 0 && mockStore.interactions[0].Type != "tiered_selection" {
		t.Errorf("Expected type 'tiered_selection', got %s", mockStore.interactions[0].Type)
	}
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestTieredCapabilityProvider_ContextCancellation(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Get capabilities with cancelled context - should fall back gracefully
	capabilities, err := provider.GetCapabilities(ctx, "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful fallback on cancelled context, got error: %v", err)
	}

	// Verify fallback returns all tools
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
	if !strings.Contains(capabilities, "capability_10") {
		t.Error("Expected fallback to include capability_10")
	}
}

func TestTieredCapabilityProvider_ContextCancellationNoLLMCall(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Get capabilities with cancelled context
	_, _ = provider.GetCapabilities(ctx, "test request", nil)

	// Verify NO LLM call was made (context was checked before expensive operation)
	if len(aiClient.GetCalls()) > 0 {
		t.Error("Expected no LLM call when context is cancelled before selectRelevantTools")
	}
}

func TestTieredCapabilityProvider_ContextTimeout(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)

	// Create a slow AI client
	slowAIClient := &slowTestAIClient{
		delay:    500 * time.Millisecond,
		response: `["test-agent/capability_0"]`,
	}

	provider := NewTieredCapabilityProvider(catalog, slowAIClient, nil)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Get capabilities - context will timeout
	capabilities, err := provider.GetCapabilities(ctx, "test request", nil)
	if err != nil {
		t.Fatalf("Expected graceful fallback on timeout, got error: %v", err)
	}

	// Verify fallback returns all tools
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools on timeout")
	}
}

// slowTestAIClient simulates a slow AI client for testing timeout behavior
type slowTestAIClient struct {
	delay    time.Duration
	response string
}

func (s *slowTestAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.delay):
		return &core.AIResponse{
			Content:  s.response,
			Model:    "test-model",
			Provider: "test-provider",
		}, nil
	}
}

// tieredTestDebugStore is a simple in-memory debug store for tiered provider testing
type tieredTestDebugStore struct {
	interactions []LLMInteraction
	mu           sync.Mutex
}

func (m *tieredTestDebugStore) RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactions = append(m.interactions, interaction)
	return nil
}

func (m *tieredTestDebugStore) GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error) {
	return nil, nil
}

func (m *tieredTestDebugStore) SetMetadata(ctx context.Context, requestID string, key, value string) error {
	return nil
}

func (m *tieredTestDebugStore) ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error {
	return nil
}

func (m *tieredTestDebugStore) ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error) {
	return nil, nil
}

// =============================================================================
// Error Wrapping Tests
// =============================================================================

func TestTieredCapabilityProvider_ErrorWrapping(t *testing.T) {
	// Setup: Create catalog with 25 tools (above threshold)
	catalog := setupTestCatalog(25)
	aiClient := NewTieredTestAIClient()

	// Set error to simulate LLM failure
	testErr := errors.New("LLM service unavailable")
	aiClient.SetError(testErr)

	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Get capabilities - should fall back gracefully
	capabilities, err := provider.GetCapabilities(context.Background(), "test request with some context", nil)
	if err != nil {
		t.Fatalf("Expected graceful fallback, got error: %v", err)
	}

	// Verify fallback worked
	if !strings.Contains(capabilities, "capability_0") {
		t.Error("Expected fallback to include all tools")
	}
}

func TestTieredCapabilityProvider_TruncateRequest(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long request that should be truncated", 20, "this is a long reque..."},
		{"", 10, ""},
		{"test", 0, "..."},
	}

	for _, tc := range tests {
		result := truncateRequest(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateRequest(%q, %d) = %q, expected %q",
				tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

// =============================================================================
// SetCircuitBreaker Method Tests
// =============================================================================

func TestTieredCapabilityProvider_SetCircuitBreaker(t *testing.T) {
	catalog := setupTestCatalog(5)
	aiClient := NewTieredTestAIClient()
	provider := NewTieredCapabilityProvider(catalog, aiClient, nil)

	// Initially no circuit breaker
	cb := newTieredTestCircuitBreaker()
	provider.SetCircuitBreaker(cb)

	// Verify circuit breaker was set (by checking it's used in a call)
	aiClient.SetResponse(`["test-agent/capability_0"]`)

	// Below threshold - circuit breaker won't be used
	// But we can verify setting nil doesn't crash
	provider.SetCircuitBreaker(nil)

	// Should not panic
	_, err := provider.GetCapabilities(context.Background(), "test", nil)
	if err != nil {
		t.Errorf("Unexpected error with nil circuit breaker: %v", err)
	}
}
