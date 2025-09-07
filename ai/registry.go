package ai

import (
	"fmt"
	"sort"
	"sync"

	"github.com/itsneelabh/gomind/core"
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
func detectBestProvider() (string, error) {
	type candidate struct {
		name     string
		priority int
	}
	
	var candidates []candidate
	
	// Check all registered providers
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	
	for name, factory := range registry.providers {
		if priority, available := factory.DetectEnvironment(); available {
			candidates = append(candidates, candidate{
				name:     name,
				priority: priority,
			})
		}
	}
	
	if len(candidates) == 0 {
		return "", fmt.Errorf("no provider detected in environment")
	}
	
	// Sort by priority (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})
	
	return candidates[0].name, nil
}