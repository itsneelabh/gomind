package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Agent interface - agents have full discovery capabilities
type Agent interface {
	Component
	Start(ctx context.Context, port int) error
	RegisterCapability(cap Capability)
	Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error)
}

// HTTPComponent represents a component that can be run with HTTP server.
// Both Tools and Agents implement this interface, allowing the Framework
// to work with both types of components uniformly.
type HTTPComponent interface {
	Component
	Start(ctx context.Context, port int) error
	RegisterCapability(cap Capability)
}

// Capability represents a capability that an agent provides
type Capability struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Endpoint    string           `json:"endpoint"`
	InputTypes  []string         `json:"input_types"`
	OutputTypes []string         `json:"output_types"`
	Handler     http.HandlerFunc `json:"-"` // Optional custom handler, excluded from JSON
}

// BaseAgent provides the core agent functionality
// Agents are active components that can discover and orchestrate both tools and agents
type BaseAgent struct {
	// Core fields (always available)
	ID           string
	Name         string
	Type         ComponentType
	Capabilities []Capability
	Logger       Logger
	Discovery    Discovery // Agents get full discovery powers
	Memory       Memory

	// Optional fields (set by modules)
	Telemetry Telemetry
	AI        AIClient

	// Configuration
	Config *Config

	// HTTP server
	server *http.Server
	mux    *http.ServeMux

	// Handler registration tracking
	registeredPatterns map[string]bool // Track registered patterns to prevent duplicates
	serverStarted      bool            // Track if server has started
	mu                 sync.RWMutex    // Protect concurrent access
}

// NewBaseAgent creates a new base agent with minimal dependencies
func NewBaseAgent(name string) *BaseAgent {
	config := DefaultConfig()
	config.Name = name
	return NewBaseAgentWithConfig(config)
}

// NewBaseAgentWithConfig creates a new base agent with configuration
func NewBaseAgentWithConfig(config *Config) *BaseAgent {
	if config == nil {
		config = DefaultConfig()
	}

	// Ensure name is set
	if config.Name == "" {
		config.Name = "gomind-agent"
	}

	// Generate ID if not set
	if config.ID == "" {
		config.ID = fmt.Sprintf("%s-%s", config.Name, uuid.New().String()[:8])
	}

	return &BaseAgent{
		ID:                 config.ID,
		Name:               config.Name,
		Type:               ComponentTypeAgent,
		Capabilities:       []Capability{},
		Logger:             &NoOpLogger{},      // Will be initialized based on config
		Memory:             NewInMemoryStore(), // Will be initialized based on config
		Telemetry:          &NoOpTelemetry{},   // Will be initialized based on config
		Config:             config,
		mux:                http.NewServeMux(),
		registeredPatterns: make(map[string]bool),
		serverStarted:      false,
	}
}

