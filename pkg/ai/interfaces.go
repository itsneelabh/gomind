package ai

import (
	"context"
)

// AIClient provides a unified interface for different AI providers
type AIClient interface {
	GenerateResponse(ctx context.Context, prompt string, options *GenerationOptions) (*AIResponse, error)
	StreamResponse(ctx context.Context, prompt string, options *GenerationOptions) (<-chan AIStreamChunk, error)
	GetProviderInfo() ProviderInfo
}

// GenerationOptions configures AI generation parameters
type GenerationOptions struct {
	Model          string            `json:"model,omitempty"`
	Temperature    float64           `json:"temperature,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	SystemPrompt   string            `json:"system_prompt,omitempty"`
	ConversationID string            `json:"conversation_id,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// AIResponse represents a complete AI model response
type AIResponse struct {
	Content      string            `json:"content"`
	Model        string            `json:"model"`
	Usage        TokenUsage        `json:"usage"`
	FinishReason string            `json:"finish_reason"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Confidence   float64           `json:"confidence,omitempty"`
}

// AIStreamChunk represents a streaming response chunk
type AIStreamChunk struct {
	Content    string            `json:"content"`
	IsComplete bool              `json:"is_complete"`
	ChunkType  string            `json:"chunk_type"` // "content", "metadata", "error"
	Error      error             `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TokenUsage tracks API usage
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ProviderInfo contains AI provider details
type ProviderInfo struct {
	Name         string   `json:"name"`
	Models       []string `json:"models"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}
