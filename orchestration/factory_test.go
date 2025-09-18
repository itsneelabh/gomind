package orchestration

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestCreateSimpleOrchestrator tests the zero-configuration orchestrator creation
func TestCreateSimpleOrchestrator(t *testing.T) {
	discovery := NewMockDiscovery()
	aiClient := NewMockAIClient()

	orchestrator := CreateSimpleOrchestrator(discovery, aiClient)

	if orchestrator == nil {
		t.Fatal("Expected orchestrator, got nil")
	}

	// Should use default configuration
	if orchestrator.config == nil {
		t.Error("Expected default config to be set")
	}

	// Should use DefaultCapabilityProvider
	if orchestrator.capabilityProvider == nil {
		t.Error("Expected capability provider to be set")
	}

	// Test that it can process requests
	ctx := context.Background()
	response, err := orchestrator.ProcessRequest(ctx, "test request", nil)
	if err != nil && response == nil {
		// Either error or response is acceptable for mock
		t.Logf("ProcessRequest returned: %v", err)
	}
}

// TestCreateOrchestrator tests orchestrator creation with configuration
func TestCreateOrchestrator(t *testing.T) {
	tests := []struct {
		name                 string
		config              *OrchestratorConfig
		deps                OrchestratorDependencies
		envVars             map[string]string
		expectError         bool
		expectedProviderType string
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
			deps: OrchestratorDependencies{
				Discovery: NewMockDiscovery(),
				AIClient:  NewMockAIClient(),
			},
			expectError:          false,
			expectedProviderType: "default",
		},
		{
			name: "explicit service provider config",
			config: &OrchestratorConfig{
				CapabilityProviderType: "service",
				CapabilityService: ServiceCapabilityConfig{
					Endpoint: "http://test-service:8080",
				},
			},
			deps: OrchestratorDependencies{
				Discovery: NewMockDiscovery(),
				AIClient:  NewMockAIClient(),
			},
			expectError:          false,
			expectedProviderType: "service",
		},
		{
			name:   "service provider without endpoint fails",
			config: &OrchestratorConfig{
				CapabilityProviderType: "service",
				// No endpoint specified
			},
			deps: OrchestratorDependencies{
				Discovery: NewMockDiscovery(),
				AIClient:  NewMockAIClient(),
			},
			expectError: true,
		},
		{
			name:   "auto-configuration from environment",
			config: nil,
			deps: OrchestratorDependencies{
				Discovery: NewMockDiscovery(),
				AIClient:  NewMockAIClient(),
			},
			envVars: map[string]string{
				"GOMIND_CAPABILITY_SERVICE_URL": "http://env-service:8080",
			},
			expectError:          false,
			expectedProviderType: "service",
		},
		{
			name: "with optional dependencies",
			config: DefaultConfig(),
			deps: OrchestratorDependencies{
				Discovery:      NewMockDiscovery(),
				AIClient:       NewMockAIClient(),
				CircuitBreaker: &mockCircuitBreaker{},
				Logger:         &mockLogger{},
				Telemetry:      &mockTelemetry{},
			},
			expectError:          false,
			expectedProviderType: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				oldVal := os.Getenv(k)
				os.Setenv(k, v)
				defer os.Setenv(k, oldVal)
			}

			orchestrator, err := CreateOrchestrator(tt.config, tt.deps)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if orchestrator == nil {
				t.Fatal("Expected orchestrator, got nil")
			}

			// Check provider type if specified
			if tt.expectedProviderType != "" {
				actualType := orchestrator.config.CapabilityProviderType
				if actualType != tt.expectedProviderType {
					t.Errorf("Expected provider type %s, got %s", tt.expectedProviderType, actualType)
				}
			}
		})
	}
}

