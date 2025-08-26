// Package agent provides the core agent interface and base implementation for the GoMind Agent Framework.
//
// The Agent interface is the fundamental building block of the framework, defining the contract
// that all agents must implement. Agents encapsulate specific functionality and can be discovered
// and invoked by other agents or external systems through the framework's discovery mechanism.
//
// # Core Concepts
//
// An agent in the GoMind framework is a self-contained unit of functionality that:
//   - Implements specific business logic through capabilities
//   - Can be automatically discovered by other agents
//   - Receives dependency injection for framework services
//   - Supports telemetry and observability out of the box
//
// # Basic Usage
//
// To create an agent, implement the Agent interface:
//
//	type MyAgent struct {
//	    framework.BaseAgent
//	}
//
//	func (m *MyAgent) Initialize(ctx context.Context) error {
//	    // Custom initialization logic
//	    return nil
//	}
//
// The BaseAgent type provides convenient access to framework services like logging,
// memory, discovery, and AI capabilities through embedded fields.
//
// # Capabilities
//
// Agent capabilities are methods that can be discovered and invoked by the framework.
// They are identified through reflection or metadata annotations:
//
//	// @capability: data_processing
//	// @description: Processes incoming data and returns results
//	func (m *MyAgent) ProcessData(input string) (string, error) {
//	    // Implementation
//	}
//
// # Framework Integration
//
// The framework automatically handles:
//   - Service discovery and registration
//   - Dependency injection
//   - Capability discovery
//   - Telemetry and metrics collection
//   - Health checking and lifecycle management
package agent