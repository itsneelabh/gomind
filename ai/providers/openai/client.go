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
	apiKey  string
	baseURL string
}

// NewClient creates a new OpenAI client with configuration
func NewClient(apiKey, baseURL string, logger core.Logger) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	base := providers.NewBaseClient(30*time.Second, logger)
	base.DefaultModel = "gpt-3.5-turbo"
	
	return &Client{
		BaseClient: base,
		apiKey:     apiKey,
		baseURL:    baseURL,
	}
}

// GenerateResponse generates a response using OpenAI
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}
	
	// Apply defaults
	options = c.ApplyDefaults(options)
	
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Execute with retry
	resp, err := c.ExecuteWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.HandleError(resp.StatusCode, body, "OpenAI")
	}
	
	// Parse response
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
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
	
	// Log response
	c.LogResponse("openai", result.Model, result.Usage, time.Since(startTime))
	
	return result, nil
}