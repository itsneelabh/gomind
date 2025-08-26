// Package communication provides inter-agent communication capabilities for the GoMind Framework.
//
// This package enables agents to communicate with each other using natural language over HTTP,
// following Kubernetes service discovery conventions. It abstracts the complexity of service
// discovery, retry logic, and error handling.
//
// # Core Components
//
// The package provides the following key components:
//   - AgentCommunicator: Interface for inter-agent communication
//   - K8sCommunicator: Kubernetes-based implementation using service discovery
//   - CommunicationError: Structured error type for communication failures
//
// # Usage Example
//
// Basic agent-to-agent communication:
//
//	communicator := communication.NewK8sCommunicator(discovery, logger, "default")
//	
//	// Call another agent with natural language
//	response, err := communicator.CallAgent(ctx, "portfolio-service", 
//	    "What stocks does user john own?")
//	if err != nil {
//	    log.Error("Failed to get portfolio", err)
//	}
//	
//	// Call agent in different namespace
//	response, err = communicator.CallAgent(ctx, "risk-analyzer.analytics",
//	    "Calculate risk for this portfolio")
//
// # Service Discovery Convention
//
// The package follows Kubernetes service naming conventions:
//   - Service URL: {agent-name}.{namespace}.svc.cluster.local:port
//   - Default namespace: "default"
//   - Default port: 8080
//
// Agents can be identified using:
//   - Simple name: "portfolio-service" (uses default namespace)
//   - Qualified name: "portfolio-service.finance" (explicit namespace)
//
// # Natural Language Protocol
//
// All communication uses plain text:
//   - Request: Plain text instruction (Content-Type: text/plain)
//   - Response: Plain text response
//   - No JSON marshaling or API contracts required
//
// # Error Handling
//
// The package provides automatic retry logic with exponential backoff:
//   - Default: 3 retries with increasing delays
//   - 4xx errors: No retry (client errors)
//   - 5xx errors: Automatic retry (server errors)
//   - Network errors: Automatic retry
//
// # Headers and Tracing
//
// The package automatically adds tracing headers:
//   - X-From-Agent: Identifies the calling agent
//   - X-Request-ID: Unique request identifier
//   - X-Trace-ID: Distributed tracing ID (if available)
//
// # Configuration
//
// The communicator can be configured with:
//   - Default namespace
//   - Service port
//   - Cluster domain
//   - Timeout settings
//   - Retry policies
package communication