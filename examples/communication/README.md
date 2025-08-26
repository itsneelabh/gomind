# Inter-Agent Communication Examples

This directory contains examples demonstrating the Phase 1 implementation of inter-agent communication in the GoMind Framework.

## Overview

The inter-agent communication feature enables agents to:
- Send natural language instructions to other agents
- Receive and process requests from other agents
- Discover available agents in the network
- Coordinate complex tasks across multiple agents

## Architecture

```
┌─────────────┐                    ┌─────────────┐
│ Coordinator │◄──────────────────►│ Calculator  │
│   Agent     │  Natural Language   │   Agent     │
│ (Port 8082) │   Communication     │ (Port 8081) │
└─────────────┘                    └─────────────┘
       │                                   │
       └───────────┬───────────────────────┘
                   │
              ┌────▼────┐
              │  Redis  │
              │Discovery│
              └─────────┘
```

## Components

### 1. Calculator Agent (`calculator_agent.go`)
A simple agent that can perform basic mathematical operations:
- Addition
- Subtraction
- Multiplication
- Division

Implements the `ProcessRequestAgent` interface to handle natural language calculation requests.

### 2. Coordinator Agent (`coordinator_agent.go`)
An orchestrator agent that:
- Receives user requests
- Identifies calculation requests
- Delegates them to the Calculator Agent
- Returns consolidated responses

Also implements `ProcessRequestAgent` and demonstrates the use of the `AskAgent` helper method.

### 3. Test Script (`test_communication.sh`)
An automated test script that demonstrates various communication patterns:
- Direct requests to individual agents
- Delegated requests through the coordinator
- Agent discovery
- Complex calculations

## How to Run

### Prerequisites
- Go 1.19 or later
- Redis server running on localhost:6379 (optional, agents work without it)

### Step 1: Start Redis (Optional)
```bash
redis-server
```

### Step 2: Start the Calculator Agent
In terminal 1:
```bash
go run examples/communication/calculator_agent.go
```

### Step 3: Start the Coordinator Agent
In terminal 2:
```bash
go run examples/communication/coordinator_agent.go
```

### Step 4: Run Tests
In terminal 3:
```bash
./examples/communication/test_communication.sh
```

## Manual Testing

You can also test the agents manually using curl:

### Direct request to Calculator Agent:
```bash
curl -X POST http://localhost:8081/process \
    -H "Content-Type: text/plain" \
    -d "Please add 42 and 58"
```

### Request to Coordinator (delegates to Calculator):
```bash
curl -X POST http://localhost:8082/process \
    -H "Content-Type: text/plain" \
    -d "Can you calculate 100 times 3?"
```

### Check agent health:
```bash
curl http://localhost:8081/health
curl http://localhost:8082/health
```

## Key Features Demonstrated

### 1. ProcessRequestAgent Interface
Agents implement this interface to handle natural language requests:
```go
type ProcessRequestAgent interface {
    Agent
    ProcessRequest(ctx context.Context, instruction string) (string, error)
}
```

### 2. AskAgent Helper Method
BaseAgent provides a simple helper for inter-agent communication:
```go
response := baseAgent.AskAgent("calculator-agent", "Please add 10 and 20")
```

### 3. K8s-Compatible Communication
The communication layer uses Kubernetes service conventions:
- Service URL: `http://{agent-name}.{namespace}.svc.cluster.local:8080`
- Automatic retry with exponential backoff
- Request tracing with headers

### 4. Natural Language Processing
Agents process natural language instructions without rigid APIs:
- "Please add 15 and 25"
- "What is 100 multiplied by 3?"
- "Can you divide 150 by 5?"

## Implementation Details

### Communication Flow
1. **Request Reception**: Agent receives POST request on `/process` endpoint
2. **Processing**: Agent's `ProcessRequest` method handles the instruction
3. **Delegation** (if needed): Agent uses `AskAgent` to contact other agents
4. **Response**: Natural language response returned to caller

### Error Handling
- Automatic retry (3 attempts) for transient failures
- Timeout handling (30 seconds default)
- Graceful degradation when agents are unavailable

### Headers
The following headers are used for tracking and debugging:
- `X-From-Agent`: Identifies the calling agent
- `X-Request-ID`: Unique request identifier
- `X-Trace-ID`: Distributed tracing ID

## Next Steps

This is Phase 1 of the natural language communication implementation. Future phases will add:

- **Phase 2**: Enhanced Redis discovery with local catalog
- **Phase 3**: Routing abstraction (autonomous and workflow modes)
- **Phase 4**: Orchestrator pattern for complex multi-agent workflows
- **Phase 5**: Full framework integration
- **Phase 6**: Production-ready examples and testing

## Troubleshooting

### Agents can't communicate
- Ensure both agents are running
- Check that ports 8081 and 8082 are not in use
- Verify Redis is running (if using discovery)

### "Agent does not support natural language processing" error
- Ensure the agent implements the `ProcessRequestAgent` interface
- Check that the `ProcessRequest` method is properly implemented

### Connection refused errors
- Verify agents are running on the expected ports
- Check firewall settings
- Ensure localhost resolution is working