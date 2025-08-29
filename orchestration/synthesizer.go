package orchestration

import (
	"context"
	"strings"
)

// SimpleSynthesizer implements the Synthesizer interface
type SimpleSynthesizer struct {
	strategy SynthesisStrategy
}

// NewSynthesizer creates a new synthesizer
func NewSynthesizer() *SimpleSynthesizer {
	return &SimpleSynthesizer{
		strategy: StrategySimple,
	}
}

// Synthesize combines agent responses into a final response
func (s *SimpleSynthesizer) Synthesize(ctx context.Context, request string, results *ExecutionResult) (string, error) {
	switch s.strategy {
	case StrategySimple:
		return s.simpleSynthesize(results)
	case StrategyTemplate:
		return s.templateSynthesize(request, results)
	case StrategyLLM:
		// In a full implementation, this would use an LLM
		return s.simpleSynthesize(results)
	default:
		return s.simpleSynthesize(results)
	}
}

// SetStrategy sets the synthesis strategy
func (s *SimpleSynthesizer) SetStrategy(strategy SynthesisStrategy) {
	s.strategy = strategy
}

// simpleSynthesize concatenates all responses
func (s *SimpleSynthesizer) simpleSynthesize(results *ExecutionResult) (string, error) {
	var responses []string
	for _, step := range results.Steps {
		if step.Success && step.Response != "" {
			responses = append(responses, step.Response)
		}
	}
	
	if len(responses) == 0 {
		return "No responses available", nil
	}
	
	return strings.Join(responses, "\n"), nil
}

// templateSynthesize uses a template to format responses
func (s *SimpleSynthesizer) templateSynthesize(request string, results *ExecutionResult) (string, error) {
	// Simplified template synthesis
	var builder strings.Builder
	builder.WriteString("Request: " + request + "\n")
	builder.WriteString("Results:\n")
	
	for _, step := range results.Steps {
		if step.Success {
			builder.WriteString("- " + step.AgentName + ": " + step.Response + "\n")
		}
	}
	
	return builder.String(), nil
}