// Initialize initializes the agent
func (b *BaseAgent) Initialize(ctx context.Context) error {
	b.Logger.Info("Starting agent initialization", map[string]interface{}{
		"id":                 b.ID,
		"name":               b.Name,
		"type":               b.Type,
		"config_provided":    b.Config != nil,
		"discovery_enabled":  b.Config != nil && b.Config.Discovery.Enabled,
		"namespace":          getNamespaceFromConfig(b.Config),
	})

	// Initialize components based on config
	if b.Config != nil {
		// Initialize discovery if configured
		if b.Config.Discovery.Enabled && b.Discovery == nil {
			b.Logger.Info("Initializing service discovery", map[string]interface{}{
				"provider":      b.Config.Discovery.Provider,
				"mock_mode":     b.Config.Development.MockDiscovery,
				"redis_url":     b.Config.Discovery.RedisURL != "",
			})

			if b.Config.Development.MockDiscovery {
				// Use mock discovery for development
				b.Discovery = NewMockDiscovery()
				b.Logger.Info("Using mock discovery for development", map[string]interface{}{
					"provider": "mock",
					"reason":   "development_mode",
				})
			} else if b.Config.Discovery.Provider == "redis" && b.Config.Discovery.RedisURL != "" {
				// Initialize Redis discovery
				if discovery, err := NewRedisDiscovery(b.Config.Discovery.RedisURL); err == nil {
					// Set logger for better observability
					discovery.SetLogger(b.Logger)
					b.Discovery = discovery
					b.Logger.Info("Redis discovery initialized successfully", map[string]interface{}{
						"provider":  "redis",
						"redis_url": b.Config.Discovery.RedisURL,
					})
				} else {
					// Enhance existing error logging with dependency context
					b.Logger.Error("Failed to initialize Redis discovery", map[string]interface{}{
						"error":      err,
						"error_type": fmt.Sprintf("%T", err),
						"redis_url":  b.Config.Discovery.RedisURL,
						"impact":     "agent_will_run_without_discovery",
						"fallback":   "manual_configuration_required",
					})
				}
			}
		}

		// Initialize memory based on config
		if b.Config.Memory.Provider == "redis" && b.Config.Memory.RedisURL != "" {
			// TODO: Initialize Redis memory when available
			b.Memory = NewInMemoryStore()
		} else {
			b.Memory = NewInMemoryStore()
		}
	}

	if b.Discovery != nil {
		address, port := ResolveServiceAddress(b.Config, b.Logger)

		b.Logger.Info("Attempting service registration", map[string]interface{}{
			"service_id":         b.ID,
			"service_name":       b.Name,
			"resolved_address":   address,
			"resolved_port":      port,
			"capabilities_count": len(b.Capabilities),
			"namespace":          getNamespaceFromConfig(b.Config),
		})

		capabilities := make([]string, len(b.Capabilities))
		for i, cap := range b.Capabilities {
			capabilities[i] = cap.Name
		}

		registration := &ServiceInfo{
			ID:           b.ID,
			Name:         b.Name,
			Type:         b.Type,
			Address:      address,
			Port:         port,
			Capabilities: b.Capabilities,
			Health:       HealthHealthy,
			LastSeen:     time.Now(),
			Metadata:     BuildServiceMetadata(b.Config),
		}

		if err := b.Discovery.Register(ctx, registration); err != nil {
			b.Logger.Error("Failed to register with discovery", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"agent_id":   b.ID,
				"agent_name": b.Name,
			})
			// Continue anyway - graceful degradation
		} else {
			// Start heartbeat to keep registration alive (Redis-specific)
			if redisDiscovery, ok := b.Discovery.(*RedisDiscovery); ok {
				redisDiscovery.StartHeartbeat(ctx, b.ID)
				b.Logger.Debug("Started heartbeat for agent registration", map[string]interface{}{
					"agent_id": b.ID,
					"ttl":      redisDiscovery.ttl,
				})
			}
		}
	} else {
		b.Logger.Warn("Agent running without service discovery", map[string]interface{}{
			"reason":          "discovery_not_configured",
			"impact":          "agent_not_discoverable",
			"manual_config":   "required_for_service_mesh",
		})
	}

	b.Logger.Info("Agent initialization completed", map[string]interface{}{
		"id":                 b.ID,
		"name":               b.Name,
		"discovery_enabled":  b.Discovery != nil,
		"capabilities_count": len(b.Capabilities),
	})

	return nil
}

// determineRegistrationAddress is deprecated - use ResolveServiceAddress instead.
// Kept for backward compatibility but delegates to the shared resolver.
func (b *BaseAgent) determineRegistrationAddress() (string, int) {
	return ResolveServiceAddress(b.Config, b.Logger)
}

// getNamespaceFromConfig safely extracts namespace from config for logging
func getNamespaceFromConfig(config *Config) string {
	if config == nil {
		return ""
	}
	return config.Namespace
}

// GetID returns the agent ID
func (b *BaseAgent) GetID() string {
	return b.ID
}

// GetName returns the agent name
func (b *BaseAgent) GetName() string {
	return b.Name
}

// GetCapabilities returns the agent capabilities
func (b *BaseAgent) GetCapabilities() []Capability {
	return b.Capabilities
}

// GetType returns ComponentTypeAgent
func (b *BaseAgent) GetType() ComponentType {
	return b.Type
}

// Discover allows agents to discover both tools and other agents
func (b *BaseAgent) Discover(ctx context.Context, filter DiscoveryFilter) ([]*ServiceInfo, error) {
	if b.Discovery == nil {
		return nil, fmt.Errorf("discovery not configured for agent %s", b.Name)
	}
	return b.Discovery.Discover(ctx, filter)
}

