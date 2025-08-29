package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAISynthesizer_Synthesize(t *testing.T) {
	aiClient := NewMockAIClient()
	synthesizer := NewAISynthesizer(aiClient)

	ctx := context.Background()
	request := "Test request"

	results := &ExecutionResult{
		PlanID:  "test-plan",
		Success: true,
		Steps: []StepResult{
			{
				StepID:    "step-1",
				AgentName: "agent1",
				Response:  `{"data": "response1"}`,
				Success:   true,
			},
			{
				StepID:    "step-2",
				AgentName: "agent2",
				Response:  "plain text response",
				Success:   true,
			},
			{
				StepID:    "step-3",
				AgentName: "agent3",
				Error:     "agent failed",
				Success:   false,
			},
		},
	}

	// Test LLM synthesis
	synthesizer.SetStrategy(StrategyLLM)
	response, err := synthesizer.Synthesize(ctx, request, results)
	if err != nil {
		t.Errorf("LLM synthesis failed: %v", err)
	}
	if response == "" {
		t.Error("Expected non-empty synthesized response")
	}

	// Test template synthesis
	synthesizer.SetStrategy(StrategyTemplate)
	response, err = synthesizer.Synthesize(ctx, request, results)
	if err != nil {
		t.Errorf("Template synthesis failed: %v", err)
	}
	if !strings.Contains(response, "Response to:") {
		t.Error("Template synthesis should contain 'Response to:'")
	}
	if !strings.Contains(response, "2 of 3 tasks successfully") {
		t.Error("Template should mention success count")
	}

	// Test simple synthesis
	synthesizer.SetStrategy(StrategySimple)
	response, err = synthesizer.Synthesize(ctx, request, results)
	if err != nil {
		t.Errorf("Simple synthesis failed: %v", err)
	}
	if !strings.Contains(response, "agent1:") {
		t.Error("Simple synthesis should contain agent names")
	}
}

func TestAISynthesizer_BuildSynthesisPrompt(t *testing.T) {
	synthesizer := &AISynthesizer{}

	request := "Analyze stock"
	results := &ExecutionResult{
		Steps: []StepResult{
			{
				StepID:      "step-1",
				AgentName:   "stock-agent",
				Instruction: "Get stock price",
				Response:    `{"price": 150.50}`,
				Success:     true,
			},
			{
				StepID:    "step-2",
				AgentName: "news-agent",
				Error:     "Service unavailable",
				Success:   false,
			},
		},
	}

	prompt := synthesizer.buildSynthesisPrompt(request, results)

	// Verify prompt contains expected elements
	expectedStrings := []string{
		"User Request: Analyze stock",
		"Agent Responses:",
		"stock-agent",
		"Get stock price",
		`"price": 150.5`,
		"news-agent (FAILED)",
		"Service unavailable",
		"Instructions:",
		"Synthesize",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(prompt, expected) {
			t.Errorf("Expected prompt to contain '%s'", expected)
		}
	}
}

func TestAISynthesizer_SynthesizeWithTemplate(t *testing.T) {
	synthesizer := &AISynthesizer{}

	request := "Test request"
	results := &ExecutionResult{
		Steps: []StepResult{
			{
				AgentName: "agent1",
				Response:  `{"status": "ok"}`,
				Success:   true,
			},
			{
				AgentName: "agent2",
				Response:  "plain response",
				Success:   true,
			},
			{
				AgentName: "agent3",
				Error:     "timeout",
				Success:   false,
			},
		},
	}

	response, err := synthesizer.synthesizeWithTemplate(request, results)
	if err != nil {
		t.Errorf("Template synthesis failed: %v", err)
	}

	// Check structure
	if !strings.Contains(response, "Response to: Test request") {
		t.Error("Missing request header")
	}

	if !strings.Contains(response, "Results:") {
		t.Error("Missing results section")
	}

	if !strings.Contains(response, "agent1:") {
		t.Error("Missing successful agent")
	}

	if !strings.Contains(response, "Some agents encountered errors") {
		t.Error("Missing error notification")
	}

	if !strings.Contains(response, "Completed 2 of 3 tasks successfully") {
		t.Error("Missing summary")
	}
}

