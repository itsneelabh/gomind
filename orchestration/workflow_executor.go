package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// WorkflowExecutor handles service calls for workflow steps
type WorkflowExecutor struct {
	discovery core.Discovery
	client    *WorkflowHTTPClient
}

// WorkflowHTTPClient wraps HTTP client for service calls
type WorkflowHTTPClient struct {
	httpClient *http.Client
}

// NewWorkflowHTTPClient creates a new HTTP client for workflows
func NewWorkflowHTTPClient() *WorkflowHTTPClient {
	return &WorkflowHTTPClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CallService calls a service endpoint with the given action and inputs
func (e *WorkflowExecutor) CallService(ctx context.Context, service *core.ServiceRegistration, action string, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Construct service URL
	url := fmt.Sprintf("http://%s:%d/%s", service.Address, service.Port, action)

	// Prepare request body
	requestBody, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if workflowID := ctx.Value("workflow_id"); workflowID != nil {
		req.Header.Set("X-Workflow-ID", workflowID.(string))
	}
	if stepID := ctx.Value("step_id"); stepID != nil {
		req.Header.Set("X-Step-ID", stepID.(string))
	}

	// Execute request
	resp, err := e.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return result, nil
}

// CallAgent calls an agent with discovery lookup
func (e *WorkflowExecutor) CallAgent(ctx context.Context, agentName string, action string, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Find agent using discovery
	services, err := e.discovery.FindService(ctx, agentName)
	if err != nil {
		return nil, fmt.Errorf("finding agent %s: %w", agentName, err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("agent %s: %w", agentName, core.ErrAgentNotFound)
	}

	// Select best service (first healthy one)
	var service *core.ServiceRegistration
	for _, svc := range services {
		if svc.Health == core.HealthHealthy {
			service = svc
			break
		}
	}

	if service == nil {
		// No healthy service, use first one
		service = services[0]
	}

	return e.CallService(ctx, service, action, inputs)
}

// CallCapability calls any service with the specified capability
func (e *WorkflowExecutor) CallCapability(ctx context.Context, capability string, action string, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Find services by capability
	services, err := e.discovery.FindByCapability(ctx, capability)
	if err != nil {
		return nil, fmt.Errorf("finding capability %s: %w", capability, err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services with capability %s: %w", capability, core.ErrCapabilityNotFound)
	}

	// Select best service
	var service *core.ServiceRegistration
	for _, svc := range services {
		if svc.Health == core.HealthHealthy {
			service = svc
			break
		}
	}

	if service == nil {
		service = services[0]
	}

	return e.CallService(ctx, service, action, inputs)
}

// BatchCall executes multiple service calls in parallel
func (e *WorkflowExecutor) BatchCall(ctx context.Context, calls []ServiceCall) []ServiceCallResult {
	results := make([]ServiceCallResult, len(calls))

	// Execute calls in parallel
	type indexedResult struct {
		index  int
		result ServiceCallResult
	}

	resultChan := make(chan indexedResult, len(calls))

	for i, call := range calls {
		go func(idx int, c ServiceCall) {
			defer func() {
				if r := recover(); r != nil {
					// Capture panic and convert to error result
					panicErr := fmt.Errorf("service call %s panic: %v", c.ID, r)
					stackTrace := string(debug.Stack())

					// Try to send result with timeout to avoid blocking
					sendTimeout := time.After(5 * time.Second)
					select {
					case resultChan <- indexedResult{
						index: idx,
						result: ServiceCallResult{
							CallID:  c.ID,
							Success: false,
							Error:   panicErr.Error(),
							Output: map[string]interface{}{
								"panic":       fmt.Sprintf("%v", r), // The panic value for debugging
								"call_id":     c.ID,                 // Identifies which call failed
								"call_type":   c.Type,               // Type of call (agent/capability)
								"target":      c.Target,             // Target service or capability
								"stack_trace": stackTrace,           // Full stack trace for debugging
							},
						},
					}:
						// Successfully sent panic result
					case <-sendTimeout:
						// Timeout occurred while sending panic result.
						// This indicates the result channel might be blocked or closed.
						// In production, this should be logged for monitoring.
						// TODO: Add proper logging/metrics here
						_ = panicErr // Prevent unused variable warning
					}
				}
			}()

			var result ServiceCallResult
			result.CallID = c.ID

			// Execute based on call type
			var output map[string]interface{}
			var err error

			switch c.Type {
			case CallTypeAgent:
				output, err = e.CallAgent(ctx, c.Target, c.Action, c.Inputs)
			case CallTypeCapability:
				output, err = e.CallCapability(ctx, c.Target, c.Action, c.Inputs)
			default:
				err = fmt.Errorf("unknown call type: %s", c.Type)
			}

			if err != nil {
				result.Error = err.Error()
			} else {
				result.Success = true
				result.Output = output
			}

			resultChan <- indexedResult{index: idx, result: result}
		}(i, call)
	}

	// Collect results
	for i := 0; i < len(calls); i++ {
		r := <-resultChan
		results[r.index] = r.result
	}

	return results
}

// ServiceCall represents a service call request
type ServiceCall struct {
	ID     string                 `json:"id"`
	Type   CallType               `json:"type"`
	Target string                 `json:"target"` // Agent name or capability
	Action string                 `json:"action"`
	Inputs map[string]interface{} `json:"inputs"`
}

// CallType defines the type of service call
type CallType string

const (
	CallTypeAgent      CallType = "agent"
	CallTypeCapability CallType = "capability"
)

// ServiceCallResult represents the result of a service call
type ServiceCallResult struct {
	CallID  string                 `json:"call_id"`
	Success bool                   `json:"success"`
	Output  map[string]interface{} `json:"output,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// HealthCheck checks if a service is healthy
func (e *WorkflowExecutor) HealthCheck(ctx context.Context, service *core.ServiceRegistration) bool {
	url := fmt.Sprintf("http://%s:%d/health", service.Address, service.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := e.client.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}
