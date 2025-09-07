# GoMind AI Module

Multi-provider LLM integration with automatic detection, universal compatibility, and extensible architecture.

## 🎯 What Does This Module Do?

Think of this module as your **universal translator for AI services**. Just like how a power adapter lets you plug your laptop into outlets worldwide, this module lets your agents talk to any AI service - OpenAI, Anthropic, Google, or even your company's private LLM.

It's the bridge between your agents and the world of AI, handling all the complexity so you can focus on building great features.

### Real-World Analogy: The Universal Remote

Remember universal TV remotes? One remote controls any TV brand. That's exactly what this module does for AI:

- **Without this module**: Write different code for each AI provider (OpenAI code, Anthropic code, etc.)
- **With this module**: Write once, use ANY provider with a single configuration change

```go
// Monday: Using OpenAI
client, _ := ai.NewClient(ai.WithProvider("openai"))

// Tuesday: Switch to Anthropic
client, _ := ai.NewClient(ai.WithProvider("anthropic"))

// Wednesday: Use your company's internal LLM
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Uses OpenAI-compatible interface
    ai.WithBaseURL("https://llm.company.internal/v1"),
)

// Your code doesn't change! Same interface, different providers
response, _ := client.GenerateResponse(ctx, "Hello AI!", nil)
```

## 🚀 Quick Start

### Installation

```go
import "github.com/itsneelabh/gomind/ai"
```

### The Simplest Thing That Works

```go
// Zero configuration - just works!
client, _ := ai.NewClient()

// Ask a question
response, _ := client.GenerateResponse(
    context.Background(),
    "What is the meaning of life?",
    nil,
)

fmt.Println(response.Content)
// Output: "The meaning of life is a philosophical question..."
```

That's it! The module automatically:
1. Checks your environment for API keys
2. Finds the best available provider
3. Configures everything for you
4. Returns the response

## 🧠 How It Works

### The Magic of Auto-Detection

The module is like a smart assistant that checks what's available:

```
Your Code: ai.NewClient()
          ↓
Module thinks: "Let me check what's available..."
          ↓
1. Checks: OPENAI_API_KEY exists? → Use OpenAI ✓
2. Checks: ANTHROPIC_API_KEY exists? → Use Anthropic ✓
3. Checks: GROQ_API_KEY exists? → Configure OpenAI provider for Groq ✓
4. Checks: Local Ollama running? → Use local model ✓
          ↓
Auto-configures the best option
          ↓
Ready to use!
```

### The Universal Provider Pattern

Here's the brilliant part - we have ONE OpenAI-compatible implementation that works with 20+ services:

```
┌──────────────────────────────────────────┐
│         Your Application Code             │
│         client.GenerateResponse()         │
└────────────────┬─────────────────────────┘
                 │
        ┌────────▼────────┐
        │   AI Module     │
        │                 │
        │ "One Interface  │
        │  To Rule       │
        │  Them All"     │
        └────────┬────────┘
                 │
    ┌────────────┼───────────────┐
    │            │               │
┌───▼──┐    ┌────▼────┐    ┌────▼────┐
│OpenAI│    │Anthropic│    │ Gemini  │
│      │    │ (Native)│    │(Native) │
└──┬───┘    └─────────┘    └─────────┘
   │
   └─► Works with 20+ services:
       OpenAI, Groq, DeepSeek, xAI,
       Ollama, vLLM, llama.cpp,
       Any OpenAI-compatible API!
```

## 📚 Core Concepts Explained

### The Provider Registry - Plugin Architecture

The registry is like a plugin system that keeps track of all available providers:

```go
// Providers register themselves automatically when imported
import (
    _ "github.com/itsneelabh/gomind/ai/providers/openai"    // Universal provider
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Native Anthropic
    _ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Native Gemini
    _ "mycompany/providers/internal_llm"                    // Your custom provider
)

// Now you can use any of them
client, _ := ai.NewClient(ai.WithProvider("internal_llm"))
```

### Universal OpenAI Provider - One Implementation, Many Services

The OpenAI provider is special - it's designed to work with ANY OpenAI-compatible API:

```go
// All these services use the SAME "openai" provider:

// Original OpenAI
client, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithAPIKey("sk-..."),
)

// Groq (ultra-fast inference)
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://api.groq.com/openai/v1"),
    ai.WithAPIKey("gsk-..."),
)

// DeepSeek (reasoning models)
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://api.deepseek.com"),
    ai.WithAPIKey("..."),
)

// Local Ollama
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("http://localhost:11434/v1"),
    // No API key needed for local
)

// Your company's OpenAI-compatible API
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://llm.company.internal/v1"),
    ai.WithAPIKey("internal-key"),
)
```