// HandleFunc registers a custom HTTP handler for the given pattern.
// This method must be called before Start() is invoked.
// It returns an error if:
//   - The server has already been started
//   - The pattern has already been registered
//
// Example:
//
//	agent := core.NewBaseAgent("my-agent")
//	err := agent.HandleFunc("/api/custom", myHandler)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (b *BaseAgent) HandleFunc(pattern string, handler http.HandlerFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if server has already started
	if b.serverStarted {
		// Keep the exact error message for backward compatibility with tests
		return fmt.Errorf("cannot register handler for pattern %s: server already started", pattern)
	}

	// Check for duplicate pattern registration
	if b.registeredPatterns[pattern] {
		// Keep the exact error message for backward compatibility with tests
		return fmt.Errorf("handler already registered for pattern: %s", pattern)
	}

	// Register the handler
	b.mux.HandleFunc(pattern, handler)
	b.registeredPatterns[pattern] = true

	// Log the registration
	b.Logger.Info("Registered custom handler", map[string]interface{}{
		"pattern": pattern,
	})

	return nil
}

// RegisterCapability registers a new capability with optional custom handler.
// If cap.Handler is provided, it will be used instead of the generic handler.
// If cap.Endpoint is empty, it will be auto-generated as /api/capabilities/{name}.
func (b *BaseAgent) RegisterCapability(cap Capability) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Auto-generate endpoint if not provided
	endpoint := cap.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("/api/capabilities/%s", cap.Name)
	}

	// Update the capability's endpoint for consistency
	cap.Endpoint = endpoint

	// Append to capabilities list
	b.Capabilities = append(b.Capabilities, cap)

	// Register HTTP endpoint for the capability
	if cap.Handler != nil {
		// Use custom handler if provided (no automatic telemetry/logging)
		b.mux.HandleFunc(endpoint, cap.Handler)
	} else {
		// Use generic handler with telemetry and logging
		b.mux.HandleFunc(endpoint, b.handleCapabilityRequest(cap))
	}

	// Track this pattern internally
	b.registeredPatterns[endpoint] = true

	b.Logger.Info("Registered capability", map[string]interface{}{
		"name":           cap.Name,
		"endpoint":       endpoint,
		"custom_handler": cap.Handler != nil,
	})
}

// handleCapabilityRequest creates an HTTP handler for a capability
func (b *BaseAgent) handleCapabilityRequest(cap Capability) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Start telemetry span if available
		if b.Telemetry != nil {
			var span Span
			_, span = b.Telemetry.StartSpan(ctx, fmt.Sprintf("capability.%s", cap.Name))
			defer span.End()
			span.SetAttribute("capability.name", cap.Name)
		}

		// Log request
		b.Logger.Info("Handling capability request", map[string]interface{}{
			"capability": cap.Name,
			"method":     r.Method,
		})

		// Parse request
		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			b.Logger.Error("Failed to parse request", map[string]interface{}{
				"error":      err,
				"error_type": fmt.Sprintf("%T", err),
				"path":       r.URL.Path,
				"method":     r.Method,
			})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO: Actual capability implementation would go here
		// This is where tool-specific logic would be implemented

		// Return response
		response := map[string]interface{}{
			"capability": cap.Name,
			"status":     "success",
			"result":     "Tool capability executed successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			// Log error but response is already partially written
			if b.Logger != nil {
				b.Logger.Error("Failed to encode response", map[string]interface{}{
					"error":             err,
					"error_type":        fmt.Sprintf("%T", err),
					"agent_id":          b.ID,
					"request_method":    r.Method,
					"request_path":      r.URL.Path,
					"request_remote":    r.RemoteAddr,
					"capabilities_count": len(b.Capabilities),
					"user_agent":        r.Header.Get("User-Agent"),
					"content_length":    r.ContentLength,
				})
			}
		}
	}
}

