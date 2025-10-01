package core

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Logger interface - minimal logging interface
type Logger interface {
	// Basic logging methods
	Info(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Debug(msg string, fields map[string]interface{})

	// Context-aware methods for distributed tracing and request correlation
	InfoWithContext(ctx context.Context, msg string, fields map[string]interface{})
	ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{})
	WarnWithContext(ctx context.Context, msg string, fields map[string]interface{})
	DebugWithContext(ctx context.Context, msg string, fields map[string]interface{})
}

// Telemetry interface - optional telemetry support
type Telemetry interface {
	StartSpan(ctx context.Context, name string) (context.Context, Span)
	RecordMetric(name string, value float64, labels map[string]string)
}

// Span represents a telemetry span
type Span interface {
	End()
	SetAttribute(key string, value interface{})
	RecordError(err error)
}

// AIClient interface - optional AI support
type AIClient interface {
	GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
}

// AIOptions for AI generation
type AIOptions struct {
	Model        string
	Temperature  float32
	MaxTokens    int
	SystemPrompt string
}

// AIResponse from AI client
type AIResponse struct {
	Content string
	Model   string
	Usage   TokenUsage
}

// TokenUsage for AI responses
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Registry interface for tools (registration only)
type Registry interface {
	Register(ctx context.Context, info *ServiceInfo) error
	UpdateHealth(ctx context.Context, id string, status HealthStatus) error
	Unregister(ctx context.Context, id string) error
}

// Discovery interface for agents (registration + discovery)
type Discovery interface {
	Registry // Embed Registry
	Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
	// Backward compatibility methods
	FindService(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
	FindByCapability(ctx context.Context, capability string) ([]*ServiceInfo, error)
}

// CapabilityExample provides example usage of a capability
type CapabilityExample struct {
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output"`
}

// ServiceRegistration is deprecated - use ServiceInfo from component.go instead
type ServiceRegistration = ServiceInfo

// HealthStatus for services
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// Memory interface for state storage
type Memory interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// Default no-op implementations

// NoOpLogger provides a no-op logger implementation
type NoOpLogger struct{}

func (n *NoOpLogger) Info(msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) Error(msg string, fields map[string]interface{}) {}
func (n *NoOpLogger) Warn(msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) Debug(msg string, fields map[string]interface{}) {}

func (n *NoOpLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {}
func (n *NoOpLogger) WarnWithContext(ctx context.Context, msg string, fields map[string]interface{})  {}
func (n *NoOpLogger) DebugWithContext(ctx context.Context, msg string, fields map[string]interface{}) {}

// NoOpTelemetry provides a no-op telemetry implementation
type NoOpTelemetry struct{}

func (n *NoOpTelemetry) StartSpan(ctx context.Context, name string) (context.Context, Span) {
	return ctx, &NoOpSpan{}
}

func (n *NoOpTelemetry) RecordMetric(name string, value float64, labels map[string]string) {}

// NoOpSpan provides a no-op span implementation
type NoOpSpan struct{}

func (n *NoOpSpan) End()                                       {}
func (n *NoOpSpan) SetAttribute(key string, value interface{}) {}
func (n *NoOpSpan) RecordError(err error)                      {}

// InMemoryStore provides a simple in-memory implementation of Memory
type InMemoryStore struct {
	data map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]string),
	}
}

func (m *InMemoryStore) Get(ctx context.Context, key string) (string, error) {
	value, exists := m.data[key]
	if !exists {
		return "", nil
	}
	return value, nil
}

func (m *InMemoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *InMemoryStore) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *InMemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

// ============================================================================
// Global Registry Pattern for Telemetry Integration
// ============================================================================

// MetricsRegistry enables telemetry module to register itself with core
// This avoids circular dependencies while enabling metrics emission
type MetricsRegistry interface {
	Counter(name string, labels ...string)
	EmitWithContext(ctx context.Context, name string, value float64, labels ...string)
	GetBaggage(ctx context.Context) map[string]string
}

// Global registry - set by telemetry module when it initializes
var globalMetricsRegistry MetricsRegistry

// SetMetricsRegistry allows telemetry module to register itself
func SetMetricsRegistry(registry MetricsRegistry) {
	globalMetricsRegistry = registry

	// Enable metrics on all existing loggers
	enableMetricsOnExistingLoggers()
}

// GetGlobalMetricsRegistry returns the global metrics registry if available.
// Returns nil if telemetry module has not registered a metrics registry yet.
// This enables framework modules to emit metrics without creating circular dependencies.
//
// Usage pattern:
//   if registry := core.GetGlobalMetricsRegistry(); registry != nil {
//       registry.EmitWithContext(ctx, "metric.name", value, labels...)
//   }
func GetGlobalMetricsRegistry() MetricsRegistry {
	return globalMetricsRegistry
}

// Track created loggers to enable metrics when telemetry becomes available
var createdLoggers []*ProductionLogger
var loggersMutex sync.RWMutex

func trackLogger(logger *ProductionLogger) {
	loggersMutex.Lock()
	defer loggersMutex.Unlock()

	createdLoggers = append(createdLoggers, logger)

	// If metrics already available, enable immediately
	if globalMetricsRegistry != nil {
		logger.EnableMetrics()
	}
}

func enableMetricsOnExistingLoggers() {
	loggersMutex.Lock()
	defer loggersMutex.Unlock()

	for _, logger := range createdLoggers {
		logger.EnableMetrics()
	}
}
