// Package memory provides persistent storage capabilities for agent state and conversation
// history in the GoMind Agent Framework.
//
// This package abstracts the complexity of state management, offering multiple backend
// implementations while maintaining a consistent interface for storing and retrieving
// agent data, conversation history, and shared context.
//
// # Memory Interface
//
// The Memory interface defines the contract for all storage implementations:
//
//	type Memory interface {
//	    Store(key string, value interface{}) error
//	    Retrieve(key string) (interface{}, error)
//	    Delete(key string) error
//	    List(pattern string) ([]string, error)
//	    Clear() error
//	}
//
// # Backend Implementations
//
// Available storage backends:
//
// Redis Backend (Distributed):
//   - Persistent storage across agent restarts
//   - Shared memory between multiple agents
//   - TTL support for automatic expiration
//   - Atomic operations for consistency
//   - Suitable for production deployments
//
// In-Memory Backend (Local):
//   - Fast, ephemeral storage
//   - No external dependencies
//   - Thread-safe operations
//   - Ideal for development and testing
//
// # Usage Patterns
//
// Storing conversation history:
//
//	conversation := Conversation{
//	    ID: "conv-123",
//	    Messages: messages,
//	    Context: context,
//	}
//	err := memory.Store("conversation:123", conversation)
//
// Retrieving agent state:
//
//	state, err := memory.Retrieve("agent:state:my-agent")
//	if err != nil {
//	    // Handle missing state
//	}
//
// Listing keys with pattern:
//
//	// Find all conversation keys
//	keys, err := memory.List("conversation:*")
//	for _, key := range keys {
//	    // Process each conversation
//	}
//
// # Key Namespacing
//
// Recommended key naming conventions:
//   - Conversations: "conversation:{id}"
//   - Agent state: "agent:state:{agent_id}"
//   - Shared context: "context:{namespace}:{key}"
//   - Temporary data: "temp:{purpose}:{id}"
//
// # Serialization
//
// The memory system handles automatic serialization:
//   - Complex types are JSON-encoded
//   - Simple types are stored directly
//   - Custom types should implement json.Marshaler
//
// # TTL and Expiration
//
// Redis backend supports time-to-live (TTL):
//
//	// Store with 1-hour expiration
//	err := memory.StoreWithTTL("temp:cache:123", data, time.Hour)
//
// # Integration with Agents
//
// Memory is automatically injected into agents that embed BaseAgent:
//
//	func (a *MyAgent) SaveState(state State) error {
//	    return a.Memory().Store("agent:state:"+a.GetAgentID(), state)
//	}
//
// # Best Practices
//
//   - Use consistent key naming patterns
//   - Implement proper error handling for missing keys
//   - Clean up temporary data when no longer needed
//   - Consider TTL for cache-like data
//   - Use transactions for related operations (Redis)
//   - Avoid storing large binary data directly
package memory