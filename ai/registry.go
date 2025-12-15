package ai

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// ProviderFactory defines the interface for AI provider factories
type ProviderFactory interface {
	// Create creates a new AI client instance with the given configuration
	Create(config *AIConfig) core.AIClient

	// DetectEnvironment checks if this provider can be used with current environment
	// Returns priority (higher = preferred) and availability
	DetectEnvironment() (priority int, available bool)

	// Name returns the provider's name
	Name() string

	// Description returns a human-readable description
	Description() string
}

// ProviderRegistry manages registered AI providers
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]ProviderFactory
}

// Global registry instance
var registry = &ProviderRegistry{
	providers: make(map[string]ProviderFactory),
}

// Register registers a new AI provider factory
// This is typically called from init() functions in provider packages
func Register(factory ProviderFactory) error {
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	name := factory.Name()
	if name == "" {
		return fmt.Errorf("factory.Name() cannot be empty")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.providers[name]; exists {
		return fmt.Errorf("provider '%s' already registered", name)
	}

	registry.providers[name] = factory
	return nil
}

// MustRegister registers a provider and panics on error
// Use this in init() functions where errors cannot be handled
func MustRegister(factory ProviderFactory) {
	if err := Register(factory); err != nil {
		panic(fmt.Sprintf("failed to register provider: %v", err))
	}
}

// GetProvider retrieves a registered provider by name
func GetProvider(name string) (ProviderFactory, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	factory, exists := registry.providers[name]
	return factory, exists
}

// ListProviders returns all registered provider names
func ListProviders() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	names := make([]string, 0, len(registry.providers))
	for name := range registry.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetProviderInfo returns information about all registered providers
func GetProviderInfo() []ProviderInfo {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	info := make([]ProviderInfo, 0, len(registry.providers))
	for name, factory := range registry.providers {
		priority, available := factory.DetectEnvironment()
		info = append(info, ProviderInfo{
			Name:        name,
			Description: factory.Description(),
			Available:   available,
			Priority:    priority,
		})
	}

	// Sort by priority (highest first), then by name
	sort.Slice(info, func(i, j int) bool {
		if info[i].Priority != info[j].Priority {
			return info[i].Priority > info[j].Priority
		}
		return info[i].Name < info[j].Name
	})

	return info
}

// ProviderInfo contains information about a registered provider
type ProviderInfo struct {
	Name        string
	Description string
	Available   bool
	Priority    int
}

// detectBestProvider finds the best available provider from registry
func detectBestProvider(logger core.Logger) (string, error) {
	startTime := time.Now()
	var candidates []candidate

	if logger != nil {
		logger.Info("Starting AI provider environment detection", map[string]interface{}{
			"operation":             "ai_provider_detection",
			"registered_providers":  len(registry.providers),
		})
	}

	// Check all registered providers
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	for name, factory := range registry.providers {
		priority, available := factory.DetectEnvironment()

		if logger != nil {
			logger.Debug("Provider environment check", map[string]interface{}{
				"operation": "ai_provider_check",
				"provider":  name,
				"priority":  priority,
				"available": available,
			})
		}

		if available {
			candidates = append(candidates, candidate{
				name:     name,
				priority: priority,
			})

			if logger != nil {
				logger.Info("AI provider available", map[string]interface{}{
					"operation": "ai_provider_available",
					"provider":  name,
					"priority":  priority,
				})
			}
		}
	}

	if len(candidates) == 0 {
		// Record detection failure metric
		telemetry.Counter("ai.provider.detection",
			"module", telemetry.ModuleAI,
			"status", "no_providers",
		)

		if logger != nil {
			logger.Error("No AI providers detected in environment", map[string]interface{}{
				"operation":         "ai_provider_detection",
				"checked_providers": len(registry.providers),
				"environment_check": "failed",
				"suggestion":        "Set API keys (OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)",
			})
		}
		return "", fmt.Errorf("no provider detected in environment")
	}

	// Sort by priority (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	selected := candidates[0].name
	detectionDuration := time.Since(startTime)

	// Record successful detection metrics
	telemetry.Histogram("ai.provider.detection.duration_ms",
		float64(detectionDuration.Milliseconds()),
		"module", telemetry.ModuleAI,
		"status", "success",
	)
	telemetry.Counter("ai.provider.detection",
		"module", telemetry.ModuleAI,
		"status", "success",
	)
	telemetry.Counter("ai.provider.selected",
		"module", telemetry.ModuleAI,
		"provider", selected,
	)

	if logger != nil {
		logger.Info("AI provider selected", map[string]interface{}{
			"operation":             "ai_provider_selection",
			"selected_provider":     selected,
			"selection_priority":    candidates[0].priority,
			"total_candidates":      len(candidates),
			"alternative_providers": extractNames(candidates[1:]),
		})
	}

	return selected, nil
}

func extractNames(candidates []candidate) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.name
	}
	return names
}

// candidate represents a provider candidate for selection
type candidate struct {
	name     string
	priority int
}
