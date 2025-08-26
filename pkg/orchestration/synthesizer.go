package orchestration

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/logger"
)

// ResponseSynthesizer implements the Synthesizer interface
type ResponseSynthesizer struct {
	aiClient  ai.AIClient
	logger    logger.Logger
	strategy  SynthesisStrategy
	templates map[string]*template.Template
	
	// Custom synthesis function
	customFunc func(context.Context, string, *ExecutionResult) (string, error)
	
	// Metrics
	synthesisCount  int64
	synthesisErrors int64
}

// NewResponseSynthesizer creates a new response synthesizer
func NewResponseSynthesizer(
	aiClient ai.AIClient,
	logger logger.Logger,
	strategy SynthesisStrategy,
) *ResponseSynthesizer {
	return &ResponseSynthesizer{
		aiClient:  aiClient,
		logger:    logger,
		strategy:  strategy,
		templates: make(map[string]*template.Template),
	}
}

// Synthesize combines agent responses into a final response
func (s *ResponseSynthesizer) Synthesize(
	ctx context.Context,
	request string,
	results *ExecutionResult,
) (string, error) {
	startTime := time.Now()
	s.synthesisCount++
	
	s.logger.Debug("Synthesizing response", map[string]interface{}{
		"strategy":     s.strategy,
		"steps_count":  len(results.Steps),
		"request":      request,
	})
	
	var synthesized string
	var err error
	
	switch s.strategy {
	case StrategyLLM:
		synthesized, err = s.synthesizeWithLLM(ctx, request, results)
		
	case StrategyTemplate:
		synthesized, err = s.synthesizeWithTemplate(ctx, request, results)
		
	case StrategySimple:
		synthesized = s.synthesizeSimple(results)
		
	case StrategyCustom:
		if s.customFunc != nil {
			synthesized, err = s.customFunc(ctx, request, results)
		} else {
			err = fmt.Errorf("custom synthesis function not set")
		}
		
	default:
		err = fmt.Errorf("unknown synthesis strategy: %s", s.strategy)
	}
	
	if err != nil {
		s.synthesisErrors++
		s.logger.Error("Synthesis failed", map[string]interface{}{
			"error":    err.Error(),
			"strategy": s.strategy,
			"duration": time.Since(startTime),
		})
		return "", err
	}
	
	s.logger.Debug("Synthesis completed", map[string]interface{}{
		"strategy":       s.strategy,
		"response_length": len(synthesized),
		"duration":       time.Since(startTime),
	})
	
	return synthesized, nil
}

// synthesizeWithLLM uses an LLM to create a coherent response
func (s *ResponseSynthesizer) synthesizeWithLLM(
	ctx context.Context,
	request string,
	results *ExecutionResult,
) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI client not configured for LLM synthesis")
	}
	
	// Build the synthesis prompt
	prompt := s.buildLLMPrompt(request, results)
	
	// Call LLM to synthesize
	response, err := s.aiClient.GenerateResponse(ctx, prompt, &ai.GenerationOptions{
		Temperature: 0.3, // Lower temperature for more consistent synthesis
		MaxTokens:   1000,
		SystemPrompt: `You are a helpful assistant that synthesizes information from multiple sources into coherent, comprehensive responses. 
Your task is to combine agent responses into a single, well-structured answer that directly addresses the user's request.
Be concise but complete. Maintain a professional and helpful tone.`,
	})
	
	if err != nil {
		return "", fmt.Errorf("LLM synthesis failed: %w", err)
	}
	
	return response.Content, nil
}

// buildLLMPrompt creates the prompt for LLM synthesis
func (s *ResponseSynthesizer) buildLLMPrompt(request string, results *ExecutionResult) string {
	var prompt strings.Builder
	
	prompt.WriteString("USER REQUEST:\n")
	prompt.WriteString(request)
	prompt.WriteString("\n\n")
	
	prompt.WriteString("AGENT RESPONSES:\n\n")
	
	for _, step := range results.Steps {
		if step.Success && step.Response != "" {
			prompt.WriteString(fmt.Sprintf("Agent: %s (namespace: %s)\n", 
				step.AgentName, step.Namespace))
			prompt.WriteString(fmt.Sprintf("Task: %s\n", step.Instruction))
			prompt.WriteString(fmt.Sprintf("Response: %s\n\n", step.Response))
		} else if !step.Success {
			prompt.WriteString(fmt.Sprintf("Agent: %s (namespace: %s)\n", 
				step.AgentName, step.Namespace))
			prompt.WriteString(fmt.Sprintf("Task: %s\n", step.Instruction))
			prompt.WriteString(fmt.Sprintf("Status: FAILED - %s\n\n", step.Error))
		}
	}
	
	prompt.WriteString(`TASK:
Based on the user's request and the agent responses above, create a comprehensive and coherent response.
If some agents failed, work with the available information.
Structure your response clearly and address all aspects of the user's request.

SYNTHESIZED RESPONSE:`)
	
	return prompt.String()
}

