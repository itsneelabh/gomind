package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// ChainClient implements automatic failover across multiple providers (Phase 3)
// FOLLOWS FRAMEWORK PRINCIPLE: Fail-Fast for Configuration Errors, Resilient Runtime Behavior
type ChainClient struct {
	providers       []core.AIClient
	providerAliases []string // Provider aliases for logging (e.g., "openai", "anthropic")
	logger          core.Logger
	telemetry       core.Telemetry
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
		providers:       make([]core.AIClient, 0, len(config.ProviderAliases)),
		providerAliases: make([]string, 0, len(config.ProviderAliases)),
		logger:          logger,
		telemetry:       config.Telemetry,
	}

	// Create a client for each provider alias
	// RESILIENT: Runtime provider creation failures are handled gracefully
	successCount := 0
	for _, alias := range config.ProviderAliases {
		provider, err := NewClient(
			WithProviderAlias(alias),
			WithLogger(config.Logger),
			WithTelemetry(config.Telemetry),
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
		client.providerAliases = append(client.providerAliases, alias)
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

// SetLogger updates the logger after client creation.
// This is called by Framework.applyConfigToComponent() to propagate
// the real logger to the ChainClient after framework initialization.
//
// This fixes the critical bug where ChainClient captures NoOpLogger during
// agent construction (before Framework sets the real logger).
//
// See: ai/notes/LOGGING_TELEMETRY_AUDIT.md - "CRITICAL BUG: AI Module Logger Not Propagated"
func (c *ChainClient) SetLogger(logger core.Logger) {
	if logger == nil {
		c.logger = &core.NoOpLogger{}
	} else if cal, ok := logger.(core.ComponentAwareLogger); ok {
		c.logger = cal.WithComponent("framework/ai")
	} else {
		c.logger = logger
	}

	// Propagate to all underlying providers
	for _, provider := range c.providers {
		if loggable, ok := provider.(interface{ SetLogger(core.Logger) }); ok {
			loggable.SetLogger(logger)
		}
	}
}

// GenerateResponse tries each provider until one succeeds
// FOLLOWS FRAMEWORK PRINCIPLE: Circuit Breaker Integration for external API calls
//
// Behavior:
// - Clones options for each provider to avoid mutation bleeding across providers
// - Preserves original model setting so each provider can apply its own defaults/resolution
// - Tries each provider in order until one succeeds
// - Fails fast on client errors (4xx) - these are not retryable
// - Continues on server errors (5xx) - these might work on different provider
// - Returns first successful response
// - Comprehensive telemetry and logging for failover debugging
//
// See: ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md for the options mutation bug fix
func (c *ChainClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	startTime := time.Now()

	// Start parent span for the entire chain operation
	var span core.Span = &core.NoOpSpan{}
	if c.telemetry != nil {
		ctx, span = c.telemetry.StartSpan(ctx, "ai.chain.generate_response")
	}
	defer span.End()

	// Preserve original model setting (empty or alias like "smart")
	// This is CRITICAL: providers mutate options.Model in ApplyDefaults()
	// Without this, the first provider's model bleeds into subsequent providers
	originalModel := ""
	if options != nil {
		originalModel = options.Model
	}

	// Set span attributes for the chain operation
	span.SetAttribute("ai.chain.providers_count", len(c.providers))
	span.SetAttribute("ai.chain.original_model", originalModel)
	span.SetAttribute("ai.prompt_length", len(prompt))

	// Log chain request start with trace correlation
	if c.logger != nil {
		c.logger.InfoWithContext(ctx, "Chain client request started", map[string]interface{}{
			"operation":        "ai_chain_request",
			"original_model":   originalModel,
			"providers_count":  len(c.providers),
			"provider_aliases": c.providerAliases,
			"prompt_length":    len(prompt),
		})
	}

	var lastErr error
	var failedProviders []string

	for i, provider := range c.providers {
		providerAlias := c.providerAliases[i]
		attemptStart := time.Now()

		// Clone options for each provider to avoid mutation bleeding across providers
		// This fixes the bug where first provider's resolved model is passed to all subsequent providers
		providerOpts := cloneAIOptions(options)

		// Reset model to original so each provider can apply its own defaults/resolution
		// - If original was empty: provider applies its default via "default" alias
		// - If original was alias ("smart"): provider resolves to its own model
		if providerOpts != nil {
			providerOpts.Model = originalModel
		}

		// Create child span for this provider attempt
		var attemptSpan core.Span = &core.NoOpSpan{}
		if c.telemetry != nil {
			_, attemptSpan = c.telemetry.StartSpan(ctx, "ai.chain.provider_attempt")
		}
		attemptSpan.SetAttribute("ai.chain.provider_index", i)
		attemptSpan.SetAttribute("ai.chain.provider_alias", providerAlias)
		attemptSpan.SetAttribute("ai.chain.original_model", originalModel)
		attemptSpan.SetAttribute("ai.chain.is_retry", i > 0)

		// Log provider attempt with trace correlation
		if c.logger != nil {
			c.logger.DebugWithContext(ctx, "Trying provider in chain", map[string]interface{}{
				"operation":       "ai_chain_attempt",
				"provider_index":  i,
				"provider_alias":  providerAlias,
				"original_model":  originalModel,
				"remaining":       len(c.providers) - i - 1,
				"failed_so_far":   failedProviders,
			})
		}

		// Each provider already has circuit breaker protection internally
		// This follows the framework principle: "External API calls must be protected by circuit breakers"
		resp, err := provider.GenerateResponse(ctx, prompt, providerOpts)
		attemptDuration := time.Since(attemptStart)

		if err == nil {
			// Success!
			attemptSpan.SetAttribute("ai.chain.attempt_status", "success")
			attemptSpan.SetAttribute("ai.chain.attempt_duration_ms", attemptDuration.Milliseconds())
			attemptSpan.End()

			// Record successful attempt metric
			telemetry.Counter("ai.chain.attempt",
				"module", telemetry.ModuleAI,
				"provider", providerAlias,
				"status", "success",
				"attempt", fmt.Sprintf("%d", i+1),
			)

			if i > 0 {
				// Record successful failover metric with details
				telemetry.Counter("ai.chain.failover",
					"module", telemetry.ModuleAI,
					"from_provider", failedProviders[len(failedProviders)-1],
					"to_provider", providerAlias,
					"failed_count", fmt.Sprintf("%d", i),
				)

				span.SetAttribute("ai.chain.failover_count", i)
				span.SetAttribute("ai.chain.successful_provider", providerAlias)

				if c.logger != nil {
					c.logger.InfoWithContext(ctx, "Chain failover succeeded", map[string]interface{}{
						"operation":           "ai_chain_failover_success",
						"failed_providers":    failedProviders,
						"successful_provider": providerAlias,
						"successful_index":    i,
						"total_duration_ms":   time.Since(startTime).Milliseconds(),
					})
				}
			} else {
				if c.logger != nil {
					c.logger.InfoWithContext(ctx, "Chain request succeeded on primary provider", map[string]interface{}{
						"operation":      "ai_chain_success",
						"provider":       providerAlias,
						"duration_ms":    attemptDuration.Milliseconds(),
						"prompt_tokens":  resp.Usage.PromptTokens,
						"output_tokens":  resp.Usage.CompletionTokens,
					})
				}
			}

			span.SetAttribute("ai.chain.status", "success")
			span.SetAttribute("ai.chain.total_duration_ms", time.Since(startTime).Milliseconds())
			return resp, nil
		}

		// Provider failed
		lastErr = err
		failedProviders = append(failedProviders, providerAlias)
		isClient := isClientError(err)

		attemptSpan.SetAttribute("ai.chain.attempt_status", "failed")
		attemptSpan.SetAttribute("ai.chain.attempt_duration_ms", attemptDuration.Milliseconds())
		attemptSpan.SetAttribute("ai.chain.error", err.Error())
		attemptSpan.SetAttribute("ai.chain.is_client_error", isClient)
		attemptSpan.RecordError(err)
		attemptSpan.End()

		// Record failed attempt metric
		telemetry.Counter("ai.chain.attempt",
			"module", telemetry.ModuleAI,
			"provider", providerAlias,
			"status", "failed",
			"attempt", fmt.Sprintf("%d", i+1),
		)

		// Determine if error is retryable (follows framework's resilient runtime behavior)
		// Client errors (4xx except auth) are not retryable, server errors (5xx) are
		if isClient {
			// Fail fast on client errors - don't try other providers
			span.SetAttribute("ai.chain.status", "client_error")
			span.SetAttribute("ai.chain.abort_reason", "non_retryable_client_error")
			span.RecordError(err)

			if c.logger != nil {
				c.logger.ErrorWithContext(ctx, "Chain aborted - client error not retryable", map[string]interface{}{
					"operation":        "ai_chain_abort",
					"provider":         providerAlias,
					"provider_index":   i,
					"error":            err.Error(),
					"failed_providers": failedProviders,
					"duration_ms":      time.Since(startTime).Milliseconds(),
				})
			}

			return nil, fmt.Errorf("client error (not retrying): %w", err)
		}

		// Log provider failure with trace correlation
		if c.logger != nil {
			c.logger.WarnWithContext(ctx, "Provider failed in chain, trying next", map[string]interface{}{
				"operation":        "ai_chain_provider_failed",
				"provider":         providerAlias,
				"provider_index":   i,
				"error":            err.Error(),
				"remaining":        len(c.providers) - i - 1,
				"duration_ms":      attemptDuration.Milliseconds(),
				"failed_providers": failedProviders,
			})
		}
	}

	// Record chain exhausted metric - all providers failed
	telemetry.Counter("ai.chain.exhausted",
		"module", telemetry.ModuleAI,
		"providers_tried", fmt.Sprintf("%d", len(c.providers)),
	)

	span.SetAttribute("ai.chain.status", "exhausted")
	span.SetAttribute("ai.chain.failed_providers", strings.Join(failedProviders, ","))
	span.SetAttribute("ai.chain.total_duration_ms", time.Since(startTime).Milliseconds())
	span.RecordError(lastErr)

	if c.logger != nil {
		c.logger.ErrorWithContext(ctx, "All chain providers exhausted", map[string]interface{}{
			"operation":        "ai_chain_exhausted",
			"providers_tried":  len(c.providers),
			"failed_providers": failedProviders,
			"final_error":      lastErr.Error(),
			"total_duration_ms": time.Since(startTime).Milliseconds(),
		})
	}

	return nil, fmt.Errorf("all %d providers failed, last error: %w", len(c.providers), lastErr)
}

// cloneAIOptions creates a shallow copy of AIOptions to prevent mutation bleeding across providers.
// This is critical for chain failover: without cloning, the first provider's ApplyDefaults()
// mutates options.Model, and all subsequent providers receive that mutated model name.
//
// IMPORTANT: This is a shallow copy. It works correctly because core.AIOptions contains only
// value types (string, int, float64) that are copied by value. If AIOptions ever gains slice,
// map, or pointer fields, this function must be updated to deep copy those fields.
//
// See: ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md - "CRITICAL BUG: Options Mutation During Failover"
func cloneAIOptions(opts *core.AIOptions) *core.AIOptions {
	if opts == nil {
		return nil
	}
	clone := *opts
	return &clone
}

// isClientError checks if the error is a non-retryable client error
// In a provider chain, we WANT to failover on authentication errors because
// each provider has its own API key. Auth errors on OpenAI should try Anthropic.
//
// Non-retryable errors (don't try other providers):
// - Bad request (malformed input)
// - Content policy violations
// - Invalid parameters
//
// Retryable errors (DO try other providers):
// - Authentication/API key errors (each provider has different key!)
// - Rate limits
// - Server errors
func isClientError(err error) bool {
	errStr := err.Error()
	errLower := strings.ToLower(errStr)

	// Authentication errors SHOULD trigger failover to next provider
	// because each provider in the chain has its own API key
	authPatterns := []string{
		"api key",
		"authentication",
		"unauthorized",
		"invalid key",
		"missing key",
		"401",
	}
	for _, pattern := range authPatterns {
		if strings.Contains(errLower, pattern) {
			return false // Allow failover to next provider
		}
	}

	// These errors are true client errors - don't retry
	// (same bad input would fail on any provider)
	clientErrorPatterns := []string{
		"bad request",
		"content policy",
		"invalid parameter",
		"malformed",
	}

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
	Telemetry       core.Telemetry
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

// WithChainTelemetry sets the telemetry provider for the chain client
// This enables distributed tracing for AI operations across all providers in the chain.
func WithChainTelemetry(telemetry core.Telemetry) ChainOption {
	return func(c *ChainConfig) {
		c.Telemetry = telemetry
	}
}
