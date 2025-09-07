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
func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
	// Use API key from config or environment
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	// Use base URL from config or environment
	baseURL := config.BaseURL
	if baseURL == "" {
		// Check environment for custom endpoint
		baseURL = os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			// Default to OpenAI
			baseURL = "https://api.openai.com/v1"
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
func (f *Factory) DetectEnvironment() (priority int, available bool) {
	// Check for OpenAI API key first (highest priority)
	if os.Getenv("OPENAI_API_KEY") != "" {
		return 100, true
	}
	
	// Check for Groq (ultra-fast inference)
	if os.Getenv("GROQ_API_KEY") != "" {
		os.Setenv("OPENAI_BASE_URL", "https://api.groq.com/openai/v1")
		os.Setenv("OPENAI_API_KEY", os.Getenv("GROQ_API_KEY"))
		return 95, true
	}
	
	// Check for DeepSeek (reasoning model)
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		os.Setenv("OPENAI_BASE_URL", "https://api.deepseek.com")
		os.Setenv("OPENAI_API_KEY", os.Getenv("DEEPSEEK_API_KEY"))
		return 90, true
	}
	
	// Check for xAI Grok
	if os.Getenv("XAI_API_KEY") != "" {
		os.Setenv("OPENAI_BASE_URL", "https://api.x.ai/v1")
		os.Setenv("OPENAI_API_KEY", os.Getenv("XAI_API_KEY"))
		return 85, true
	}
	
	// Check for Qwen (Alibaba)
	if os.Getenv("QWEN_API_KEY") != "" {
		os.Setenv("OPENAI_BASE_URL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1")
		os.Setenv("OPENAI_API_KEY", os.Getenv("QWEN_API_KEY"))
		return 80, true
	}
	
	// Check for local Ollama (no API key needed)
	if isLocalServiceAvailable("http://localhost:11434/v1/models") {
		os.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1")
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