// Start starts the HTTP server for the agent
func (b *BaseAgent) Start(ctx context.Context, port int) error {
	b.mu.Lock()

	// Check if already started
	if b.serverStarted {
		b.mu.Unlock()
		return fmt.Errorf("server already started")
	}

	// Apply configuration precedence: explicit parameter > config > default
	// Only use Config.Port if no explicit port provided (port < 0)
	if port < 0 && b.Config != nil && b.Config.Port >= 0 {
		port = b.Config.Port
	}

	// Validate port range (0 is allowed for automatic assignment)
	if port < 0 || port > 65535 {
		b.mu.Unlock()
		b.Logger.Error("Invalid port specified", map[string]interface{}{
			"requested_port": port,
			"valid_range":    "0-65535",
			"port_zero_note": "0_enables_automatic_assignment",
		})
		return fmt.Errorf("invalid port %d: must be between 0-65535 (0 for automatic assignment)", port)
	}

	addr := fmt.Sprintf("%s:%d", b.Config.Address, port)
	if b.Config.Address == "" {
		addr = fmt.Sprintf(":%d", port)
	}

	b.Logger.Info("Configuring HTTP server", map[string]interface{}{
		"port":                   port,
		"cors_enabled":           b.Config.HTTP.CORS.Enabled,
		"health_check_enabled":   b.Config.HTTP.EnableHealthCheck,
		"read_timeout":           b.Config.HTTP.ReadTimeout.String(),
		"write_timeout":          b.Config.HTTP.WriteTimeout.String(),
		"registered_endpoints":   len(b.registeredPatterns),
	})

	// Add health endpoint if enabled
	if b.Config.HTTP.EnableHealthCheck {
		healthPath := b.Config.HTTP.HealthCheckPath
		// Check if health path is already registered (shouldn't be, but be safe)
		if !b.registeredPatterns[healthPath] {
			b.mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "healthy",
					"agent":  b.Name,
					"id":     b.ID,
				}); err != nil {
					// Log error but response is already partially written
					if b.Logger != nil {
						b.Logger.Error("Failed to encode health response", map[string]interface{}{
							"error":             err,
							"error_type":        fmt.Sprintf("%T", err),
							"agent_id":          b.ID,
									"request_method":    r.Method,
							"request_path":      r.URL.Path,
							"request_remote":    r.RemoteAddr,
							"capabilities_count": len(b.Capabilities),
							"user_agent":        r.Header.Get("User-Agent"),
							"content_length":    r.ContentLength,
						})
					}
				}
			})
			b.registeredPatterns[healthPath] = true
		}
	}

	// Add capabilities listing endpoint
	capabilitiesPath := "/api/capabilities"
	if !b.registeredPatterns[capabilitiesPath] {
		b.mux.HandleFunc(capabilitiesPath, func(w http.ResponseWriter, r *http.Request) {
			ApplyCORS(w, r, &b.Config.HTTP.CORS)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(b.Capabilities); err != nil {
				// Log error but response is already partially written
				if b.Logger != nil {
					b.Logger.Error("Failed to encode capabilities", map[string]interface{}{
						"error":             err,
						"error_type":        fmt.Sprintf("%T", err),
						"agent_id":          b.ID,
							"request_method":    r.Method,
						"request_path":      r.URL.Path,
						"request_remote":    r.RemoteAddr,
						"capabilities_count": len(b.Capabilities),
						"user_agent":        r.Header.Get("User-Agent"),
						"content_length":    r.ContentLength,
					})
				}
			}
		})
		b.registeredPatterns[capabilitiesPath] = true
	}

	if len(b.registeredPatterns) > 0 {
		endpoints := make([]string, 0, len(b.registeredPatterns))
		for pattern := range b.registeredPatterns {
			endpoints = append(endpoints, pattern)
		}
		b.Logger.Info("HTTP endpoints registered", map[string]interface{}{
			"endpoints":      endpoints,
			"total_count":    len(endpoints),
			"capabilities":   len(b.Capabilities),
		})
	}

	// Create handler with CORS middleware if enabled
	var handler http.Handler = b.mux
	if b.Config.HTTP.CORS.Enabled {
		handler = CORSMiddleware(&b.Config.HTTP.CORS)(handler)
	}

	// Always wrap with panic recovery middleware
	handler = RecoveryMiddleware(b.Logger)(handler)

	b.server = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       b.Config.HTTP.ReadTimeout,
		ReadHeaderTimeout: b.Config.HTTP.ReadHeaderTimeout,
		WriteTimeout:      b.Config.HTTP.WriteTimeout,
		IdleTimeout:       b.Config.HTTP.IdleTimeout,
		MaxHeaderBytes:    b.Config.HTTP.MaxHeaderBytes,
	}

	if b.Discovery != nil {
		address, registrationPort := ResolveServiceAddress(b.Config, b.Logger)
		b.Logger.Info("Updating service registration with server details", map[string]interface{}{
			"service_id":            b.ID,
			"registration_address":  address,
			"registration_port":     registrationPort,
			"server_port":           port,
		})
	}

	// Mark server as started (before actually starting to prevent race conditions)
	b.serverStarted = true
	b.mu.Unlock() // Unlock before blocking ListenAndServe call

	b.Logger.Info("Starting HTTP server", map[string]interface{}{
		"address":           addr,
		"cors":              b.Config.HTTP.CORS.Enabled,
		"capabilities":      len(b.Capabilities),
		"discovery_enabled": b.Discovery != nil,
	})

	if err := b.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		b.Logger.Error("HTTP server failed to start", map[string]interface{}{
			"error":      err.Error(),
			"error_type": fmt.Sprintf("%T", err),
			"address":    addr,
			"port":       port,
		})
		return err
	}

	return nil
}

