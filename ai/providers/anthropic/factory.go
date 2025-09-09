package anthropic

import (
	"os"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

func init() {
	ai.MustRegister(&Factory{})
}

// Factory creates Anthropic AI clients
type Factory struct{}

// Name returns the provider name
func (f *Factory) Name() string {
	return "anthropic"
}

// Description returns provider description
func (f *Factory) Description() string {
	return "Anthropic Claude models with native Messages API"
}

// Priority returns provider priority
func (f *Factory) Priority() int {
	return 80 // Lower than OpenAI but higher than local providers
}

// Create creates a new Anthropic client
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
	// Get API key from config or environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// Use base URL from config or environment, with default
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROPIC_BASE_URL")
		if baseURL == "" {
			baseURL = DefaultBaseURL
		}
	}

	// Create logger (nil will use NoOpLogger)
	var logger core.Logger

	// Create the client with full configuration
	client := NewClient(apiKey, baseURL, logger)

	// Apply timeout if specified
	if config.Timeout > 0 {
		client.BaseClient.HTTPClient.Timeout = config.Timeout
	}

	// Apply retry configuration
	if config.MaxRetries > 0 {
		client.BaseClient.MaxRetries = config.MaxRetries
	}

	// Apply model defaults
	if config.Model != "" {
		client.BaseClient.DefaultModel = config.Model
	}

	// Apply temperature default
	if config.Temperature > 0 {
		client.BaseClient.DefaultTemperature = config.Temperature
	}

	// Apply max tokens default
	if config.MaxTokens > 0 {
		client.BaseClient.DefaultMaxTokens = config.MaxTokens
	}

	return client
}

// DetectEnvironment checks if Anthropic is configured and returns priority
func (f *Factory) DetectEnvironment() (priority int, available bool) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return f.Priority(), true
	}
	return 0, false
}
