package ui

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// DefaultRegistry is the global transport registry
var DefaultRegistry = NewTransportRegistry()

// transportRegistry implements TransportRegistry
type transportRegistry struct {
	mu         sync.RWMutex
	transports map[string]Transport
	listeners  []TransportEventHandler
	logger     core.Logger // Logger field for observability
}

// NewTransportRegistry creates a new transport registry
func NewTransportRegistry() TransportRegistry {
	return &transportRegistry{
		transports: make(map[string]Transport),
		listeners:  make([]TransportEventHandler, 0),
		logger:     &core.NoOpLogger{}, // Default to NoOpLogger
	}
}

// SetLogger sets the logger for this transport registry.
// This enables dependency injection of loggers from the ChatAgent.
// Follows the Interface-First Design principle from FRAMEWORK_DESIGN_PRINCIPLES.md.
// The component is always set to "framework/ui" to ensure proper log attribution
// regardless of which agent or tool is using the UI module.
func (r *transportRegistry) SetLogger(logger core.Logger) {
	if logger != nil {
		r.mu.Lock()
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			r.logger = cal.WithComponent("framework/ui")
		} else {
			r.logger = logger
		}
		r.mu.Unlock()
	}
}

// Register registers a new transport
func (r *transportRegistry) Register(transport Transport) error {
	if transport == nil {
		// ERROR: Invalid input
		if r.logger != nil {
			r.logger.Error("Transport registration failed", map[string]interface{}{
				"operation": "transport_register",
				"error":     "transport cannot be nil",
				"reason":    "invalid_input",
			})
		}
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("transport cannot be nil"))
	}

	name := transport.Name()
	if name == "" {
		// ERROR: Invalid transport name
		if r.logger != nil {
			r.logger.Error("Transport registration failed", map[string]interface{}{
				"operation": "transport_register",
				"error":     "transport name cannot be empty",
				"reason":    "invalid_name",
			})
		}
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("transport name cannot be empty"))
	}

	// DEBUG: Log detailed registration attempt
	if r.logger != nil {
		r.logger.Debug("Registering transport", map[string]interface{}{
			"operation":     "transport_register",
			"transport":     name,
			"priority":      transport.Priority(),
			"capabilities":  transport.Capabilities(),
			"available":     transport.Available(),
		})
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[name]; exists {
		// ERROR: Transport already exists
		if r.logger != nil {
			r.logger.Error("Transport registration failed", map[string]interface{}{
				"operation":  "transport_register",
				"transport":  name,
				"error":      "transport already exists",
				"reason":     "duplicate_registration",
				"registered_count": len(r.transports),
			})
		}
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportAlreadyExists, name))
	}

	r.transports[name] = transport

	// INFO: Important operational event - transport successfully registered
	if r.logger != nil {
		r.logger.Info("Transport registered successfully", map[string]interface{}{
			"operation":        "transport_register",
			"transport":        name,
			"priority":         transport.Priority(),
			"capabilities":     transport.Capabilities(),
			"available":        transport.Available(),
			"total_transports": len(r.transports),
			"registered_transports": r.getTransportNames(),
		})
	}

	// Notify listeners
	r.notifyListeners(TransportLifecycleEvent{
		Transport: name,
		Event:     EventTransportInitialized,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"priority":     transport.Priority(),
			"capabilities": transport.Capabilities(),
		},
	})

	return nil
}

