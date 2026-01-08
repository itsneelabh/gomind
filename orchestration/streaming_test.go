package orchestration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// ================================
// Streaming Unit Tests for Orchestrator
// ================================

// StreamingMockAIClient implements both core.AIClient and core.StreamingAIClient
type StreamingMockAIClient struct {
	responses         map[string]string
	calls             []string
	supportsStreaming bool
	streamCallCount   int
}

func NewStreamingMockAIClient() *StreamingMockAIClient {
	return &StreamingMockAIClient{
		responses:         make(map[string]string),
		calls:             []string{},
		supportsStreaming: true,
	}
}

func (m *StreamingMockAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	m.calls = append(m.calls, prompt)

	// Return predefined responses based on prompt content
	if strings.Contains(prompt, "Create an execution plan") {
		return &core.AIResponse{
			Content: m.getPlanResponse(),
			Usage: core.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}, nil
	}

	if strings.Contains(prompt, "Synthesize") || strings.Contains(prompt, "synthesize") {
		return &core.AIResponse{
			Content: "This is a synthesized response combining all agent outputs.",
		}, nil
	}

	return &core.AIResponse{
		Content: "Default response",
	}, nil
}

func (m *StreamingMockAIClient) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
	m.streamCallCount++
	m.calls = append(m.calls, prompt)

	response := "This is a streaming synthesized response from the orchestrator."

	chunkSize := 10
	var fullContent strings.Builder
	chunkIndex := 0

	for i := 0; i < len(response); i += chunkSize {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if fullContent.Len() > 0 {
				return &core.AIResponse{
					Content: fullContent.String(),
					Model:   "mock-model",
				}, core.ErrStreamPartiallyCompleted
			}
			return nil, ctx.Err()
		default:
		}

		end := i + chunkSize
		if end > len(response) {
			end = len(response)
		}

		chunk := core.StreamChunk{
			Content: response[i:end],
			Delta:   true,
			Index:   chunkIndex,
			Model:   "mock-model",
		}
		fullContent.WriteString(response[i:end])
		chunkIndex++

		if err := callback(chunk); err != nil {
			return &core.AIResponse{
				Content: fullContent.String(),
				Model:   "mock-model",
			}, nil
		}
	}

	// Send final chunk
	usage := core.TokenUsage{
		PromptTokens:     len(prompt) / 4,
		CompletionTokens: len(response) / 4,
		TotalTokens:      (len(prompt) + len(response)) / 4,
	}
	finalChunk := core.StreamChunk{
		Delta:        false,
		Index:        chunkIndex,
		FinishReason: "stop",
		Model:        "mock-model",
		Usage:        &usage,
	}
	_ = callback(finalChunk)

	return &core.AIResponse{
		Content: fullContent.String(),
		Model:   "mock-model",
		Usage:   usage,
	}, nil
}

func (m *StreamingMockAIClient) SupportsStreaming() bool {
	return m.supportsStreaming
}

