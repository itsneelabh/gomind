package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// OpenAIClient implements core.AIClient for OpenAI
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     core.Logger
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string, logger core.Logger) *OpenAIClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	if logger == nil {
		logger = &core.NoOpLogger{}
	}
	
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GenerateResponse generates a response using OpenAI
func (c *OpenAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}
	
	// Default options
	if options == nil {
		options = &core.AIOptions{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		}
	}
	
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
	
	// Build request
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
	
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}
	
	return &core.AIResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Model:   openAIResp.Model,
		Usage: core.TokenUsage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, nil
}