package orchestration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// stringContains is a helper for checking if a string contains a substring
func stringContains(s, substr string) bool {
	return strings.Contains(s, substr)
}

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
		config               *OrchestratorConfig
		deps                 OrchestratorDependencies
		envVars              map[string]string
		expectError          bool
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
			name: "service provider without endpoint fails",
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
			name:   "with optional dependencies",
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

	// Check plan parse retry defaults
	if !config.PlanParseRetryEnabled {
		t.Error("Expected PlanParseRetryEnabled to be true by default")
	}
	if config.PlanParseMaxRetries != 2 {
		t.Errorf("Expected PlanParseMaxRetries 2, got %d", config.PlanParseMaxRetries)
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

// =============================================================================
// PromptBuilder Factory Integration Tests
// =============================================================================

// TestCreateOrchestrator_PromptBuilder_Layer1_Default tests that factory creates
// DefaultPromptBuilder when no template or custom builder is provided
func TestCreateOrchestrator_PromptBuilder_Layer1_Default(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	// No template file, no template string, no custom builder
	// Should use DefaultPromptBuilder (Layer 1)

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set")
	}

	// Verify it's a DefaultPromptBuilder
	_, ok := orchestrator.promptBuilder.(*DefaultPromptBuilder)
	if !ok {
		t.Errorf("Expected DefaultPromptBuilder, got %T", orchestrator.promptBuilder)
	}
}

// TestCreateOrchestrator_PromptBuilder_Layer1_WithTypeRules tests DefaultPromptBuilder
// with additional type rules
func TestCreateOrchestrator_PromptBuilder_Layer1_WithTypeRules(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		Domain: "healthcare",
		AdditionalTypeRules: []TypeRule{
			{
				TypeNames: []string{"patient_id"},
				JsonType:  "JSON strings",
				Example:   `"P12345"`,
			},
		},
	}

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set")
	}

	// Verify it's a DefaultPromptBuilder with additional rules
	builder, ok := orchestrator.promptBuilder.(*DefaultPromptBuilder)
	if !ok {
		t.Fatalf("Expected DefaultPromptBuilder, got %T", orchestrator.promptBuilder)
	}

	// Should have default rules + 1 additional rule
	rules := builder.GetTypeRules()
	if len(rules) < 7 { // 6 default + 1 additional
		t.Errorf("Expected at least 7 type rules, got %d", len(rules))
	}

	// Verify domain is set
	cfg := builder.GetConfig()
	if cfg.Domain != "healthcare" {
		t.Errorf("Expected domain 'healthcare', got '%s'", cfg.Domain)
	}
}

// TestCreateOrchestrator_PromptBuilder_Layer2_Template tests that factory creates
// TemplatePromptBuilder when template is provided
func TestCreateOrchestrator_PromptBuilder_Layer2_Template(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		Template: `You are orchestrating: {{.Request}}
Available: {{.CapabilityInfo}}
{{.TypeRules}}`,
	}

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set")
	}

	// Verify it's a TemplatePromptBuilder
	_, ok := orchestrator.promptBuilder.(*TemplatePromptBuilder)
	if !ok {
		t.Errorf("Expected TemplatePromptBuilder, got %T", orchestrator.promptBuilder)
	}
}

