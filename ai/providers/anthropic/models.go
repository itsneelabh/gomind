package anthropic

// AnthropicRequest represents the native Anthropic Messages API request
type AnthropicRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float32   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
	TopP        float32   `json:"top_p,omitempty"`
	TopK        int       `json:"top_k,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// AnthropicResponse represents the response from Anthropic API
type AnthropicResponse struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Role         string        `json:"role"`
	Content      []ContentItem `json:"content"`
	Model        string        `json:"model"`
	StopReason   string        `json:"stop_reason"`
	StopSequence *string       `json:"stop_sequence"`
	Usage        Usage         `json:"usage"`
}

// ContentItem represents a content block in the response
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ErrorResponse represents an error from Anthropic API
type ErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