// synthesizeWithTemplate uses predefined templates
func (s *ResponseSynthesizer) synthesizeWithTemplate(
	ctx context.Context,
	request string,
	results *ExecutionResult,
) (string, error) {
	// Select template based on request type or use default
	templateName := "default"
	
	// Check if we have a custom template for this type of request
	if strings.Contains(strings.ToLower(request), "analyze") {
		templateName = "analysis"
	} else if strings.Contains(strings.ToLower(request), "report") {
		templateName = "report"
	} else if strings.Contains(strings.ToLower(request), "summary") {
		templateName = "summary"
	}
	
	tmpl, exists := s.templates[templateName]
	if !exists {
		// Use default template
		tmpl = s.getDefaultTemplate()
	}
	
	// Prepare template data
	data := struct {
		Request string
		Results *ExecutionResult
		Steps   []StepResult
		Success bool
	}{
		Request: request,
		Results: results,
		Steps:   results.Steps,
		Success: results.Success,
	}
	
	var output strings.Builder
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}
	
	return output.String(), nil
}

// getDefaultTemplate returns the default synthesis template
func (s *ResponseSynthesizer) getDefaultTemplate() *template.Template {
	templateStr := `Based on your request: "{{.Request}}"

Here's what I found:

{{range .Steps}}{{if .Success}}## {{.AgentName}}
{{.Response}}

{{end}}{{end}}
{{if not .Success}}
Note: Some agents encountered issues during processing:
{{range .Steps}}{{if not .Success}}- {{.AgentName}}: {{.Error}}
{{end}}{{end}}
{{end}}`
	
	tmpl, _ := template.New("default").Parse(templateStr)
	return tmpl
}

// synthesizeSimple concatenates responses with basic formatting
func (s *ResponseSynthesizer) synthesizeSimple(results *ExecutionResult) string {
	var response strings.Builder
	
	response.WriteString("Here are the results from the agents:\n\n")
	
	successCount := 0
	for _, step := range results.Steps {
		if step.Success && step.Response != "" {
			response.WriteString(fmt.Sprintf("**%s** (%s):\n", 
				step.AgentName, step.Namespace))
			response.WriteString(step.Response)
			response.WriteString("\n\n")
			successCount++
		}
	}
	
	if successCount == 0 {
		response.WriteString("Unfortunately, no agents were able to provide a response.\n\n")
		
		// List failures
		for _, step := range results.Steps {
			if !step.Success {
				response.WriteString(fmt.Sprintf("- %s failed: %s\n", 
					step.AgentName, step.Error))
			}
		}
	}
	
	return response.String()
}

// SetStrategy sets the synthesis strategy
func (s *ResponseSynthesizer) SetStrategy(strategy SynthesisStrategy) {
	s.strategy = strategy
}

// SetCustomSynthesisFunc sets a custom synthesis function
func (s *ResponseSynthesizer) SetCustomSynthesisFunc(
	fn func(context.Context, string, *ExecutionResult) (string, error),
) {
	s.customFunc = fn
	s.strategy = StrategyCustom
}

// AddTemplate adds a custom template for synthesis
func (s *ResponseSynthesizer) AddTemplate(name string, templateStr string) error {
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	
	s.templates[name] = tmpl
	return nil
}

// GetMetrics returns synthesis metrics
func (s *ResponseSynthesizer) GetMetrics() map[string]int64 {
	return map[string]int64{
		"synthesis_count":  s.synthesisCount,
		"synthesis_errors": s.synthesisErrors,
	}
}

// TemplateSynthesizer provides template-based synthesis
type TemplateSynthesizer struct {
	templates map[string]*template.Template
	logger    logger.Logger
}

// NewTemplateSynthesizer creates a template-based synthesizer
func NewTemplateSynthesizer(logger logger.Logger) *TemplateSynthesizer {
	ts := &TemplateSynthesizer{
		templates: make(map[string]*template.Template),
		logger:    logger,
	}
	
	// Load default templates
	ts.loadDefaultTemplates()
	
	return ts
}

// loadDefaultTemplates loads built-in templates
func (ts *TemplateSynthesizer) loadDefaultTemplates() {
	// Analysis template
	analysisTemplate := `# Analysis Results

## Request
{{.Request}}

## Findings
{{range .Steps}}{{if .Success}}
### {{.AgentName}}
{{.Response}}
{{end}}{{end}}

## Summary
Based on the analysis from {{len .Steps}} agents, the results show that the request has been {{if .Success}}successfully{{else}}partially{{end}} processed.
`
	
	// Report template
	reportTemplate := `# Report

**Date:** {{.Timestamp}}
**Request:** {{.Request}}

## Executive Summary
{{range .Steps}}{{if .Success}}{{.Response}}{{end}}{{end}}

## Detailed Results
{{range .Steps}}
### {{.AgentName}}
- **Status:** {{if .Success}}Success{{else}}Failed{{end}}
- **Duration:** {{.Duration}}
{{if .Success}}- **Result:** {{.Response}}{{else}}- **Error:** {{.Error}}{{end}}
{{end}}
`
	
	// Summary template  
	summaryTemplate := `## Summary

Request: "{{.Request}}"

Key Points:
{{range .Steps}}{{if .Success}}
• {{.Response}}
{{end}}{{end}}

{{if not .Success}}Note: Some operations failed during processing.{{end}}
`
	
	// Parse and store templates
	ts.templates["analysis"], _ = template.New("analysis").Parse(analysisTemplate)
	ts.templates["report"], _ = template.New("report").Parse(reportTemplate)
	ts.templates["summary"], _ = template.New("summary").Parse(summaryTemplate)
}