package communication

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/pkg/discovery"
	"github.com/itsneelabh/gomind/pkg/logger"
	"github.com/itsneelabh/gomind/pkg/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// K8sCommunicator implements AgentCommunicator for Kubernetes environments
type K8sCommunicator struct {
	discovery        discovery.Discovery
	httpClient       *http.Client
	logger           logger.Logger
	defaultNamespace string
	clusterDomain    string
	servicePort      int
	
	// For testing - allows overriding the service URL builder
	serviceURLBuilder func(agentName, namespace string) string
}

// NewK8sCommunicator creates a new K8s-based agent communicator
func NewK8sCommunicator(discovery discovery.Discovery, logger logger.Logger, namespace string) *K8sCommunicator {
	return &K8sCommunicator{
		discovery: discovery,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:           logger,
		defaultNamespace: namespace,
		clusterDomain:    "cluster.local",
		servicePort:      8080,
	}
}

// CallAgent sends a natural language instruction to another agent
func (k *K8sCommunicator) CallAgent(ctx context.Context, agentIdentifier string, instruction string) (string, error) {
	return k.CallAgentWithTimeout(ctx, agentIdentifier, instruction, 30*time.Second)
}

// CallAgentWithTimeout sends an instruction with a custom timeout
func (k *K8sCommunicator) CallAgentWithTimeout(ctx context.Context, agentIdentifier string, instruction string, timeout time.Duration) (string, error) {
	// Start a new span for this operation
	tracer := otel.Tracer("gomind.communication")
	ctx, span := tracer.Start(ctx, "CallAgent",
		trace.WithAttributes(
			attribute.String("agent.identifier", agentIdentifier),
			attribute.Int("instruction.length", len(instruction)),
			attribute.Float64("timeout.seconds", timeout.Seconds()),
		),
	)
	defer span.End()
	
	// Parse agent identifier (could be "agent-name" or "agent-name.namespace")
	agentName, namespace := k.parseAgentIdentifier(agentIdentifier)
	
	// Add agent details to span
	span.SetAttributes(
		attribute.String("agent.name", agentName),
		attribute.String("agent.namespace", namespace),
	)
	
	// Build service URL using K8s convention
	var serviceURL string
	if k.serviceURLBuilder != nil {
		serviceURL = k.serviceURLBuilder(agentName, namespace)
	} else {
		serviceURL = k.buildServiceURL(agentName, namespace)
	}
	
	span.SetAttributes(attribute.String("service.url", serviceURL))
	
	// Enrich log fields with correlation IDs
	logFields := telemetry.EnrichLogFields(ctx, map[string]interface{}{
		"agent":      agentName,
		"namespace":  namespace,
		"url":        serviceURL,
		"timeout":    timeout.String(),
		"instruction_preview": k.truncateString(instruction, 50),
	})
	k.logger.Info("Calling agent", logFields)
	
	// Create context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Create HTTP request
	url := fmt.Sprintf("%s/process", serviceURL)
	req, err := http.NewRequestWithContext(reqCtx, "POST", url, strings.NewReader(instruction))
	if err != nil {
		return "", &CommunicationError{
			Agent:   agentIdentifier,
			Message: "failed to create request",
			Cause:   err,
		}
	}
	
	// Set headers
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-From-Agent", k.getLocalAgentID())
	
	// Inject correlation IDs
	telemetry.InjectCorrelationHeaders(ctx, req.Header)
	
	// If no request ID was in context, generate one
	if req.Header.Get(telemetry.HeaderRequestID) == "" {
		req.Header.Set(telemetry.HeaderRequestID, uuid.New().String())
	}
	
	// Properly inject trace context using W3C Trace Context propagation
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	
	// Send request with retry logic
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
			k.logger.Debug("Retrying agent call", map[string]interface{}{
				"agent":   agentIdentifier,
				"attempt": attempt + 1,
			})
		}
		
		resp, err := k.httpClient.Do(req)
		if err != nil {
			lastErr = err
			span.RecordError(err)
			span.SetAttributes(attribute.Int("retry.attempt", attempt+1))
			continue
		}
		defer resp.Body.Close()
		
		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			span.RecordError(err)
			continue
		}
		
		// Check response status
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
			span.SetAttributes(
				attribute.Int("http.status_code", resp.StatusCode),
				attribute.String("http.status_text", http.StatusText(resp.StatusCode)),
			)
			
			// Don't retry on 4xx errors (client errors)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				span.SetStatus(codes.Error, "Client error")
				break
			}
			continue
		}
		
		// Success
		span.SetAttributes(
			attribute.Int("response.size", len(body)),
			attribute.Int("retry.attempts", attempt+1),
			attribute.Bool("success", true),
		)
		span.SetStatus(codes.Ok, "Agent call successful")
		
		k.logger.Info("Agent call successful", telemetry.EnrichLogFields(ctx, map[string]interface{}{
			"agent":         agentIdentifier,
			"response_size": len(body),
			"attempt":       attempt + 1,
		}))
		
		return string(body), nil
	}
	
	// All retries failed
	span.RecordError(lastErr)
	span.SetStatus(codes.Error, "All retries exhausted")
	return "", &CommunicationError{
		Agent:   agentIdentifier,
		Message: "all retries exhausted",
		Cause:   lastErr,
	}
}