// Unregister removes a transport from the registry
func (r *transportRegistry) Unregister(name string) error {
	if name == "" {
		// ERROR: Invalid input
		if r.logger != nil {
			r.logger.Error("Transport unregistration failed", map[string]interface{}{
				"operation": "transport_unregister",
				"error":     "transport name cannot be empty",
				"reason":    "invalid_name",
			})
		}
		return NewUIError("Registry.Unregister", ErrorKindTransport,
			fmt.Errorf("transport name cannot be empty"))
	}

	// DEBUG: Log unregistration attempt
	if r.logger != nil {
		r.logger.Debug("Unregistering transport", map[string]interface{}{
			"operation": "transport_unregister",
			"transport": name,
		})
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[name]; !exists {
		// ERROR: Transport not found
		if r.logger != nil {
			r.logger.Error("Transport unregistration failed", map[string]interface{}{
				"operation":        "transport_unregister",
				"transport":        name,
				"error":            "transport not found",
				"reason":           "not_registered",
				"registered_count": len(r.transports),
				"available_transports": r.getTransportNames(),
			})
		}
		return NewUIError("Registry.Unregister", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	delete(r.transports, name)

	// INFO: Important operational event - transport successfully unregistered
	if r.logger != nil {
		r.logger.Info("Transport unregistered successfully", map[string]interface{}{
			"operation":         "transport_unregister",
			"transport":         name,
			"remaining_transports": len(r.transports),
			"registered_transports": r.getTransportNames(),
		})
	}

	// Notify listeners
	r.notifyListeners(TransportLifecycleEvent{
		Transport: name,
		Event:     EventTransportStopped,
		Timestamp: time.Now(),
	})

	return nil
}

// Get retrieves a transport by name
func (r *transportRegistry) Get(name string) (Transport, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transport, exists := r.transports[name]
	return transport, exists
}

// List returns all registered transports
func (r *transportRegistry) List() []Transport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	transports := make([]Transport, 0, len(r.transports))
	for _, t := range r.transports {
		transports = append(transports, t)
	}

	// Sort by priority (highest first), then by name
	sort.Slice(transports, func(i, j int) bool {
		if transports[i].Priority() != transports[j].Priority() {
			return transports[i].Priority() > transports[j].Priority()
		}
		return transports[i].Name() < transports[j].Name()
	})

	return transports
}

// ListAvailable returns all available transports sorted by priority
func (r *transportRegistry) ListAvailable() []Transport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	available := make([]Transport, 0)
	for _, t := range r.transports {
		if t.Available() {
			available = append(available, t)
		}
	}

	// Sort by priority (highest first), then by name
	sort.Slice(available, func(i, j int) bool {
		if available[i].Priority() != available[j].Priority() {
			return available[i].Priority() > available[j].Priority()
		}
		return available[i].Name() < available[j].Name()
	})

	return available
}

// AddListener adds an event listener
func (r *transportRegistry) AddListener(handler TransportEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, handler)
}

// notifyListeners notifies all registered listeners of an event
func (r *transportRegistry) notifyListeners(event TransportLifecycleEvent) {
	for _, listener := range r.listeners {
		// Call listeners in goroutines to prevent blocking
		go listener(event)
	}
}

// MustRegister registers a transport and panics on error
// Use this in init() functions where errors cannot be handled
func MustRegister(transport Transport) {
	if err := DefaultRegistry.Register(transport); err != nil {
		panic(fmt.Sprintf("failed to register transport: %v", err))
	}
}

// RegisterTransport registers a transport with the default registry
func RegisterTransport(transport Transport) error {
	return DefaultRegistry.Register(transport)
}

// GetTransport retrieves a transport from the default registry
func GetTransport(name string) (Transport, bool) {
	return DefaultRegistry.Get(name)
}

// ListTransports returns all registered transports from the default registry
func ListTransports() []Transport {
	return DefaultRegistry.List()
}

// ListAvailableTransports returns all available transports from the default registry
func ListAvailableTransports() []Transport {
	return DefaultRegistry.ListAvailable()
}

// TransportSelector selects the best transport based on capabilities
type TransportSelector struct {
	registry TransportRegistry
}

// NewTransportSelector creates a new transport selector
func NewTransportSelector(registry TransportRegistry) *TransportSelector {
	if registry == nil {
		registry = DefaultRegistry
	}
	return &TransportSelector{
		registry: registry,
	}
}

// SelectBest selects the best available transport
func (s *TransportSelector) SelectBest() (Transport, error) {
	available := s.registry.ListAvailable()
	if len(available) == 0 {
		return nil, NewUIError("TransportSelector.SelectBest", ErrorKindTransport,
			ErrTransportNotAvailable)
	}

	// Available transports are already sorted by priority
	return available[0], nil
}

// SelectWithCapabilities selects the best transport with required capabilities
func (s *TransportSelector) SelectWithCapabilities(required []TransportCapability) (Transport, error) {
	available := s.registry.ListAvailable()

	for _, transport := range available {
		if hasAllCapabilities(transport, required) {
			return transport, nil
		}
	}

	return nil, NewUIError("TransportSelector.SelectWithCapabilities", ErrorKindTransport,
		fmt.Errorf("no transport found with required capabilities: %v", required))
}

