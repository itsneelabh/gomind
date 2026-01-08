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
		Logger:      nil, // Will be set by framework or options
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Apply component-specific logging for AI module
	if config.Logger != nil {
		if cal, ok := config.Logger.(core.ComponentAwareLogger); ok {
			config.Logger = cal.WithComponent("framework/ai")
		}
		config.Logger.Info("Starting AI client creation", map[string]interface{}{
			"operation":        "ai_client_creation",
			"provider_setting": config.Provider,
			"auto_detect":      config.Provider == string(ProviderAuto),
		})
	}

	// Auto-detection logic with enhanced logging
	if config.Provider == string(ProviderAuto) {
		provider, err := detectBestProvider(config.Logger)
		if err != nil {
			if config.Logger != nil {
				config.Logger.Error("AI provider auto-detection failed", map[string]interface{}{
					"operation":           "ai_provider_detection",
					"error":               err.Error(),
					"available_providers": ListProviders(),
					"suggestion":          "Set explicit provider or configure API keys",
				})
			}
			return nil, fmt.Errorf("no AI provider available: %w", err)
		}
		config.Provider = provider

		if config.Logger != nil {
			config.Logger.Info("AI provider auto-detected", map[string]interface{}{
				"operation":         "ai_provider_detection",
				"selected_provider": provider,
				"detection_method":  "environment_scan",
				"status":            "success",
			})
		}
	}

	factory, exists := GetProvider(config.Provider)
	if !exists {
		if config.Logger != nil {
			config.Logger.Error("AI provider not registered", map[string]interface{}{
				"operation":           "ai_provider_lookup",
				"requested_provider":  config.Provider,
				"available_providers": ListProviders(),
				"import_hint":         fmt.Sprintf("Import _ \"github.com/itsneelabh/gomind/ai/providers/%s\"", config.Provider),
			})
		}
		return nil, fmt.Errorf("provider '%s' not registered. Import _ \"github.com/itsneelabh/gomind/ai/providers/%s\"",
			config.Provider, config.Provider)
	}

	client := factory.Create(config)
	if config.Logger != nil {
		config.Logger.Info("AI client created successfully", map[string]interface{}{
			"operation":   "ai_client_creation",
			"provider":    config.Provider,
			"client_type": fmt.Sprintf("%T", client),
			"status":      "success",
		})
	}

	return client, nil
}

// MustNewClient creates a new AI client and panics on error
func MustNewClient(opts ...AIOption) core.AIClient {
	client, err := NewClient(opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create AI client: %v", err))
	}
	return client
}
