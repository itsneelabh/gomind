// Package mock provides a mock AI provider for testing
package mock

import (
	"context"
	"errors"
	"fmt"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

func init() {
	// Register only if explicitly enabled via environment or test
	// This prevents mock from being auto-detected in production
	if err := ai.Register(&Factory{}); err != nil {
		// Panic in init() is acceptable for registration errors (caught in tests/development)
		panic(fmt.Sprintf("failed to register mock AI provider: %v", err))
	}
}

// Factory creates mock AI clients for testing
type Factory struct{}

// Name returns the provider name
func (f *Factory) Name() string {
	return "mock"
}

// Description returns provider description
func (f *Factory) Description() string {
	return "Mock provider for testing"
}

// Priority returns provider priority
func (f *Factory) Priority() int {
	return 1 // Very low priority
}

// Create creates a new mock client
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
	return NewClient(config)
}

// DetectEnvironment checks if mock is enabled
func (f *Factory) DetectEnvironment() (priority int, available bool) {
	// Mock is never auto-detected
	return 0, false
}

// Client implements core.AIClient for testing
type Client struct {
	Config        *ai.AIConfig
	Responses     []string
	ResponseIndex int
	Error         error
	CallCount     int
	LastPrompt    string
	LastOptions   *core.AIOptions
}

// NewClient creates a new mock client
func NewClient(config *ai.AIConfig) *Client {
	return &Client{
		Config:    config,
		Responses: []string{"Mock response"},
	}
}

// GenerateResponse returns a mock response
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	c.CallCount++
	c.LastPrompt = prompt
	c.LastOptions = options

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return configured error if set
	if c.Error != nil {
		return nil, c.Error
	}

	// Return next response from list
	if c.ResponseIndex >= len(c.Responses) {
		return nil, errors.New("no more mock responses")
	}

	response := c.Responses[c.ResponseIndex]
	c.ResponseIndex++

	// Use options if provided, otherwise use defaults
	model := "mock-model"
	if options != nil && options.Model != "" {
		model = options.Model
	} else if c.Config != nil && c.Config.Model != "" {
		model = c.Config.Model
	}

	return &core.AIResponse{
		Content: response,
		Model:   model,
		Usage: core.TokenUsage{
			PromptTokens:     len(prompt) / 4, // Rough estimate
			CompletionTokens: len(response) / 4,
			TotalTokens:      (len(prompt) + len(response)) / 4,
		},
	}, nil
}

// SetResponses sets the responses to return
func (c *Client) SetResponses(responses ...string) {
	c.Responses = responses
	c.ResponseIndex = 0
}

// SetError sets an error to return
func (c *Client) SetError(err error) {
	c.Error = err
}

// Reset resets the mock client
func (c *Client) Reset() {
	c.ResponseIndex = 0
	c.CallCount = 0
	c.LastPrompt = ""
	c.LastOptions = nil
	c.Error = nil
}
