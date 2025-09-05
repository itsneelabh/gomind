package ai

import (
	"context"
	"github.com/itsneelabh/gomind/core"
)

// AIClient is the interface for AI/LLM clients
// This re-exports the core interface for convenience
type AIClient interface {
	GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error)
}

// Ensure OpenAIClient implements AIClient
var _ AIClient = (*OpenAIClient)(nil)
