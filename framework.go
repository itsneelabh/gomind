// Package framework provides a lightweight meta-module that re-exports from submodules
// This is the main entry point for the GoMind framework
// Users should import specific modules based on their needs:
//   - github.com/itsneelabh/gomind/core - For lightweight tools (8MB)
//   - github.com/itsneelabh/gomind/ai - For AI capabilities
//   - github.com/itsneelabh/gomind/telemetry - For observability
package framework

import (
	"context"
	
	"github.com/itsneelabh/gomind/core"
)

// Re-export core types for backward compatibility
type (
	// Core agent types
	Agent        = core.Agent
	BaseAgent    = core.BaseAgent
	Capability   = core.Capability
	
	// Configuration types
	Config            = core.Config
	Option            = core.Option
	HTTPConfig        = core.HTTPConfig
	CORSConfig        = core.CORSConfig
	DiscoveryConfig   = core.DiscoveryConfig
	AIConfig          = core.AIConfig
	TelemetryConfig   = core.TelemetryConfig
	MemoryConfig      = core.MemoryConfig
	ResilienceConfig  = core.ResilienceConfig
	LoggingConfig     = core.LoggingConfig
	DevelopmentConfig = core.DevelopmentConfig
	KubernetesConfig  = core.KubernetesConfig
	
	// Interfaces
	Logger       = core.Logger
	Discovery    = core.Discovery
	Memory       = core.Memory
	Telemetry    = core.Telemetry
	AIClient     = core.AIClient
	
	// Service types
	ServiceRegistration = core.ServiceRegistration
	HealthStatus       = core.HealthStatus
	
	// AI types
	AIOptions   = core.AIOptions
	AIResponse  = core.AIResponse
	TokenUsage  = core.TokenUsage
	
	// Telemetry types
	Span = core.Span
)

// Re-export constants
const (
	HealthHealthy   = core.HealthHealthy
	HealthUnhealthy = core.HealthUnhealthy
	HealthUnknown   = core.HealthUnknown
)

// Re-export core functions
var (
	NewBaseAgent             = core.NewBaseAgent
	NewBaseAgentWithConfig   = core.NewBaseAgentWithConfig
	NewFramework             = core.NewFramework
	NewRedisDiscovery        = core.NewRedisDiscovery
	NewMockDiscovery         = core.NewMockDiscovery
	NewInMemoryStore         = core.NewInMemoryStore
	NewConfig                = core.NewConfig
	DefaultConfig            = core.DefaultConfig
	
	// Configuration options
	WithName                 = core.WithName
	WithPort                 = core.WithPort
	WithAddress              = core.WithAddress
	WithNamespace            = core.WithNamespace
	WithCORS                 = core.WithCORS
	WithCORSDefaults         = core.WithCORSDefaults
	WithRedisURL             = core.WithRedisURL
	WithDiscovery            = core.WithDiscovery
	WithDiscoveryCacheEnabled = core.WithDiscoveryCacheEnabled
	WithOpenAIAPIKey         = core.WithOpenAIAPIKey
	WithAI                   = core.WithAI
	WithAIModel              = core.WithAIModel
	WithTelemetry            = core.WithTelemetry
	WithEnableMetrics        = core.WithEnableMetrics
	WithEnableTracing        = core.WithEnableTracing
	WithOTELEndpoint         = core.WithOTELEndpoint
	WithLogLevel             = core.WithLogLevel
	WithLogFormat            = core.WithLogFormat
	WithMemoryProvider       = core.WithMemoryProvider
	WithCircuitBreaker       = core.WithCircuitBreaker
	WithRetry                = core.WithRetry
	WithKubernetes           = core.WithKubernetes
	WithConfigFile           = core.WithConfigFile
	WithDevelopmentMode      = core.WithDevelopmentMode
	WithMockAI               = core.WithMockAI
	WithMockDiscovery        = core.WithMockDiscovery
)

// RunAgent provides a simplified way to run an agent
// DEPRECATED: Use NewFramework with options instead
func RunAgent(agent Agent, port int) error {
	framework, err := core.NewFramework(agent, core.WithPort(port))
	if err != nil {
		return err
	}
	ctx := context.Background()
	return framework.Run(ctx)
}