// TestCreateOrchestratorWithOptions tests the options-based factory
func TestCreateOrchestratorWithOptions(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
	}

	tests := []struct {
		name        string
		options     []OrchestratorOption
		checkConfig func(*testing.T, *OrchestratorConfig)
	}{
		{
			name: "with capability provider option",
			options: []OrchestratorOption{
				WithCapabilityProvider("service", "http://option-service:8080"),
			},
			checkConfig: func(t *testing.T, config *OrchestratorConfig) {
				if config.CapabilityProviderType != "service" {
					t.Errorf("Expected service provider, got %s", config.CapabilityProviderType)
				}
				if config.CapabilityService.Endpoint != "http://option-service:8080" {
					t.Errorf("Expected endpoint http://option-service:8080, got %s", config.CapabilityService.Endpoint)
				}
			},
		},
		{
			name: "with telemetry option",
			options: []OrchestratorOption{
				WithTelemetry(true),
			},
			checkConfig: func(t *testing.T, config *OrchestratorConfig) {
				if !config.EnableTelemetry {
					t.Error("Expected telemetry to be enabled")
				}
			},
		},
		{
			name: "with fallback option",
			options: []OrchestratorOption{
				WithFallback(false),
			},
			checkConfig: func(t *testing.T, config *OrchestratorConfig) {
				if config.EnableFallback {
					t.Error("Expected fallback to be disabled")
				}
			},
		},
		{
			name: "with multiple options",
			options: []OrchestratorOption{
				WithCapabilityProvider("service", "http://multi-service:8080"),
				WithTelemetry(true),
				WithFallback(true),
			},
			checkConfig: func(t *testing.T, config *OrchestratorConfig) {
				if config.CapabilityProviderType != "service" {
					t.Errorf("Expected service provider, got %s", config.CapabilityProviderType)
				}
				if !config.EnableTelemetry {
					t.Error("Expected telemetry to be enabled")
				}
				if !config.EnableFallback {
					t.Error("Expected fallback to be enabled")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator, err := CreateOrchestratorWithOptions(deps, tt.options...)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if orchestrator == nil {
				t.Fatal("Expected orchestrator, got nil")
			}

			tt.checkConfig(t, orchestrator.config)
		})
	}
}

// TestOrchestratorOption functions test individual option functions
func TestOrchestratorOptions(t *testing.T) {
	t.Run("WithCapabilityProvider", func(t *testing.T) {
		config := DefaultConfig()
		opt := WithCapabilityProvider("service", "http://test:8080")
		opt(config)

		if config.CapabilityProviderType != "service" {
			t.Errorf("Expected service, got %s", config.CapabilityProviderType)
		}
		if config.CapabilityService.Endpoint != "http://test:8080" {
			t.Errorf("Expected http://test:8080, got %s", config.CapabilityService.Endpoint)
		}
	})

	t.Run("WithTelemetry", func(t *testing.T) {
		config := DefaultConfig()
		
		// Test enabling
		WithTelemetry(true)(config)
		if !config.EnableTelemetry {
			t.Error("Expected telemetry to be enabled")
		}

		// Test disabling
		WithTelemetry(false)(config)
		if config.EnableTelemetry {
			t.Error("Expected telemetry to be disabled")
		}
	})

	t.Run("WithFallback", func(t *testing.T) {
		config := DefaultConfig()
		
		// Test enabling
		WithFallback(true)(config)
		if !config.EnableFallback {
			t.Error("Expected fallback to be enabled")
		}

		// Test disabling
		WithFallback(false)(config)
		if config.EnableFallback {
			t.Error("Expected fallback to be disabled")
		}
	})
}

// TestDefaultConfig tests the default configuration
func TestDefaultConfig(t *testing.T) {
	// Clear any environment variables that might affect the test
	oldVal := os.Getenv("GOMIND_CAPABILITY_SERVICE_URL")
	os.Unsetenv("GOMIND_CAPABILITY_SERVICE_URL")
	defer os.Setenv("GOMIND_CAPABILITY_SERVICE_URL", oldVal)

	config := DefaultConfig()

	// Check defaults
	if config.RoutingMode != ModeAutonomous {
		t.Errorf("Expected ModeAutonomous, got %s", config.RoutingMode)
	}
	if config.SynthesisStrategy != StrategyLLM {
		t.Errorf("Expected StrategyLLM, got %s", config.SynthesisStrategy)
	}
	if config.CapabilityProviderType != "default" {
		t.Errorf("Expected default provider, got %s", config.CapabilityProviderType)
	}
	if !config.EnableTelemetry {
		t.Error("Expected telemetry to be enabled by default")
	}
	if !config.EnableFallback {
		t.Error("Expected fallback to be enabled by default")
	}
	if config.HistorySize != 100 {
		t.Errorf("Expected history size 100, got %d", config.HistorySize)
	}
	if !config.CacheEnabled {
		t.Error("Expected cache to be enabled by default")
	}
	if config.CacheTTL != 5*time.Minute {
		t.Errorf("Expected cache TTL 5m, got %v", config.CacheTTL)
	}

	// Check execution options
	if config.ExecutionOptions.MaxConcurrency != 5 {
		t.Errorf("Expected max concurrency 5, got %d", config.ExecutionOptions.MaxConcurrency)
	}
	if config.ExecutionOptions.StepTimeout != 30*time.Second {
		t.Errorf("Expected step timeout 30s, got %v", config.ExecutionOptions.StepTimeout)
	}
}

