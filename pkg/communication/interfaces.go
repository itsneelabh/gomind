package communication

import (
	"context"
	"time"
)

// AgentCommunicator defines the interface for inter-agent communication
type AgentCommunicator interface {
	// CallAgent sends a natural language instruction to another agent and returns the response
	CallAgent(ctx context.Context, agentIdentifier string, instruction string) (string, error)

	// CallAgentWithTimeout sends an instruction with a custom timeout
	CallAgentWithTimeout(ctx context.Context, agentIdentifier string, instruction string, timeout time.Duration) (string, error)

	// GetAvailableAgents returns a list of currently available agents
	GetAvailableAgents(ctx context.Context) ([]AgentInfo, error)

	// Ping checks if an agent is reachable
	Ping(ctx context.Context, agentIdentifier string) error
}

// AgentInfo contains basic information about an available agent
type AgentInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	ServiceName  string   `json:"service_name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	LastSeen     string   `json:"last_seen"`
}

// CommunicationError represents an error during inter-agent communication
type CommunicationError struct {
	Agent   string
	Message string
	Cause   error
}

func (e *CommunicationError) Error() string {
	if e.Cause != nil {
		return "communication with " + e.Agent + " failed: " + e.Message + ": " + e.Cause.Error()
	}
	return "communication with " + e.Agent + " failed: " + e.Message
}

func (e *CommunicationError) Unwrap() error {
	return e.Cause
}

// CommunicationOptions contains optional parameters for communication
type CommunicationOptions struct {
	Timeout     time.Duration
	Retries     int
	RetryDelay  time.Duration
	Headers     map[string]string
	TraceID     string
}

// DefaultCommunicationOptions returns default communication options
func DefaultCommunicationOptions() *CommunicationOptions {
	return &CommunicationOptions{
		Timeout:    30 * time.Second,
		Retries:    3,
		RetryDelay: 1 * time.Second,
		Headers:    make(map[string]string),
	}
}