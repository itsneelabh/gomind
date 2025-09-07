package core

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

// generateID generates a unique ID for components
func generateID() string {
	return uuid.New().String()[:8]
}

// Tool interface - tools have NO discovery methods (compile-time safety)
type Tool interface {
	Component
	// Tools cannot discover other components
}

// BaseTool provides the core tool functionality
// Tools are passive components that only respond to requests
type BaseTool struct {
	ID           string
	Name         string
	Type         ComponentType
	Capabilities []Capability
	capMutex     sync.RWMutex // Protects capabilities slice

	// Dependencies (all modules can enhance tools)
	Registry  Registry  // Can register only - no discovery
	Logger    Logger
	Memory    Memory
	Telemetry Telemetry // Telemetry still works
	AI        AIClient  // Can be AI-enhanced

	// Configuration
	Config *Config // Configuration for K8s support and more

	// HTTP server
	server *http.Server
	mux    *http.ServeMux
}

// NewTool creates a new tool with default implementations
func NewTool(name string) *BaseTool {
	config := DefaultConfig()
	config.Name = name
	return NewToolWithConfig(config)
}

// NewToolWithConfig creates a new tool with custom configuration
func NewToolWithConfig(config *Config) *BaseTool {
	if config == nil {
		config = DefaultConfig()
	}

	// Ensure name is set
	if config.Name == "" {
		config.Name = "gomind-tool"
	}

	// Generate ID if not set
	if config.ID == "" {
		config.ID = fmt.Sprintf("%s-%s", config.Name, generateID())
	}

	return &BaseTool{
		ID:        config.ID,
		Name:      config.Name,
		Type:      ComponentTypeTool,
		Logger:    &NoOpLogger{},
		Memory:    NewInMemoryStore(),
		Telemetry: &NoOpTelemetry{},
		Config:    config,
		mux:       http.NewServeMux(),
	}
}

// Initialize initializes the tool
func (t *BaseTool) Initialize(ctx context.Context) error {
	t.Logger.Info("Initializing tool", map[string]interface{}{
		"name": t.Name,
		"id":   t.ID,
		"type": t.Type,
	})

	// Register with registry if available
	if t.Registry != nil && t.Config != nil && t.Config.Discovery.Enabled {
		// Use the shared resolver to determine address
		address, port := ResolveServiceAddress(t.Config, t.Logger)
		
		info := &ServiceInfo{
			ID:           t.ID,
			Name:         t.Name,
			Type:         t.Type,
			Address:      address,
			Port:         port,
			Capabilities: t.Capabilities,
			Health:       HealthHealthy,
			Metadata:     BuildServiceMetadata(t.Config),
		}
		if err := t.Registry.Register(ctx, info); err != nil {
			return fmt.Errorf("failed to register tool: %w", err)
		}
	}

	return nil
}

// GetID returns the tool's unique identifier
func (t *BaseTool) GetID() string {
	return t.ID
}

// GetName returns the tool's name
func (t *BaseTool) GetName() string {
	return t.Name
}

// GetCapabilities returns the tool's capabilities
func (t *BaseTool) GetCapabilities() []Capability {
	t.capMutex.RLock()
	defer t.capMutex.RUnlock()
	
	// Return a copy to prevent external modification
	caps := make([]Capability, len(t.Capabilities))
	copy(caps, t.Capabilities)
	return caps
}

// GetType returns ComponentTypeTool
func (t *BaseTool) GetType() ComponentType {
	return t.Type
}

// RegisterCapability registers a new capability for the tool
func (t *BaseTool) RegisterCapability(cap Capability) {
	t.capMutex.Lock()
	defer t.capMutex.Unlock()
	
	t.Capabilities = append(t.Capabilities, cap)
	
	// Register HTTP handler if provided
	if cap.Handler != nil && cap.Endpoint != "" {
		t.mux.HandleFunc(cap.Endpoint, cap.Handler)
	}
	
	t.Logger.Info("Registered capability", map[string]interface{}{
		"tool":       t.Name,
		"capability": cap.Name,
		"endpoint":   cap.Endpoint,
	})
}

// Start starts the HTTP server for the tool
func (t *BaseTool) Start(ctx context.Context, port int) error {
	// Override port from config if provided
	if t.Config != nil && t.Config.Port > 0 {
		port = t.Config.Port
	}
	
	addr := fmt.Sprintf(":%d", port)
	t.server = &http.Server{
		Addr:    addr,
		Handler: t.mux,
	}

	t.Logger.Info("Starting tool HTTP server", map[string]interface{}{
		"tool":    t.Name,
		"address": addr,
	})

	// Update registration with resolved address
	if t.Registry != nil {
		// Use the shared resolver for proper K8s support
		address, registrationPort := ResolveServiceAddress(t.Config, t.Logger)
		
		info := &ServiceInfo{
			ID:           t.ID,
			Name:         t.Name,
			Type:         t.Type,
			Address:      address,
			Port:         registrationPort,
			Capabilities: t.Capabilities,
			Health:       HealthHealthy,
			Metadata:     BuildServiceMetadata(t.Config),
		}
		if err := t.Registry.Register(ctx, info); err != nil {
			t.Logger.Error("Failed to update registration", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Start server in background
	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logger.Error("HTTP server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the tool
func (t *BaseTool) Shutdown(ctx context.Context) error {
	t.Logger.Info("Shutting down tool", map[string]interface{}{
		"name": t.Name,
	})

	// Unregister from registry
	if t.Registry != nil {
		if err := t.Registry.Unregister(ctx, t.ID); err != nil {
			t.Logger.Error("Failed to unregister", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Shutdown HTTP server
	if t.server != nil {
		return t.server.Shutdown(ctx)
	}

	return nil
}