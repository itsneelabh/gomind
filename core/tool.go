package core

import (
	"context"
	"encoding/json"
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
	Start(ctx context.Context, port int) error
	RegisterCapability(cap Capability)
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

	// Handler registration tracking (same as BaseAgent for consistency)
	registeredPatterns map[string]bool // Track registered patterns to prevent duplicates
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
		ID:                 config.ID,
		Name:               config.Name,
		Type:               ComponentTypeTool,
		Logger:             &NoOpLogger{},
		Memory:             NewInMemoryStore(),
		Telemetry:          &NoOpTelemetry{},
		Config:             config,
		mux:                http.NewServeMux(),
		registeredPatterns: make(map[string]bool), // Initialize pattern tracking
	}
}

// Initialize initializes the tool
func (t *BaseTool) Initialize(ctx context.Context) error {
	// 🔥 ADD: Enhanced initialization context
	t.Logger.Info("Starting tool initialization", map[string]interface{}{
		"id":                t.ID,
		"name":              t.Name,
		"type":              t.Type,
		"config_provided":   t.Config != nil,
		"discovery_enabled": t.Config != nil && t.Config.Discovery.Enabled,
		"namespace":         getNamespaceFromConfig(t.Config),
	})

	// Initialize components based on config (following BaseAgent pattern)
	if t.Config != nil {
		// Initialize registry if configured
		if t.Config.Discovery.Enabled && t.Registry == nil {
			// 🔥 ADD: Registry initialization visibility
			t.Logger.Info("Initializing service registry", map[string]interface{}{
				"provider":      t.Config.Discovery.Provider,
				"mock_mode":     t.Config.Development.MockDiscovery,
				"redis_url":     t.Config.Discovery.RedisURL != "",
			})

			if t.Config.Development.MockDiscovery {
				// Use mock registry for development
				t.Registry = NewMockDiscovery()
				// 🔥 ADD: Mock registry confirmation
				t.Logger.Info("Using mock registry for development", map[string]interface{}{
					"provider": "mock",
					"reason":   "development_mode",
				})
			} else if t.Config.Discovery.Provider == "redis" && t.Config.Discovery.RedisURL != "" {
				// Initialize Redis registry
				if registry, err := NewRedisRegistry(t.Config.Discovery.RedisURL); err == nil {
					// Set logger for better observability
					registry.SetLogger(t.Logger)
					t.Registry = registry
					// 🔥 ADD: Redis registry success
					t.Logger.Info("Redis registry initialized successfully", map[string]interface{}{
						"provider":  "redis",
						"redis_url": t.Config.Discovery.RedisURL,
					})
				} else {
					// Enhance existing error logging with dependency context
					t.Logger.Error("Failed to initialize Redis registry", map[string]interface{}{
						"error":      err,
						"error_type": fmt.Sprintf("%T", err),
						"redis_url":  t.Config.Discovery.RedisURL,
						// 🔥 ADD: Dependency impact context
						"impact":     "tool_will_run_without_registry",
						"fallback":   "manual_configuration_required",
					})
				}
			}
		}
	}

	// 🔥 ADD: Service registration attempt context
	if t.Registry != nil {
		address, port := ResolveServiceAddress(t.Config, t.Logger)

		t.Logger.Info("Attempting service registration", map[string]interface{}{
			"service_id":         t.ID,
			"service_name":       t.Name,
			"resolved_address":   address,
			"resolved_port":      port,
			"capabilities_count": len(t.Capabilities),
			"namespace":          getNamespaceFromConfig(t.Config),
		})

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

		// Start heartbeat to keep registration alive (Redis-specific)
		if redisRegistry, ok := t.Registry.(*RedisRegistry); ok {
			redisRegistry.StartHeartbeat(ctx, t.ID)
			t.Logger.Debug("Started heartbeat for tool registration", map[string]interface{}{
				"tool_id": t.ID,
				"ttl":     redisRegistry.ttl,
			})
		}
	} else {
		// 🔥 ADD: No registry context
		t.Logger.Warn("Tool running without service registry", map[string]interface{}{
			"reason":          "registry_not_configured",
			"impact":          "tool_not_discoverable",
			"manual_config":   "required_for_service_mesh",
		})
	}

	// 🔥 ADD: Initialization completion
	t.Logger.Info("Tool initialization completed", map[string]interface{}{
		"id":                t.ID,
		"name":              t.Name,
		"discovery_enabled": t.Registry != nil,
		"capabilities_count": len(t.Capabilities),
	})

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

// RegisterCapability registers a new capability for the tool.
// Follows the same pattern as BaseAgent for consistency.
// If cap.Handler is provided, it will be used instead of the generic handler.
// If cap.Endpoint is empty, it will be auto-generated as /api/capabilities/{name}.
func (t *BaseTool) RegisterCapability(cap Capability) {
	t.capMutex.Lock()
	defer t.capMutex.Unlock()
	
	// Auto-generate endpoint if not provided (same as Agent)
	if cap.Endpoint == "" {
		cap.Endpoint = fmt.Sprintf("/api/capabilities/%s", cap.Name)
	}
	
	t.Capabilities = append(t.Capabilities, cap)
	
	// Register HTTP endpoint (same pattern as Agent)
	if cap.Handler != nil {
		// Use custom handler if provided
		t.mux.HandleFunc(cap.Endpoint, cap.Handler)
	} else {
		// Use generic handler with telemetry and logging
		t.mux.HandleFunc(cap.Endpoint, t.handleCapabilityRequest(cap))
	}

	// Track this pattern to prevent duplicates
	t.registeredPatterns[cap.Endpoint] = true
	
	t.Logger.Info("Registered capability", map[string]interface{}{
		"name":           cap.Name,
		"endpoint":       cap.Endpoint,
		"custom_handler": cap.Handler != nil,
	})
}

// handleCapabilityRequest creates an HTTP handler for a capability.
// This provides a generic handler for capabilities without custom handlers.
func (t *BaseTool) handleCapabilityRequest(cap Capability) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Start telemetry span if available
		if t.Telemetry != nil {
			var span Span
			_, span = t.Telemetry.StartSpan(ctx, fmt.Sprintf("capability.%s", cap.Name))
			defer span.End()
			span.SetAttribute("capability.name", cap.Name)
			span.SetAttribute("component.type", "tool")
		}

		// Log request
		t.Logger.Info("Handling capability request", map[string]interface{}{
			"capability": cap.Name,
			"method":     r.Method,
			"tool":       t.Name,
		})

		// Since tools are passive and this is a generic handler,
		// we return capability information
		response := map[string]interface{}{
			"capability":   cap.Name,
			"description":  cap.Description,
			"input_types":  cap.InputTypes,
			"output_types": cap.OutputTypes,
			"message":      "This capability is registered but has no custom handler implementation",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			// Log error but response is already partially written
			t.Logger.Error("Failed to encode response", map[string]interface{}{
				"error":             err,
				"error_type":        fmt.Sprintf("%T", err),
				"tool_id":           t.ID,
				// 🔥 ADD: Request context for troubleshooting
				"request_method":    r.Method,
				"request_path":      r.URL.Path,
				"request_remote":    r.RemoteAddr,
				"capabilities_count": len(t.Capabilities),
				"user_agent":        r.Header.Get("User-Agent"),
				"content_length":    r.ContentLength,
			})
		}
	}
}