// GetAvailableAgents returns a list of available agents from discovery
func (k *K8sCommunicator) GetAvailableAgents(ctx context.Context) ([]AgentInfo, error) {
	// Phase 2: Use the full catalog from discovery
	catalog := k.discovery.GetFullCatalog()
	
	var agents []AgentInfo
	for _, agent := range catalog {
		// Convert discovery.AgentRegistration to communication.AgentInfo
		info := AgentInfo{
			Name:        agent.Name,
			Namespace:   agent.Namespace,
			ServiceName: agent.ServiceName,
			Description: agent.Description,
			Status:      string(agent.Status),
			LastSeen:    agent.LastHeartbeat.Format(time.RFC3339),
		}
		
		// Extract capability names
		for _, cap := range agent.Capabilities {
			info.Capabilities = append(info.Capabilities, cap.Name)
		}
		
		agents = append(agents, info)
	}
	
	k.logger.Debug("Listing available agents", map[string]interface{}{
		"count": len(agents),
	})
	
	return agents, nil
}

// Ping checks if an agent is reachable
func (k *K8sCommunicator) Ping(ctx context.Context, agentIdentifier string) error {
	agentName, namespace := k.parseAgentIdentifier(agentIdentifier)
	
	var serviceURL string
	if k.serviceURLBuilder != nil {
		serviceURL = k.serviceURLBuilder(agentName, namespace)
	} else {
		serviceURL = k.buildServiceURL(agentName, namespace)
	}
	
	// Create a health check request
	url := fmt.Sprintf("%s/health", serviceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &CommunicationError{
			Agent:   agentIdentifier,
			Message: "failed to create ping request",
			Cause:   err,
		}
	}
	
	resp, err := k.httpClient.Do(req)
	if err != nil {
		return &CommunicationError{
			Agent:   agentIdentifier,
			Message: "ping failed",
			Cause:   err,
		}
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return &CommunicationError{
			Agent:   agentIdentifier,
			Message: fmt.Sprintf("unhealthy status: %d", resp.StatusCode),
		}
	}
	
	return nil
}

// parseAgentIdentifier splits an agent identifier into name and namespace
func (k *K8sCommunicator) parseAgentIdentifier(identifier string) (string, string) {
	parts := strings.Split(identifier, ".")
	agentName := parts[0]
	namespace := k.defaultNamespace
	
	if len(parts) > 1 {
		namespace = parts[1]
	}
	
	// If no namespace specified and no default, use "default"
	if namespace == "" {
		namespace = "default"
	}
	
	return agentName, namespace
}

// buildServiceURL constructs the K8s service URL for an agent
func (k *K8sCommunicator) buildServiceURL(agentName, namespace string) string {
	// Convention: {agent-name}.{namespace}.svc.cluster.local:port
	return fmt.Sprintf("http://%s.%s.svc.%s:%d",
		agentName,
		namespace,
		k.clusterDomain,
		k.servicePort)
}

// truncateString truncates a string to a maximum length
func (k *K8sCommunicator) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getLocalAgentID returns the ID of the local agent (for headers)
func (k *K8sCommunicator) getLocalAgentID() string {
	// This should be set from the framework
	// For now, return a placeholder
	return "local-agent"
}

// SetDefaultNamespace updates the default namespace
func (k *K8sCommunicator) SetDefaultNamespace(namespace string) {
	k.defaultNamespace = namespace
}

// SetServicePort updates the default service port
func (k *K8sCommunicator) SetServicePort(port int) {
	k.servicePort = port
}

// SetClusterDomain updates the cluster domain
func (k *K8sCommunicator) SetClusterDomain(domain string) {
	k.clusterDomain = domain
}