package ai

import (
	"context"
	"testing"

	"github.com/itsneelabh/gomind/core"
)

// MockProviderFactory implements ProviderFactory for testing
type MockProviderFactory struct {
	name        string
	description string
	priority    int
	available   bool
	createFunc  func(*AIConfig) core.AIClient
}

func (m *MockProviderFactory) Create(config *AIConfig) core.AIClient {
	if m.createFunc != nil {
		return m.createFunc(config)
	}
	return &mockRegistryAIClient{}
}

func (m *MockProviderFactory) DetectEnvironment() (int, bool) {
	return m.priority, m.available
}

func (m *MockProviderFactory) Name() string {
	return m.name
}

func (m *MockProviderFactory) Description() string {
	return m.description
}

// mockRegistryAIClient implements core.AIClient for testing registry
type mockRegistryAIClient struct{}

func (m *mockRegistryAIClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	return &core.AIResponse{
		Content: "mock response",
		Model:   "mock-model",
	}, nil
}

func TestRegister(t *testing.T) {
	// Clear registry for testing
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	registry.mu.Unlock()
	
	tests := []struct {
		name      string
		factory   ProviderFactory
		wantError bool
	}{
		{
			name: "register new provider",
			factory: &MockProviderFactory{
				name:        "test-provider",
				description: "Test Provider",
			},
			wantError: false,
		},
		{
			name: "register duplicate provider",
			factory: &MockProviderFactory{
				name: "test-provider",
			},
			wantError: true,
		},
		{
			name:      "register nil factory",
			factory:   nil,
			wantError: true,
		},
		{
			name: "register empty name",
			factory: &MockProviderFactory{
				name:        "",
				description: "Empty name",
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Register(tt.factory)
			if (err != nil) != tt.wantError {
				t.Errorf("Register() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetProvider(t *testing.T) {
	// Clear and setup registry
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	testFactory := &MockProviderFactory{
		name:        "test-provider",
		description: "Test Provider",
	}
	registry.providers["test-provider"] = testFactory
	registry.mu.Unlock()
	
	tests := []struct {
		name         string
		providerName string
		wantExists   bool
	}{
		{
			name:         "get existing provider",
			providerName: "test-provider",
			wantExists:   true,
		},
		{
			name:         "get non-existent provider",
			providerName: "non-existent",
			wantExists:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, exists := GetProvider(tt.providerName)
			if exists != tt.wantExists {
				t.Errorf("GetProvider() exists = %v, want %v", exists, tt.wantExists)
			}
			if exists && factory == nil {
				t.Error("GetProvider() returned nil factory for existing provider")
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	// Clear and setup registry
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	registry.providers["provider-a"] = &MockProviderFactory{name: "provider-a"}
	registry.providers["provider-b"] = &MockProviderFactory{name: "provider-b"}
	registry.providers["provider-c"] = &MockProviderFactory{name: "provider-c"}
	registry.mu.Unlock()
	
	providers := ListProviders()
	
	// Check count
	if len(providers) != 3 {
		t.Errorf("ListProviders() returned %d providers, want 3", len(providers))
	}
	
	// Check sorting
	expected := []string{"provider-a", "provider-b", "provider-c"}
	for i, p := range providers {
		if p != expected[i] {
			t.Errorf("ListProviders()[%d] = %s, want %s", i, p, expected[i])
		}
	}
}

func TestDetectBestProvider(t *testing.T) {
	// Clear and setup registry with providers of different priorities
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	registry.providers["high-priority"] = &MockProviderFactory{
		name:      "high-priority",
		priority:  100,
		available: true,
	}
	registry.providers["medium-priority"] = &MockProviderFactory{
		name:      "medium-priority",
		priority:  50,
		available: true,
	}
	registry.providers["low-priority"] = &MockProviderFactory{
		name:      "low-priority",
		priority:  10,
		available: true,
	}
	registry.providers["unavailable"] = &MockProviderFactory{
		name:      "unavailable",
		priority:  200,
		available: false,
	}
	registry.mu.Unlock()
	
	provider, err := detectBestProvider()
	if err != nil {
		t.Fatalf("detectBestProvider() error = %v", err)
	}
	
	if provider != "high-priority" {
		t.Errorf("detectBestProvider() = %s, want high-priority", provider)
	}
}

func TestDetectBestProviderNoAvailable(t *testing.T) {
	// Clear registry and add only unavailable providers
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	registry.providers["unavailable1"] = &MockProviderFactory{
		name:      "unavailable1",
		priority:  100,
		available: false,
	}
	registry.providers["unavailable2"] = &MockProviderFactory{
		name:      "unavailable2",
		priority:  50,
		available: false,
	}
	registry.mu.Unlock()
	
	_, err := detectBestProvider()
	if err == nil {
		t.Error("detectBestProvider() should return error when no providers available")
	}
}

func TestGetProviderInfo(t *testing.T) {
	// Clear and setup registry
	registry.mu.Lock()
	registry.providers = make(map[string]ProviderFactory)
	registry.providers["available-high"] = &MockProviderFactory{
		name:        "available-high",
		description: "High priority available",
		priority:    100,
		available:   true,
	}
	registry.providers["available-low"] = &MockProviderFactory{
		name:        "available-low",
		description: "Low priority available",
		priority:    10,
		available:   true,
	}
	registry.providers["unavailable"] = &MockProviderFactory{
		name:        "unavailable",
		description: "Unavailable provider",
		priority:    50,
		available:   false,
	}
	registry.mu.Unlock()
	
	info := GetProviderInfo()
	
	// Check count
	if len(info) != 3 {
		t.Errorf("GetProviderInfo() returned %d providers, want 3", len(info))
	}
	
	// Check sorting by priority (highest first)
	if info[0].Name != "available-high" {
		t.Errorf("First provider should be available-high, got %s", info[0].Name)
	}
	
	// Check availability flags
	for _, p := range info {
		switch p.Name {
		case "available-high", "available-low":
			if !p.Available {
				t.Errorf("Provider %s should be available", p.Name)
			}
		case "unavailable":
			if p.Available {
				t.Errorf("Provider %s should not be available", p.Name)
			}
		}
	}
}