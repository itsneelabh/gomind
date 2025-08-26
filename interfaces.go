package framework

import (
	"github.com/itsneelabh/gomind/pkg/ai"
	"github.com/itsneelabh/gomind/pkg/discovery"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/memory"
	"github.com/itsneelabh/gomind/pkg/telemetry"
)

// Type aliases for backward compatibility
type Discovery = discovery.Discovery
type Memory = memory.Memory
type Logger = logger.Logger
type AutoOTEL = telemetry.AutoOTEL
type AIClient = ai.AIClient
type AIResponse = ai.AIResponse
type AIStreamChunk = ai.AIStreamChunk
type GenerationOptions = ai.GenerationOptions
type TokenUsage = ai.TokenUsage
type ProviderInfo = ai.ProviderInfo
type AgentRegistration = discovery.AgentRegistration
type AgentStatus = discovery.AgentStatus
type HealthStatus = discovery.HealthStatus

// Constants for backward compatibility
const (
	StatusHealthy   = discovery.StatusHealthy
	StatusUnhealthy = discovery.StatusUnhealthy
	StatusStarting  = discovery.StatusStarting
	StatusStopping  = discovery.StatusStopping
)