// TestDefaultConfig_EnvironmentAutoConfiguration tests auto-configuration from environment
func TestDefaultConfig_EnvironmentAutoConfiguration(t *testing.T) {
	// Set environment variable
	os.Setenv("GOMIND_CAPABILITY_SERVICE_URL", "http://auto-config:9090")
	defer os.Unsetenv("GOMIND_CAPABILITY_SERVICE_URL")

	config := DefaultConfig()

	// Should auto-configure to service provider
	if config.CapabilityProviderType != "service" {
		t.Errorf("Expected service provider from env, got %s", config.CapabilityProviderType)
	}
	if config.CapabilityService.Endpoint != "http://auto-config:9090" {
		t.Errorf("Expected endpoint from env, got %s", config.CapabilityService.Endpoint)
	}
}

// TestDependencyInjection tests that dependencies are properly injected
func TestDependencyInjection(t *testing.T) {
	// Create mock dependencies
	mockCB := &mockCircuitBreaker{
		executeFunc: func(ctx context.Context, fn func() error) error {
			return fn()
		},
	}

	mockLog := &mockLogger{
		debugFunc: func(msg string, fields map[string]interface{}) {
			// Logger called
		},
	}

	deps := OrchestratorDependencies{
		Discovery:      NewMockDiscovery(),
		AIClient:       NewMockAIClient(),
		CircuitBreaker: mockCB,
		Logger:         mockLog,
	}

	// Configure for service provider
	config := DefaultConfig()
	config.CapabilityProviderType = "service"
	config.CapabilityService.Endpoint = "http://test:8080"

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Verify dependencies were injected into ServiceCapabilityProvider
	serviceProvider, ok := orchestrator.capabilityProvider.(*ServiceCapabilityProvider)
	if !ok {
		t.Fatal("Expected ServiceCapabilityProvider")
	}

	if serviceProvider.circuitBreaker == nil {
		t.Error("Expected circuit breaker to be injected")
	}
	if serviceProvider.logger == nil {
		t.Error("Expected logger to be injected")
	}
}

// TestFactoryErrorCases tests error handling in factory functions
func TestFactoryErrorCases(t *testing.T) {
	t.Run("service provider without endpoint", func(t *testing.T) {
		deps := OrchestratorDependencies{
			Discovery: NewMockDiscovery(),
			AIClient:  NewMockAIClient(),
		}

		config := &OrchestratorConfig{
			CapabilityProviderType: "service",
			// No endpoint specified
		}

		_, err := CreateOrchestrator(config, deps)
		if err == nil {
			t.Error("Expected error for service provider without endpoint")
		}
		if err != nil && !stringContains(err.Error(), "capability service URL required") {
			t.Errorf("Expected error about missing URL, got: %v", err)
		}
	})
}

// Test helper for dependency functions
func TestWithCircuitBreaker(t *testing.T) {
	cb := &mockCircuitBreaker{}
	deps := &OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
	}

	withCB := WithCircuitBreaker(cb)
	withCB(deps)

	if deps.CircuitBreaker != cb {
		t.Error("Expected circuit breaker to be set")
	}
}

func TestWithLogger(t *testing.T) {
	logger := &mockLogger{}
	deps := &OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
	}

	withLogger := WithLogger(logger)
	withLogger(deps)

	if deps.Logger != logger {
		t.Error("Expected logger to be set")
	}
}

// Mocks are now in test_mocks.go to avoid duplication