## 🎮 Three Ways to Use AI

### Method 1: Zero Configuration (Auto-Pilot)

Perfect for getting started - the module figures everything out:

```go
// Just set an environment variable
export OPENAI_API_KEY=sk-...

// In your code - that's it!
client, _ := ai.NewClient()
response, _ := client.GenerateResponse(ctx, "Hello!", nil)
```

**Behind the scenes:**
1. Checks environment variables
2. Finds available API keys
3. Auto-configures the appropriate provider
4. Ready to use!

### Method 2: Explicit Provider (You Choose)

When you want a specific provider:

```go
// Use native Anthropic implementation
client, _ := ai.NewClient(
    ai.WithProvider("anthropic"),
    ai.WithAPIKey("sk-ant-..."),
    ai.WithModel("claude-3-sonnet-20240229"),
)

// Use native Gemini implementation
client, _ := ai.NewClient(
    ai.WithProvider("gemini"),
    ai.WithAPIKey("..."),
    ai.WithModel("gemini-pro"),
)

// Use AWS Bedrock (requires build tag)
client, _ := ai.NewClient(
    ai.WithProvider("bedrock"),
    ai.WithRegion("us-east-1"),
)
```

### Method 3: Multi-Provider Strategy (Advanced)

Use different providers for different purposes in your application:

```go
type AISystem struct {
    primary   core.AIClient  // Your main provider
    fallback  core.AIClient  // Backup provider
    local     core.AIClient  // For sensitive data
}

func NewAISystem() *AISystem {
    // Primary: OpenAI for general use
    primary, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    )
    
    // Fallback: Another OpenAI-compatible service
    fallback, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithBaseURL("https://api.groq.com/openai/v1"),
        ai.WithAPIKey(os.Getenv("GROQ_API_KEY")),
    )
    
    // Local: For sensitive data
    local, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithBaseURL("http://localhost:11434/v1"),
    )
    
    return &AISystem{primary, fallback, local}
}

func (s *AISystem) Process(ctx context.Context, prompt string, sensitive bool) (*core.AIResponse, error) {
    if sensitive {
        // Use local model for sensitive data
        return s.local.GenerateResponse(ctx, prompt, nil)
    }
    
    // Try primary first
    response, err := s.primary.GenerateResponse(ctx, prompt, nil)
    if err != nil {
        // Fallback on error
        return s.fallback.GenerateResponse(ctx, prompt, nil)
    }
    
    return response, nil
}
```

## 🔧 Provider Configuration

### Environment Variables - Set and Forget

The module automatically detects and configures based on environment:

```bash
# Native providers (each has its own implementation)
export OPENAI_API_KEY=sk-...          # OpenAI
export ANTHROPIC_API_KEY=sk-ant-...   # Anthropic Claude
export GEMINI_API_KEY=...             # Google Gemini

# OpenAI-compatible services (auto-configured with the universal provider)
export GROQ_API_KEY=gsk-...           # Automatically configures Groq endpoint
export DEEPSEEK_API_KEY=...           # Automatically configures DeepSeek endpoint
export XAI_API_KEY=...                # Automatically configures xAI endpoint

# Custom OpenAI-compatible endpoint
export OPENAI_BASE_URL=https://llm.company.internal/v1
export OPENAI_API_KEY=internal-key

# AWS Bedrock (requires -tags bedrock during build)
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
```

### Configuration Options - Fine Control

```go
client, _ := ai.NewClient(
    // Provider selection
    ai.WithProvider("openai"),           // Which provider implementation
    ai.WithAPIKey("your-key"),          // Authentication
    ai.WithBaseURL("https://..."),      // Custom endpoint (OpenAI provider only)
    
    // Model configuration
    ai.WithModel("gpt-4"),               // Which model to use
    ai.WithTemperature(0.7),            // Creativity (0=deterministic, 1=creative)
    ai.WithMaxTokens(2000),             // Response length limit
    
    // Connection settings
    ai.WithTimeout(60 * time.Second),   // Request timeout
    ai.WithMaxRetries(3),               // Retry failed requests
    
    // Custom headers (if needed)
    ai.WithHeaders(map[string]string{
        "X-Custom-Header": "value",
    }),
    
    // AWS-specific (for Bedrock)
    ai.WithRegion("us-west-2"),
    ai.WithAWSCredentials(accessKey, secretKey, sessionToken),
)
```

## 🏗️ How It Fits in GoMind

### The Architecture

