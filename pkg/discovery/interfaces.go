package discovery

import (
	"context"
	"time"
)

// Discovery interface for service discovery
type Discovery interface {
	Register(ctx context.Context, registration *AgentRegistration) error
	FindCapability(ctx context.Context, capability string) ([]AgentRegistration, error)
	FindAgent(ctx context.Context, agentID string) (*AgentRegistration, error)
	Unregister(ctx context.Context, agentID string) error
	GetHealthStatus(ctx context.Context) HealthStatus
	RefreshHeartbeat(ctx context.Context, agentID string) error
	Close() error
	
	// Phase 2: Catalog management methods
	DownloadFullCatalog(ctx context.Context) error
	GetFullCatalog() map[string]*AgentRegistration
	GetCatalogForLLM() string
	StartCatalogSync(ctx context.Context, interval time.Duration)
	GetCatalogStats() (agentCount int, lastSync time.Time, syncErrors int)
}

// AgentRegistration represents an agent's registration in service discovery
type AgentRegistration struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Address            string                 `json:"address"`
	Port               int                    `json:"port"`
	Capabilities       []CapabilityMetadata   `json:"capabilities"`
	Metadata           map[string]string      `json:"metadata"`
	LastHeartbeat      time.Time              `json:"last_heartbeat"`
	Status             AgentStatus            `json:"status"`
	KubernetesMetadata map[string]interface{} `json:"kubernetes_metadata,omitempty"`
	
	// Phase 2: K8s service information for inter-agent communication
	ServiceName     string `json:"service_name,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	ServiceEndpoint string `json:"service_endpoint,omitempty"`
	
	// Phase 2: Natural language descriptions for LLM consumption
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	LLMHints    string   `json:"llm_hints,omitempty"` // Additional hints for LLM routing
}

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	StatusHealthy   AgentStatus = "healthy"
	StatusUnhealthy AgentStatus = "unhealthy"
	StatusStarting  AgentStatus = "starting"
	StatusStopping  AgentStatus = "stopping"
)

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details"`
	Timestamp time.Time         `json:"timestamp"`
}

// CapabilityMetadata represents rich metadata for agent capabilities
type CapabilityMetadata struct {
	// Core Capability Identity
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Domain      string `json:"domain" yaml:"domain"`

	// Performance Characteristics
	Complexity      string  `json:"complexity" yaml:"complexity"`
	Latency         string  `json:"latency" yaml:"latency"`
	Cost            string  `json:"cost" yaml:"cost"`
	ConfidenceLevel float64 `json:"confidence_level" yaml:"confidence_level"`

	// Business Context
	BusinessValue []string `json:"business_value" yaml:"business_value"`
	UseCases      []string `json:"use_cases" yaml:"use_cases"`

	// LLM-Friendly Fields for Agentic Communication
	LLMPrompt   string   `json:"llm_prompt,omitempty" yaml:"llm_prompt,omitempty"`
	Specialties []string `json:"specialties,omitempty" yaml:"specialties,omitempty"`

	// Technical Requirements
	Prerequisites []string `json:"prerequisites" yaml:"prerequisites"`
	Dependencies  []string `json:"dependencies" yaml:"dependencies"`
	InputTypes    []string `json:"input_types" yaml:"input_types"`
	OutputFormats []string `json:"output_formats" yaml:"output_formats"`

	// Capability Relationships
	ComplementaryTo []string `json:"complementary_to" yaml:"complementary_to"`
	AlternativeTo   []string `json:"alternative_to" yaml:"alternative_to"`

	// Runtime Information (populated by framework)
	Method interface{}     `json:"-"` // reflect.Method for runtime execution
	Source *MetadataSource `json:"source,omitempty" yaml:"source,omitempty"`
}

// MetadataSource tracks where capability metadata came from
type MetadataSource struct {
	Type        string `json:"type" yaml:"type"` // "comment", "yaml", "reflection"
	File        string `json:"file,omitempty" yaml:"file,omitempty"`
	Line        int    `json:"line,omitempty" yaml:"line,omitempty"`
	LastUpdated string `json:"last_updated" yaml:"last_updated"`
}