// setupStandardEndpoints adds standard endpoints like /api/capabilities and /health
func (t *BaseTool) setupStandardEndpoints() {
	// Add capabilities listing endpoint (same as Agent)
	capabilitiesPath := "/api/capabilities"
	if !t.registeredPatterns[capabilitiesPath] {
		t.mux.HandleFunc(capabilitiesPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(t.Capabilities); err != nil {
				// Log error but response is already partially written
				t.Logger.Error("Failed to encode capabilities", map[string]interface{}{
					"error":             err,
					"error_type":        fmt.Sprintf("%T", err),
					"tool_id":           t.ID,
					// 🔥 ADD: Request context for troubleshooting
					"request_method":    r.Method,
					"request_path":      r.URL.Path,
					"request_remote":    r.RemoteAddr,
					"capabilities_count": len(t.Capabilities),
					"user_agent":        r.Header.Get("User-Agent"),
					"content_length":    r.ContentLength,
				})
			}
		})
		t.registeredPatterns[capabilitiesPath] = true
	}

	// Add health endpoint if enabled (same as Agent)
	if t.Config != nil && t.Config.HTTP.EnableHealthCheck {
		healthPath := t.Config.HTTP.HealthCheckPath
		if healthPath == "" {
			healthPath = "/health"
		}
		if !t.registeredPatterns[healthPath] {
			t.mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "healthy",
					"type":   "tool",
					"name":   t.Name,
					"id":     t.ID,
				}); err != nil {
					// Log error but response is already partially written
					t.Logger.Error("Failed to encode health response", map[string]interface{}{
						"error":             err,
						"error_type":        fmt.Sprintf("%T", err),
						"tool_id":           t.ID,
						// 🔥 ADD: Request context for troubleshooting
						"request_method":    r.Method,
						"request_path":      r.URL.Path,
						"request_remote":    r.RemoteAddr,
						"capabilities_count": len(t.Capabilities),
						"user_agent":        r.Header.Get("User-Agent"),
						"content_length":    r.ContentLength,
					})
				}
			})
			t.registeredPatterns[healthPath] = true
		}
	}
}

