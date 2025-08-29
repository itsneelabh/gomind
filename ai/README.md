# GoMind AI Module

The AI module provides basic artificial intelligence capabilities for the GoMind framework, currently focusing on OpenAI integration. This module enables agents to leverage GPT models for generating responses and building intelligent agent behaviors.

## Table of Contents
- [Current Features](#current-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Components](#components)
- [Examples](#examples)
- [API Reference](#api-reference)
- [Roadmap](#roadmap)
- [Contributing](#contributing)

## Current Features

✅ **Implemented:**
- OpenAI GPT integration (GPT-3.5, GPT-4)
- Basic response generation with customizable parameters
- System prompt support
- Token usage tracking
- Intelligent agent pattern with AI-powered tool discovery
- Integration with GoMind's discovery service

⚠️ **Limitations:**
- Only OpenAI provider currently supported
- No streaming support
- No conversation memory management
- No prompt templates or builders
- Basic error handling without retries

## Installation

```bash
go get github.com/itsneelabh/gomind/ai
```

## Quick Start

### Basic OpenAI Client Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create OpenAI client (uses OPENAI_API_KEY env var if not provided)
    client := ai.NewOpenAIClient("your-api-key")
    
    // Generate a response
    response, err := client.GenerateResponse(
        context.Background(),
        "What is the capital of France?",
        &core.AIOptions{
            Model:       "gpt-4",        // or "gpt-3.5-turbo"
            Temperature: 0.7,            // 0.0 to 1.0
            MaxTokens:   1000,           // max tokens to generate
            SystemPrompt: "You are a helpful assistant.",
        },
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response: %s\n", response.Content)
    fmt.Printf("Model used: %s\n", response.Model)
    fmt.Printf("Tokens used: %d\n", response.Usage.TotalTokens)
}
```

### Creating an Intelligent Agent

```go
package main

import (
    "context"
    "github.com/itsneelabh/gomind/ai"
)

func main() {
    // Create intelligent agent with AI capabilities
    // Note: This creates both a BaseAgent and OpenAI client internally
    agent := ai.NewIntelligentAgent("my-assistant", "your-api-key")
    
    // The agent can discover and use tools via AI
    ctx := context.Background()
    
    // This method uses AI to:
    // 1. Understand user intent
    // 2. Discover available tools via Discovery service
    // 3. Plan tool usage
    // 4. Synthesize results
    response, err := agent.DiscoverAndUseTools(ctx, 
        "What were the Q3 sales figures?")
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(response)
}
```

## Components

### 1. AIClient Interface

The minimal interface that all AI providers must implement:

```go
type AIClient interface {
    GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error)
}
```

### 2. OpenAIClient

Basic OpenAI API integration:

```go
// Create client with API key
client := ai.NewOpenAIClient(apiKey)

// Or use environment variable OPENAI_API_KEY
client := ai.NewOpenAIClient("")

// Client configuration is currently limited
// Default timeout: 30 seconds
// Default base URL: https://api.openai.com/v1
```

**Supported Options in AIOptions:**
- `Model`: "gpt-4", "gpt-3.5-turbo", etc.
- `Temperature`: Creativity level (0.0-1.0)
- `MaxTokens`: Maximum tokens to generate
- `SystemPrompt`: System message to set context

### 3. IntelligentAgent

An agent that combines BaseAgent with AI capabilities:

```go
type IntelligentAgent struct {
    *core.BaseAgent
    aiClient core.AIClient
}
```

**Key Methods:**
- `NewIntelligentAgent(name, apiKey)`: Creates agent with AI
- `EnableAI(agent, apiKey)`: Adds AI to existing BaseAgent
- `DiscoverAndUseTools(ctx, query)`: AI-powered tool discovery and usage

## Examples

### Example 1: Simple Q&A Agent

```go
func createQAAgent() *ai.IntelligentAgent {
    agent := ai.NewIntelligentAgent("qa-bot", os.Getenv("OPENAI_API_KEY"))
    
    // The agent inherits all BaseAgent capabilities
    // You can add HTTP handlers, capabilities, etc.
    agent.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Question string `json:"question"`
        }
        json.NewDecoder(r.Body).Decode(&req)
        
        // Use the internal AI client
        response, err := agent.aiClient.GenerateResponse(
            r.Context(), 
            req.Question,
            &core.AIOptions{
                Model: "gpt-3.5-turbo",
                MaxTokens: 500,
            },
        )
        
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        json.NewEncoder(w).Encode(map[string]string{
            "answer": response.Content,
        })
    })
    
    return agent
}
```

### Example 2: Adding AI to Existing Agent

```go
// Start with a regular BaseAgent
baseAgent := core.NewBaseAgent("my-agent")

// Add your capabilities, handlers, etc.
baseAgent.AddCapability(core.Capability{
    Name: "data_processing",
    Endpoint: "/process",
})

// Later, enable AI capabilities
ai.EnableAI(baseAgent, "your-api-key")

// Now baseAgent.AI is available for AI operations
if baseAgent.AI != nil {
    response, _ := baseAgent.AI.GenerateResponse(ctx, prompt, options)
}
```

### Example 3: Tool Discovery Pattern

```go
// This example shows how the IntelligentAgent discovers tools
agent := ai.NewIntelligentAgent("orchestrator", apiKey)

// Set up discovery service (required for tool discovery)
agent.Discovery = core.NewRedisDiscovery("redis://localhost:6379")

// Register some services with capabilities
agent.Discovery.Register(ctx, &core.ServiceRegistration{
    ID: "calc-service",
    Name: "calculator",
    Capabilities: []string{"calculation", "math"},
})

// Now the agent can discover and plan tool usage
result, _ := agent.DiscoverAndUseTools(ctx, 
    "Calculate the compound interest on $1000 at 5% for 3 years")
```

## API Reference

### OpenAIClient

| Method | Description |
|--------|-------------|
| `NewOpenAIClient(apiKey string, logger ...core.Logger)` | Create new OpenAI client |
| `GenerateResponse(ctx, prompt, options)` | Generate AI response |

### IntelligentAgent

| Method | Description |
|--------|-------------|
| `NewIntelligentAgent(name, apiKey string)` | Create new intelligent agent |
| `EnableAI(agent, apiKey string)` | Add AI to existing agent |
| `DiscoverAndUseTools(ctx, query string)` | AI-powered tool discovery and orchestration |

### AIOptions

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `Model` | string | OpenAI model to use | "gpt-4" |
| `Temperature` | float32 | Creativity (0.0-1.0) | 0.7 |
| `MaxTokens` | int | Max tokens to generate | 1000 |
| `SystemPrompt` | string | System context message | "" |

### AIResponse

| Field | Type | Description |
|-------|------|-------------|
| `Content` | string | Generated text response |
| `Model` | string | Model that was used |
| `Usage` | TokenUsage | Token consumption details |

## Roadmap

### Near-term (Planned)
- [ ] Response streaming support
- [ ] Retry logic with exponential backoff
- [ ] Rate limiting and quota management
- [ ] Response caching for repeated queries
- [ ] Better error handling and logging

### Medium-term (Under Consideration)
- [ ] Additional providers (Anthropic Claude, Google PaLM)
- [ ] Conversation memory management
- [ ] Prompt templates and builders
- [ ] Function calling support
- [ ] Embeddings support

### Long-term (Future)
- [ ] Local model support (Ollama, llama.cpp)
- [ ] Model selection strategies
- [ ] Cost optimization features
- [ ] Response validation
- [ ] Fine-tuning integration

## Contributing

We welcome contributions! Current priorities:
1. Improving error handling and retries
2. Adding streaming support
3. Implementing conversation memory
4. Adding more providers

Please ensure:
- All code includes tests
- Examples are functional
- Documentation is updated

## License

See the main GoMind repository for license information.