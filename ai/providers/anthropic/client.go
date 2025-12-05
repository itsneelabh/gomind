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
	base.DefaultModel = "claude-3-5-sonnet-20240620"
	base.DefaultMaxTokens = 1000

	return &Client{
		BaseClient: base,
		apiKey:     apiKey,
		baseURL:    baseURL,
	}
}

// GenerateResponse generates a response using Anthropic's native Messages API
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("anthropic API key not configured")
	}

	// Apply defaults
	options = c.ApplyDefaults(options)

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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request to native Messages API endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Anthropic-specific headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", APIVersion)

	// Execute with retry
	resp, err := c.ExecuteWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Error can be safely ignored as we've read the body
	}()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.HandleError(resp.StatusCode, body, "Anthropic")
	}

	// Parse response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
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
		return nil, fmt.Errorf("no text content in Anthropic response")
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

	// Log response
	c.LogResponse("anthropic", result.Model, result.Usage, time.Since(startTime))
	c.LogResponseContent("anthropic", result.Model, result.Content)

	return result, nil
}