// TestCreateOrchestrator_PromptBuilder_Layer2_TemplateFile tests template file loading
func TestCreateOrchestrator_PromptBuilder_Layer2_TemplateFile(t *testing.T) {
	// Create a temporary template file
	tmpFile, err := os.CreateTemp("", "prompt-template-*.tmpl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	templateContent := `You are an AI orchestrator.
Request: {{.Request}}
Capabilities: {{.CapabilityInfo}}
{{.TypeRules}}`
	if _, err := tmpFile.WriteString(templateContent); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}
	tmpFile.Close()

	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		TemplateFile: tmpFile.Name(),
	}

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set")
	}

	// Verify it's a TemplatePromptBuilder
	builder, ok := orchestrator.promptBuilder.(*TemplatePromptBuilder)
	if !ok {
		t.Errorf("Expected TemplatePromptBuilder, got %T", orchestrator.promptBuilder)
	}

	// Verify fallback is also initialized
	if builder.GetFallback() == nil {
		t.Error("Expected fallback builder to be set")
	}
}

// TestCreateOrchestrator_PromptBuilder_Layer3_Custom tests custom builder injection
func TestCreateOrchestrator_PromptBuilder_Layer3_Custom(t *testing.T) {
	customBuilder := &mockPromptBuilder{
		buildFunc: func(ctx context.Context, input PromptInput) (string, error) {
			return "Custom prompt: " + input.Request, nil
		},
	}

	deps := OrchestratorDependencies{
		Discovery:     NewMockDiscovery(),
		AIClient:      NewMockAIClient(),
		Logger:        &mockLogger{},
		PromptBuilder: customBuilder,
	}

	config := DefaultConfig()
	// Even if template is set, custom builder takes precedence
	config.PromptConfig = PromptConfig{
		Template: "This should be ignored",
	}

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set")
	}

	// Verify it's our custom builder (Layer 3 takes precedence)
	if orchestrator.promptBuilder != customBuilder {
		t.Errorf("Expected custom builder, got %T", orchestrator.promptBuilder)
	}
}

// TestCreateOrchestrator_PromptBuilder_GracefulDegradation tests fallback when
// template fails to load
func TestCreateOrchestrator_PromptBuilder_GracefulDegradation(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		TemplateFile: "/nonexistent/path/template.tmpl", // File doesn't exist
	}

	// Should gracefully degrade to DefaultPromptBuilder, not error
	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Expected graceful degradation, got error: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set (fallback)")
	}

	// Should fall back to DefaultPromptBuilder
	_, ok := orchestrator.promptBuilder.(*DefaultPromptBuilder)
	if !ok {
		t.Errorf("Expected DefaultPromptBuilder fallback, got %T", orchestrator.promptBuilder)
	}
}

// TestCreateOrchestrator_PromptBuilder_InvalidTemplate tests graceful degradation
// when template has syntax errors
func TestCreateOrchestrator_PromptBuilder_InvalidTemplate(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    &mockLogger{},
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		Template: "{{.Invalid syntax here", // Invalid template
	}

	// Should gracefully degrade to DefaultPromptBuilder
	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Expected graceful degradation, got error: %v", err)
	}

	if orchestrator.promptBuilder == nil {
		t.Fatal("Expected promptBuilder to be set (fallback)")
	}

	// Should fall back to DefaultPromptBuilder
	_, ok := orchestrator.promptBuilder.(*DefaultPromptBuilder)
	if !ok {
		t.Errorf("Expected DefaultPromptBuilder fallback, got %T", orchestrator.promptBuilder)
	}
}

// TestCreateOrchestrator_PromptBuilder_DependencyInjection tests that logger and
// telemetry are properly injected into PromptBuilder
func TestCreateOrchestrator_PromptBuilder_DependencyInjection(t *testing.T) {
	mockLog := &mockLogger{}
	mockTel := &mockTelemetry{}

	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
		Logger:    mockLog,
		Telemetry: mockTel,
	}

	config := DefaultConfig()
	config.PromptConfig = PromptConfig{
		Domain: "finance",
	}

	orchestrator, err := CreateOrchestrator(config, deps)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Build a prompt to verify dependencies work
	builder, ok := orchestrator.promptBuilder.(*DefaultPromptBuilder)
	if !ok {
		t.Fatalf("Expected DefaultPromptBuilder, got %T", orchestrator.promptBuilder)
	}

	// The builder should have logger set (we can verify by building a prompt)
	prompt, err := builder.BuildPlanningPrompt(context.Background(), PromptInput{
		CapabilityInfo: "Test capabilities",
		Request:        "Test request",
	})
	if err != nil {
		t.Errorf("BuildPlanningPrompt failed: %v", err)
	}

	// Prompt should include finance domain section
	if !stringContains(prompt, "FINANCE DOMAIN") {
		t.Error("Expected finance domain section in prompt")
	}
}

// mockPromptBuilder implements PromptBuilder for testing
type mockPromptBuilder struct {
	buildFunc func(ctx context.Context, input PromptInput) (string, error)
}

func (m *mockPromptBuilder) BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, input)
	}
	return "mock prompt", nil
}

// =============================================================================
// Plan Parse Retry Configuration Tests
// =============================================================================

// TestDefaultConfig_PlanParseRetry_EnvironmentConfiguration tests env var loading
func TestDefaultConfig_PlanParseRetry_EnvironmentConfiguration(t *testing.T) {
	// Save and clear any existing env vars
	oldEnabled := os.Getenv("GOMIND_PLAN_RETRY_ENABLED")
	oldMax := os.Getenv("GOMIND_PLAN_RETRY_MAX")
	defer func() {
		if oldEnabled != "" {
			os.Setenv("GOMIND_PLAN_RETRY_ENABLED", oldEnabled)
		} else {
			os.Unsetenv("GOMIND_PLAN_RETRY_ENABLED")
		}
		if oldMax != "" {
			os.Setenv("GOMIND_PLAN_RETRY_MAX", oldMax)
		} else {
			os.Unsetenv("GOMIND_PLAN_RETRY_MAX")
		}
	}()

	t.Run("disable retry via env", func(t *testing.T) {
		os.Setenv("GOMIND_PLAN_RETRY_ENABLED", "false")
		os.Unsetenv("GOMIND_PLAN_RETRY_MAX")

		config := DefaultConfig()

		if config.PlanParseRetryEnabled {
			t.Error("Expected PlanParseRetryEnabled to be false from env")
		}
	})

	t.Run("enable retry via env", func(t *testing.T) {
		os.Setenv("GOMIND_PLAN_RETRY_ENABLED", "true")
		os.Unsetenv("GOMIND_PLAN_RETRY_MAX")

		config := DefaultConfig()

		if !config.PlanParseRetryEnabled {
			t.Error("Expected PlanParseRetryEnabled to be true from env")
		}
	})

	t.Run("set max retries via env", func(t *testing.T) {
		os.Unsetenv("GOMIND_PLAN_RETRY_ENABLED")
		os.Setenv("GOMIND_PLAN_RETRY_MAX", "5")

		config := DefaultConfig()

		if config.PlanParseMaxRetries != 5 {
			t.Errorf("Expected PlanParseMaxRetries 5, got %d", config.PlanParseMaxRetries)
		}
	})

	t.Run("invalid max retries ignored", func(t *testing.T) {
		os.Unsetenv("GOMIND_PLAN_RETRY_ENABLED")
		os.Setenv("GOMIND_PLAN_RETRY_MAX", "invalid")

		config := DefaultConfig()

		// Should keep default value of 2
		if config.PlanParseMaxRetries != 2 {
			t.Errorf("Expected PlanParseMaxRetries 2 (default), got %d", config.PlanParseMaxRetries)
		}
	})

	t.Run("negative max retries ignored", func(t *testing.T) {
		os.Unsetenv("GOMIND_PLAN_RETRY_ENABLED")
		os.Setenv("GOMIND_PLAN_RETRY_MAX", "-1")

		config := DefaultConfig()

		// Should keep default value of 2 (negative values are invalid)
		if config.PlanParseMaxRetries != 2 {
			t.Errorf("Expected PlanParseMaxRetries 2 (default), got %d", config.PlanParseMaxRetries)
		}
	})

	t.Run("zero max retries is valid", func(t *testing.T) {
		os.Unsetenv("GOMIND_PLAN_RETRY_ENABLED")
		os.Setenv("GOMIND_PLAN_RETRY_MAX", "0")

		config := DefaultConfig()

		// Zero is valid (means no retries)
		if config.PlanParseMaxRetries != 0 {
			t.Errorf("Expected PlanParseMaxRetries 0, got %d", config.PlanParseMaxRetries)
		}
	})
}

// TestWithPlanParseRetry tests the functional option
func TestWithPlanParseRetry(t *testing.T) {
	t.Run("enable with max retries", func(t *testing.T) {
		config := DefaultConfig()
		opt := WithPlanParseRetry(true, 5)
		opt(config)

		if !config.PlanParseRetryEnabled {
			t.Error("Expected PlanParseRetryEnabled to be true")
		}
		if config.PlanParseMaxRetries != 5 {
			t.Errorf("Expected PlanParseMaxRetries 5, got %d", config.PlanParseMaxRetries)
		}
	})

	t.Run("disable retry", func(t *testing.T) {
		config := DefaultConfig()
		opt := WithPlanParseRetry(false, 0)
		opt(config)

		if config.PlanParseRetryEnabled {
			t.Error("Expected PlanParseRetryEnabled to be false")
		}
		if config.PlanParseMaxRetries != 0 {
			t.Errorf("Expected PlanParseMaxRetries 0, got %d", config.PlanParseMaxRetries)
		}
	})

	t.Run("negative max retries ignored", func(t *testing.T) {
		config := DefaultConfig()
		// Start with known values
		config.PlanParseMaxRetries = 3

		opt := WithPlanParseRetry(true, -1)
		opt(config)

		// Should not change the max retries when negative
		if config.PlanParseMaxRetries != 3 {
			t.Errorf("Expected PlanParseMaxRetries to remain 3, got %d", config.PlanParseMaxRetries)
		}
	})
}

// TestCreateOrchestratorWithOptions_PlanParseRetry tests orchestrator creation with retry options
func TestCreateOrchestratorWithOptions_PlanParseRetry(t *testing.T) {
	deps := OrchestratorDependencies{
		Discovery: NewMockDiscovery(),
		AIClient:  NewMockAIClient(),
	}

	orchestrator, err := CreateOrchestratorWithOptions(deps, WithPlanParseRetry(true, 3))
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if !orchestrator.config.PlanParseRetryEnabled {
		t.Error("Expected PlanParseRetryEnabled to be true")
	}
	if orchestrator.config.PlanParseMaxRetries != 3 {
		t.Errorf("Expected PlanParseMaxRetries 3, got %d", orchestrator.config.PlanParseMaxRetries)
	}
}
