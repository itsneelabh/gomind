package ai

import (
	"time"

	"github.com/itsneelabh/gomind/core"
)

// Provider represents an AI provider type
type Provider string

// Standard provider constants
const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderOllama    Provider = "ollama"
	ProviderAuto      Provider = "auto"   // Auto-detect from environment
	ProviderCustom    Provider = "custom" // For custom providers
)

// AIConfig holds configuration for AI client creation
type AIConfig struct {
	// Provider to use
	Provider string

	// API credentials
	APIKey  string
	BaseURL string

	// Connection settings
	Timeout    time.Duration
	MaxRetries int

	// Model configuration
	Model       string
	Temperature float32
	MaxTokens   int

	Logger core.Logger

	// Advanced options
	Headers map[string]string
	Extra   map[string]interface{}
}

// AIOption configures an AI client
type AIOption func(*AIConfig)

// WithProvider sets the AI provider
func WithProvider(provider string) AIOption {
	return func(c *AIConfig) {
		c.Provider = provider
	}
}

// WithAPIKey sets the API key
func WithAPIKey(key string) AIOption {
	return func(c *AIConfig) {
		c.APIKey = key
	}
}

// WithBaseURL sets the base URL for the API
func WithBaseURL(url string) AIOption {
	return func(c *AIConfig) {
		c.BaseURL = url
	}
}

// WithRegion sets the AWS region for AWS Bedrock provider
func WithRegion(region string) AIOption {
	return func(c *AIConfig) {
		if c.Extra == nil {
			c.Extra = make(map[string]interface{})
		}
		c.Extra["region"] = region
	}
}

// WithAWSCredentials sets explicit AWS credentials for Bedrock provider
func WithAWSCredentials(accessKey, secretKey, sessionToken string) AIOption {
	return func(c *AIConfig) {
		if c.Extra == nil {
			c.Extra = make(map[string]interface{})
		}
		c.Extra["aws_access_key_id"] = accessKey
		c.Extra["aws_secret_access_key"] = secretKey
		if sessionToken != "" {
			c.Extra["aws_session_token"] = sessionToken
		}
	}
}

// WithTimeout sets the request timeout
func WithTimeout(timeout time.Duration) AIOption {
	return func(c *AIConfig) {
		c.Timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(retries int) AIOption {
	return func(c *AIConfig) {
		c.MaxRetries = retries
	}
}

// WithModel sets the model to use
func WithModel(model string) AIOption {
	return func(c *AIConfig) {
		c.Model = model
	}
}

// WithTemperature sets the temperature for generation
func WithTemperature(temp float32) AIOption {
	return func(c *AIConfig) {
		c.Temperature = temp
	}
}

// WithMaxTokens sets the maximum tokens for generation
func WithMaxTokens(tokens int) AIOption {
	return func(c *AIConfig) {
		c.MaxTokens = tokens
	}
}

// WithHeaders sets custom headers
func WithHeaders(headers map[string]string) AIOption {
	return func(c *AIConfig) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		for k, v := range headers {
			c.Headers[k] = v
		}
	}
}

// WithExtra sets extra configuration options
func WithExtra(key string, value interface{}) AIOption {
	return func(c *AIConfig) {
		if c.Extra == nil {
			c.Extra = make(map[string]interface{})
		}
		c.Extra[key] = value
	}
}

// WithLogger sets the logger for AI operations
// This is typically called by the framework to provide observability
func WithLogger(logger core.Logger) AIOption {
	return func(c *AIConfig) {
		c.Logger = logger
	}
}
