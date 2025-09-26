# GoMind AI Module

Multi-provider LLM integration with automatic detection, universal compatibility, and extensible architecture.

## 📚 Table of Contents

- [🎯 What Does This Module Do?](#-what-does-this-module-do)
- [🚀 Quick Start](#-quick-start)
- [🌍 Supported Providers](#-supported-providers)
- [🧠 How It Works](#-how-it-works)
- [📚 Core Concepts](#-core-concepts-explained)
- [🎮 Three Ways to Use AI](#-three-ways-to-use-ai)
- [🔧 Provider Configuration](#-provider-configuration)
- [🏗️ How It Fits in GoMind](#️-how-it-fits-in-gomind)
- [🚀 Advanced Features](#-advanced-features)
- [🎯 Common Use Cases](#-common-use-cases)
- [💡 Best Practices](#-best-practices)
- [🔮 Roadmap](#-roadmap)
- [🎉 Summary](#-summary)

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
import (
    "github.com/itsneelabh/gomind/ai"
    
    // Import the providers you plan to use
    _ "github.com/itsneelabh/gomind/ai/providers/openai"    // OpenAI and compatible services
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Anthropic Claude (optional)
    _ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Google Gemini (optional)
)
```

### The Simplest Thing That Works

```go
import (
    "github.com/itsneelabh/gomind/ai"
    // Import providers you want to use (they self-register)
    _ "github.com/itsneelabh/gomind/ai/providers/openai"    // For OpenAI, Groq, DeepSeek, Ollama, etc.
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic" // For Claude
    _ "github.com/itsneelabh/gomind/ai/providers/gemini"    // For Gemini
)

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

**Behind the scenes, here's what happens:**

1. **Provider Registration**: When you import providers with `_`, their `init()` functions automatically register them with the AI module's registry
2. **Environment Scanning**: `ai.NewClient()` calls each registered provider's `DetectEnvironment()` method to check if it's available (looking for API keys, local services, etc.)
3. **Priority Selection**: Available providers return a priority score - the module picks the highest priority provider that's configured
4. **Automatic Configuration**: The selected provider configures itself with found credentials and endpoints
5. **Ready to Use**: You get a working client without specifying any configuration

For example, if you have `OPENAI_API_KEY` set, it uses OpenAI. If you have `GROQ_API_KEY` instead, it automatically configures the OpenAI provider to use Groq's endpoint. No code changes needed!

## 🌍 Supported Providers

### Universal OpenAI-Compatible Provider

The GoMind AI module features a **universal OpenAI-compatible provider** that works with 20+ services using a single implementation. This means one provider implementation handles OpenAI, Groq, DeepSeek, local models, and any OpenAI-compatible API!

#### Quick Examples

```go
// Using OpenAI (Default)
client, _ := ai.NewClient(
    ai.WithAPIKey("your-openai-key"),
)

// Using Groq (300 tokens/sec, free tier available)
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://api.groq.com/openai/v1"),
    ai.WithAPIKey("your-groq-key"),
    ai.WithModel("llama-3.3-70b-versatile"),
)

// Using DeepSeek (advanced reasoning)
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://api.deepseek.com"),
    ai.WithAPIKey("your-deepseek-key"),
    ai.WithModel("deepseek-reasoner"),
)

// Using Local Ollama
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("http://localhost:11434/v1"),
    ai.WithModel("llama3:70b"),
)

// Your company's OpenAI-compatible deployment
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://llm.company.internal/v1"),
    ai.WithAPIKey("internal-key"),
)
```

### Complete Provider List

| Provider | Type | Base URL | Auto-Detection | Build Tag |
|----------|------|----------|----------------|-----------|
| **OpenAI** | Native | `https://api.openai.com/v1` | ✅ `OPENAI_API_KEY` | Default |
| **Anthropic Claude** | Native | N/A | ✅ `ANTHROPIC_API_KEY` | Default |
| **Google Gemini** | Native | N/A | ✅ `GEMINI_API_KEY` | Default |
| **AWS Bedrock** | Native | Region-based | ✅ AWS credentials, IAM roles, profiles | `bedrock` |
| **Groq** | OpenAI-compatible | `https://api.groq.com/openai/v1` | ✅ `GROQ_API_KEY` | Default |
| **DeepSeek** | OpenAI-compatible | `https://api.deepseek.com` | ✅ `DEEPSEEK_API_KEY` | Default |
| **xAI Grok** | OpenAI-compatible | `https://api.x.ai/v1` | ✅ `XAI_API_KEY` | Default |
| **Qwen (Alibaba)** | OpenAI-compatible | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | ✅ `QWEN_API_KEY` | Default |
| **Together AI** | OpenAI-compatible | Custom endpoint | Use `OPENAI_BASE_URL` | Default |
| **Perplexity** | OpenAI-compatible | Custom endpoint | Use `OPENAI_BASE_URL` | Default |
| **OpenRouter** | OpenAI-compatible | Custom endpoint | Use `OPENAI_BASE_URL` | Default |
| **Azure OpenAI** | OpenAI-compatible | `https://{resource}.openai.azure.com` | Use `OPENAI_BASE_URL` | Default |
| **Ollama** | OpenAI-compatible | `http://localhost:11434/v1` | ✅ Auto-detected if running | Default |
| **vLLM** | OpenAI-compatible | `http://localhost:8000/v1` | Use `OPENAI_BASE_URL` | Default |
| **llama.cpp** | OpenAI-compatible | `http://localhost:8080/v1` | Use `OPENAI_BASE_URL` | Default |
| **Any OpenAI-compatible API** | OpenAI-compatible | Your endpoint | Use `OPENAI_BASE_URL` | Default |

### Auto-Detection Priority

When you use `ai.NewClient()` without specifying a provider, the module checks for available services in this order:

1. **OpenAI** (priority: 100) - Checks for `OPENAI_API_KEY`
2. **Groq** (priority: 95) - Checks for `GROQ_API_KEY`, configures endpoint automatically
3. **DeepSeek** (priority: 90) - Checks for `DEEPSEEK_API_KEY`, configures endpoint automatically
4. **xAI Grok** (priority: 85) - Checks for `XAI_API_KEY`, configures endpoint automatically  
5. **Qwen** (priority: 80) - Checks for `QWEN_API_KEY`, configures endpoint automatically
6. **Anthropic** (priority: 80) - Checks for `ANTHROPIC_API_KEY` (native implementation)
7. **Gemini** (priority: 70) - Checks for `GEMINI_API_KEY` or `GOOGLE_API_KEY`
8. **AWS Bedrock** (priority: 60+) - Checks for AWS credentials, IAM roles, or profiles
   - Gets +10 priority when running on AWS infrastructure (EC2/ECS/Lambda)
9. **Ollama** (priority: 50) - Checks if local Ollama is running at `localhost:11434`

### Environment Variable Configuration

#### Method 1: Standard OpenAI
```bash
export OPENAI_API_KEY=your-key
```

#### Method 2: Custom OpenAI-Compatible Endpoint
```bash
export OPENAI_BASE_URL=https://api.groq.com/openai/v1
export OPENAI_API_KEY=your-groq-key
```

#### Method 3: Service-Specific (Auto-Configured)
The provider automatically detects and configures these services:

```bash
# Groq - Ultra-fast inference
export GROQ_API_KEY=your-key
# Automatically uses https://api.groq.com/openai/v1

# DeepSeek - Advanced reasoning models
export DEEPSEEK_API_KEY=your-key
# Automatically uses https://api.deepseek.com

# xAI Grok - Elon's AI
export XAI_API_KEY=your-key
# Automatically uses https://api.x.ai/v1

# Qwen (Alibaba) - Multilingual excellence
export QWEN_API_KEY=your-key
# Automatically uses https://dashscope-intl.aliyuncs.com/compatible-mode/v1

# Anthropic Claude - Native implementation
export ANTHROPIC_API_KEY=your-key

# Google Gemini - Native implementation
export GEMINI_API_KEY=your-key
```

### Key Benefits of Universal Provider

1. **Zero Code Duplication**: One implementation for all OpenAI-compatible services
2. **Future-Proof**: New OpenAI-compatible services work immediately without code changes
3. **Flexibility**: Use cloud providers, local models, or private deployments
4. **Simple Migration**: Switch providers by changing base URL only
5. **Auto-Detection**: Automatically finds and configures available services

## 🧠 How It Works

### Auto-Detection

When you call `ai.NewClient()` without specifying a provider, the module automatically checks for available services in priority order and configures the best option. See the [Auto-Detection Priority](#auto-detection-priority) section for details.

## 📚 Core Concepts Explained

### The Provider Registry - Plugin Architecture

The registry is like a plugin system that keeps track of all available providers. Providers register themselves automatically when imported:

```go
// Import the providers you need - each registers itself via init()
import (
    _ "github.com/itsneelabh/gomind/ai/providers/openai"    // Universal provider for 20+ services
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic" // Native Anthropic Claude
    _ "github.com/itsneelabh/gomind/ai/providers/gemini"    // Native Google Gemini
)

// Once imported, you can list all registered providers
providers := ai.ListProviders()
// Returns: ["anthropic", "gemini", "openai"]

// Get detailed info about available providers
info := ai.GetProviderInfo()
// Returns provider names, descriptions, availability, and priority
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
    ai.WithModel("gemini-1.5-pro"),
)

```

#### AWS Bedrock Provider

AWS Bedrock provides unified access to multiple foundation models including Claude, Llama, Titan, and more. It requires the `bedrock` build tag:

```bash
# Build with Bedrock support
go build -tags bedrock
```

**Configuration Methods:**

```go
// Method 1: Use AWS environment variables or IAM role
client, _ := ai.NewClient(
    ai.WithProvider("bedrock"),
    ai.WithRegion("us-east-1"),
)

// Method 2: Explicit credentials
client, _ := ai.NewClient(
    ai.WithProvider("bedrock"),
    ai.WithRegion("us-west-2"),
    ai.WithAWSCredentials(accessKey, secretKey, sessionToken),
)

// Method 3: Specify a model
client, _ := ai.NewClient(
    ai.WithProvider("bedrock"),
    ai.WithModel("anthropic.claude-3-sonnet-20240229-v1:0"),
)
```

**Supported Models in Bedrock:**
- Anthropic Claude (Opus, Sonnet, Haiku, Instant)
- Meta Llama 2 & 3 (8B, 13B, 70B variants)
- Amazon Titan (Text and Embeddings)
- Mistral and Mixtral models
- Cohere Command models

**Authentication Priority:**
1. Explicit credentials via `WithAWSCredentials()`
2. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
3. AWS Profile (`AWS_PROFILE`)
4. IAM role (when running on EC2/ECS/Lambda)
5. Credentials file (`~/.aws/credentials`)

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
export AWS_REGION=us-east-1              # or AWS_DEFAULT_REGION
export AWS_ACCESS_KEY_ID=...             # or use IAM role/profile
export AWS_SECRET_ACCESS_KEY=...         # or use IAM role/profile
export AWS_PROFILE=...                   # Alternative: use named profile
```

### Configuration Options - Fine Control

All configuration options available when creating a client:

```go
client, _ := ai.NewClient(
    // Provider selection
    ai.WithProvider("openai"),           // Which provider to use ("openai", "anthropic", "gemini", "auto")
    ai.WithAPIKey("your-key"),          // API key for authentication
    ai.WithBaseURL("https://..."),      // Custom endpoint (for OpenAI-compatible providers)
    
    // Model configuration
    ai.WithModel("gpt-4"),               // Model to use (provider-specific)
    ai.WithTemperature(0.7),            // Creativity level (0.0 = focused, 1.0 = creative)
    ai.WithMaxTokens(2000),             // Maximum tokens in response
    
    // Connection settings
    ai.WithTimeout(60 * time.Second),   // Request timeout (default: 30s)
    ai.WithMaxRetries(3),               // Number of retries on failure (default: 3)
    
    // Custom headers (for special requirements)
    ai.WithHeaders(map[string]string{
        "X-Custom-Header": "value",
        "Authorization": "Bearer custom-token",
    }),
    
    // AWS Bedrock specific (requires -tags bedrock)
    ai.WithRegion("us-west-2"),
    ai.WithAWSCredentials(accessKey, secretKey, sessionToken),
    
    // Advanced configuration
    ai.WithExtra("custom_param", value), // Provider-specific extra parameters
)
```

#### Default Configuration Values

- **Provider**: "auto" (auto-detects from environment)
- **Timeout**: 30 seconds
- **MaxRetries**: 3
- **Temperature**: 0.7
- **MaxTokens**: 1000

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

### Provider Registry Functions

The module provides several useful functions to work with providers:

```go
// List all registered providers
providers := ai.ListProviders()
// Returns: ["anthropic", "gemini", "openai"]

// Get detailed provider information
info := ai.GetProviderInfo()
for _, provider := range info {
    fmt.Printf("Provider: %s\n", provider.Name)
    fmt.Printf("  Description: %s\n", provider.Description)
    fmt.Printf("  Available: %v\n", provider.Available)
    fmt.Printf("  Priority: %d\n", provider.Priority)
}

// Check if a specific provider exists
factory, exists := ai.GetProvider("openai")
if exists {
    // Provider is available
}

// Create client with fallback on error
client, err := ai.NewClient()
if err != nil {
    // Use MustNewClient if you want to panic on error
    client = ai.MustNewClient(ai.WithProvider("openai"))
}
```

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

### Adding New OpenAI-Compatible Services

Any new OpenAI-compatible service works immediately without code changes:

```go
// Example: Using a new AI service that just launched
// No code changes needed in the module!
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Use the universal provider
    ai.WithBaseURL("https://new-ai-service.com/v1"),
    ai.WithAPIKey("your-api-key"),
)

// Example: Using a self-hosted model
client, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithBaseURL("https://your-gpu-server.com:8080/v1"),
    ai.WithAPIKey("optional-key"),
)
```

This future-proofs your code - as new services emerge, they'll work automatically if they follow the OpenAI API standard.

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

### Common Provider Features

All providers share these built-in features from the base client:

#### Automatic Retry with Exponential Backoff

```go
// Configure retry behavior
client, _ := ai.NewClient(
    ai.WithMaxRetries(5),        // Default: 3
    ai.WithTimeout(60 * time.Second),
)

// The module automatically retries on:
// - Network errors
// - 5xx server errors 
// - Rate limiting (429)
// - Timeout errors
```

#### Request/Response Logging

All providers support structured logging for debugging:

```go
// Logs include:
// - Request details (provider, model, prompt length)
// - Response metrics (tokens used, duration)
// - Retry attempts
// - Errors with context
```

#### Default Configuration

Each provider applies sensible defaults that can be overridden:

```go
// These defaults are applied if not specified:
// - Temperature: 0.7
// - MaxTokens: 1000
// - Timeout: 30 seconds
// - MaxRetries: 3
// - RetryDelay: 1 second (with exponential backoff)
```

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

## 🔄 Migration Guide

### Switching Between Providers

Switching providers is as simple as changing configuration:

```go
// From OpenAI to Anthropic
// Before:
client, _ := ai.NewClient(ai.WithProvider("openai"))

// After:
client, _ := ai.NewClient(ai.WithProvider("anthropic"))
// Your code doesn't change!
```

### Moving to OpenAI-Compatible Services

```go
// From OpenAI to Groq (faster, cheaper)
// Before:
client, _ := ai.NewClient(
    ai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
)

// After:
client, _ := ai.NewClient(
    ai.WithProvider("openai"),  // Same provider!
    ai.WithBaseURL("https://api.groq.com/openai/v1"),
    ai.WithAPIKey(os.Getenv("GROQ_API_KEY")),
)
```

### Using Environment Variables for Easy Switching

```bash
# Development: Use fast, cheap Groq
export GROQ_API_KEY=your-groq-key

# Staging: Use OpenAI
export OPENAI_API_KEY=your-openai-key

# Production: Use your custom deployment
export OPENAI_BASE_URL=https://llm.company.com/v1
export OPENAI_API_KEY=internal-key
```

Your code stays the same:
```go
client, _ := ai.NewClient()  // Auto-detects from environment
```

## 🎉 Summary

### What This Module Gives You

1. **Universal OpenAI Provider** - One implementation works with 20+ services (OpenAI, Groq, DeepSeek, xAI, Qwen, Ollama, and any OpenAI-compatible API)
2. **Native Providers** - Optimized implementations for Anthropic Claude, Google Gemini, and AWS Bedrock
3. **Auto-Detection** - Automatically finds and configures the best available provider from your environment
4. **Zero Code Changes** - Switch between providers by changing configuration, not code
5. **Provider Registry** - Plugin architecture for easy extension with custom providers
6. **AI Components** - Build intelligent agents that can discover and orchestrate other components
7. **Smart Configuration** - Sensible defaults with fine-grained control when needed
8. **Binary Optimization** - Cloud providers use build tags to keep binaries small
9. **Future-Proof** - New OpenAI-compatible services work instantly without any code changes
10. **Production Ready** - Built-in retries, timeouts, and error handling

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

---

**🎊 Congratulations!** You now understand the AI module - your universal interface to the world of AI. The module handles all the complexity of different providers, letting you focus on building amazing AI-powered features.

Remember: Start simple with auto-detection, then customize as your needs grow. The module scales with you from prototype to production. Happy building! 🚀