package core

import (
	"context"
	"time"
)

// ComponentType distinguishes between tools and agents
type ComponentType string

const (
	ComponentTypeTool  ComponentType = "tool"
	ComponentTypeAgent ComponentType = "agent"
)

// Component is the base interface for all components in the framework
type Component interface {
	Initialize(ctx context.Context) error
	GetID() string
	GetName() string
	GetCapabilities() []Capability
	GetType() ComponentType
}

// ServiceInfo replaces ServiceRegistration for unified registry
type ServiceInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         ComponentType          `json:"type"`
	Description  string                 `json:"description"`
	Address      string                 `json:"address"`
	Port         int                    `json:"port"`
	Capabilities []Capability           `json:"capabilities"`
	Metadata     map[string]interface{} `json:"metadata"`
	Health       HealthStatus           `json:"health"`
	LastSeen     time.Time              `json:"last_seen"`
}

// DiscoveryFilter allows filtering during discovery
type DiscoveryFilter struct {
	Type         ComponentType          `json:"type,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}