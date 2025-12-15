package ai

import (
	"os"
	"strings"
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

	// ProviderAlias for OpenAI-compatible services (Phase 2)
	// Examples: "openai.deepseek", "openai.groq", "openai.together"
	// This enables multiple OpenAI-compatible providers to coexist without conflicts
	ProviderAlias string

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

	Logger    core.Logger
	Telemetry core.Telemetry

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

// WithTelemetry sets the telemetry provider for distributed tracing
// This enables spans to be created for AI operations, providing visibility
// in distributed tracing systems like Jaeger.
func WithTelemetry(telemetry core.Telemetry) AIOption {
	return func(c *AIConfig) {
		c.Telemetry = telemetry
	}
}

// WithProviderAlias sets the provider alias for OpenAI-compatible services (Phase 2)
// Examples: "openai.deepseek", "openai.groq", "openai.together"
// FOLLOWS FRAMEWORK PRINCIPLE: Intelligent Configuration Over Convention
//
// This function implements smart auto-configuration when user intent is clear:
// - Parses alias to extract base provider ("openai" from "openai.deepseek")
// - Auto-configures API keys and base URLs from environment variables
// - Only auto-configures if user hasn't explicitly set these values
//
// The auto-configuration follows the three-tier hierarchy:
// 1. Explicit config (if user set APIKey/BaseURL) - highest priority
// 2. Environment variables (provider-specific like DEEPSEEK_API_KEY)
// 3. Hardcoded defaults (well-known provider URLs)
func WithProviderAlias(alias string) AIOption {
	return func(c *AIConfig) {
		c.ProviderAlias = alias

		// Parse the alias to set the base provider
		// "openai.deepseek" → provider="openai", subprovider="deepseek"
		parts := strings.Split(alias, ".")
		if len(parts) > 0 {
			c.Provider = parts[0] // Set base provider from alias

			// INTELLIGENT AUTO-CONFIGURATION: When intent is clear, auto-configure related settings
			// This follows the framework's "Intelligent Configuration" principle
			// Only auto-configure if user hasn't explicitly set these (respects explicit override)
			if len(parts) > 1 && c.APIKey == "" && c.BaseURL == "" {
				subprovider := parts[1]

				// Auto-configure based on the subprovider
				switch subprovider {
				case "deepseek":
					c.APIKey = os.Getenv("DEEPSEEK_API_KEY")
					c.BaseURL = firstNonEmpty(
						os.Getenv("DEEPSEEK_BASE_URL"),
						"https://api.deepseek.com",
					)

				case "groq":
					c.APIKey = os.Getenv("GROQ_API_KEY")
					c.BaseURL = firstNonEmpty(
						os.Getenv("GROQ_BASE_URL"),
						"https://api.groq.com/openai/v1",
					)

				case "xai":
					c.APIKey = os.Getenv("XAI_API_KEY")
					c.BaseURL = firstNonEmpty(
						os.Getenv("XAI_BASE_URL"),
						"https://api.x.ai/v1",
					)

				case "qwen":
					c.APIKey = os.Getenv("QWEN_API_KEY")
					c.BaseURL = firstNonEmpty(
						os.Getenv("QWEN_BASE_URL"),
						"https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
					)

				case "together":
					c.APIKey = os.Getenv("TOGETHER_API_KEY")
					c.BaseURL = firstNonEmpty(
						os.Getenv("TOGETHER_BASE_URL"),
						"https://api.together.xyz/v1",
					)

				case "ollama":
					// Ollama doesn't need API key
					c.BaseURL = firstNonEmpty(
						os.Getenv("OLLAMA_BASE_URL"),
						"http://localhost:11434/v1",
					)

				// Add more providers as needed
				}
			}
		}
	}
}

// firstNonEmpty returns the first non-empty string from the provided values
// This helper implements the configuration precedence pattern used throughout the framework
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