// hasAllCapabilities checks if a transport has all required capabilities
func hasAllCapabilities(transport Transport, required []TransportCapability) bool {
	capabilities := transport.Capabilities()
	capMap := make(map[TransportCapability]bool)

	for _, cap := range capabilities {
		capMap[cap] = true
	}

	for _, req := range required {
		if !capMap[req] {
			return false
		}
	}

	return true
}

// TransportManager manages transport lifecycle
type TransportManager struct {
	registry TransportRegistry
	configs  map[string]TransportConfig
	logger   core.Logger // Logger field for comprehensive lifecycle observability
	mu       sync.RWMutex
}

// NewTransportManager creates a new transport manager
func NewTransportManager(registry TransportRegistry) *TransportManager {
	if registry == nil {
		registry = DefaultRegistry
	}
	return &TransportManager{
		registry: registry,
		configs:  make(map[string]TransportConfig),
		logger:   &core.NoOpLogger{}, // Default to NoOpLogger
	}
}

// NewTransportManagerWithLogger creates a new transport manager with logger
func NewTransportManagerWithLogger(registry TransportRegistry, logger core.Logger) *TransportManager {
	if registry == nil {
		registry = DefaultRegistry
	}
	if logger == nil {
		logger = &core.NoOpLogger{}
	}
	return &TransportManager{
		registry: registry,
		configs:  make(map[string]TransportConfig),
		logger:   logger,
	}
}

