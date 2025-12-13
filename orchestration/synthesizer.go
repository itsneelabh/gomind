package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// AISynthesizer uses AI to synthesize agent responses
type AISynthesizer struct {
	aiClient core.AIClient
	strategy SynthesisStrategy
}

// NewAISynthesizer creates a new AI-powered synthesizer
func NewAISynthesizer(aiClient core.AIClient) *AISynthesizer {
	return &AISynthesizer{
		aiClient: aiClient,
		strategy: StrategyLLM,
	}
}

// Synthesize combines agent responses into a final response using AI
func (s *AISynthesizer) Synthesize(ctx context.Context, request string, results *ExecutionResult) (string, error) {
	switch s.strategy {
	case StrategyLLM:
		return s.synthesizeWithLLM(ctx, request, results)
	case StrategyTemplate:
		return s.synthesizeWithTemplate(request, results)
	case StrategySimple:
		return s.synthesizeSimple(results)
	default:
		return s.synthesizeWithLLM(ctx, request, results)
	}
}

// synthesizeWithLLM uses the LLM to create a coherent response
func (s *AISynthesizer) synthesizeWithLLM(ctx context.Context, request string, results *ExecutionResult) (string, error) {
	// Build prompt with all agent responses
	prompt := s.buildSynthesisPrompt(request, results)

	// Telemetry: Record LLM prompt for synthesis
	telemetry.AddSpanEvent(ctx, "llm.synthesis.request",
		attribute.String("original_request", truncateString(request, 500)),
		attribute.String("prompt", truncateString(prompt, 2000)),
		attribute.Int("prompt_length", len(prompt)),
		attribute.Int("step_count", len(results.Steps)),
		attribute.Float64("temperature", 0.5),
		attribute.Int("max_tokens", 1500),
	)

	// Call LLM for synthesis
	llmStartTime := time.Now()
	aiResponse, err := s.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature:  0.5, // Balanced creativity
		MaxTokens:    1500,
		SystemPrompt: "You are an AI that synthesizes multiple agent responses into coherent, helpful answers.",
	})
	llmDuration := time.Since(llmStartTime)

	if err != nil {
		telemetry.AddSpanEvent(ctx, "llm.synthesis.error",
			attribute.String("error", err.Error()),
			attribute.Int64("duration_ms", llmDuration.Milliseconds()),
		)
		return "", fmt.Errorf("synthesis failed: %w", err)
	}

	// Telemetry: Record LLM response for synthesis
	telemetry.AddSpanEvent(ctx, "llm.synthesis.response",
		attribute.String("response", truncateString(aiResponse.Content, 2000)),
		attribute.Int("response_length", len(aiResponse.Content)),
		attribute.Int("prompt_tokens", aiResponse.Usage.PromptTokens),
		attribute.Int("completion_tokens", aiResponse.Usage.CompletionTokens),
		attribute.Int("total_tokens", aiResponse.Usage.TotalTokens),
		attribute.Int64("duration_ms", llmDuration.Milliseconds()),
	)

	return aiResponse.Content, nil
}

// buildSynthesisPrompt creates the prompt for response synthesis
func (s *AISynthesizer) buildSynthesisPrompt(request string, results *ExecutionResult) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("User Request: %s\n\n", request))
	builder.WriteString("Agent Responses:\n\n")

	// Include all successful step results
	for _, step := range results.Steps {
		if step.Success {
			builder.WriteString(fmt.Sprintf("Agent: %s\n", step.AgentName))
			builder.WriteString(fmt.Sprintf("Task: %s\n", step.Instruction))

			// Try to parse and format the response
			var responseData interface{}
			if err := json.Unmarshal([]byte(step.Response), &responseData); err == nil {
				// Successfully parsed as JSON
				formatted, _ := json.MarshalIndent(responseData, "", "  ")
				builder.WriteString(fmt.Sprintf("Response:\n%s\n\n", string(formatted)))
			} else {
				// Plain text response
				builder.WriteString(fmt.Sprintf("Response: %s\n\n", step.Response))
			}
		} else {
			// Include error information
			builder.WriteString(fmt.Sprintf("Agent: %s (FAILED)\n", step.AgentName))
			builder.WriteString(fmt.Sprintf("Error: %s\n\n", step.Error))
		}
	}

	builder.WriteString("\nInstructions:\n")
	builder.WriteString("1. Synthesize the above agent responses into a comprehensive answer\n")
	builder.WriteString("2. Address the user's original request directly\n")
	builder.WriteString("3. Combine information from multiple agents where relevant\n")
	builder.WriteString("4. Highlight any important findings or recommendations\n")
	builder.WriteString("5. Be concise but thorough\n")
	builder.WriteString("6. If some agents failed, work with available information\n\n")
	builder.WriteString("Synthesized Response:")

	return builder.String()
}

