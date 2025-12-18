package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/itsneelabh/gomind/ai/providers"
	"github.com/itsneelabh/gomind/core"
)

const (
	// DefaultBaseURL is the default Gemini API endpoint
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Client implements core.AIClient for Google Gemini
type Client struct {
	*providers.BaseClient
	apiKey  string
	baseURL string
}

// NewClient creates a new Gemini client with configuration
func NewClient(apiKey, baseURL string, logger core.Logger) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	base := providers.NewBaseClient(30*time.Second, logger)
	base.DefaultModel = "gemini-1.5-flash"
	base.DefaultMaxTokens = 1000

	return &Client{
		BaseClient: base,
		apiKey:     apiKey,
		baseURL:    baseURL,
	}
}

// GenerateResponse generates a response using Gemini's native GenerateContent API
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	// Start distributed tracing span
	ctx, span := c.StartSpan(ctx, "ai.generate_response")
	defer span.End()

	// Set initial span attributes
	span.SetAttribute("ai.provider", "gemini")
	span.SetAttribute("ai.prompt_length", len(prompt))

	if c.apiKey == "" {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - API key not configured", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     "api_key_missing",
			})
		}
		span.RecordError(fmt.Errorf("API key not configured"))
		return nil, fmt.Errorf("gemini API key not configured")
	}

	// Apply defaults
	options = c.ApplyDefaults(options)

	// Resolve model alias (e.g., "smart" -> "gemini-1.5-pro")
	options.Model = resolveModel(options.Model)

	// Add model to span attributes after defaults are applied
	span.SetAttribute("ai.model", options.Model)

	// Log request
	c.LogRequest("gemini", options.Model, prompt)
	startTime := time.Now()

	// Build contents in Gemini format
	contents := []Content{
		{
			Role: "user",
			Parts: []Part{
				{Text: prompt},
			},
		},
	}

	// Build request body using native Gemini format
	reqBody := GeminiRequest{
		Contents: contents,
		GenerationConfig: &GenerationConfig{
			Temperature:     options.Temperature,
			MaxOutputTokens: options.MaxTokens,
		},
	}

	// Add system instruction if provided
	if options.SystemPrompt != "" {
		reqBody.SystemInstruction = &SystemInstruction{
			Parts: []Part{
				{Text: options.SystemPrompt},
			},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - marshal error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     err.Error(),
				"phase":     "request_preparation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request to native GenerateContent API endpoint
	// Format: /models/{model}:generateContent?key={api_key}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, options.Model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - create request error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     err.Error(),
				"phase":     "request_creation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Execute with retry
	resp, err := c.ExecuteWithRetry(ctx, req)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - send error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     err.Error(),
				"phase":     "request_execution",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Error can be safely ignored as we've read the body
	}()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - read response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     err.Error(),
				"phase":     "response_read",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - API error", map[string]interface{}{
				"operation":   "ai_request_error",
				"provider":    "gemini",
				"status_code": resp.StatusCode,
				"phase":       "api_response",
			})
		}
		apiErr := c.HandleError(resp.StatusCode, body, "Gemini")
		span.RecordError(apiErr)
		span.SetAttribute("http.status_code", resp.StatusCode)
		return nil, apiErr
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - parse response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     err.Error(),
				"phase":     "response_parse",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text content from response
	if len(geminiResp.Candidates) == 0 {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - no candidates", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     "no_candidates_returned",
				"phase":     "response_validation",
			})
		}
		noCandidatesErr := fmt.Errorf("no candidates in Gemini response")
		span.RecordError(noCandidatesErr)
		return nil, noCandidatesErr
	}

	var content string
	candidate := geminiResp.Candidates[0]
	for _, part := range candidate.Content.Parts {
		content += part.Text
	}

	if content == "" {
		if c.Logger != nil {
			c.Logger.Error("Gemini request failed - empty response", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "gemini",
				"error":     "no_text_content",
				"phase":     "response_validation",
			})
		}
		emptyErr := fmt.Errorf("no text content in Gemini response")
		span.RecordError(emptyErr)
		return nil, emptyErr
	}

	result := &core.AIResponse{
		Content: content,
		Model:   options.Model,
		Usage: core.TokenUsage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}

	// Add token usage to span for cost tracking and debugging
	span.SetAttribute("ai.prompt_tokens", result.Usage.PromptTokens)
	span.SetAttribute("ai.completion_tokens", result.Usage.CompletionTokens)
	span.SetAttribute("ai.total_tokens", result.Usage.TotalTokens)
	span.SetAttribute("ai.response_length", len(result.Content))

	// Log response
	c.LogResponse("gemini", result.Model, result.Usage, time.Since(startTime))
	c.LogResponseContent("gemini", result.Model, result.Content)

	return result, nil
}
