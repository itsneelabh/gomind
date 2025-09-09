package ui

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultRegistry is the global transport registry
var DefaultRegistry = NewTransportRegistry()

// transportRegistry implements TransportRegistry
type transportRegistry struct {
	mu         sync.RWMutex
	transports map[string]Transport
	listeners  []TransportEventHandler
}

// NewTransportRegistry creates a new transport registry
func NewTransportRegistry() TransportRegistry {
	return &transportRegistry{
		transports: make(map[string]Transport),
		listeners:  make([]TransportEventHandler, 0),
	}
}

// Register registers a new transport
func (r *transportRegistry) Register(transport Transport) error {
	if transport == nil {
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("transport cannot be nil"))
	}

	name := transport.Name()
	if name == "" {
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("transport name cannot be empty"))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[name]; exists {
		return NewUIError("Registry.Register", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportAlreadyExists, name))
	}

	r.transports[name] = transport

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
		return NewUIError("Registry.Unregister", ErrorKindTransport,
			fmt.Errorf("transport name cannot be empty"))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.transports[name]; !exists {
		return NewUIError("Registry.Unregister", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	delete(r.transports, name)

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
	}
}

// InitializeTransport initializes a transport with configuration
func (m *TransportManager) InitializeTransport(name string, config TransportConfig) error {
	transport, exists := m.registry.Get(name)
	if !exists {
		return NewUIError("TransportManager.InitializeTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	if err := transport.Initialize(config); err != nil {
		return NewUIError("TransportManager.InitializeTransport", ErrorKindTransport,
			fmt.Errorf("failed to initialize transport %s: %w", name, err))
	}

	m.mu.Lock()
	m.configs[name] = config
	m.mu.Unlock()

	return nil
}

// StartTransport starts a transport
func (m *TransportManager) StartTransport(ctx context.Context, name string) error {
	transport, exists := m.registry.Get(name)
	if !exists {
		return NewUIError("TransportManager.StartTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	if err := transport.Start(ctx); err != nil {
		return NewUIError("TransportManager.StartTransport", ErrorKindTransport,
			fmt.Errorf("failed to start transport %s: %w", name, err))
	}

	return nil
}

// StopTransport stops a transport
func (m *TransportManager) StopTransport(ctx context.Context, name string) error {
	transport, exists := m.registry.Get(name)
	if !exists {
		return NewUIError("TransportManager.StopTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	if err := transport.Stop(ctx); err != nil {
		return NewUIError("TransportManager.StopTransport", ErrorKindTransport,
			fmt.Errorf("failed to stop transport %s: %w", name, err))
	}

	return nil
}

// HealthCheckTransport performs a health check on a transport
func (m *TransportManager) HealthCheckTransport(ctx context.Context, name string) error {
	transport, exists := m.registry.Get(name)
	if !exists {
		return NewUIError("TransportManager.HealthCheckTransport", ErrorKindTransport,
			fmt.Errorf("%w: %s", ErrTransportNotFound, name))
	}

	if err := transport.HealthCheck(ctx); err != nil {
		return NewUIError("TransportManager.HealthCheckTransport", ErrorKindTransport,
			fmt.Errorf("transport %s health check failed: %w", name, err))
	}

	return nil
}

// StartAll starts all available transports
func (m *TransportManager) StartAll(ctx context.Context) error {
	available := m.registry.ListAvailable()

	for _, transport := range available {
		// Initialize with default config if not configured
		if _, configured := m.configs[transport.Name()]; !configured {
			config := TransportConfig{
				MaxConnections: 100,
				Timeout:        30 * time.Second,
			}
			if err := transport.Initialize(config); err != nil {
				return fmt.Errorf("failed to initialize %s: %w", transport.Name(), err)
			}
		}

		// Start the transport
		if err := transport.Start(ctx); err != nil {
			return fmt.Errorf("failed to start %s: %w", transport.Name(), err)
		}
	}

	return nil
}

// StopAll stops all transports
func (m *TransportManager) StopAll(ctx context.Context) error {
	transports := m.registry.List()

	var firstErr error
	for _, transport := range transports {
		if err := transport.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