// Start starts the HTTP server for the tool
func (t *BaseTool) Start(ctx context.Context, port int) error {
	// Apply configuration precedence: explicit parameter > config > default  
	// Only use Config.Port if no explicit port provided (port < 0)
	if port < 0 && t.Config != nil && t.Config.Port >= 0 {
		port = t.Config.Port
	}
	
	// Validate port range (0 is allowed for automatic assignment)
	if port < 0 || port > 65535 {
		// 🔥 ADD: Port validation with context
		t.Logger.Error("Invalid port specified", map[string]interface{}{
			"requested_port": port,
			"valid_range":    "0-65535",
			"port_zero_note": "0_enables_automatic_assignment",
		})
		return fmt.Errorf("invalid port %d: must be between 0-65535 (0 for automatic assignment)", port)
	}
	
	addr := fmt.Sprintf(":%d", port)
	
	// Use default timeouts if config is not provided
	if t.Config == nil {
		t.Config = DefaultConfig()
	}
	
	// Setup standard endpoints (/api/capabilities, /health)
	t.setupStandardEndpoints()

	// 🔥 ADD: Server configuration logging
	t.Logger.Info("Configuring HTTP server", map[string]interface{}{
		"port":                   port,
		"cors_enabled":           t.Config.HTTP.CORS.Enabled,
		"health_check_enabled":   t.Config.HTTP.EnableHealthCheck,
		"read_timeout":           t.Config.HTTP.ReadTimeout.String(),
		"write_timeout":          t.Config.HTTP.WriteTimeout.String(),
		"registered_endpoints":   len(t.registeredPatterns),
	})

	// 🔥 ADD: Endpoint registration summary
	if len(t.registeredPatterns) > 0 {
		endpoints := make([]string, 0, len(t.registeredPatterns))
		for pattern := range t.registeredPatterns {
			endpoints = append(endpoints, pattern)
		}
		t.Logger.Info("HTTP endpoints registered", map[string]interface{}{
			"endpoints":      endpoints,
			"total_count":    len(endpoints),
			"capabilities":   len(t.Capabilities),
		})
	}
	
	t.server = &http.Server{
		Addr:              addr,
		Handler:           t.mux,
		ReadTimeout:       t.Config.HTTP.ReadTimeout,
		ReadHeaderTimeout: t.Config.HTTP.ReadHeaderTimeout,
		WriteTimeout:      t.Config.HTTP.WriteTimeout,
		IdleTimeout:       t.Config.HTTP.IdleTimeout,
		MaxHeaderBytes:    t.Config.HTTP.MaxHeaderBytes,
	}

	// 🔥 ADD: Service update registration context
	if t.Registry != nil {
		// Use the shared resolver for proper K8s support
		address, registrationPort := ResolveServiceAddress(t.Config, t.Logger)
		t.Logger.Info("Updating service registration with server details", map[string]interface{}{
			"service_id":            t.ID,
			"registration_address":  address,
			"registration_port":     registrationPort,
			"server_port":           port,
		})

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

	t.Logger.Info("Starting HTTP server", map[string]interface{}{
		"address":           addr,
		"cors":              t.Config.HTTP.CORS.Enabled,
		"capabilities":      len(t.Capabilities),
		"registry_enabled":  t.Registry != nil,
	})

	// 🔥 ADD: Server startup failure context
	if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		t.Logger.Error("HTTP server failed to start", map[string]interface{}{
			"error":      err.Error(),
			"error_type": fmt.Sprintf("%T", err),
			"address":    addr,
			"port":       port,
		})
		return err
	}

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