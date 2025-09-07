package ai

import (
	"fmt"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// NewClient creates an AI client using registered providers
func NewClient(opts ...AIOption) (core.AIClient, error) {
	// Default configuration
	config := &AIConfig{
		Provider:    string(ProviderAuto),
		MaxRetries:  3,
		Timeout:     30 * time.Second,
		Temperature: 0.7,
		MaxTokens:   1000,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(config)
	}
	
	// Auto-detection logic
	if config.Provider == string(ProviderAuto) {
		provider, err := detectBestProvider()
		if err != nil {
			return nil, fmt.Errorf("no AI provider available: %w", err)
		}
		config.Provider = provider
	}
	
	// Get provider from registry
	factory, exists := GetProvider(config.Provider)
	if !exists {
		return nil, fmt.Errorf("provider '%s' not registered. Import _ \"github.com/itsneelabh/gomind/ai/providers/%s\"", 
			config.Provider, config.Provider)
	}
	
	// Create client using provider factory
	return factory.Create(config), nil
}

// MustNewClient creates a new AI client and panics on error
func MustNewClient(opts ...AIOption) core.AIClient {
	client, err := NewClient(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create AI client: %v", err))
	}
	return client
}