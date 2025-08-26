# Basic Agent Example

This example demonstrates a simple agent implementation using the GoMind Agent Framework.

## What This Example Shows

- Creating a basic agent with custom capabilities
- Using framework auto-discovery for agent methods
- Starting an HTTP server with the framework
- Graceful shutdown handling

## Agent Capabilities

The `BasicAgent` implements two capabilities:

1. **greet** - Greets users with a friendly message
2. **echo** - Echoes back the provided message

## Running the Example

1. Make sure you have Go 1.23+ installed
2. Navigate to this directory:
   ```bash
   cd examples/basic-agent
   ```
3. Run the agent:
   ```bash
   go run main.go
   ```
4. Open your browser to `http://localhost:8080` to access the chat interface

## Testing the Agent

Once running, you can test the agent capabilities:

### Using the Web Interface
1. Visit `http://localhost:8080`
2. Type messages in the chat interface
3. The agent will respond using its capabilities

### Using HTTP API
```bash
# Test the greet capability
curl -X POST http://localhost:8080/agents/basic-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "greet", "input": {"name": "Alice"}}'

# Test the echo capability  
curl -X POST http://localhost:8080/agents/basic-agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"capability": "echo", "input": {"message": "Hello World"}}'
```

## Key Concepts Demonstrated

### 1. Agent Structure
```go
type BasicAgent struct {
    framework.BaseAgent  // Embeds framework functionality
}
```

### 2. Capability Annotations
Methods are automatically discovered as capabilities using comment annotations:
```go
// @capability: greet
// @description: Greets users with a friendly message
// @input: name string "Name of the person to greet"
// @output: greeting string "Personalized greeting message"
func (b *BasicAgent) Greet(name string) string {
    // Implementation
}
```

### 3. Framework Initialization
```go
fw, err := framework.NewFramework(
    framework.WithPort(8080),
    framework.WithAgentName("basic-agent"),
)
```

### 4. Agent Registration and Startup
```go
err = fw.InitializeAgent(ctx, basicAgent)
err = fw.StartHTTPServer(ctx, basicAgent)
```

## Next Steps

- Check out the [Market Agent Example](../market-agent/) for AI integration
- Review the [Multi-Agent Example](../multi-agent/) for agent collaboration
- Read the [API Documentation](../../docs/API.md) for advanced features