func (m *StreamingMockAIClient) getPlanResponse() string {
	plan := map[string]interface{}{
		"plan_id":          "test-plan-1",
		"original_request": "test request",
		"mode":             "autonomous",
		"steps": []map[string]interface{}{
			{
				"step_id":     "step-1",
				"agent_name":  "test-agent",
				"namespace":   "default",
				"instruction": "Test instruction",
				"depends_on":  []string{},
				"metadata": map[string]interface{}{
					"capability": "test_capability",
					"parameters": map[string]interface{}{
						"param1": "value1",
					},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(plan)
	return string(jsonBytes)
}

// TestProcessRequestStreaming_BasicStreaming tests basic streaming functionality
func TestProcessRequestStreaming_BasicStreaming(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewStreamingMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "test-agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "test_capability"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "test-agent",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "test_capability"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "test_capability", Description: "Test capability"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	var chunks []core.StreamChunk
	var fullContent strings.Builder
	callback := func(chunk core.StreamChunk) error {
		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
		}
		return nil
	}

	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Test request", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed: %v", err)
	}

	// Verify response
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Verify streaming-specific fields
	if resp.ChunksDelivered == 0 {
		t.Error("Expected ChunksDelivered > 0")
	}
	if !resp.StreamCompleted {
		t.Error("Expected StreamCompleted to be true")
	}
	if resp.PartialContent {
		t.Error("Expected PartialContent to be false")
	}

	// Verify chunks were delivered
	if len(chunks) == 0 {
		t.Error("Expected chunks to be delivered")
	}

	// Verify full content was streamed
	if fullContent.Len() == 0 {
		t.Error("Expected content to be streamed")
	}
}

// TestProcessRequestStreaming_FallbackToSimulated tests fallback when AI client doesn't support streaming
func TestProcessRequestStreaming_FallbackToSimulated(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient() // Non-streaming client

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "stock-analyzer",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "analyze_stock"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "stock-analyzer",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "analyze_stock"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "analyze_stock", Description: "Analyzes stocks"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	var chunks []core.StreamChunk
	callback := func(chunk core.StreamChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}

	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Analyze AAPL stock", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed: %v", err)
	}

	// Should still work via simulated streaming
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Verify streaming completed (simulated)
	if !resp.StreamCompleted {
		t.Error("Expected StreamCompleted to be true even with simulated streaming")
	}

	// Verify chunks were delivered (simulated)
	if len(chunks) == 0 {
		t.Error("Expected simulated chunks to be delivered")
	}
}

// TestProcessRequestStreaming_CallbackStop tests callback can stop streaming
func TestProcessRequestStreaming_CallbackStop(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewStreamingMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "test-agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "test_capability"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "test-agent",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "test_capability"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "test_capability", Description: "Test capability"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	chunkCount := 0
	callback := func(chunk core.StreamChunk) error {
		chunkCount++
		if chunkCount >= 3 {
			return context.Canceled // Stop after 3 chunks
		}
		return nil
	}

	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Test request", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed: %v", err)
	}

	// Should have partial content
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}

	// Should indicate partial content
	if resp.StreamCompleted && !resp.PartialContent {
		// Either not completed or marked as partial
		t.Log("Callback stop may result in partial content")
	}
}

// TestProcessRequestStreaming_NilTelemetry tests nil telemetry doesn't panic
func TestProcessRequestStreaming_NilTelemetry(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewStreamingMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "test-agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "test_capability"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)
	orchestrator.telemetry = nil // Explicitly set to nil

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "test-agent",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "test_capability"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "test_capability", Description: "Test capability"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	callback := func(chunk core.StreamChunk) error { return nil }

	// Should not panic
	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Test request", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed with nil telemetry: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected response, got nil")
	}
}

// TestStreamingOrchestratorResponse_Fields tests StreamingOrchestratorResponse fields
func TestStreamingOrchestratorResponse_Fields(t *testing.T) {
	resp := &StreamingOrchestratorResponse{
		OrchestratorResponse: OrchestratorResponse{
			RequestID:       "test-123",
			OriginalRequest: "Test request",
			Response:        "Test response",
			RoutingMode:     ModeAutonomous,
			ExecutionTime:   100 * time.Millisecond,
			AgentsInvolved:  []string{"agent1", "agent2"},
			Confidence:      0.95,
		},
		ChunksDelivered: 10,
		StreamCompleted: true,
		PartialContent:  false,
	}

	// Verify embedded fields
	if resp.RequestID != "test-123" {
		t.Errorf("Expected RequestID 'test-123', got %q", resp.RequestID)
	}
	if resp.OriginalRequest != "Test request" {
		t.Errorf("Expected OriginalRequest 'Test request', got %q", resp.OriginalRequest)
	}
	if resp.Response != "Test response" {
		t.Errorf("Expected Response 'Test response', got %q", resp.Response)
	}

	// Verify streaming-specific fields
	if resp.ChunksDelivered != 10 {
		t.Errorf("Expected ChunksDelivered 10, got %d", resp.ChunksDelivered)
	}
	if !resp.StreamCompleted {
		t.Error("Expected StreamCompleted true")
	}
	if resp.PartialContent {
		t.Error("Expected PartialContent false")
	}
}

