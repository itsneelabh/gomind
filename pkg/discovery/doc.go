// Package discovery provides service discovery mechanisms for multi-agent orchestration
// in the GoMind Agent Framework.
//
// This package enables agents to dynamically discover, register, and communicate with
// other agents in a distributed system, supporting both single-node and multi-node
// deployments.
//
// # Core Concepts
//
// Service discovery in the framework operates on two levels:
//   - Agent Discovery: Finding and cataloging available agents and their capabilities
//   - Capability Discovery: Understanding what each agent can do
//
// # Discovery Interface
//
// The Discovery interface defines the contract for all discovery implementations:
//
//	type Discovery interface {
//	    RegisterAgent(registration AgentRegistration) error
//	    DeregisterAgent(agentID string) error
//	    DiscoverAgents() ([]AgentRegistration, error)
//	    GetAgent(agentID string) (*AgentRegistration, error)
//	    UpdateHeartbeat(agentID string) error
//	}
//
// # Redis Discovery
//
// The default implementation uses Redis for distributed discovery:
//   - Agents register themselves with metadata and capabilities
//   - Heartbeat mechanism maintains agent liveness
//   - Automatic cleanup of stale registrations
//   - Caching layer for improved performance
//   - Circuit breaker for resilience
//
// # Registration Scopes
//
// Agents can register at different scopes:
//   - Pod scope: Individual instance registration
//   - Service scope: Kubernetes service-level registration
//
// # Caching and Performance
//
// The discovery system includes sophisticated caching:
//   - In-memory cache with configurable refresh intervals
//   - Persistent snapshots for quick startup
//   - Background refresh to minimize latency
//   - Stale data warnings for operational awareness
//
// # Usage Example
//
//	discovery, err := discovery.NewRedisDiscovery(
//	    "redis://localhost:6379",
//	    "my-agent",
//	    "production",
//	)
//	
//	// Register agent
//	err = discovery.RegisterAgent(AgentRegistration{
//	    AgentID: "my-agent-123",
//	    Capabilities: capabilities,
//	    Metadata: metadata,
//	})
//	
//	// Discover other agents
//	agents, err := discovery.DiscoverAgents()
//	for _, agent := range agents {
//	    fmt.Printf("Found agent: %s\n", agent.AgentID)
//	}
//
// # Health Monitoring
//
// The discovery system monitors agent health through:
//   - Regular heartbeat updates
//   - Automatic deregistration of unresponsive agents
//   - Health status tracking (healthy, unhealthy, starting, stopping)
//
// # Integration with Framework
//
// Discovery is automatically configured and injected into agents,
// enabling seamless agent-to-agent communication without manual setup.
package discovery