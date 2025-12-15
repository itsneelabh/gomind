package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// ChainClient implements automatic failover across multiple providers (Phase 3)
// FOLLOWS FRAMEWORK PRINCIPLE: Fail-Fast for Configuration Errors, Resilient Runtime Behavior
type ChainClient struct {
	providers []core.AIClient
	logger    core.Logger
}

// NewChainClient creates a client that automatically fails over between providers
// This implements the "backup provider support" feature that enables automatic failover.
//
// FOLLOWS FRAMEWORK PRINCIPLES:
// - Fail-Fast Configuration: Invalid provider aliases fail immediately at creation time
// - Resilient Runtime: Missing API keys are warnings, not errors (allows partial chains)
// - Circuit Breaker Integration: Each provider already has circuit breaker protection
func NewChainClient(opts ...ChainOption) (*ChainClient, error) {
	config := &ChainConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// FAIL-FAST: Configuration problems should fail immediately
	if len(config.ProviderAliases) == 0 {
		return nil, fmt.Errorf("configuration error: at least one provider required for chain")
	}

	// Validate provider aliases at configuration time
	validProviders := []string{
		"openai", "anthropic", "gemini", // Base providers
		"openai.deepseek", "openai.groq", "openai.xai", // OpenAI-compatible
		"openai.together", "openai.qwen", "openai.ollama",
	}

	for _, alias := range config.ProviderAliases {
		valid := false
		for _, v := range validProviders {
			if alias == v || strings.HasPrefix(alias, v+".") {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("configuration error: unknown provider alias %q", alias)
		}
	}

	// Wrap logger with component for consistent attribution
	logger := config.Logger
	if logger == nil {
		logger = &core.NoOpLogger{}
	} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
		logger = cal.WithComponent("framework/ai")
	}

	client := &ChainClient{
		providers: make([]core.AIClient, 0, len(config.ProviderAliases)),
		logger:    logger,
	}

	// Create a client for each provider alias
	// RESILIENT: Runtime provider creation failures are handled gracefully
	successCount := 0
	for _, alias := range config.ProviderAliases {
		provider, err := NewClient(
			WithProviderAlias(alias),
			WithLogger(config.Logger),
		)
		if err != nil {
			// Runtime failures (e.g., missing API keys) are warnings, not errors
			// This allows partial chain creation when some providers aren't configured
			logger.Warn("Provider not available (will skip in chain)", map[string]interface{}{
				"operation": "ai_chain_init",
				"alias":     alias,
				"error":     err.Error(),
				"note":      "This provider will be skipped during failover",
			})
			continue // Skip unavailable providers gracefully
		}
		client.providers = append(client.providers, provider)
		successCount++
	}

	// FAIL-FAST: If NO providers could be created, that's a configuration error
	if successCount == 0 {
		return nil, fmt.Errorf("configuration error: no providers could be initialized (check API keys)")
	}

	logger.Info("Chain client initialized", map[string]interface{}{
		"operation":           "ai_chain_init",
		"requested_providers": len(config.ProviderAliases),
		"available_providers": successCount,
	})

	return client, nil
}

// GenerateResponse tries each provider until one succeeds
// FOLLOWS FRAMEWORK PRINCIPLE: Circuit Breaker Integration for external API calls
//
// Behavior:
// - Tries each provider in order until one succeeds
// - Fails fast on client errors (4xx) - these are not retryable
// - Continues on server errors (5xx) - these might work on different provider
// - Returns first successful response
// - Logs failover attempts for monitoring
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	var lastErr error

	for i, provider := range c.providers {
		if c.logger != nil {
			c.logger.Debug("Trying provider", map[string]interface{}{
				"index":    i,
				"provider": fmt.Sprintf("%T", provider),
			})
		}

		// Each provider already has circuit breaker protection internally
		// This follows the framework principle: "External API calls must be protected by circuit breakers"
		resp, err := provider.GenerateResponse(ctx, prompt, options)
		if err == nil {
			if i > 0 {
				// Record successful failover metric
				telemetry.Counter("ai.chain.failover",
					"module", telemetry.ModuleAI,
					"failed_count", fmt.Sprintf("%d", i),
				)

				if c.logger != nil {
					c.logger.Info("Failover succeeded", map[string]interface{}{
						"failed_attempts":     i,
						"successful_provider": i,
					})
				}
			}
			return resp, nil
		}

		lastErr = err

		// Determine if error is retryable (follows framework's resilient runtime behavior)
		// Client errors (4xx) are not retryable, server errors (5xx) are
		if isClientError(err) {
			// Fail fast on client errors - don't try other providers
			return nil, fmt.Errorf("client error (not retrying): %w", err)
		}

		if c.logger != nil {
			c.logger.Warn("Provider failed, trying next", map[string]interface{}{
				"index":     i,
				"error":     err.Error(),
				"remaining": len(c.providers) - i - 1,
			})
		}
	}

	// Record chain exhausted metric - all providers failed
	telemetry.Counter("ai.chain.exhausted",
		"module", telemetry.ModuleAI,
		"providers_tried", fmt.Sprintf("%d", len(c.providers)),
	)

	if c.logger != nil {
		c.logger.Error("All chain providers exhausted", map[string]interface{}{
			"operation":       "ai_chain_exhausted",
			"providers_tried": len(c.providers),
			"final_error":     lastErr.Error(),
		})
	}

	return nil, fmt.Errorf("all %d providers failed, last error: %w", len(c.providers), lastErr)
}

// isClientError checks if the error is a client error (4xx)
// Client errors indicate problems with the request (bad parameters, authentication, etc.)
// and should not be retried on different providers.
func isClientError(err error) bool {
	// Check for common client error patterns
	errStr := err.Error()

	// Common patterns that indicate client errors (4xx)
	clientErrorPatterns := []string{
		"authentication",
		"unauthorized",
		"invalid",
		"bad request",
		"not found",
		"forbidden",
		"api key",
	}

	errLower := strings.ToLower(errStr)
	for _, pattern := range clientErrorPatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}

	// Default to false (retry on other providers)
	// This is conservative - better to retry than fail fast
	return false
}

// ChainConfig holds configuration for chain client
type ChainConfig struct {
	ProviderAliases []string
	Logger          core.Logger
}

// ChainOption configures a chain client
type ChainOption func(*ChainConfig)

// WithProviderChain sets the provider aliases to try in order
// Example: WithProviderChain("openai", "openai.deepseek", "anthropic")
// The client will try OpenAI first, then DeepSeek, then Anthropic until one succeeds.
func WithProviderChain(aliases ...string) ChainOption {
	return func(c *ChainConfig) {
		c.ProviderAliases = aliases
	}
}

// WithChainLogger sets the logger for the chain client
// This enables tracking of failover attempts and provider selection.
func WithChainLogger(logger core.Logger) ChainOption {
	return func(c *ChainConfig) {
		c.Logger = logger
	}
}
