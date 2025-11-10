package gemini

import (
	"os"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

func init() {
	ai.MustRegister(&Factory{})
}

// Factory creates Gemini AI clients
type Factory struct{}

// Name returns the provider name
func (f *Factory) Name() string {
	return "gemini"
}

// Description returns provider description
func (f *Factory) Description() string {
	return "Google Gemini models with native GenerateContent API"
}

// Priority returns provider priority
func (f *Factory) Priority() int {
	return 70 // Lower than Anthropic but higher than local providers
}

// Create creates a new Gemini client
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
	// Get API key from config or environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			// Also check for GOOGLE_API_KEY as an alternative
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
	}

	// Use base URL from config or environment, with default
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("GEMINI_BASE_URL")
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
		client.HTTPClient.Timeout = config.Timeout
	}

	// Apply retry configuration
	if config.MaxRetries > 0 {
		client.MaxRetries = config.MaxRetries
	}

	// Apply model defaults
	if config.Model != "" {
		client.DefaultModel = config.Model
	}

	// Apply temperature default
	if config.Temperature > 0 {
		client.DefaultTemperature = config.Temperature
	}

	// Apply max tokens default
	if config.MaxTokens > 0 {
		client.DefaultMaxTokens = config.MaxTokens
	}

	return client
}

// DetectEnvironment checks if Gemini is configured and returns priority
func (f *Factory) DetectEnvironment() (priority int, available bool) {
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		return f.Priority(), true
	}
	return 0, false
}
