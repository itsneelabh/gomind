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

	// Optional: Custom prompt building (Layer 3)
	// If nil, DefaultPromptBuilder is used based on config.PromptConfig
	PromptBuilder PromptBuilder

	// Optional: Enable LLM-based error analysis (Layer 3: Error Analysis)
	// When true, creates an ErrorAnalyzer that uses LLM to determine if errors
	// can be fixed with different parameters. This removes the need for tools
	// to set Retryable flags. See PARAMETER_BINDING_FIX.md for design rationale.
	EnableErrorAnalyzer bool
}

// CreateOrchestrator creates an orchestrator with proper module integration and dependency injection
func CreateOrchestrator(config *OrchestratorConfig, deps OrchestratorDependencies) (*AIOrchestrator, error) {
	if config == nil {
		config = DefaultConfig()
	}

	var factoryLogger core.Logger
	if deps.Logger != nil {
		// Apply component-specific logging for orchestration module
		if cal, ok := deps.Logger.(core.ComponentAwareLogger); ok {
			factoryLogger = cal.WithComponent("framework/orchestration")
		} else {
			factoryLogger = deps.Logger
		}
	} else {
		// Use NoOpLogger to avoid creating a parallel logging setup.
		// The framework's logging is configured centrally via core.NewFramework().
		// If you want orchestration logs, pass the agent's Logger in OrchestratorDependencies:
		//   deps := OrchestratorDependencies{Logger: agent.Logger, ...}
		// This follows the same pattern as core/agent.go which uses NoOpLogger as the default.
		factoryLogger = &core.NoOpLogger{}
	}
	deps.Logger = factoryLogger

	factoryLogger.Info("Creating orchestrator instance", map[string]interface{}{
		"operation":               "orchestrator_creation",
		"routing_mode":            string(config.RoutingMode),
		"capability_provider_type": config.CapabilityProviderType,
		"telemetry_enabled":       config.EnableTelemetry,
	})

	// Pass optional dependencies to service capability provider if configured
	if config.CapabilityProviderType == "service" {
		// Inject optional dependencies into service config
		config.CapabilityService.CircuitBreaker = deps.CircuitBreaker
		config.CapabilityService.Logger = deps.Logger
		config.CapabilityService.Telemetry = deps.Telemetry
	}

	// Create orchestrator
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
		otelProvider, err := telemetry.NewOTelProvider("orchestrator", "orchestrator", endpoint)
		if err != nil {
			// Resilient runtime behavior - continue without telemetry
			if factoryLogger != nil {
				factoryLogger.Warn("Failed to initialize telemetry", map[string]interface{}{
					"operation": "telemetry_initialization",
					"error":     err.Error(),
					"endpoint":  endpoint,
				})
			}
		} else {
			orchestrator.SetTelemetry(otelProvider)
		}
	}

	orchestrator.SetLogger(deps.Logger)

	// Initialize prompt builder based on configuration
	// Priority: 1) Injected builder (Layer 3), 2) Template (Layer 2), 3) Default (Layer 1)
	if deps.PromptBuilder != nil {
		// Layer 3: Custom builder injected by application
		orchestrator.SetPromptBuilder(deps.PromptBuilder)
		factoryLogger.Info("Using custom PromptBuilder", map[string]interface{}{
			"operation":    "prompt_builder_initialization",
			"builder_type": "custom",
		})
	} else if config.PromptConfig.TemplateFile != "" || config.PromptConfig.Template != "" {
		// Layer 2: Template-based customization
		builder, err := NewTemplatePromptBuilder(&config.PromptConfig)
		if err != nil {
			// Graceful degradation to DefaultPromptBuilder
			factoryLogger.Warn("Failed to create TemplatePromptBuilder, using default", map[string]interface{}{
				"operation":     "prompt_builder_initialization",
				"error":         err.Error(),
				"template_file": config.PromptConfig.TemplateFile,
			})
			defaultBuilder, _ := NewDefaultPromptBuilder(&config.PromptConfig)
			defaultBuilder.SetLogger(deps.Logger)
			defaultBuilder.SetTelemetry(deps.Telemetry)
			orchestrator.SetPromptBuilder(defaultBuilder)
		} else {
			builder.SetLogger(deps.Logger)
			builder.SetTelemetry(deps.Telemetry)
			orchestrator.SetPromptBuilder(builder)
			factoryLogger.Info("Using TemplatePromptBuilder", map[string]interface{}{
				"operation":     "prompt_builder_initialization",
				"builder_type":  "template",
				"template_file": config.PromptConfig.TemplateFile,
			})
		}
	} else {
		// Layer 1: Default with optional type rule extensions
		defaultBuilder, _ := NewDefaultPromptBuilder(&config.PromptConfig)
		defaultBuilder.SetLogger(deps.Logger)
		defaultBuilder.SetTelemetry(deps.Telemetry)
		orchestrator.SetPromptBuilder(defaultBuilder)
		factoryLogger.Info("Using DefaultPromptBuilder", map[string]interface{}{
			"operation":        "prompt_builder_initialization",
			"builder_type":     "default",
			"additional_rules": len(config.PromptConfig.AdditionalTypeRules),
			"domain":           config.PromptConfig.Domain,
		})
	}

	// Configure LLM-based error analyzer if enabled (Layer 3: Error Analysis)
	// This removes the need for tools to set Retryable flags - the LLM decides
	if deps.EnableErrorAnalyzer && deps.AIClient != nil {
		errorAnalyzer := NewErrorAnalyzer(deps.AIClient, deps.Logger)
		orchestrator.SetErrorAnalyzer(errorAnalyzer)
		factoryLogger.Info("LLM error analyzer enabled", map[string]interface{}{
			"operation": "error_analyzer_initialization",
		})
	}

	factoryLogger.Info("Orchestrator created successfully", map[string]interface{}{
		"operation":            "orchestrator_creation_complete",
		"success":              true,
		"error_analyzer":       deps.EnableErrorAnalyzer,
	})

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
// - Use NoOpLogger by default (pass Logger in dependencies for logging)
func CreateSimpleOrchestrator(discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
	// Use proper dependency injection to ensure all framework features work
	deps := OrchestratorDependencies{
		Discovery: discovery,
		AIClient:  aiClient,
		// Logger, Telemetry, CircuitBreaker will be auto-created with smart defaults
	}

	orchestrator, err := CreateOrchestrator(nil, deps)
	if err != nil {
		// This should never happen with default config, but follow fail-safe principles
		return NewAIOrchestrator(nil, discovery, aiClient)
	}

	return orchestrator
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