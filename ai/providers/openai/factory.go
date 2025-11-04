package openai

import (
	"net/http"
	"os"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"
)

// Factory implements ai.ProviderFactory for OpenAI
type Factory struct{}

// Create creates a new OpenAI client instance
// UPDATED: Now uses resolveCredentials() to properly handle multiple OpenAI-compatible providers
// without mutating environment variables. This maintains backward compatibility while fixing
// the critical configuration corruption bug.
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
	// Resolve credentials using the three-tier configuration hierarchy:
	// 1. Explicit config (highest priority)
	// 2. Environment variables with provider-specific overrides
	// 3. Hardcoded defaults (lowest priority)
	apiKey, baseURL := f.resolveCredentials(config)

	logger := config.Logger
	if logger == nil {
		logger = &core.NoOpLogger{}
	}

	// Phase 2: Resolve model aliases
	// This enables portable model names like "smart" to work across providers
	if config.Model != "" {
		config.Model = ResolveModel(config.ProviderAlias, config.Model)
	}

	logger.Info("OpenAI provider initialized", map[string]interface{}{
		"operation":      "ai_provider_init",
		"provider":       "openai",
		"provider_alias": config.ProviderAlias, // Phase 2: Log which alias is used
		"base_url":       baseURL,
		"has_api_key":    apiKey != "",
		"timeout":        config.Timeout.String(),
		"max_retries":    config.MaxRetries,
		"model":          config.Model,
	})

	// Create the client with resolved configuration
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

	// Apply custom headers if any
	if len(config.Headers) > 0 {
		// Create a custom transport to add headers
		transport := &headerTransport{
			headers: config.Headers,
			base:    http.DefaultTransport,
		}
		client.BaseClient.HTTPClient.Transport = transport
	}

	return client
}

