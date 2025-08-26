package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient implements AIClient for OpenAI API
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     interface{} // Accept any logger interface
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string, logger interface{}) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

func (c *OpenAIClient) GenerateResponse(ctx context.Context, prompt string, options *GenerationOptions) (*AIResponse, error) {
	// Set defaults
	if options == nil {
		options = &GenerationOptions{
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   1000,
		}
	}

	// Build OpenAI API request
	messages := []map[string]string{
		{"role": "user", "content": prompt},
	}

	if options.SystemPrompt != "" {
		messages = append([]map[string]string{
			{"role": "system", "content": options.SystemPrompt},
		}, messages...)
	}

	payload := map[string]interface{}{
		"model":       options.Model,
		"messages":    messages,
		"temperature": options.Temperature,
		"max_tokens":  options.MaxTokens,
	}

	// Make API request
	response, err := c.makeRequest(ctx, "/chat/completions", payload)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %w", err)
	}

	// Parse response
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("invalid response structure: no choices")
	}

	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	usage := response["usage"].(map[string]interface{})

	aiResponse := &AIResponse{
		Model:   options.Model,
		Content: message["content"].(string),
		Usage: TokenUsage{
			PromptTokens:     int(usage["prompt_tokens"].(float64)),
			CompletionTokens: int(usage["completion_tokens"].(float64)),
			TotalTokens:      int(usage["total_tokens"].(float64)),
		},
		FinishReason: choice["finish_reason"].(string),
		Confidence:   0.95, // Default confidence for OpenAI
	}

	return aiResponse, nil
}

func (c *OpenAIClient) StreamResponse(ctx context.Context, prompt string, options *GenerationOptions) (<-chan AIStreamChunk, error) {
	chunks := make(chan AIStreamChunk)

	go func() {
		defer close(chunks)

		// For now, simulate streaming by splitting the response
		// In a full implementation, this would use OpenAI's streaming API
		response, err := c.GenerateResponse(ctx, prompt, options)
		if err != nil {
			chunks <- AIStreamChunk{Error: err, ChunkType: "error"}
			return
		}

		// Split response into chunks for streaming simulation
		words := strings.Split(response.Content, " ")
		for i, word := range words {
			select {
			case <-ctx.Done():
				return
			case chunks <- AIStreamChunk{
				Content:    word + " ",
				ChunkType:  "content",
				IsComplete: i == len(words)-1,
			}:
				time.Sleep(50 * time.Millisecond) // Simulate streaming delay
			}
		}

		chunks <- AIStreamChunk{IsComplete: true, ChunkType: "metadata"}
	}()

	return chunks, nil
}

func (c *OpenAIClient) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Name:         "OpenAI",
		Models:       []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		Capabilities: []string{"text-generation", "conversation", "streaming"},
		Version:      "v1",
	}
}

func (c *OpenAIClient) makeRequest(ctx context.Context, endpoint string, payload map[string]interface{}) (map[string]interface{}, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}
