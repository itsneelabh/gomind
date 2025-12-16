package anthropic

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
	// DefaultBaseURL is the default Anthropic API endpoint
	DefaultBaseURL = "https://api.anthropic.com/v1"
	// APIVersion is the required Anthropic API version header
	APIVersion = "2023-06-01"
)

// Client implements core.AIClient for Anthropic
type Client struct {
	*providers.BaseClient
	apiKey  string
	baseURL string
}

// NewClient creates a new Anthropic client with configuration
func NewClient(apiKey, baseURL string, logger core.Logger) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	base := providers.NewBaseClient(30*time.Second, logger)
	// Use "default" alias so resolveModel() is always called, enabling env var overrides
	// The actual model is resolved at request-time via modelAliases["default"]
	// or GOMIND_ANTHROPIC_MODEL_DEFAULT env var
	base.DefaultModel = "default"
	base.DefaultMaxTokens = 1000

	return &Client{
		BaseClient: base,
		apiKey:     apiKey,
		baseURL:    baseURL,
	}
}

// GenerateResponse generates a response using Anthropic's native Messages API
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	// Start distributed tracing span
	ctx, span := c.StartSpan(ctx, "ai.generate_response")
	defer span.End()

	// Set initial span attributes
	span.SetAttribute("ai.provider", "anthropic")
	span.SetAttribute("ai.prompt_length", len(prompt))

	if c.apiKey == "" {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - API key not configured", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
				"error":     "api_key_missing",
			})
		}
		span.RecordError(fmt.Errorf("API key not configured"))
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	// Apply defaults
	options = c.ApplyDefaults(options)

	// Resolve model alias (e.g., "smart" -> "claude-3-5-sonnet-20241022")
	options.Model = resolveModel(options.Model)

	// Add model to span attributes after defaults are applied
	span.SetAttribute("ai.model", options.Model)

	// Log request
	c.LogRequest("anthropic", options.Model, prompt)
	startTime := time.Now()

	// Build messages in Anthropic format
	messages := []Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Build request body using native Anthropic format
	reqBody := AnthropicRequest{
		Model:       options.Model,
		Messages:    messages,
		MaxTokens:   options.MaxTokens,
		Temperature: options.Temperature,
	}

	// Add system prompt if provided
	if options.SystemPrompt != "" {
		reqBody.System = options.SystemPrompt
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - marshal error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
				"error":     err.Error(),
				"phase":     "request_preparation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request to native Messages API endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - create request error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
				"error":     err.Error(),
				"phase":     "request_creation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Anthropic-specific headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", APIVersion)

	// Execute with retry
	resp, err := c.ExecuteWithRetry(ctx, req)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - send error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
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
			c.Logger.Error("Anthropic request failed - read response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
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
			c.Logger.Error("Anthropic request failed - API error", map[string]interface{}{
				"operation":   "ai_request_error",
				"provider":    "anthropic",
				"status_code": resp.StatusCode,
				"phase":       "api_response",
			})
		}
		apiErr := c.HandleError(resp.StatusCode, body, "Anthropic")
		span.RecordError(apiErr)
		span.SetAttribute("http.status_code", resp.StatusCode)
		return nil, apiErr
	}

	// Parse response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - parse response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
				"error":     err.Error(),
				"phase":     "response_parse",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text content from response
	var content string
	for _, item := range anthropicResp.Content {
		if item.Type == "text" {
			content += item.Text
		}
	}

	if content == "" {
		if c.Logger != nil {
			c.Logger.Error("Anthropic request failed - empty response", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "anthropic",
				"error":     "no_text_content",
				"phase":     "response_validation",
			})
		}
		emptyErr := fmt.Errorf("no text content in Anthropic response")
		span.RecordError(emptyErr)
		return nil, emptyErr
	}

	result := &core.AIResponse{
		Content: content,
		Model:   anthropicResp.Model,
		Usage: core.TokenUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}

	// Add token usage to span for cost tracking and debugging
	span.SetAttribute("ai.prompt_tokens", result.Usage.PromptTokens)
	span.SetAttribute("ai.completion_tokens", result.Usage.CompletionTokens)
	span.SetAttribute("ai.total_tokens", result.Usage.TotalTokens)
	span.SetAttribute("ai.response_length", len(result.Content))

	// Log response
	c.LogResponse("anthropic", result.Model, result.Usage, time.Since(startTime))
	c.LogResponseContent("anthropic", result.Model, result.Content)

	return result, nil
}