```
┌─────────────────────────────────────────┐
│            Your Application              │
│                                          │
│  "I need AI to analyze this data"       │
└────────────────┬────────────────────────┘
                 │
    ┌────────────▼────────────┐
    │     GoMind Core         │
    │                         │
    │  Tools & Agents with AI │
    └────────────┬────────────┘
                 │
    ┌────────────▼────────────┐
    │      AI Module          │ ← You are here!
    │                         │
    │  • Provider Registry    │
    │  • Universal Interface  │
    │  • Auto-detection       │
    └────────────┬────────────┘
                 │
         ┌───────┼───────┐
         │       │       │
    ┌────▼──┐ ┌──▼──┐ ┌──▼──┐
    │OpenAI │ │Anthro│ │Custom│
    │Provider│ │ pic  │ │ LLM  │
    └────────┘ └─────┘ └──────┘
```

### AI-Enhanced Components: Tools vs Agents

The AI module provides two types of AI-enhanced components:

#### AI Tools (Passive, Single-Purpose)

```go
// Create an AI-powered tool (passive component)
translator := ai.NewAITool("translator", "your-api-key")

// Tools do ONE thing well - they don't orchestrate
translator.RegisterAICapability(
    "translate",
    "Translates text between languages",
    "You are a professional translator. Translate the following text.",
)

// The tool responds to requests but doesn't discover others
```

#### AI Agents (Active Orchestrators)

```go
// Create an AI-powered agent (active orchestrator)
orchestrator := ai.NewAIAgent("orchestrator", "your-api-key")

// Agents can discover and coordinate components
tools, _ := orchestrator.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
})

// Use AI to plan and execute workflows
response, _ := orchestrator.ProcessWithAI(ctx, 
    "Analyze sales data and create a report")
```

### The Power of AI Orchestration

```go
// AI Agents orchestrate multiple tools intelligently
agent := ai.NewAIAgent("assistant", "your-api-key")

// The agent discovers available tools and coordinates them
response, err := agent.DiscoverAndOrchestrate(ctx, 
    "Get the latest sales data and create a summary")
```

## 🚀 Advanced Features

### Creating Custom Providers

The module is designed to be extended with your own providers:

```go
// mycompany/providers/custom_llm/provider.go
package custom_llm

import (
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/core"
)

type CustomProvider struct{}

func (p *CustomProvider) Name() string {
    return "custom-llm"
}

func (p *CustomProvider) Create(config *ai.AIConfig) core.AIClient {
    return &CustomClient{
        endpoint: config.BaseURL,
        apiKey:   config.APIKey,
        // Your implementation
    }
}

func (p *CustomProvider) DetectEnvironment() (priority int, available bool) {
    if os.Getenv("CUSTOM_LLM_KEY") != "" {
        return 200, true  // High priority
    }
    return 0, false
}

// Auto-register when imported
func init() {
    ai.MustRegister(&CustomProvider{})
}
```

Using your custom provider:

```go
// main.go
import _ "mycompany/providers/custom_llm"  // Auto-registers!

client, _ := ai.NewClient(ai.WithProvider("custom-llm"))
```

### Binary Size Management

The framework uses build tags to keep binaries lightweight:

```bash
# Default build: ~5.5MB (includes OpenAI, Anthropic, Gemini)
go build

# With AWS Bedrock: ~8.2MB (adds AWS SDK)
go build -tags bedrock

# With multiple cloud providers: ~12MB
go build -tags "bedrock,azure,vertex"
```

**The Rule:** Cloud SDK providers (AWS, Azure, GCP) require explicit build tags to avoid bloating binaries. All other providers are included by default if they add less than 1MB.

### Provider Capabilities

Each provider implementation can offer different capabilities:

```go
// Check if a provider supports streaming
if streamer, ok := client.(ai.StreamingClient); ok {
    stream, _ := streamer.StreamResponse(ctx, prompt, options)
    for chunk := range stream {
        fmt.Print(chunk.Content)  // Real-time streaming
    }
}

// Check if a provider supports embeddings
if embedder, ok := client.(ai.EmbeddingClient); ok {
    embeddings, _ := embedder.GenerateEmbeddings(ctx, text)
}
```

## 🎯 Common Use Cases

### Simple Q&A Bot

```go
func handleQuestion(question string) string {
    // Auto-detects provider from environment
    client, _ := ai.NewClient()
    
    response, err := client.GenerateResponse(
        context.Background(),
        question,
        &core.AIOptions{
            MaxTokens: 500,  // Keep responses concise
            Temperature: 0.7, // Balanced creativity
        },
    )
    
    if err != nil {
        return "Sorry, I couldn't process that question."
    }
    
    return response.Content
}
```

### Document Analysis

