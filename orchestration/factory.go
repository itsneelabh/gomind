package orchestration

import (
	"fmt"
	"os"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// OrchestratorDependencies holds optional dependencies for the orchestrator
// This follows the dependency injection pattern used by the UI module
type OrchestratorDependencies struct {
	// Required dependencies
	Discovery core.Discovery
	AIClient  core.AIClient
	
	// Optional dependencies (can be nil)
	CircuitBreaker core.CircuitBreaker // For sophisticated resilience patterns
	Logger         core.Logger         // For structured logging
	Telemetry      core.Telemetry      // For observability
}

// CreateOrchestrator creates an orchestrator with proper module integration and dependency injection
func CreateOrchestrator(config *OrchestratorConfig, deps OrchestratorDependencies) (*AIOrchestrator, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	// Pass optional dependencies to service capability provider if configured
	if config.CapabilityProviderType == "service" {
		// Inject optional dependencies into service config
		config.CapabilityService.CircuitBreaker = deps.CircuitBreaker
		config.CapabilityService.Logger = deps.Logger
		config.CapabilityService.Telemetry = deps.Telemetry
	}
	
	// NewAIOrchestrator now handles capability provider setup based on config
	orchestrator := NewAIOrchestrator(config, deps.Discovery, deps.AIClient)
	
	// Validate service configuration if using service provider
	if config.CapabilityProviderType == "service" && config.CapabilityService.Endpoint == "" {
		// Check if auto-configuration found it
		if endpoint := os.Getenv("GOMIND_CAPABILITY_SERVICE_URL"); endpoint == "" {
			return nil, fmt.Errorf("capability service URL required: set CapabilityService.Endpoint in config or GOMIND_CAPABILITY_SERVICE_URL environment variable")
		}
	}

	// Set up telemetry if provided or create one if enabled
	if deps.Telemetry != nil {
		orchestrator.SetTelemetry(deps.Telemetry)
	} else if config.EnableTelemetry {
		// Check for telemetry endpoint in environment
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = os.Getenv("GOMIND_TELEMETRY_ENDPOINT")
		}

		// Use the framework's telemetry module
		otelProvider, err := telemetry.NewOTelProvider("orchestrator", endpoint)
		if err != nil {
			// Resilient runtime behavior - continue without telemetry
			fmt.Printf("Warning: Failed to initialize telemetry: %v\n", err)
		} else {
			orchestrator.SetTelemetry(otelProvider)
		}
	}

	return orchestrator, nil
}

// OrchestratorOption is a function that configures the orchestrator
type OrchestratorOption func(*OrchestratorConfig)

// WithCapabilityProvider creates an option for setting capability provider
func WithCapabilityProvider(providerType string, serviceURL string) OrchestratorOption {
	return func(c *OrchestratorConfig) {
		c.CapabilityProviderType = providerType
		if providerType == "service" && serviceURL != "" {
			// Auto-configure related settings when intent is clear
			c.CapabilityService.Endpoint = serviceURL
			c.EnableFallback = true // Smart default for production
		}
	}
}

// WithTelemetry creates an option for enabling/disabling telemetry
func WithTelemetry(enabled bool) OrchestratorOption {
	return func(c *OrchestratorConfig) {
		c.EnableTelemetry = enabled
	}
}

// WithFallback creates an option for enabling/disabling fallback
func WithFallback(enabled bool) OrchestratorOption {
	return func(c *OrchestratorConfig) {
		c.EnableFallback = enabled
	}
}

// CreateOrchestratorWithOptions creates an orchestrator with option functions
func CreateOrchestratorWithOptions(deps OrchestratorDependencies, opts ...OrchestratorOption) (*AIOrchestrator, error) {
	config := DefaultConfig()
	
	// Apply all options
	for _, opt := range opts {
		opt(config)
	}
	
	return CreateOrchestrator(config, deps)
}

// CreateSimpleOrchestrator creates an orchestrator with zero configuration
// This is perfect for developers who just want to get started quickly.
// It will:
// - Use the default capability provider (sends all capabilities to LLM)
// - Work with small to medium deployments (up to ~100 agents/tools)
// - Not require any external services
// - Use intelligent defaults for all settings
func CreateSimpleOrchestrator(discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
	// Pass nil config to use all defaults
	return NewAIOrchestrator(nil, discovery, aiClient)
}

// WithCircuitBreaker creates an option for injecting a circuit breaker
func WithCircuitBreaker(cb core.CircuitBreaker) func(*OrchestratorDependencies) {
	return func(d *OrchestratorDependencies) {
		d.CircuitBreaker = cb
	}
}

// WithLogger creates an option for injecting a logger
func WithLogger(logger core.Logger) func(*OrchestratorDependencies) {
	return func(d *OrchestratorDependencies) {
		d.Logger = logger
	}
}