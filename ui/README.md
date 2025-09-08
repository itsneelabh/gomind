# GoMind UI Module

The UI module provides a pluggable transport registry for building chat agents with multiple communication protocols. It follows the successful registry pattern from the AI module, allowing automatic discovery and configuration of transports.

## Architecture

### Transport Registry Pattern

The module uses a registry pattern that allows transports to self-register via `init()`:

```go
// SSE transport auto-registers when imported
import _ "github.com/itsneelabh/gomind/ui/transports/sse"
```

### Core Components

1. **Transport Interface**: Defines the contract for all UI communication protocols
2. **SessionManager**: Manages chat sessions with Redis or in-memory storage
3. **ChatAgent**: Orchestrates transports and sessions with AI integration

## Features

- **Auto-Configuration**: Transports are automatically discovered and configured
- **Multiple Transports**: SSE (built-in), WebSocket (optional), future support for WebTransport
- **Distributed Sessions**: Redis-backed sessions work across multiple instances
- **Rate Limiting**: Built-in rate limiting per session
- **AI Integration**: Native integration with GoMind AI module
- **Zero Dependencies**: SSE transport has no external dependencies

## Usage

### Basic Example

```go
package main

import (
    "log"
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/ui"
    _ "github.com/itsneelabh/gomind/ui/transports/sse"
)

func main() {
    // Create AI client
    aiClient, _ := ai.NewClient()
    
    // Create session manager (Redis or Mock)
    sessions := ui.NewMockSessionManager(ui.DefaultSessionConfig())
    
    // Create chat agent with auto-configured transports
    config := ui.DefaultChatAgentConfig("assistant")
    agent := ui.NewChatAgent(config, aiClient, sessions)
    
    // Transports are automatically configured!
    // Available at: /chat/sse, /chat/transports, /chat/health
    
    log.Printf("Chat agent ready with transports: %v", agent.ListTransports())
    agent.Start(8080)
}
```

### With Redis Sessions (Production)

```go
// Use Redis for distributed sessions
sessions, _ := ui.NewRedisSessionManager(
    "redis://localhost:6379",
    ui.DefaultSessionConfig(),
)
```

## Session Management

### Redis Schema

```
gomind:chat:session:{sessionID}         → Hash (session data)
gomind:chat:session:{sessionID}:msgs    → List (messages)
gomind:chat:session:{sessionID}:rate    → String (rate limit counter)
gomind:chat:sessions:active              → Set (active session IDs)
```

### Configuration

```go
config := ui.SessionConfig{
    TTL:             30 * time.Minute,  // Session timeout
    MaxMessages:     50,                // Sliding window size
    MaxTokens:       4000,               // Max tokens per session
    RateLimitWindow: time.Minute,       // Rate limit window
    RateLimitMax:    20,                // Max messages per window
}
```

## Available Transports

### SSE (Server-Sent Events)
- **Always included**: Zero external dependencies
- **Priority**: 100 (default)
- **Use case**: Simple one-way streaming
- **Endpoint**: `/chat/sse`

### WebSocket (Optional)
- **Build tag**: `-tags websocket`
- **Priority**: 90
- **Use case**: Bidirectional communication
- **Endpoint**: `/chat/websocket`

### Future: WebTransport
- **Build tag**: `-tags webtransport`
- **Priority**: 110 (highest)
- **Use case**: Next-gen HTTP/3 + QUIC
- **Endpoint**: `/chat/webtransport`

## Client Integration

### JavaScript Client

```javascript
// Auto-discover best transport
const response = await fetch('/chat/transports');
const data = await response.json();
const transports = data.transports;

// Use SSE transport
const eventSource = new EventSource('/chat/sse?message=Hello');
eventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
};
```

### HTML Client Example

See `examples/chat_client.html` for a complete web client with:
- Transport auto-discovery
- Session management
- Real-time streaming
- Error handling

## Testing

```bash
# Run all tests
go test ./ui/...

# Run with coverage
go test -cover ./ui/...

# Test specific component
go test ./ui -run TestSessionManager
```

## Build Options

```bash
# Default build (SSE only)
go build ./cmd/agent                    # ~7MB

# With WebSocket support
go build -tags websocket ./cmd/agent    # ~7.5MB

# Future with multiple transports
go build -tags websocket,webtransport   # ~8MB
```

## API Endpoints

- `GET /chat/transports` - Discover available transports
- `GET /chat/health` - Health check for all transports
- `GET /chat/sse` - SSE streaming endpoint
- `GET /chat/websocket` - WebSocket endpoint (if enabled)

## Architecture Benefits

1. **Future-Proof**: New transports just implement the interface
2. **Optimal Binary Size**: Only include what you need via build tags
3. **Developer Experience**: Single import, auto-configuration
4. **Production Ready**: Distributed sessions, rate limiting, health checks
5. **Follows GoMind Patterns**: Mirrors successful AI provider registry

## Implementation Status

### Phase 4: Session Management ✅
- [x] Redis session schema and key patterns
- [x] SessionManager interface with Redis implementation
- [x] Session lifecycle management
- [x] Distributed rate limiting patterns
- [x] MockSessionManager for testing
- [x] Unit tests for core operations

### Phase 5: First Transport (SSE) ✅
- [x] SSE transport with zero dependencies
- [x] Auto-registration via init()
- [x] Transport interface compliance
- [x] Basic functionality tests

### Phase 6: ChatAgent ✅
- [x] Transport auto-discovery
- [x] Integration with SessionManager
- [x] AI module integration
- [x] Basic lifecycle management
- [x] Unit tests
- [x] Configuration documentation

## Contributing

To add a new transport:

1. Implement the `Transport` interface
2. Register via `init()` function
3. Add build tag if heavy dependencies
4. Pass `TestTransportComplianceTestSuite`
5. Document capabilities and usage

## License

See the main GoMind LICENSE file.