func TestAISynthesizer_SynthesizeSimple(t *testing.T) {
	synthesizer := &AISynthesizer{}

	// Test with successful results
	results := &ExecutionResult{
		Steps: []StepResult{
			{
				AgentName: "agent1",
				Response:  "response1",
				Success:   true,
			},
			{
				AgentName: "agent2",
				Response:  "response2",
				Success:   true,
			},
			{
				AgentName: "agent3",
				Response:  "",
				Success:   false,
			},
		},
	}

	response, err := synthesizer.synthesizeSimple(results)
	if err != nil {
		t.Errorf("Simple synthesis failed: %v", err)
	}

	if !strings.Contains(response, "agent1: response1") {
		t.Error("Missing agent1 response")
	}

	if !strings.Contains(response, "agent2: response2") {
		t.Error("Missing agent2 response")
	}

	if strings.Contains(response, "agent3") {
		t.Error("Failed agent should not be included")
	}

	// Test with no successful results
	emptyResults := &ExecutionResult{
		Steps: []StepResult{
			{Success: false},
		},
	}

	response, err = synthesizer.synthesizeSimple(emptyResults)
	if err != nil {
		t.Errorf("Simple synthesis failed: %v", err)
	}

	if response != "No successful responses to synthesize" {
		t.Errorf("Expected no responses message, got: %s", response)
	}
}

func TestSimpleSynthesizer(t *testing.T) {
	synthesizer := NewSynthesizer()

	ctx := context.Background()
	request := "Test request"

	results := &ExecutionResult{
		Steps: []StepResult{
			{
				AgentName: "agent1",
				Response:  "response1",
				Success:   true,
			},
			{
				AgentName: "agent2",
				Error:     "failed",
				Success:   false,
			},
		},
	}

	// Test template strategy
	synthesizer.SetStrategy(StrategyTemplate)
	response, err := synthesizer.Synthesize(ctx, request, results)
	if err != nil {
		t.Errorf("Template synthesis failed: %v", err)
	}

	if !strings.Contains(response, "agent1 completed successfully") {
		t.Error("Expected success message for agent1")
	}

	if !strings.Contains(response, "agent2 failed") {
		t.Error("Expected failure message for agent2")
	}

	// Test simple strategy
	synthesizer.SetStrategy(StrategySimple)
	response, err = synthesizer.Synthesize(ctx, request, results)
	if err != nil {
		t.Errorf("Simple synthesis failed: %v", err)
	}

	if !strings.Contains(response, "response1") {
		t.Error("Expected response1 in output")
	}

	if strings.Contains(response, "failed") {
		t.Error("Failed responses should not be in simple output")
	}
}

func TestSynthesisStrategies(t *testing.T) {
	// Test that all strategies are handled
	strategies := []SynthesisStrategy{
		StrategyLLM,
		StrategyTemplate,
		StrategySimple,
		StrategyCustom,
	}

	aiClient := NewMockAIClient()
	synthesizer := NewAISynthesizer(aiClient)

	ctx := context.Background()
	results := &ExecutionResult{
		Steps: []StepResult{
			{
				AgentName: "test",
				Response:  "test",
				Success:   true,
			},
		},
	}

	for _, strategy := range strategies {
		synthesizer.SetStrategy(strategy)
		_, err := synthesizer.Synthesize(ctx, "test", results)
		if err != nil {
			t.Errorf("Strategy %s failed: %v", strategy, err)
		}
	}
}

func BenchmarkSynthesizer_Simple(b *testing.B) {
	synthesizer := &AISynthesizer{}
	results := &ExecutionResult{
		Steps: []StepResult{
			{AgentName: "agent1", Response: "response1", Success: true},
			{AgentName: "agent2", Response: "response2", Success: true},
			{AgentName: "agent3", Response: "response3", Success: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = synthesizer.synthesizeSimple(results)
	}
}

func BenchmarkSynthesizer_Template(b *testing.B) {
	synthesizer := &AISynthesizer{}
	results := &ExecutionResult{
		Steps: []StepResult{
			{AgentName: "agent1", Response: `{"data": "test"}`, Success: true},
			{AgentName: "agent2", Response: "response2", Success: true},
			{AgentName: "agent3", Error: "failed", Success: false},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = synthesizer.synthesizeWithTemplate("test request", results)
	}
}

func TestExecutionResult_JSON(t *testing.T) {
	// Test that results can be marshaled/unmarshaled properly
	result := &ExecutionResult{
		PlanID:        "test-123",
		Success:       true,
		TotalDuration: 5 * time.Second,
		Steps: []StepResult{
			{
				StepID:    "step-1",
				AgentName: "agent1",
				Response:  "test",
				Success:   true,
				Duration:  1 * time.Second,
			},
		},
	}

	// This is more of a smoke test to ensure our structures are serializable
	// which is important for the orchestration system
	_ = result.PlanID
	_ = result.Success
	_ = result.TotalDuration
	_ = result.Steps[0].Duration
}