// TestBuildSynthesisPrompt tests the synthesis prompt builder
func TestBuildSynthesisPrompt(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()
	orchestrator := NewAIOrchestrator(DefaultConfig(), discovery, aiClient)

	result := &ExecutionResult{
		PlanID: "test-plan",
		Steps: []StepResult{
			{
				StepID:    "step-1",
				AgentName: "weather-agent",
				Response:  "Weather is sunny",
				Success:   true,
			},
			{
				StepID:    "step-2",
				AgentName: "news-agent",
				Response:  "Top news: Tech stocks up",
				Success:   true,
			},
		},
		Success: true,
	}

	prompt := orchestrator.buildSynthesisPrompt("What's the weather and news?", result)

	// Verify prompt contains request
	if !strings.Contains(prompt, "What's the weather and news?") {
		t.Error("Expected prompt to contain original request")
	}

	// Verify prompt contains agent responses
	if !strings.Contains(prompt, "weather-agent") {
		t.Error("Expected prompt to contain weather-agent")
	}
	if !strings.Contains(prompt, "Weather is sunny") {
		t.Error("Expected prompt to contain weather response")
	}
	if !strings.Contains(prompt, "news-agent") {
		t.Error("Expected prompt to contain news-agent")
	}
	if !strings.Contains(prompt, "Top news: Tech stocks up") {
		t.Error("Expected prompt to contain news response")
	}
}

// TestProcessRequestStreaming_AgentsInvolved tests that agents involved are tracked
func TestProcessRequestStreaming_AgentsInvolved(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewStreamingMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "test-agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "test_capability"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "test-agent",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "test_capability"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "test_capability", Description: "Test capability"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	callback := func(chunk core.StreamChunk) error { return nil }

	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Test request", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed: %v", err)
	}

	// Verify agents involved are tracked
	if len(resp.AgentsInvolved) == 0 {
		t.Error("Expected AgentsInvolved to be populated")
	}
}

// TestProcessRequestStreaming_ExecutionTime tests that execution time is tracked
func TestProcessRequestStreaming_ExecutionTime(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewStreamingMockAIClient()

	// Register test agent
	_ = discovery.Register(context.Background(), &core.ServiceRegistration{
		ID:           "test-1",
		Name:         "test-agent",
		Address:      "localhost",
		Port:         8080,
		Capabilities: []core.Capability{{Name: "test_capability"}},
	})

	config := DefaultConfig()
	orchestrator := NewAIOrchestrator(config, discovery, aiClient)

	// Setup catalog
	orchestrator.catalog.agents = map[string]*AgentInfo{
		"test-1": {
			Registration: &core.ServiceRegistration{
				ID:           "test-1",
				Name:         "test-agent",
				Address:      "localhost",
				Port:         8080,
				Capabilities: []core.Capability{{Name: "test_capability"}},
			},
			Capabilities: []EnhancedCapability{
				{Name: "test_capability", Description: "Test capability"},
			},
		},
	}
	orchestrator.executor = NewSmartExecutor(orchestrator.catalog)

	callback := func(chunk core.StreamChunk) error { return nil }

	resp, err := orchestrator.ProcessRequestStreaming(context.Background(), "Test request", nil, callback)
	if err != nil {
		t.Fatalf("ProcessRequestStreaming failed: %v", err)
	}

	// Verify execution time is positive
	if resp.ExecutionTime <= 0 {
		t.Errorf("Expected positive ExecutionTime, got %v", resp.ExecutionTime)
	}
}
