package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/itsneelabh/gomind/core"
	"github.com/itsneelabh/gomind/telemetry"
)

// CapabilityProvider defines the interface for providing agent/tool capabilities to the orchestrator
type CapabilityProvider interface {
	// GetCapabilities returns relevant agent/tool capabilities for a given request
	// Returns formatted capability information ready for LLM consumption
	GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error)
}

// DefaultCapabilityProvider uses the existing AgentCatalog.FormatForLLM approach
type DefaultCapabilityProvider struct {
	catalog *AgentCatalog
}

// NewDefaultCapabilityProvider creates a provider using the existing catalog approach
func NewDefaultCapabilityProvider(catalog *AgentCatalog) *DefaultCapabilityProvider {
	return &DefaultCapabilityProvider{
		catalog: catalog,
	}
}

// GetCapabilities returns all agents/tools formatted for LLM
func (d *DefaultCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
	// Use existing catalog.FormatForLLM() method
	return d.catalog.FormatForLLM(), nil
}

// ServiceCapabilityProvider queries an external service for relevant capabilities
type ServiceCapabilityProvider struct {
	endpoint        string
	client          *http.Client
	timeout         time.Duration
	topK            int
	threshold       float64
	
	// Optional dependencies (injected by application)
	circuitBreaker  core.CircuitBreaker  // Optional: sophisticated resilience
	logger          core.Logger          // Optional: observability
	telemetry       core.Telemetry       // Optional: metrics
	fallback        CapabilityProvider   // Optional: graceful degradation
	
	// Built-in simple resilience (when no circuit breaker provided)
	retryAttempts   int
	retryDelay      time.Duration
	failureCount    int
	lastFailureTime time.Time
	mu              sync.RWMutex
}


// NewServiceCapabilityProvider creates a provider with intelligent configuration
func NewServiceCapabilityProvider(config *ServiceCapabilityConfig) *ServiceCapabilityProvider {
	// Apply intelligent defaults and environment variable precedence
	if config == nil {
		config = &ServiceCapabilityConfig{}
	}

	// Environment variable precedence (per framework principles)
	// 1. Explicit configuration (already set in config)
	// 2. Standard environment variables
	// 3. GOMIND_* prefixed variables
	// 4. Sensible defaults

	if config.Endpoint == "" {
		// Check environment variables
		if endpoint := os.Getenv("CAPABILITY_SERVICE_URL"); endpoint != "" {
			config.Endpoint = endpoint
		} else if endpoint := os.Getenv("GOMIND_CAPABILITY_SERVICE_URL"); endpoint != "" {
			config.Endpoint = endpoint
		}
		// No default endpoint - this must be explicitly configured
	}

	if config.TopK == 0 {
		// Check environment variables
		if topK := os.Getenv("GOMIND_CAPABILITY_TOP_K"); topK != "" {
			if k, err := strconv.Atoi(topK); err == nil {
				config.TopK = k
			}
		}
		if config.TopK == 0 {
			config.TopK = 20 // Sensible default
		}
	}

	if config.Threshold == 0 {
		// Check environment variables
		if threshold := os.Getenv("GOMIND_CAPABILITY_THRESHOLD"); threshold != "" {
			if t, err := strconv.ParseFloat(threshold, 64); err == nil {
				config.Threshold = t
			}
		}
		if config.Threshold == 0 {
			config.Threshold = 0.7 // Sensible default
		}
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Use TracedHTTPClient for distributed tracing context propagation
	tracedClient := telemetry.NewTracedHTTPClient(nil)
	tracedClient.Timeout = config.Timeout

	return &ServiceCapabilityProvider{
		endpoint:       config.Endpoint,
		client:         tracedClient,
		timeout:        config.Timeout,
		topK:           config.TopK,
		threshold:      config.Threshold,
		circuitBreaker: config.CircuitBreaker,  // May be nil (optional)
		logger:         config.Logger,          // May be nil (optional)
		telemetry:      config.Telemetry,       // May be nil (optional)
		fallback:       config.FallbackProvider,// May be nil (optional)
		retryAttempts:  3,                      // Default retry attempts
		retryDelay:     2 * time.Second,        // Base retry delay
	}
}

// GetCapabilities queries external service with resilience layers
func (s *ServiceCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
	// Layer 2: Use injected circuit breaker if provided
	if s.circuitBreaker != nil {
		var result string
		err := s.circuitBreaker.Execute(ctx, func() error {
			var err error
			result, err = s.queryExternalService(ctx, request, metadata)
			return err
		})
		
		if err != nil {
			// Layer 3: Try fallback for graceful degradation
			if s.fallback != nil {
				s.logDebug("Circuit breaker open, using fallback provider")
				return s.fallback.GetCapabilities(ctx, request, metadata)
			}
			return "", fmt.Errorf("capability service failed: %w", err)
		}
		return result, nil
	}
	
	// Layer 1: Use simple built-in resilience (when no circuit breaker provided)
	return s.getCapabilitiesWithSimpleResilience(ctx, request, metadata)
}

// getCapabilitiesWithSimpleResilience provides basic resilience when no circuit breaker is injected
func (s *ServiceCapabilityProvider) getCapabilitiesWithSimpleResilience(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
	// Simple circuit breaker check
	if s.isCircuitOpen() {
		if s.fallback != nil {
			s.logDebug("Simple circuit open, using fallback provider")
			return s.fallback.GetCapabilities(ctx, request, metadata)
		}
		return "", fmt.Errorf("capability service circuit open, too many recent failures")
	}
	
	// Try with exponential backoff retry
	var lastErr error
	for attempt := 0; attempt <= s.retryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(attempt) * s.retryDelay
			s.logDebug(fmt.Sprintf("Retrying after %v (attempt %d/%d)", delay, attempt, s.retryAttempts))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		
		result, err := s.queryExternalService(ctx, request, metadata)
		if err == nil {
			s.recordSuccess()
			return result, nil
		}
		lastErr = err
		s.logError(fmt.Sprintf("Attempt %d failed: %v", attempt+1, err))
	}
	
	// All retries failed
	s.recordFailure()
	
	// Layer 3: Try fallback for graceful degradation
	if s.fallback != nil {
		s.logDebug("All retries failed, using fallback provider")
		return s.fallback.GetCapabilities(ctx, request, metadata)
	}
	
	return "", fmt.Errorf("capability service unavailable after %d retries: %w", s.retryAttempts, lastErr)
}