// InitializeTransport initializes a transport with configuration
func (m *TransportManager) InitializeTransport(name string, config TransportConfig) error {
	// Emit framework-level operation metrics
	if globalMetricsRegistry := core.GetGlobalMetricsRegistry(); globalMetricsRegistry != nil {
		globalMetricsRegistry.EmitWithContext(context.Background(), "gomind.ui.operations", 1.0,
			"level", "INFO",
			"service", "transport_manager",
			"component", "ui",
			"operation", "transport_initialize",
		)
	}

	// INFO: Log initialization attempt with comprehensive configuration details
	if m.logger != nil {
		m.logger.Info("Initializing transport", map[string]interface{}{
			"operation":              "transport_initialize",
			"transport":              name,
			"max_connections":        config.MaxConnections,
			"timeout":                config.Timeout.String(),
			"cors_enabled":           config.CORS.Enabled,
			"cors_allowed_origins":   config.CORS.AllowedOrigins,
			"cors_allowed_methods":   config.CORS.AllowedMethods,
			"cors_allowed_headers":   config.CORS.AllowedHeaders,
			"cors_max_age":           config.CORS.MaxAge,
			"rate_limit_enabled":     config.RateLimit.Enabled,
			"rate_limit_rpm":         config.RateLimit.RequestsPerMinute,
			"rate_limit_burst":       config.RateLimit.BurstSize,
			"custom_options_count":   len(config.Options),
			"custom_options_keys":    getOptionKeys(config.Options),
		})
	}

	transport, exists := m.registry.Get(name)
	if !exists {
		// ERROR: Transport not found in registry
		if m.logger != nil {
			m.logger.Error("Transport initialization failed", map[string]interface{}{
				"operation":  "transport_initialize",
				"transport":  name,
				"error":      "transport not found in registry",
				"error_type": "not_found",
			})
		}
		return NewUIError("TransportManager.InitializeTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	// Measure initialization performance
	startTime := time.Now()
	if err := transport.Initialize(config); err != nil {
		// ERROR: Transport initialization failure with comprehensive context
		if m.logger != nil {
			m.logger.Error("Transport initialization failed", map[string]interface{}{
				"operation":  "transport_initialize",
				"transport":  name,
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"duration":   time.Since(startTime).String(),
			})
		}
		return NewUIError("TransportManager.InitializeTransport", ErrorKindTransport,
			fmt.Errorf("failed to initialize transport %s: %w", name, err))
	}

	m.mu.Lock()
	m.configs[name] = config
	m.mu.Unlock()

	// INFO: Log successful initialization with transport details and performance metrics
	if m.logger != nil {
		m.logger.Info("Transport initialized successfully", map[string]interface{}{
			"operation": "transport_initialize",
			"transport": name,
			"duration":  time.Since(startTime).String(),
			"priority":  transport.Priority(),
			"available": transport.Available(),
		})
	}

	// Emit framework-level outcome metrics
	if globalMetricsRegistry := core.GetGlobalMetricsRegistry(); globalMetricsRegistry != nil {
		globalMetricsRegistry.EmitWithContext(context.Background(), "gomind.ui.transport.operations", 1.0,
			"operation", "initialize",
			"status", "success",
			"transport", name,
		)
		globalMetricsRegistry.EmitWithContext(context.Background(), "gomind.ui.transport.duration", float64(time.Since(startTime).Milliseconds()),
			"operation", "initialize",
			"transport", name,
		)
	}

	return nil
}

// StartTransport starts a transport
func (m *TransportManager) StartTransport(ctx context.Context, name string) error {
	// Emit framework-level operation metrics
	if globalMetricsRegistry := core.GetGlobalMetricsRegistry(); globalMetricsRegistry != nil {
		globalMetricsRegistry.EmitWithContext(ctx, "gomind.ui.operations", 1.0,
			"level", "INFO",
			"service", "transport_manager",
			"component", "ui",
			"operation", "transport_start",
		)
	}

	// INFO: Log startup attempt
	if m.logger != nil {
		m.logger.Info("Starting transport", map[string]interface{}{
			"operation": "transport_start",
			"transport": name,
		})
	}

	transport, exists := m.registry.Get(name)
	if !exists {
		// ERROR: Transport not found in registry
		if m.logger != nil {
			m.logger.Error("Transport startup failed", map[string]interface{}{
				"operation":  "transport_start",
				"transport":  name,
				"error":      "transport not found in registry",
				"error_type": "not_found",
			})
		}
		return NewUIError("TransportManager.StartTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	// Measure startup performance
	startTime := time.Now()
	if err := transport.Start(ctx); err != nil {
		// ERROR: Transport startup failure with comprehensive context
		if m.logger != nil {
			m.logger.Error("Transport startup failed", map[string]interface{}{
				"operation":        "transport_start",
				"transport":        name,
				"error":            err.Error(),
				"error_type":       fmt.Sprintf("%T", err),
				"startup_duration": time.Since(startTime).String(),
			})
		}
		return NewUIError("TransportManager.StartTransport", ErrorKindTransport,
			fmt.Errorf("failed to start transport %s: %w", name, err))
	}

	// INFO: Log successful startup with performance metrics
	if m.logger != nil {
		m.logger.Info("Transport started successfully", map[string]interface{}{
			"operation":        "transport_start",
			"transport":        name,
			"startup_duration": time.Since(startTime).String(),
			"priority":         transport.Priority(),
			"available":        transport.Available(),
		})
	}

	// Emit framework-level outcome metrics
	if globalMetricsRegistry := core.GetGlobalMetricsRegistry(); globalMetricsRegistry != nil {
		globalMetricsRegistry.EmitWithContext(ctx, "gomind.ui.transport.operations", 1.0,
			"operation", "start",
			"status", "success",
			"transport", name,
		)
		globalMetricsRegistry.EmitWithContext(ctx, "gomind.ui.transport.duration", float64(time.Since(startTime).Milliseconds()),
			"operation", "start",
			"transport", name,
		)
	}

	return nil
}

// StopTransport stops a transport
func (m *TransportManager) StopTransport(ctx context.Context, name string) error {
	// Determine shutdown type based on context deadline
	shutdownType := "graceful"
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		timeToDeadline := time.Until(deadline)
		if timeToDeadline < 5*time.Second {
			shutdownType = "forced"
		}
	}

	// INFO: Log shutdown attempt with context information
	if m.logger != nil {
		m.logger.Info("Stopping transport", map[string]interface{}{
			"operation":     "transport_stop",
			"transport":     name,
			"shutdown_type": shutdownType,
		})
	}

	transport, exists := m.registry.Get(name)
	if !exists {
		// ERROR: Transport not found in registry
		if m.logger != nil {
			m.logger.Error("Transport shutdown failed", map[string]interface{}{
				"operation":  "transport_stop",
				"transport":  name,
				"error":      "transport not found in registry",
				"error_type": "not_found",
			})
		}
		return NewUIError("TransportManager.StopTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	// Measure shutdown performance
	startTime := time.Now()
	if err := transport.Stop(ctx); err != nil {
		// ERROR: Transport shutdown failure with comprehensive context
		if m.logger != nil {
			m.logger.Error("Transport shutdown failed", map[string]interface{}{
				"operation":         "transport_stop",
				"transport":         name,
				"error":             err.Error(),
				"error_type":        fmt.Sprintf("%T", err),
				"shutdown_duration": time.Since(startTime).String(),
				"shutdown_type":     shutdownType,
			})
		}
		return NewUIError("TransportManager.StopTransport", ErrorKindTransport,
			fmt.Errorf("failed to stop transport %s: %w", name, err))
	}

	// INFO: Log successful shutdown with performance metrics
	if m.logger != nil {
		m.logger.Info("Transport stopped successfully", map[string]interface{}{
			"operation":         "transport_stop",
			"transport":         name,
			"shutdown_duration": time.Since(startTime).String(),
			"shutdown_type":     shutdownType,
		})
	}

	return nil
}

// HealthCheckTransport performs a health check on a transport
func (m *TransportManager) HealthCheckTransport(ctx context.Context, name string) error {
	// DEBUG: Log health check attempt
	if m.logger != nil {
		m.logger.Debug("Performing transport health check", map[string]interface{}{
			"operation": "transport_health_check",
			"transport": name,
		})
	}

	transport, exists := m.registry.Get(name)
	if !exists {
		// ERROR: Transport not found in registry
		if m.logger != nil {
			m.logger.Error("Transport health check failed", map[string]interface{}{
				"operation":  "transport_health_check",
				"transport":  name,
				"error":      "transport not found in registry",
				"error_type": "not_found",
				"status":     "unhealthy",
			})
		}
		return NewUIError("TransportManager.HealthCheckTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	// Measure health check response time
	startTime := time.Now()
	if err := transport.HealthCheck(ctx); err != nil {
		responseTime := time.Since(startTime)
		// WARN: Health check failure with response time and context
		if m.logger != nil {
			m.logger.Warn("Transport health check failed", map[string]interface{}{
				"operation":     "transport_health_check",
				"transport":     name,
				"error":         err.Error(),
				"error_type":    fmt.Sprintf("%T", err),
				"response_time": responseTime.String(),
				"status":        "unhealthy",
			})
		}
		return NewUIError("TransportManager.HealthCheckTransport", ErrorKindTransport,
			fmt.Errorf("transport %s health check failed: %w", name, err))
	}

	responseTime := time.Since(startTime)
	// INFO: Log successful health check with response time
	if m.logger != nil {
		m.logger.Info("Transport health check passed", map[string]interface{}{
			"operation":     "transport_health_check",
			"transport":     name,
			"response_time": responseTime.String(),
			"status":        "healthy",
			"priority":      transport.Priority(),
			"available":     transport.Available(),
		})
	}

	return nil
}

// StartAll starts all available transports
func (m *TransportManager) StartAll(ctx context.Context) error {
	startTime := time.Now()
	available := m.registry.ListAvailable()

	// INFO: Log bulk start operation with comprehensive summary
	if m.logger != nil {
		transportNames := make([]string, len(available))
		for i, t := range available {
			transportNames[i] = t.Name()
		}
		m.logger.Info("Starting all available transports", map[string]interface{}{
			"operation":           "transport_start_all",
			"available_count":     len(available),
			"transport_names":     transportNames,
		})
	}

	configuredCount := 0
	startedCount := 0
	var firstErr error

	for _, transport := range available {
		transportName := transport.Name()

		// Initialize with default config if not configured
		if _, configured := m.configs[transport.Name()]; !configured {
			config := TransportConfig{
				MaxConnections: 100,
				Timeout:        30 * time.Second,
			}

			// DEBUG: Log default configuration usage
			if m.logger != nil {
				m.logger.Debug("Using default config for transport", map[string]interface{}{
					"operation":       "transport_start_all",
					"transport":       transportName,
					"max_connections": config.MaxConnections,
					"timeout":         config.Timeout.String(),
					"config_source":   "default",
				})
			}

			if err := transport.Initialize(config); err != nil {
				if m.logger != nil {
					m.logger.Error("Transport initialization failed during bulk start", map[string]interface{}{
						"operation":  "transport_start_all",
						"transport":  transportName,
						"error":      err.Error(),
						"error_type": fmt.Sprintf("%T", err),
					})
				}
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to initialize %s: %w", transport.Name(), err)
				}
				continue
			}
			configuredCount++
		}

		// Start the transport
		if err := transport.Start(ctx); err != nil {
			if m.logger != nil {
				m.logger.Error("Transport startup failed during bulk start", map[string]interface{}{
					"operation":  "transport_start_all",
					"transport":  transportName,
					"error":      err.Error(),
					"error_type": fmt.Sprintf("%T", err),
				})
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to start %s: %w", transport.Name(), err)
			}
			continue
		}
		startedCount++

		// DEBUG: Log individual transport success
		if m.logger != nil {
			m.logger.Debug("Transport started successfully during bulk operation", map[string]interface{}{
				"operation": "transport_start_all",
				"transport": transportName,
				"priority":  transport.Priority(),
			})
		}
	}

	totalDuration := time.Since(startTime)
	successRate := 0.0
	if len(available) > 0 {
		successRate = float64(startedCount) / float64(len(available)) * 100
	}

	// INFO: Log final bulk operation summary
	if m.logger != nil {
		m.logger.Info("Bulk transport start operation completed", map[string]interface{}{
			"operation":         "transport_start_all",
			"total_available":   len(available),
			"configured_count":  configuredCount,
			"started_count":     startedCount,
			"failed_count":      len(available) - startedCount,
			"success_rate":      fmt.Sprintf("%.1f%%", successRate),
			"total_duration":    totalDuration.String(),
		})
	}

	return firstErr
}

// StopAll stops all transports
func (m *TransportManager) StopAll(ctx context.Context) error {
	startTime := time.Now()
	transports := m.registry.List()

	// Determine shutdown type based on context deadline
	shutdownType := "graceful"
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		timeToDeadline := time.Until(deadline)
		if timeToDeadline < 5*time.Second {
			shutdownType = "forced"
		}
	}

	// INFO: Log bulk shutdown operation with comprehensive summary
	if m.logger != nil {
		transportNames := make([]string, len(transports))
		for i, t := range transports {
			transportNames[i] = t.Name()
		}
		m.logger.Info("Stopping all transports", map[string]interface{}{
			"operation":        "transport_stop_all",
			"total_count":      len(transports),
			"transport_names":  transportNames,
			"shutdown_type":    shutdownType,
		})
	}

	stoppedCount := 0
	failedCount := 0
	var firstErr error

	for _, transport := range transports {
		transportName := transport.Name()
		transportStartTime := time.Now()

		if err := transport.Stop(ctx); err != nil {
			failedCount++
			if m.logger != nil {
				m.logger.Error("Transport shutdown failed during bulk stop", map[string]interface{}{
					"operation":         "transport_stop_all",
					"transport":         transportName,
					"error":             err.Error(),
					"error_type":        fmt.Sprintf("%T", err),
					"shutdown_duration": time.Since(transportStartTime).String(),
					"shutdown_type":     shutdownType,
				})
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		stoppedCount++
		// DEBUG: Log individual transport shutdown success
		if m.logger != nil {
			m.logger.Debug("Transport stopped successfully during bulk operation", map[string]interface{}{
				"operation":         "transport_stop_all",
				"transport":         transportName,
				"shutdown_duration": time.Since(transportStartTime).String(),
				"shutdown_type":     shutdownType,
			})
		}
	}

	totalDuration := time.Since(startTime)
	successRate := 0.0
	if len(transports) > 0 {
		successRate = float64(stoppedCount) / float64(len(transports)) * 100
	}

	// INFO: Log final bulk shutdown summary with cleanup status
	logLevel := "Info"
	if failedCount > 0 {
		logLevel = "Warn"
	}

	cleanupStatus := "complete"
	if failedCount > 0 {
		cleanupStatus = "partial"
	}

	if m.logger != nil {
		logData := map[string]interface{}{
			"operation":       "transport_stop_all",
			"total_count":     len(transports),
			"stopped_count":   stoppedCount,
			"failed_count":    failedCount,
			"success_rate":    fmt.Sprintf("%.1f%%", successRate),
			"total_duration":  totalDuration.String(),
			"shutdown_type":   shutdownType,
			"cleanup_status":  cleanupStatus,
		}

		if logLevel == "Warn" {
			m.logger.Warn("Bulk transport shutdown completed with failures", logData)
		} else {
			m.logger.Info("Bulk transport shutdown completed successfully", logData)
		}
	}

	return firstErr
}

// getTransportNames returns a list of currently registered transport names for logging
// Note: This method assumes the caller holds the appropriate lock
func (r *transportRegistry) getTransportNames() []string {
	names := make([]string, 0, len(r.transports))
	for name := range r.transports {
		names = append(names, name)
	}
	return names
}