```go
func analyzeDocument(document string) (string, error) {
    client, _ := ai.NewClient(
        ai.WithProvider("anthropic"),  // Use Claude for documents
        ai.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
        ai.WithModel("claude-3-sonnet-20240229"),
    )
    
    prompt := fmt.Sprintf(`
        Analyze this document and provide:
        1. Summary (2-3 sentences)
        2. Key points
        3. Action items
        
        Document: %s
    `, document)
    
    response, err := client.GenerateResponse(
        context.Background(),
        prompt,
        &core.AIOptions{
            Temperature: 0.3,  // More focused analysis
            MaxTokens: 1000,
        },
    )
    
    return response.Content, err
}
```

### Resilient AI System with Fallback

```go
func createResilientAI() core.AIClient {
    // Primary provider
    primary, _ := ai.NewClient()
    
    // Fallback provider (different service)
    fallback, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithBaseURL("https://api.groq.com/openai/v1"),
        ai.WithAPIKey(os.Getenv("GROQ_API_KEY")),
    )
    
    // Wrap in resilient client
    return &ResilientClient{
        primary:  primary,
        fallback: fallback,
    }
}

type ResilientClient struct {
    primary  core.AIClient
    fallback core.AIClient
}

func (r *ResilientClient) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // Try primary first
    response, err := r.primary.GenerateResponse(ctx, prompt, options)
    if err == nil {
        return response, nil
    }
    
    // Fallback on error
    log.Printf("Primary provider failed, using fallback: %v", err)
    return r.fallback.GenerateResponse(ctx, prompt, options)
}
```

## 💡 Best Practices

### The Golden Rules

1. **🔑 Never hardcode API keys**
```go
// ❌ Bad
client, _ := ai.NewClient(ai.WithAPIKey("sk-proj-123..."))

// ✅ Good
client, _ := ai.NewClient(ai.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
```

2. **🔄 Always handle errors**
```go
response, err := client.GenerateResponse(ctx, prompt, options)
if err != nil {
    // Handle error appropriately
    log.Printf("AI request failed: %v", err)
    return fallbackResponse, nil
}
```

3. **⏱️ Set appropriate timeouts**
```go
client, _ := ai.NewClient(
    ai.WithTimeout(30 * time.Second),  // Don't wait forever
    ai.WithMaxRetries(3),               // Retry transient failures
)
```

4. **📊 Monitor token usage**
```go
response, _ := client.GenerateResponse(ctx, prompt, options)
log.Printf("Request used %d tokens", response.Usage.TotalTokens)
```

5. **🎯 Use appropriate options for your use case**
```go
// For factual queries: lower temperature
factualResponse, _ := client.GenerateResponse(ctx, prompt, &core.AIOptions{
    Temperature: 0.2,  // More deterministic
})

// For creative tasks: higher temperature
creativeResponse, _ := client.GenerateResponse(ctx, prompt, &core.AIOptions{
    Temperature: 0.8,  // More creative
})
```

## 🔮 Roadmap

### Currently In Development
- **Response Streaming** - Watch responses generate in real-time
- **Conversation Memory** - Maintain context across multiple calls
- **Provider Health Monitoring** - Automatic failover on provider issues

### Planned Features
- **Function Calling** - Let AI call your defined functions
- **Embeddings Support** - Generate semantic embeddings for search
- **Multi-Modal Support** - Process images, audio, and documents
- **Cost Tracking** - Monitor spending across providers
- **Rate Limiting** - Automatic rate limit handling
- **Caching Layer** - Reduce costs with intelligent caching

## 🎉 Summary

### What This Module Gives You

1. **Universal Interface** - One API for all AI providers
2. **Provider Freedom** - Switch providers without changing code
3. **Auto-Detection** - Zero configuration to get started
4. **Extensibility** - Add custom providers easily
5. **Resilience** - Built-in retry and fallback support
6. **Lightweight** - Minimal binary size with opt-in features
7. **Future-Proof** - New OpenAI-compatible services work instantly

### The Power of Abstraction

```go
// Your code stays the same
response, _ := client.GenerateResponse(ctx, prompt, options)

// Whether you're using:
// - OpenAI's GPT-4
// - Anthropic's Claude
// - Google's Gemini
// - Your company's private LLM
// - A local Ollama model
// - Any future AI service
```

### Quick Start Guide

**Just want to start?**
```go
client, _ := ai.NewClient()  // Auto-detects from environment
```

**Want specific provider?**
```go
client, _ := ai.NewClient(ai.WithProvider("anthropic"))
```

**Want custom endpoint?**
```go
client, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithBaseURL("https://your-api.com/v1"),
)
```

---

**🎊 Congratulations!** You now understand the AI module - your universal interface to the world of AI. The module handles all the complexity of different providers, letting you focus on building amazing AI-powered features.

Remember: Start simple with auto-detection, then customize as your needs grow. The module scales with you from prototype to production. Happy building! 🚀