// isCircuitOpen checks if too many recent failures occurred
func (s *ServiceCapabilityProvider) isCircuitOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// If no failures yet, circuit is closed
	if s.failureCount == 0 {
		return false
	}
	
	// Reset failure count after cooldown period
	if !s.lastFailureTime.IsZero() && time.Since(s.lastFailureTime) > 30*time.Second {
		return false
	}
	
	// Open circuit after 5 consecutive failures
	return s.failureCount >= 5
}

// recordSuccess resets failure tracking
func (s *ServiceCapabilityProvider) recordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureCount = 0
}

// recordFailure increments failure count
func (s *ServiceCapabilityProvider) recordFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureCount++
	s.lastFailureTime = time.Now()
}

// queryExternalService performs the actual HTTP call
func (s *ServiceCapabilityProvider) queryExternalService(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
	// Construct the request payload according to the contract
	req := CapabilityRequest{
		Query:     request,
		Metadata:  metadata,
		TopK:      s.topK,      // Use configured value
		Threshold: s.threshold, // Use configured value
	}

	// Marshal request to JSON
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP POST request with context
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.endpoint+"/capabilities", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute the HTTP request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to query capability service: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Error can be safely ignored as we've read the body
	}()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		var errorBody bytes.Buffer
		_, err := errorBody.ReadFrom(resp.Body)
		if err != nil {
			return "", fmt.Errorf("capability service returned %d: unable to read error body: %w", resp.StatusCode, err)
		}
		return "", fmt.Errorf("capability service returned %d: %s", resp.StatusCode, errorBody.String())
	}

	// Decode the JSON response according to the contract
	var capabilityResp CapabilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&capabilityResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Validate response
	if capabilityResp.Capabilities == "" {
		return "", fmt.Errorf("capability service returned empty capabilities")
	}

	return capabilityResp.Capabilities, nil
}

// logDebug logs debug messages if logger is available
func (s *ServiceCapabilityProvider) logDebug(msg string) {
	if s.logger != nil {
		s.logger.Debug(msg, nil)
	}
}

// logError logs error messages if logger is available
func (s *ServiceCapabilityProvider) logError(msg string) {
	if s.logger != nil {
		s.logger.Error(msg, nil)
	}
}

// Health checks if the external service is healthy
func (s *ServiceCapabilityProvider) Health(ctx context.Context) error {
	// Implement health check for monitoring (framework requirement)
	req, err := http.NewRequestWithContext(ctx, "GET", s.endpoint+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close() // Error can be safely ignored after health check
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// CapabilityRequest defines the request to external capability service
type CapabilityRequest struct {
	Query     string                 `json:"query"`     // User's natural language request
	Metadata  map[string]interface{} `json:"metadata"`  // Optional metadata
	TopK      int                    `json:"top_k"`     // Number of results to return
	Threshold float64                `json:"threshold"` // Minimum similarity threshold
}

// CapabilityResponse defines the response from external capability service
type CapabilityResponse struct {
	Capabilities   string `json:"capabilities"`     // Formatted capabilities for LLM
	AgentsFound    int    `json:"agents_found"`     // Number of agents found
	ToolsFound     int    `json:"tools_found"`      // Number of tools found
	SearchMethod   string `json:"search_method"`    // Method used (e.g., "vector_similarity")
	ProcessingTime string `json:"processing_time"`  // Time taken to process (e.g., "100ms")
}