// Stop stops the HTTP server
func (b *BaseAgent) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.server != nil {
		// Use configured shutdown timeout or context deadline
		shutdownCtx := ctx
		if b.Config != nil && b.Config.HTTP.ShutdownTimeout > 0 {
			var cancel context.CancelFunc
			shutdownCtx, cancel = context.WithTimeout(ctx, b.Config.HTTP.ShutdownTimeout)
			defer cancel()
		}

		// Unregister from discovery if available
		if b.Discovery != nil && b.Config.Discovery.Enabled {
			if err := b.Discovery.Unregister(shutdownCtx, b.ID); err != nil {
				b.Logger.Error("Failed to unregister from discovery", map[string]interface{}{
					"error":      err,                    // Preserve full error object
					"error_type": fmt.Sprintf("%T", err), // Log error type for debugging
					"agent_id":   b.ID,
					"operation":  "unregister",
				})
			}
		}

		// Reset server state
		b.serverStarted = false

		return b.server.Shutdown(shutdownCtx)
	}
	return nil
}

// RecoveryMiddleware creates a middleware that recovers from panics in HTTP handlers
func RecoveryMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					stackTrace := debug.Stack()
					if logger != nil {
						logger.Error("HTTP handler panic recovered", map[string]interface{}{
							"panic":      err,
							"error_type": fmt.Sprintf("%T", err),
							"path":       r.URL.Path,
							"method":     r.Method,
							"stack":      string(stackTrace),
						})
					} else {
						// Fallback to standard logging if no logger available
						fmt.Printf("HTTP handler panic recovered: %v\nPath: %s\nMethod: %s\nStack trace:\n%s\n",
							err, r.URL.Path, r.Method, stackTrace)
					}

					// Return Internal Server Error to client
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Framework provides a simple way to run components (both Tools and Agents)
type Framework struct {
	component HTTPComponent
	config    *Config
}

// NewFramework creates a new framework instance with options.
// It accepts any HTTPComponent (Tool or Agent) and provides uniform initialization and execution.
func NewFramework(component HTTPComponent, opts ...Option) (*Framework, error) {
	// Create configuration with options
	config, err := NewConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Update config for BaseAgent or BaseTool
	switch base := component.(type) {
	case *BaseAgent:
		base.Config = config
		base.Name = config.Name
		if config.ID != "" {
			base.ID = config.ID
		}
		base.Logger = config.logger

	case *BaseTool:
		base.Config = config
		base.Name = config.Name
		if config.ID != "" {
			base.ID = config.ID
		}
		base.Logger = config.logger
	}

	return &Framework{
		component: component,
		config:    config,
	}, nil
}

// Run initializes and starts the component (Tool or Agent)
func (f *Framework) Run(ctx context.Context) error {
	// Initialize component
	if err := f.component.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize component: %w", err)
	}

	// Start HTTP server
	return f.component.Start(ctx, f.config.Port)
}