// resolveCredentials determines which OpenAI-compatible service to use and resolves credentials (Phase 2)
// This implements the "Stable Defaults, Flexible Overrides" principle from the Configuration Strategy:
// 1. Explicit configuration (highest priority) - values passed directly in config
// 2. Environment variable overrides (medium priority) - enables runtime flexibility
// 3. Hardcoded defaults (lowest priority) - provides zero-config experience
//
// Phase 2 Update: Now uses ProviderAlias for explicit provider selection.
// This enables multiple OpenAI-compatible providers to coexist without conflicts.
func (f *Factory) resolveCredentials(config *ai.AIConfig) (apiKey, baseURL string) {
	// Handle provider aliases (Phase 2)
	// If ProviderAlias is set, use it to determine credentials explicitly
	// This provides clear, conflict-free configuration for multi-provider scenarios
	switch config.ProviderAlias {
	case "openai.deepseek":
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("DEEPSEEK_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("DEEPSEEK_BASE_URL"),
			"https://api.deepseek.com",
		)
		return apiKey, baseURL

	case "openai.groq":
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("GROQ_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("GROQ_BASE_URL"),
			"https://api.groq.com/openai/v1",
		)
		return apiKey, baseURL

	case "openai.xai":
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("XAI_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("XAI_BASE_URL"),
			"https://api.x.ai/v1",
		)
		return apiKey, baseURL

	case "openai.qwen":
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("QWEN_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("QWEN_BASE_URL"),
			"https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
		)
		return apiKey, baseURL

	case "openai.together":
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("TOGETHER_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("TOGETHER_BASE_URL"),
			"https://api.together.xyz/v1",
		)
		return apiKey, baseURL

	case "openai.ollama":
		apiKey = config.APIKey // Ollama doesn't need API key
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("OLLAMA_BASE_URL"),
			"http://localhost:11434/v1",
		)
		return apiKey, baseURL

	default:
		// "openai" or empty - vanilla OpenAI or auto-detection fallback
		// If ProviderAlias is explicitly "openai" or not set, use OpenAI or auto-detect

		// Auto-detection path (backward compatibility with Phase 1)
		// This maintains zero-config experience when ProviderAlias is not set

		// Priority 100: OpenAI (vanilla)
		if os.Getenv("OPENAI_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("OPENAI_BASE_URL"),
				"https://api.openai.com/v1",
			)
			return apiKey, baseURL
		}

		// Priority 95: Groq (ultra-fast inference)
		if os.Getenv("GROQ_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("GROQ_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("GROQ_BASE_URL"),
				"https://api.groq.com/openai/v1",
			)
			return apiKey, baseURL
		}

		// Priority 90: DeepSeek (reasoning model)
		if os.Getenv("DEEPSEEK_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("DEEPSEEK_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("DEEPSEEK_BASE_URL"),
				"https://api.deepseek.com",
			)
			return apiKey, baseURL
		}

		// Priority 85: xAI Grok
		if os.Getenv("XAI_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("XAI_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("XAI_BASE_URL"),
				"https://api.x.ai/v1",
			)
			return apiKey, baseURL
		}

		// Priority 80: Qwen (Alibaba)
		if os.Getenv("QWEN_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("QWEN_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("QWEN_BASE_URL"),
				"https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
			)
			return apiKey, baseURL
		}

		// Priority 75: Together AI (for open models)
		if os.Getenv("TOGETHER_API_KEY") != "" {
			apiKey = firstNonEmpty(config.APIKey, os.Getenv("TOGETHER_API_KEY"))
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("TOGETHER_BASE_URL"),
				"https://api.together.xyz/v1",
			)
			return apiKey, baseURL
		}

		// Priority 50: Local Ollama (no API key needed)
		if isLocalServiceAvailable("http://localhost:11434/v1/models") {
			apiKey = config.APIKey // Ollama doesn't require API key
			baseURL = firstNonEmpty(
				config.BaseURL,
				os.Getenv("OLLAMA_BASE_URL"),
				"http://localhost:11434/v1",
			)
			return apiKey, baseURL
		}

		// Fallback: Use whatever was provided in config, or OpenAI defaults
		apiKey = firstNonEmpty(config.APIKey, os.Getenv("OPENAI_API_KEY"))
		baseURL = firstNonEmpty(
			config.BaseURL,
			os.Getenv("OPENAI_BASE_URL"),
			"https://api.openai.com/v1",
		)
		return apiKey, baseURL
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

// headerTransport adds custom headers to requests
type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add custom headers
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// DetectEnvironment checks if OpenAI-compatible services can be used
// CRITICAL FIX: This method now only READS environment, never mutates it
// This prevents race conditions and configuration corruption in production
func (f *Factory) DetectEnvironment() (priority int, available bool) {
	// Check for OpenAI API key first (highest priority)
	if os.Getenv("OPENAI_API_KEY") != "" {
		return 100, true
	}

	// Check for Groq (ultra-fast inference)
	if os.Getenv("GROQ_API_KEY") != "" {
		return 95, true
	}

	// Check for DeepSeek (reasoning model)
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return 90, true
	}

	// Check for xAI Grok
	if os.Getenv("XAI_API_KEY") != "" {
		return 85, true
	}

	// Check for Qwen (Alibaba)
	if os.Getenv("QWEN_API_KEY") != "" {
		return 80, true
	}

	// Check for Together AI (open models)
	if os.Getenv("TOGETHER_API_KEY") != "" {
		return 75, true
	}

	// Check for local Ollama (no API key needed)
	if isLocalServiceAvailable("http://localhost:11434/v1/models") {
		return 50, true
	}

	return 0, false
}

// isLocalServiceAvailable checks if a local service is running
func isLocalServiceAvailable(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Name returns the provider name
func (f *Factory) Name() string {
	return "openai"
}

// Description returns a human-readable description
func (f *Factory) Description() string {
	return "Universal OpenAI-compatible provider (OpenAI, Groq, DeepSeek, Qwen, local models, etc.)"
}

// Register registers this provider with the global registry
// This is called automatically when the package is imported
func init() {
	ai.MustRegister(&Factory{})
}