// synthesizeWithTemplate uses predefined templates for synthesis
func (s *AISynthesizer) synthesizeWithTemplate(request string, results *ExecutionResult) (string, error) {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Response to: %s\n\n", request))

	// Group results by success/failure
	var successful, failed []StepResult
	for _, step := range results.Steps {
		if step.Success {
			successful = append(successful, step)
		} else {
			failed = append(failed, step)
		}
	}

	// Present successful results
	if len(successful) > 0 {
		builder.WriteString("Results:\n")
		for _, step := range successful {
			builder.WriteString(fmt.Sprintf("\n%s:\n", step.AgentName))

			// Try to parse and present JSON nicely
			var data interface{}
			if err := json.Unmarshal([]byte(step.Response), &data); err == nil {
				formatted, _ := json.MarshalIndent(data, "  ", "  ")
				builder.WriteString(string(formatted))
			} else {
				builder.WriteString(fmt.Sprintf("  %s", step.Response))
			}
			builder.WriteString("\n")
		}
	}

	// Note any failures
	if len(failed) > 0 {
		builder.WriteString("\nNote: Some agents encountered errors:\n")
		for _, step := range failed {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", step.AgentName, step.Error))
		}
	}

	// Summary
	builder.WriteString(fmt.Sprintf("\nCompleted %d of %d tasks successfully.\n",
		len(successful), len(results.Steps)))

	return builder.String(), nil
}

// synthesizeSimple concatenates responses
func (s *AISynthesizer) synthesizeSimple(results *ExecutionResult) (string, error) {
	var responses []string

	for _, step := range results.Steps {
		if step.Success {
			responses = append(responses, fmt.Sprintf("%s: %s", step.AgentName, step.Response))
		}
	}

	if len(responses) == 0 {
		return "No successful responses to synthesize", nil
	}

	return strings.Join(responses, "\n\n"), nil
}

// SetStrategy sets the synthesis strategy
func (s *AISynthesizer) SetStrategy(strategy SynthesisStrategy) {
	s.strategy = strategy
}

// SimpleSynthesizer provides basic synthesis without AI
type SimpleSynthesizer struct {
	strategy SynthesisStrategy
}

// NewSynthesizer creates a new synthesizer (backward compatibility)
func NewSynthesizer() *SimpleSynthesizer {
	return &SimpleSynthesizer{
		strategy: StrategySimple,
	}
}

// Synthesize combines agent responses (simple version)
func (s *SimpleSynthesizer) Synthesize(ctx context.Context, request string, results *ExecutionResult) (string, error) {
	switch s.strategy {
	case StrategyTemplate:
		return s.synthesizeWithTemplate(request, results)
	case StrategySimple:
		return s.synthesizeSimple(results)
	default:
		return s.synthesizeSimple(results)
	}
}

// synthesizeWithTemplate uses predefined templates
func (s *SimpleSynthesizer) synthesizeWithTemplate(request string, results *ExecutionResult) (string, error) {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Response to: %s\n\n", request))

	for _, step := range results.Steps {
		if step.Success {
			builder.WriteString(fmt.Sprintf("%s completed successfully:\n%s\n\n",
				step.AgentName, step.Response))
		} else {
			builder.WriteString(fmt.Sprintf("%s failed: %s\n\n",
				step.AgentName, step.Error))
		}
	}

	return builder.String(), nil
}

// synthesizeSimple concatenates responses
func (s *SimpleSynthesizer) synthesizeSimple(results *ExecutionResult) (string, error) {
	var responses []string

	for _, step := range results.Steps {
		if step.Success {
			responses = append(responses, step.Response)
		}
	}

	return strings.Join(responses, "\n"), nil
}

// SetStrategy sets the synthesis strategy
func (s *SimpleSynthesizer) SetStrategy(strategy SynthesisStrategy) {
	s.strategy = strategy
}
