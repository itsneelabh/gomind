package openai

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

// Client implements core.AIClient for OpenAI
type Client struct {
	*providers.BaseClient
	apiKey        string
	baseURL       string
	providerAlias string // For request-time alias resolution (e.g., "openai.deepseek")
}

// NewClient creates a new OpenAI client with configuration
func NewClient(apiKey, baseURL, providerAlias string, logger core.Logger) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	base := providers.NewBaseClient(30*time.Second, logger)
	// Use "default" alias so ResolveModel() is always called, enabling env var overrides
	// The actual model is resolved at request-time via ModelAliases["openai"]["default"]
	// or GOMIND_OPENAI_MODEL_DEFAULT env var
	base.DefaultModel = "default"

	return &Client{
		BaseClient:    base,
		apiKey:        apiKey,
		baseURL:       baseURL,
		providerAlias: providerAlias,
	}
}

// GenerateResponse generates a response using OpenAI
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	// Start distributed tracing span
	ctx, span := c.StartSpan(ctx, "ai.generate_response")
	defer span.End()

	// Set initial span attributes
	span.SetAttribute("ai.provider", "openai")
	span.SetAttribute("ai.prompt_length", len(prompt))

	if c.apiKey == "" {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - API key not configured", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
				"error":     "api_key_missing",
			})
		}
		span.RecordError(fmt.Errorf("API key not configured"))
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Apply defaults
	options = c.ApplyDefaults(options)

	// Resolve model alias at request time (e.g., "smart" -> "gpt-4")
	options.Model = ResolveModel(c.providerAlias, options.Model)

	// Add model to span attributes after defaults are applied
	span.SetAttribute("ai.model", options.Model)

	// Log request
	c.LogRequest("openai", options.Model, prompt)
	startTime := time.Now()

	// Build messages
	messages := []map[string]string{}

	if options.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": options.SystemPrompt,
		})
	}

	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	// Build request body
	reqBody := map[string]interface{}{
		"model":       options.Model,
		"messages":    messages,
		"temperature": options.Temperature,
		"max_tokens":  options.MaxTokens,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - marshal error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
				"error":     err.Error(),
				"phase":     "request_preparation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - create request error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
				"error":     err.Error(),
				"phase":     "request_creation",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Execute with retry
	resp, err := c.ExecuteWithRetry(ctx, req)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - send error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
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
			c.Logger.Error("OpenAI request failed - read response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
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
			c.Logger.Error("OpenAI request failed - API error", map[string]interface{}{
				"operation":   "ai_request_error",
				"provider":    "openai",
				"status_code": resp.StatusCode,
				"phase":       "api_response",
			})
		}
		apiErr := c.HandleError(resp.StatusCode, body, "OpenAI")
		span.RecordError(apiErr)
		span.SetAttribute("http.status_code", resp.StatusCode)
		return nil, apiErr
	}

	// Parse response
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - parse response error", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
				"error":     err.Error(),
				"phase":     "response_parse",
			})
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		if c.Logger != nil {
			c.Logger.Error("OpenAI request failed - empty response", map[string]interface{}{
				"operation": "ai_request_error",
				"provider":  "openai",
				"error":     "no_choices_returned",
				"phase":     "response_validation",
			})
		}
		emptyErr := fmt.Errorf("no response from OpenAI")
		span.RecordError(emptyErr)
		return nil, emptyErr
	}

	result := &core.AIResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Model:   openAIResp.Model,
		Usage: core.TokenUsage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}

	// Add token usage to span for cost tracking and debugging
	span.SetAttribute("ai.prompt_tokens", result.Usage.PromptTokens)
	span.SetAttribute("ai.completion_tokens", result.Usage.CompletionTokens)
	span.SetAttribute("ai.total_tokens", result.Usage.TotalTokens)
	span.SetAttribute("ai.response_length", len(result.Content))

	// Log response
	c.LogResponse("openai", result.Model, result.Usage, time.Since(startTime))
	c.LogResponseContent("openai", result.Model, result.Content)

	return result, nil
}
