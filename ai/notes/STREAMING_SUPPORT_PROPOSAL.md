# AI Streaming Support Proposal

## Executive Summary

This document describes the current limitation in GoMind's AI module regarding streaming responses and proposes a comprehensive solution to implement true token-by-token streaming across the framework.

**Current State**: The `AIClient` interface only supports synchronous, blocking responses. The entire AI response must be generated before it can be returned to the caller.

**Proposed State**: Add a streaming interface that allows token-by-token delivery as the AI model generates them, enabling real-time user experiences and reducing perceived latency.

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Current Architecture Analysis](#current-architecture-analysis)
3. [Impact Assessment](#impact-assessment)
4. [Proposed Solution](#proposed-solution)
5. [Implementation Plan](#implementation-plan)
6. [API Design](#api-design)
7. [Provider Implementations](#provider-implementations)
8. [Orchestrator Integration](#orchestrator-integration)
9. [Migration Strategy](#migration-strategy)
10. [Testing Strategy](#testing-strategy)
11. [Performance Considerations](#performance-considerations)
12. [Timeline Estimate](#timeline-estimate)

---

## Problem Statement

### The Issue

When a user sends a chat message through the GoMind framework, they must wait for the **entire AI response** to be generated before seeing any output. This creates several problems:

1. **Poor User Experience**: Users stare at a loading spinner for 3-10+ seconds with no feedback
2. **Perceived Latency**: Even fast responses feel slow because there's no incremental progress
3. **Timeout Risks**: Long responses may hit timeout limits before completion
4. **Resource Inefficiency**: The system holds connections open waiting for complete responses

### Observed Behavior

In the `travel-chat-agent` example:

```
User sends: "Tell me about traveling to Japan"

Timeline:
0.0s - Request received
0.0s - "Analyzing your request..." status sent
3.5s - [SILENCE - AI generating full response]
3.5s - Complete response arrives all at once
3.5s - Response chunked into 50-char pieces (simulated streaming)
3.6s - Done
```

The "streaming" currently implemented is **simulated** - it takes the complete response and splits it into chunks:

```go
// Current implementation in chat_agent.go (lines 255-268)
response := result.Response  // Already complete!
chunkSize := 50
for i := 0; i < len(response); i += chunkSize {
    callback.SendChunk(response[i:end])
}
```

### What True Streaming Looks Like

```
User sends: "Tell me about traveling to Japan"

Timeline:
0.0s - Request received
0.0s - "Analyzing your request..." status sent
0.3s - First token arrives: "Japan"
0.4s - More tokens: " is a"
0.5s - More tokens: " fascinating"
...continues token by token...
3.5s - Final token + Done
```

With true streaming, users see the response being "typed out" in real-time.

---

## Current Architecture Analysis

### Core Interface (`core/interfaces.go`)

```go
// AIClient interface - optional AI support
type AIClient interface {
    GenerateResponse(ctx context.Context, prompt string, options *AIOptions) (*AIResponse, error)
}

type AIResponse struct {
    Content string      // Complete response content
    Model   string
    Usage   TokenUsage
}
```

**Limitation**: `GenerateResponse` returns a complete `*AIResponse` with the full `Content` string. There's no way to receive partial responses.

### AI Module Structure

```
ai/
├── interfaces.go          # Re-exports core.AIClient
├── client.go              # NewClient() factory
├── chain_client.go        # Multi-provider failover
├── providers/
│   ├── openai/
│   │   ├── client.go      # OpenAI implementation
│   │   └── factory.go
│   ├── anthropic/
│   │   ├── client.go      # Anthropic implementation
│   │   └── factory.go
│   ├── gemini/
│   │   └── client.go      # Google Gemini implementation
│   └── bedrock/
│       └── client.go      # AWS Bedrock implementation
```

### Provider SDK Capabilities

| Provider | Native Streaming Support | SDK Method |
|----------|-------------------------|------------|
| OpenAI | Yes | `CreateChatCompletionStream()` |
| Anthropic | Yes | `Messages.Stream()` |
| Google Gemini | Yes | `GenerateContentStream()` |
| AWS Bedrock | Yes | `InvokeModelWithResponseStream()` |

**All major providers support streaming natively** - we're just not using it.

### Orchestrator Module (`orchestration/`)

```go
// Current interfaces.go - Orchestrator interface
type Orchestrator interface {
    ProcessRequest(ctx context.Context, request string, metadata map[string]interface{}) (*OrchestratorResponse, error)
    ExecutePlan(ctx context.Context, plan *RoutingPlan) (*ExecutionResult, error)
    GetExecutionHistory() []ExecutionRecord
    GetMetrics() OrchestratorMetrics
}

// OrchestratorResponse - the actual return type
type OrchestratorResponse struct {
    RequestID       string                 `json:"request_id"`
    OriginalRequest string                 `json:"original_request"`
    Response        string                 `json:"response"`          // The synthesized response
    RoutingMode     RouterMode             `json:"routing_mode"`
    ExecutionTime   time.Duration          `json:"execution_time"`
    AgentsInvolved  []string               `json:"agents_involved"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
    Errors          []string               `json:"errors,omitempty"`
    Confidence      float64                `json:"confidence"`
}
```

The orchestrator calls `AIClient.GenerateResponse()` internally and waits for complete responses.

---

## Impact Assessment

### Current Provider State

| Provider | File | Has `GenerateResponse` | Has `StreamResponse` | Notes |
|----------|------|----------------------|---------------------|-------|
| OpenAI | `ai/providers/openai/client.go` | ✅ Yes | ❌ No | Uses raw HTTP |
| Anthropic | `ai/providers/anthropic/client.go` | ✅ Yes | ❌ No | Uses raw HTTP |
| Gemini | `ai/providers/gemini/client.go` | ✅ Yes | ❌ No | Uses raw HTTP |
| Bedrock | `ai/providers/bedrock/client.go` | ✅ Yes | ⚠️ **Exists but different signature** | Uses AWS SDK, has channel-based streaming |
| Mock | `ai/providers/mock/provider.go` | ✅ Yes | ❌ No | For testing |

**Important**: Bedrock already has a `StreamResponse` method but with a **different signature**:
```go
// Current Bedrock signature (channel-based)
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error

// Proposed unified signature (callback-based)
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error)
```

This needs to be refactored to match the unified callback-based interface.

### Modules Requiring Changes

| Module | Change Required | Complexity |
|--------|-----------------|------------|
| `core` | Add streaming interface | Low |
| `ai` | Re-export streaming interface | Low |
| `ai/providers/openai` | Implement streaming | Medium |
| `ai/providers/anthropic` | Implement streaming | Medium |
| `ai/providers/gemini` | Implement streaming | Medium |
| `ai/providers/bedrock` | **Refactor existing** to callback interface | Medium |
| `ai/providers/mock` | Add streaming for testing | Low |
| `ai/chain_client` | Support streaming failover | High |
| `orchestration` | Add streaming ProcessRequest | High |
| `examples/travel-chat-agent` | Use streaming API | Low |

### Additional Modules Analysis

#### resilience Module (`resilience/`)

**Critical**: Standard retry and circuit breaker patterns **do not work** with streaming due to stateful connections.

**Why Retry Doesn't Work for Streaming**:
1. Streams are stateful - you can't retry from the middle
2. Partial content has already been delivered to the client
3. Context may have accumulated state (tokens sent, etc.)

**Why Circuit Breaker Needs Special Handling**:
1. Circuit breaker should only wrap the **initial connection**, not individual chunks
2. A chunk delivery failure is different from a connection failure
3. Partial stream completion should be recorded as success (not failure)

**Recommended Patterns**:

```go
// resilience/streaming.go (NEW FILE)

// StreamingCircuitBreaker wraps only the stream initiation, not the chunks
type StreamingCircuitBreaker struct {
    cb *CircuitBreaker
}

// ExecuteStream wraps stream initiation with circuit breaker
// The circuit breaker only protects the connection establishment, not the streaming itself
func (scb *StreamingCircuitBreaker) ExecuteStream(
    ctx context.Context,
    streamFn func() error, // Function that initiates the stream
) error {
    // Only the initial connection is protected
    return scb.cb.Execute(ctx, streamFn)
}

// StreamErrorClassifier determines if a streaming error should trip the circuit
// Note: Uses core.ErrStreamPartiallyCompleted defined in core/errors.go
func StreamErrorClassifier(err error) bool {
    // Connection refused, DNS failures, etc. should trip the circuit
    if isConnectionError(err) {
        return true
    }

    // Partial stream completion is NOT a circuit-breaking failure
    // (core.ErrStreamPartiallyCompleted is defined in Step 1a)
    if errors.Is(err, core.ErrStreamPartiallyCompleted) {
        return false
    }

    // Context cancellation is not a failure
    if errors.Is(err, context.Canceled) {
        return false
    }

    return DefaultErrorClassifier(err)
}
```

**Resilience Module Changes Required**:

| File | Change | Complexity |
|------|--------|------------|
| `resilience/streaming.go` | NEW: Streaming-aware circuit breaker wrapper | Medium |
| `resilience/retry.go` | Add `IsStreamingContext()` check to skip retry | Low |

**Important Documentation**:
- Retry should be **disabled** for streaming operations
- Circuit breaker should only wrap stream **initiation**, not the entire stream
- Add `StreamingErrorClassifier` that handles partial completions correctly

#### telemetry Module (`telemetry/`)

**Streaming-Specific Metrics** (add to existing metrics infrastructure):

```go
// telemetry/streaming_metrics.go (NEW FILE)

// Streaming metrics to add
var StreamingMetrics = ModuleConfig{
    Metrics: []MetricDefinition{
        {
            Name:    "ai.stream.duration_ms",
            Type:    "histogram",
            Help:    "Total duration of streaming response in milliseconds",
            Labels:  []string{"provider", "model", "status"},
            Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 30000, 60000},
        },
        {
            Name:   "ai.stream.chunks_delivered",
            Type:   "counter",
            Help:   "Number of chunks delivered during streaming",
            Labels: []string{"provider", "model"},
        },
        {
            Name:   "ai.stream.bytes_delivered",
            Type:   "counter",
            Help:   "Total bytes delivered via streaming",
            Labels: []string{"provider", "model"},
        },
        {
            Name:   "ai.stream.time_to_first_chunk_ms",
            Type:   "histogram",
            Help:   "Time to first chunk (TTFB) in milliseconds",
            Labels: []string{"provider", "model"},
            Buckets: []float64{50, 100, 200, 500, 1000, 2000, 5000},
        },
        {
            Name:   "ai.stream.partial_failures",
            Type:   "counter",
            Help:   "Streams that started but failed mid-stream",
            Labels: []string{"provider", "model", "error_type"},
        },
        {
            Name:   "ai.stream.active",
            Type:   "gauge",
            Help:   "Number of currently active streams",
            Labels: []string{"provider"},
        },
    },
}

// StreamMetricsRecorder records streaming-specific metrics
type StreamMetricsRecorder struct {
    startTime       time.Time
    firstChunkTime  time.Time
    chunksDelivered int
    bytesDelivered  int
    provider        string
    model           string
}

func NewStreamMetricsRecorder(provider, model string) *StreamMetricsRecorder {
    return &StreamMetricsRecorder{
        startTime: time.Now(),
        provider:  provider,
        model:     model,
    }
}

func (r *StreamMetricsRecorder) RecordChunk(chunk core.StreamChunk) {
    if r.chunksDelivered == 0 {
        r.firstChunkTime = time.Now()
        // Record time to first chunk
        Histogram("ai.stream.time_to_first_chunk_ms",
            float64(r.firstChunkTime.Sub(r.startTime).Milliseconds()),
            "provider", r.provider,
            "model", r.model)
    }

    r.chunksDelivered++
    r.bytesDelivered += len(chunk.Content)
}

func (r *StreamMetricsRecorder) Complete(status string) {
    duration := time.Since(r.startTime).Milliseconds()

    Histogram("ai.stream.duration_ms", float64(duration),
        "provider", r.provider,
        "model", r.model,
        "status", status)

    Counter("ai.stream.chunks_delivered",
        "provider", r.provider,
        "model", r.model,
        "+", fmt.Sprintf("%d", r.chunksDelivered))

    Counter("ai.stream.bytes_delivered",
        "provider", r.provider,
        "model", r.model,
        "+", fmt.Sprintf("%d", r.bytesDelivered))
}
```

**Telemetry Module Changes Required**:

| File | Change | Complexity |
|------|--------|------------|
| `telemetry/streaming_metrics.go` | NEW: Streaming-specific metrics | Low |
| `telemetry/modules.go` | Register streaming metrics module | Low |

### Updated Module Summary

| Module | Change Required | Complexity | Priority |
|--------|-----------------|------------|----------|
| `core` | Add streaming interface | Low | P0 |
| `ai` | Re-export streaming interface | Low | P0 |
| `ai/providers/*` | Implement streaming (5 providers) | Medium | P0 |
| `ai/chain_client` | Support streaming failover | High | P1 |
| `orchestration` | Add streaming ProcessRequest | High | P1 |
| `resilience` | Add streaming-aware patterns | Medium | P2 |
| `telemetry` | Add streaming metrics | Low | P2 |
| `examples/*` | Use streaming API | Low | P3 |

### Backward Compatibility

- **Fully backward compatible**: Existing code using `GenerateResponse()` continues to work
- **Opt-in streaming**: New `StreamResponse()` method is additive
- **Interface detection**: Code can check if a client supports streaming via type assertion

---

## Proposed Solution

### Design Principles

1. **Additive, not breaking**: New streaming interface extends existing functionality
2. **Optional adoption**: Components can choose to use streaming or not
3. **Provider abstraction**: Streaming works uniformly across all providers
4. **Error resilience**: Graceful handling of stream interruptions
5. **Resource cleanup**: Proper cancellation and cleanup on context cancel

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Chat UI (SSE)                           │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ SSE Events
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Travel Chat Agent                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ProcessWithStreaming()                                   │   │
│  │   - Calls orchestrator.ProcessRequestStreaming()         │   │
│  │   - Forwards chunks to SSE callback                      │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ Streaming callback
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Orchestrator                             │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ ProcessRequestStreaming()                                │   │
│  │   - Plans tool execution                                 │   │
│  │   - Executes tools (non-streaming for tool calls)        │   │
│  │   - Calls AI.StreamResponse() for synthesis              │   │
│  │   - Forwards tokens to callback                          │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ Token callback
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      AI Chain Client                            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ StreamResponse()                                         │   │
│  │   - Tries primary provider                               │   │
│  │   - Falls back on stream error                           │   │
│  │   - Aggregates usage stats                               │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ Provider-specific streaming
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    AI Provider (OpenAI/Anthropic/etc)           │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ StreamResponse()                                         │   │
│  │   - Opens streaming connection to provider API           │   │
│  │   - Yields tokens as they arrive                         │   │
│  │   - Returns final usage stats                            │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## API Design

### New Core Interfaces (`core/interfaces.go`)

```go
// StreamChunk represents a single chunk in a streaming response
type StreamChunk struct {
    // Content is the text content of this chunk (may be empty for metadata-only chunks)
    Content string

    // Delta indicates this is an incremental addition (vs replacement)
    Delta bool

    // Index is the sequential chunk number (0-indexed)
    Index int

    // FinishReason is set on the final chunk (e.g., "stop", "length", "tool_calls")
    FinishReason string

    // Model identifies which model generated this chunk
    Model string

    // Usage is populated on the final chunk with complete token counts
    Usage *TokenUsage

    // Metadata contains optional phase/status information for non-content chunks
    // Used for progress updates during planning/tool execution phases
    Metadata map[string]interface{}
}

// StreamCallback is called for each chunk in a streaming response.
// Return an error to abort the stream.
type StreamCallback func(chunk StreamChunk) error

// StreamingAIClient extends AIClient with streaming support.
// Implementations should check if the underlying provider supports streaming
// and fall back to simulated streaming if not.
type StreamingAIClient interface {
    AIClient

    // StreamResponse generates a response and streams chunks via callback.
    // The callback is invoked for each chunk as it arrives.
    // Returns the final aggregated response (for logging/metrics) and any error.
    // If the context is canceled, the stream is aborted and context.Canceled returned.
    StreamResponse(ctx context.Context, prompt string, options *AIOptions, callback StreamCallback) (*AIResponse, error)

    // SupportsStreaming returns true if this client supports native streaming.
    // If false, StreamResponse will simulate streaming by chunking the response.
    SupportsStreaming() bool
}

// StreamOptions extends AIOptions with streaming-specific settings
type StreamOptions struct {
    AIOptions

    // ChunkSize hints at preferred chunk size (provider may ignore)
    ChunkSize int

    // IncludeUsageInStream requests usage stats in intermediate chunks (if supported)
    IncludeUsageInStream bool

    // OnToolCall is invoked when the model wants to call a tool (for function calling)
    OnToolCall func(toolName string, arguments map[string]interface{}) (string, error)
}
```

### AI Module Re-exports (`ai/interfaces.go`)

```go
package ai

import "github.com/itsneelabh/gomind/core"

// Re-export streaming types for convenience
type (
    StreamChunk         = core.StreamChunk
    StreamCallback      = core.StreamCallback
    StreamingAIClient   = core.StreamingAIClient
    StreamOptions       = core.StreamOptions
)
```

### Provider Implementation Interface

Each provider implements:

```go
// In ai/providers/openai/client.go
type Client struct {
    // ... existing fields
}

// Existing method (unchanged)
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error)

// New streaming method
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error)

// Capability check
func (c *Client) SupportsStreaming() bool {
    return true
}
```

### Chain Client Streaming (`ai/chain_client.go`)

```go
// StreamResponse implements streaming with failover support
func (c *ChainClient) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    var lastErr error

    for _, provider := range c.providers {
        // Check if provider supports streaming
        streamingProvider, ok := provider.(core.StreamingAIClient)
        if !ok {
            c.logger.Debug("Provider does not support streaming, trying next", map[string]interface{}{
                "provider": fmt.Sprintf("%T", provider),
            })
            continue
        }

        // Attempt streaming
        resp, err := streamingProvider.StreamResponse(ctx, prompt, options, callback)
        if err == nil {
            return resp, nil
        }

        lastErr = err

        // Check if error is retryable
        if !c.isRetryableError(err) {
            return nil, err
        }

        c.logger.Warn("Streaming failed, trying next provider", map[string]interface{}{
            "provider": fmt.Sprintf("%T", provider),
            "error":    err.Error(),
        })
    }

    return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

func (c *ChainClient) SupportsStreaming() bool {
    // Returns true if at least one provider supports streaming
    for _, provider := range c.providers {
        if sp, ok := provider.(core.StreamingAIClient); ok && sp.SupportsStreaming() {
            return true
        }
    }
    return false
}
```

### Orchestrator Streaming (`orchestration/orchestrator.go`)

```go
// StreamingOrchestratorResponse extends OrchestratorResponse with streaming metadata
type StreamingOrchestratorResponse struct {
    OrchestratorResponse  // Embed the standard response

    // ChunksDelivered is the count of chunks sent via callback
    ChunksDelivered int `json:"chunks_delivered"`

    // StreamDuration is how long the stream was active
    StreamDuration time.Duration `json:"stream_duration"`
}

// ProcessRequestStreaming processes a request with streaming response
// This is a new method on AIOrchestrator (the concrete implementation)
func (o *AIOrchestrator) ProcessRequestStreaming(
    ctx context.Context,
    request string,
    metadata map[string]interface{},
    callback core.StreamCallback,
) (*StreamingOrchestratorResponse, error) {
    startTime := time.Now()

    // Phase 1: Planning (non-streaming) - uses existing router
    plan, err := o.router.Route(ctx, request, o.catalog.GetServices())
    if err != nil {
        return nil, fmt.Errorf("routing failed: %w", err)
    }

    // Phase 2: Tool/Agent Execution (non-streaming, parallel where possible)
    // Uses existing executor
    execResult, err := o.executor.Execute(ctx, plan)
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w", err)
    }

    // Phase 3: Synthesis (STREAMING)
    // Check if AI client supports streaming
    streamingAI, ok := o.aiClient.(core.StreamingAIClient)
    if !ok || !streamingAI.SupportsStreaming() {
        // Fall back to non-streaming with simulated chunking
        return o.processRequestNonStreaming(ctx, request, metadata, callback)
    }

    // Build synthesis prompt from execution results
    synthesisPrompt := o.synthesizer.BuildPrompt(request, execResult)

    // Stream the synthesis response
    var chunksDelivered int
    resp, err := streamingAI.StreamResponse(ctx, synthesisPrompt, nil, func(chunk core.StreamChunk) error {
        chunksDelivered++
        return callback(chunk)
    })
    if err != nil {
        return nil, fmt.Errorf("streaming synthesis failed: %w", err)
    }

    return &StreamingOrchestratorResponse{
        OrchestratorResponse: OrchestratorResponse{
            RequestID:       generateRequestID(),
            OriginalRequest: request,
            Response:        resp.Content,
            RoutingMode:     plan.Mode,
            ExecutionTime:   time.Since(startTime),
            AgentsInvolved:  extractAgentNames(execResult),
            Confidence:      calculateConfidence(execResult),
        },
        ChunksDelivered: chunksDelivered,
        StreamDuration:  time.Since(startTime),
    }, nil
}

// Fallback for non-streaming providers - simulates streaming
func (o *AIOrchestrator) processRequestNonStreaming(
    ctx context.Context,
    request string,
    metadata map[string]interface{},
    callback core.StreamCallback,
) (*StreamingOrchestratorResponse, error) {
    // Use existing ProcessRequest (blocking)
    result, err := o.ProcessRequest(ctx, request, metadata)
    if err != nil {
        return nil, err
    }

    // Simulate streaming by chunking the complete response
    chunks := chunkResponse(result.Response, 50) // 50 chars per chunk
    for i, chunk := range chunks {
        if err := callback(core.StreamChunk{
            Content: chunk,
            Delta:   true,
            Index:   i,
        }); err != nil {
            return nil, err
        }
    }

    // Send final chunk with finish reason
    callback(core.StreamChunk{
        FinishReason: "stop",
        Index:        len(chunks),
    })

    return &StreamingOrchestratorResponse{
        OrchestratorResponse: *result,
        ChunksDelivered:      len(chunks) + 1,
        StreamDuration:       result.ExecutionTime,
    }, nil
}

// Helper function to chunk a response string
func chunkResponse(response string, chunkSize int) []string {
    var chunks []string
    for i := 0; i < len(response); i += chunkSize {
        end := i + chunkSize
        if end > len(response) {
            end = len(response)
        }
        chunks = append(chunks, response[i:end])
    }
    return chunks
}
```

---

## Provider Implementations

### OpenAI Provider (`ai/providers/openai/client.go`)

The current OpenAI provider uses raw HTTP requests (not the official SDK). Here's the implementation that matches the existing code structure:

```go
// StreamResponse implements streaming for OpenAI-compatible APIs
// NOTE: The current client uses raw HTTP, so we'll use Server-Sent Events (SSE)
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Start distributed tracing span
    ctx, span := c.StartSpan(ctx, "ai.stream_response")
    defer span.End()

    span.SetAttribute("ai.provider", "openai")
    span.SetAttribute("ai.streaming", true)

    if c.apiKey == "" {
        return nil, fmt.Errorf("OpenAI API key not configured")
    }

    // Apply defaults and resolve model
    options = c.ApplyDefaults(options)
    options.Model = ResolveModel(c.providerAlias, options.Model)

    // Build messages array
    messages := []map[string]string{}
    if options.SystemPrompt != "" {
        messages = append(messages, map[string]string{
            "role":    "system",
            "content": options.SystemPrompt,
        })
    }
    messages = append(messages, map[string]string{
        "role":    "user",
        "content": prompt,
    })

    // Build request body with stream=true
    reqBody := map[string]interface{}{
        "model":       options.Model,
        "messages":    messages,
        "temperature": options.Temperature,
        "max_tokens":  options.MaxTokens,
        "stream":      true,  // Enable streaming
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Accept", "text/event-stream")

    // Execute request (no retry for streaming - complex state management)
    resp, err := c.BaseClient.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, c.HandleError(resp.StatusCode, body, "OpenAI")
    }

    // Process SSE stream
    var fullContent strings.Builder
    var model string
    var chunkIndex int

    reader := bufio.NewReader(resp.Body)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return nil, fmt.Errorf("stream read error: %w", err)
        }

        line = strings.TrimSpace(line)
        if line == "" || line == "data: [DONE]" {
            continue
        }

        if !strings.HasPrefix(line, "data: ") {
            continue
        }

        // Parse JSON from SSE data
        data := strings.TrimPrefix(line, "data: ")
        var streamResp struct {
            Model   string `json:"model"`
            Choices []struct {
                Delta struct {
                    Content string `json:"content"`
                } `json:"delta"`
                FinishReason string `json:"finish_reason"`
            } `json:"choices"`
        }

        if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
            continue // Skip malformed chunks
        }

        model = streamResp.Model

        if len(streamResp.Choices) > 0 {
            delta := streamResp.Choices[0].Delta.Content
            if delta != "" {
                fullContent.WriteString(delta)

                chunk := core.StreamChunk{
                    Content: delta,
                    Delta:   true,
                    Index:   chunkIndex,
                    Model:   model,
                }

                if streamResp.Choices[0].FinishReason != "" {
                    chunk.FinishReason = streamResp.Choices[0].FinishReason
                }

                if err := callback(chunk); err != nil {
                    return nil, fmt.Errorf("callback error: %w", err)
                }
                chunkIndex++
            }
        }
    }

    // Log completion
    c.LogResponse("openai", model, core.TokenUsage{}, time.Since(time.Now()))

    return &core.AIResponse{
        Content: fullContent.String(),
        Model:   model,
        Usage:   core.TokenUsage{}, // OpenAI doesn't provide usage in streaming mode by default
    }, nil
}

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Key Implementation Details:**
- Uses raw HTTP with `Accept: text/event-stream` header
- Parses Server-Sent Events (SSE) format manually
- Handles `data: [DONE]` termination signal
- No retry logic for streaming (state is complex)
- Aggregates content for final response return

### Anthropic Provider (`ai/providers/anthropic/client.go`)

The current Anthropic provider uses raw HTTP requests (not an SDK). Here's the streaming implementation matching the existing code structure:

```go
// StreamResponse implements streaming for Anthropic Messages API
// NOTE: Uses raw HTTP with SSE, matching the existing GenerateResponse pattern
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Start distributed tracing span
    ctx, span := c.StartSpan(ctx, "ai.stream_response")
    defer span.End()

    span.SetAttribute("ai.provider", "anthropic")
    span.SetAttribute("ai.streaming", true)

    if c.apiKey == "" {
        return nil, fmt.Errorf("anthropic API key not configured")
    }

    // Apply defaults and resolve model
    options = c.ApplyDefaults(options)
    options.Model = resolveModel(options.Model)

    // Build messages in Anthropic format
    messages := []Message{
        {Role: "user", Content: prompt},
    }

    // Build request body with stream=true
    reqBody := map[string]interface{}{
        "model":      options.Model,
        "messages":   messages,
        "max_tokens": options.MaxTokens,
        "stream":     true,
    }

    if options.SystemPrompt != "" {
        reqBody["system"] = options.SystemPrompt
    }

    if options.Temperature > 0 {
        reqBody["temperature"] = options.Temperature
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("anthropic-version", APIVersion)
    req.Header.Set("Accept", "text/event-stream")

    // Execute request (no retry for streaming)
    resp, err := c.BaseClient.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, c.HandleError(resp.StatusCode, body, "Anthropic")
    }

    // Process SSE stream
    var fullContent strings.Builder
    var model string
    var chunkIndex int
    var usage core.TokenUsage

    reader := bufio.NewReader(resp.Body)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return nil, fmt.Errorf("stream read error: %w", err)
        }

        line = strings.TrimSpace(line)
        if line == "" || !strings.HasPrefix(line, "data: ") {
            continue
        }

        data := strings.TrimPrefix(line, "data: ")

        // Parse SSE event
        var event struct {
            Type  string `json:"type"`
            Delta struct {
                Type string `json:"type"`
                Text string `json:"text"`
            } `json:"delta"`
            Message struct {
                Model string `json:"model"`
                Usage struct {
                    InputTokens  int `json:"input_tokens"`
                    OutputTokens int `json:"output_tokens"`
                } `json:"usage"`
                StopReason string `json:"stop_reason"`
            } `json:"message"`
        }

        if err := json.Unmarshal([]byte(data), &event); err != nil {
            continue // Skip malformed events
        }

        switch event.Type {
        case "content_block_delta":
            if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
                fullContent.WriteString(event.Delta.Text)

                if err := callback(core.StreamChunk{
                    Content: event.Delta.Text,
                    Delta:   true,
                    Index:   chunkIndex,
                    Model:   model,
                }); err != nil {
                    return nil, fmt.Errorf("callback error: %w", err)
                }
                chunkIndex++
            }
        case "message_start":
            model = event.Message.Model
        case "message_delta":
            usage = core.TokenUsage{
                PromptTokens:     event.Message.Usage.InputTokens,
                CompletionTokens: event.Message.Usage.OutputTokens,
                TotalTokens:      event.Message.Usage.InputTokens + event.Message.Usage.OutputTokens,
            }
            // Send final chunk
            callback(core.StreamChunk{
                FinishReason: event.Message.StopReason,
                Index:        chunkIndex,
                Usage:        &usage,
            })
        }
    }

    return &core.AIResponse{
        Content: fullContent.String(),
        Model:   model,
        Usage:   usage,
    }, nil
}

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required imports to add:**
```go
import (
    "bufio"    // ADD
    "strings"  // ADD if not present
)
```

### Gemini Provider (`ai/providers/gemini/client.go`)

The current Gemini provider uses raw HTTP requests. Here's the streaming implementation:

```go
// StreamResponse implements streaming for Google Gemini API
// Uses the streamGenerateContent endpoint with SSE
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Start distributed tracing span
    ctx, span := c.StartSpan(ctx, "ai.stream_response")
    defer span.End()

    span.SetAttribute("ai.provider", "gemini")
    span.SetAttribute("ai.streaming", true)

    if c.apiKey == "" {
        return nil, fmt.Errorf("gemini API key not configured")
    }

    // Apply defaults
    options = c.ApplyDefaults(options)
    options.Model = resolveModel(options.Model)

    // Build contents in Gemini format
    contents := []Content{
        {
            Role: "user",
            Parts: []Part{
                {Text: prompt},
            },
        },
    }

    // Build request body
    reqBody := GeminiRequest{
        Contents: contents,
        GenerationConfig: &GenerationConfig{
            Temperature:     options.Temperature,
            MaxOutputTokens: options.MaxTokens,
        },
    }

    if options.SystemPrompt != "" {
        reqBody.SystemInstruction = &SystemInstruction{
            Parts: []Part{
                {Text: options.SystemPrompt},
            },
        }
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request to streaming endpoint
    // Format: /models/{model}:streamGenerateContent?key={api_key}&alt=sse
    url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, options.Model, c.apiKey)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "text/event-stream")

    // Execute request (no retry for streaming)
    resp, err := c.BaseClient.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, c.HandleError(resp.StatusCode, body, "Gemini")
    }

    // Process SSE stream
    var fullContent strings.Builder
    var chunkIndex int
    var usage core.TokenUsage

    reader := bufio.NewReader(resp.Body)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return nil, fmt.Errorf("stream read error: %w", err)
        }

        line = strings.TrimSpace(line)
        if line == "" || !strings.HasPrefix(line, "data: ") {
            continue
        }

        data := strings.TrimPrefix(line, "data: ")

        // Parse Gemini streaming response
        var streamResp struct {
            Candidates []struct {
                Content struct {
                    Parts []struct {
                        Text string `json:"text"`
                    } `json:"parts"`
                } `json:"content"`
                FinishReason string `json:"finishReason"`
            } `json:"candidates"`
            UsageMetadata struct {
                PromptTokenCount     int `json:"promptTokenCount"`
                CandidatesTokenCount int `json:"candidatesTokenCount"`
                TotalTokenCount      int `json:"totalTokenCount"`
            } `json:"usageMetadata"`
        }

        if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
            continue // Skip malformed chunks
        }

        if len(streamResp.Candidates) > 0 {
            candidate := streamResp.Candidates[0]
            for _, part := range candidate.Content.Parts {
                if part.Text != "" {
                    fullContent.WriteString(part.Text)

                    chunk := core.StreamChunk{
                        Content: part.Text,
                        Delta:   true,
                        Index:   chunkIndex,
                        Model:   options.Model,
                    }

                    if candidate.FinishReason != "" {
                        chunk.FinishReason = candidate.FinishReason
                    }

                    if err := callback(chunk); err != nil {
                        return nil, fmt.Errorf("callback error: %w", err)
                    }
                    chunkIndex++
                }
            }
        }

        // Capture usage metadata
        if streamResp.UsageMetadata.TotalTokenCount > 0 {
            usage = core.TokenUsage{
                PromptTokens:     streamResp.UsageMetadata.PromptTokenCount,
                CompletionTokens: streamResp.UsageMetadata.CandidatesTokenCount,
                TotalTokens:      streamResp.UsageMetadata.TotalTokenCount,
            }
        }
    }

    return &core.AIResponse{
        Content: fullContent.String(),
        Model:   options.Model,
        Usage:   usage,
    }, nil
}

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required imports to add:**
```go
import (
    "bufio"    // ADD
    "strings"  // ADD if not present
)
```

### Bedrock Provider (`ai/providers/bedrock/client.go`)

**IMPORTANT**: Bedrock already has a `StreamResponse` method (lines 211-351) but with a **different signature**. The existing implementation uses a channel-based approach:

```go
// CURRENT signature (channel-based) - needs refactoring
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error
```

This needs to be refactored to match the unified callback-based interface:

```go
// StreamResponse implements streaming for AWS Bedrock using ConverseStream API
// REFACTORED from channel-based to callback-based interface
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Start distributed tracing span
    ctx, span := c.StartSpan(ctx, "ai.stream_response")
    defer span.End()

    span.SetAttribute("ai.provider", "bedrock")
    span.SetAttribute("ai.streaming", true)
    span.SetAttribute("ai.region", c.region)

    // Apply defaults
    options = c.ApplyDefaults(options)
    span.SetAttribute("ai.model", options.Model)

    // Build messages for ConverseStream API
    messages := []types.Message{
        {
            Role: types.ConversationRoleUser,
            Content: []types.ContentBlock{
                &types.ContentBlockMemberText{
                    Value: prompt,
                },
            },
        },
    }

    // Build the ConverseStream input
    input := &bedrockruntime.ConverseStreamInput{
        ModelId:  aws.String(options.Model),
        Messages: messages,
    }

    // Add system prompt if provided
    if options.SystemPrompt != "" {
        input.System = []types.SystemContentBlock{
            &types.SystemContentBlockMemberText{
                Value: options.SystemPrompt,
            },
        }
    }

    // Add inference configuration
    inferenceConfig := &types.InferenceConfiguration{}
    if options.MaxTokens > 0 {
        inferenceConfig.MaxTokens = aws.Int32(int32(options.MaxTokens))
    }
    if options.Temperature > 0 {
        inferenceConfig.Temperature = aws.Float32(options.Temperature)
    }
    input.InferenceConfig = inferenceConfig

    // Start the stream
    output, err := c.bedrockClient.ConverseStream(ctx, input)
    if err != nil {
        span.RecordError(err)
        return nil, fmt.Errorf("bedrock stream error: %w", err)
    }

    // Process the stream
    eventStream := output.GetStream()
    defer eventStream.Close()

    var fullContent strings.Builder
    var chunkIndex int
    var usage core.TokenUsage

    for {
        event, ok := <-eventStream.Events()
        if !ok {
            break
        }

        switch v := event.(type) {
        case *types.ConverseStreamOutputMemberContentBlockDelta:
            if v.Value.Delta != nil {
                switch d := v.Value.Delta.(type) {
                case *types.ContentBlockDeltaMemberText:
                    fullContent.WriteString(d.Value)

                    // Call the callback instead of sending to channel
                    if err := callback(core.StreamChunk{
                        Content: d.Value,
                        Delta:   true,
                        Index:   chunkIndex,
                        Model:   options.Model,
                    }); err != nil {
                        span.RecordError(err)
                        return nil, fmt.Errorf("callback error: %w", err)
                    }
                    chunkIndex++
                }
            }
        case *types.ConverseStreamOutputMemberMetadata:
            // Capture usage from metadata
            if v.Value.Usage != nil {
                usage = core.TokenUsage{
                    PromptTokens:     int(*v.Value.Usage.InputTokens),
                    CompletionTokens: int(*v.Value.Usage.OutputTokens),
                    TotalTokens:      int(*v.Value.Usage.TotalTokens),
                }
            }
        case *types.ConverseStreamOutputMemberMessageStop:
            // Send final chunk with stop reason
            callback(core.StreamChunk{
                FinishReason: string(v.Value.StopReason),
                Index:        chunkIndex,
                Usage:        &usage,
            })
        }
    }

    // Check for stream errors
    if err := eventStream.Err(); err != nil {
        span.RecordError(err)
        return nil, fmt.Errorf("bedrock stream error: %w", err)
    }

    span.SetAttribute("ai.stream_completed", true)
    span.SetAttribute("ai.chunks_delivered", chunkIndex)

    return &core.AIResponse{
        Content: fullContent.String(),
        Model:   options.Model,
        Usage:   usage,
    }, nil
}

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Migration Note**: The existing channel-based `StreamResponse` should be deprecated and replaced with this callback-based version to maintain interface consistency across all providers.

### Mock Provider (`ai/providers/mock/provider.go`)

The existing Mock provider needs streaming support for testing. **Update the existing file** rather than creating a new one:

```go
// ADD to ai/providers/mock/provider.go

// StreamResponse returns a mock streaming response for testing
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    c.CallCount++
    c.LastPrompt = prompt
    c.LastOptions = options

    // Check for context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Return configured error if set
    if c.Error != nil {
        return nil, c.Error
    }

    // Return next response from list, chunked for streaming simulation
    if c.ResponseIndex >= len(c.Responses) {
        return nil, errors.New("no more mock responses")
    }

    response := c.Responses[c.ResponseIndex]
    c.ResponseIndex++

    // Use options if provided, otherwise use defaults
    model := "mock-model"
    if options != nil && options.Model != "" {
        model = options.Model
    } else if c.Config != nil && c.Config.Model != "" {
        model = c.Config.Model
    }

    // Stream the response character by character (or in configurable chunks)
    chunkSize := 10 // Default chunk size for mock
    if c.ChunkSize > 0 {
        chunkSize = c.ChunkSize
    }

    var chunkIndex int
    for i := 0; i < len(response); i += chunkSize {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        end := i + chunkSize
        if end > len(response) {
            end = len(response)
        }

        chunk := response[i:end]
        if err := callback(core.StreamChunk{
            Content: chunk,
            Delta:   true,
            Index:   chunkIndex,
            Model:   model,
        }); err != nil {
            return nil, err
        }
        chunkIndex++

        // Optional delay for realistic streaming simulation
        if c.StreamDelay > 0 {
            time.Sleep(c.StreamDelay)
        }
    }

    // Send final chunk
    usage := core.TokenUsage{
        PromptTokens:     len(prompt) / 4,
        CompletionTokens: len(response) / 4,
        TotalTokens:      (len(prompt) + len(response)) / 4,
    }

    callback(core.StreamChunk{
        FinishReason: "stop",
        Index:        chunkIndex,
        Usage:        &usage,
    })

    return &core.AIResponse{
        Content: response,
        Model:   model,
        Usage:   usage,
    }, nil
}

func (c *Client) SupportsStreaming() bool {
    return true
}

// ADD these fields to the Client struct
type Client struct {
    // ... existing fields ...
    ChunkSize   int           // Size of chunks for streaming (default: 10)
    StreamDelay time.Duration // Delay between chunks for realistic simulation
}
```

**Why**: Having streaming support in the mock provider enables comprehensive unit testing of streaming functionality without requiring real API calls.

---

## Orchestrator Integration

### Streaming-Aware Orchestration Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     ProcessRequestStreaming Flow                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. PLANNING PHASE (Non-streaming)                                      │
│     ┌──────────────────────────────────────────────────────────────┐    │
│     │ • Analyze query intent                                        │    │
│     │ • Determine required tools/agents                             │    │
│     │ • Build execution plan                                        │    │
│     │ • Emit: callback(StreamChunk{Content: "Planning..."})        │    │
│     └──────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  2. TOOL EXECUTION PHASE (Non-streaming, parallel)                      │
│     ┌──────────────────────────────────────────────────────────────┐    │
│     │ • Execute tools in parallel where possible                    │    │
│     │ • Collect results from each tool                              │    │
│     │ • Emit progress: callback(StreamChunk{Content: "..."})       │    │
│     │   - "Checking weather..."                                     │    │
│     │   - "Getting currency rates..."                               │    │
│     │   - "Fetching country info..."                                │    │
│     └──────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  3. SYNTHESIS PHASE (STREAMING)                                         │
│     ┌──────────────────────────────────────────────────────────────┐    │
│     │ • Build synthesis prompt with tool results                    │    │
│     │ • Call AI.StreamResponse()                                    │    │
│     │ • Forward each token to callback                              │    │
│     │ • Emit: callback(StreamChunk{Content: "Based on..."})        │    │
│     │        callback(StreamChunk{Content: " the weather"})        │    │
│     │        callback(StreamChunk{Content: " in Tokyo"})           │    │
│     │        ... token by token ...                                 │    │
│     └──────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│                              ▼                                           │
│  4. COMPLETION                                                          │
│     ┌──────────────────────────────────────────────────────────────┐    │
│     │ • Emit final chunk with usage stats                           │    │
│     │ • Return StreamingResult                                      │    │
│     └──────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Status Updates During Non-Streaming Phases

The orchestrator should emit status updates during planning and tool execution:

```go
// In ProcessRequestStreaming
func (o *Orchestrator) ProcessRequestStreaming(...) {
    // Planning phase - emit status
    callback(core.StreamChunk{
        Content: "", // Empty content, just metadata
        Metadata: map[string]interface{}{
            "phase": "planning",
            "status": "Analyzing your request...",
        },
    })

    plan, err := o.planExecution(ctx, query, opts)
    // ...

    // Tool execution - emit per-tool status
    for _, tool := range plan.Tools {
        callback(core.StreamChunk{
            Content: "",
            Metadata: map[string]interface{}{
                "phase": "tool_execution",
                "tool": tool.Name,
                "status": fmt.Sprintf("Calling %s...", tool.Name),
            },
        })

        result, err := o.executeTool(ctx, tool)
        // ...
    }

    // Synthesis - stream actual content
    callback(core.StreamChunk{
        Content: "",
        Metadata: map[string]interface{}{
            "phase": "synthesis",
            "status": "Generating response...",
        },
    })

    // Now stream AI response...
}
```

---

## Migration Strategy

### Phase 1: Core Interface (Week 1)

1. Add `StreamChunk`, `StreamCallback`, `StreamingAIClient` to `core/interfaces.go`
2. Re-export types in `ai/interfaces.go`
3. Add `SupportsStreaming()` method to interface
4. No breaking changes to existing code

### Phase 2: Provider Implementations (Week 2-3)

1. Implement `StreamResponse` in OpenAI provider
2. Implement `StreamResponse` in Anthropic provider
3. Implement `StreamResponse` in Gemini provider
4. Implement `StreamResponse` in Bedrock provider
5. Each provider can be done independently

### Phase 3: Chain Client (Week 3)

1. Add `StreamResponse` to `ChainClient`
2. Implement failover logic for streaming
3. Handle partial stream failures

### Phase 4: Orchestrator (Week 4)

1. Add `ProcessRequestStreaming` method
2. Implement streaming synthesis
3. Add status updates during non-streaming phases
4. Maintain backward compatibility with `ProcessRequest`

### Phase 5: Example Updates (Week 5)

1. Update `travel-chat-agent` to use streaming
2. Update SSE handler for true streaming
3. Test end-to-end streaming experience

### Phase 6: Documentation & Testing (Week 6)

1. Comprehensive unit tests
2. Integration tests
3. Update documentation
4. Performance benchmarks

---

## Detailed Implementation Checklist

This section provides explicit file paths, line references, and exact changes needed.

### Line Number Verification Summary (Verified 2026-01-07)

All line numbers in this document have been verified against the current codebase:

| File | Target Location | Verified Line | Status |
|------|-----------------|---------------|--------|
| `core/errors.go` | After ErrAIOperationFailed | Line 49 | ✅ Verified |
| `core/interfaces.go` | After TokenUsage struct | Line 87 | ✅ Verified |
| `ai/interfaces.go` | After AIClient re-export | Line 12 | ✅ Verified |
| `ai/providers/openai/client.go` | After GenerateResponse | Line 233 | ✅ Verified |
| `ai/providers/anthropic/client.go` | After GenerateResponse | Line 247 | ✅ Verified |
| `ai/providers/gemini/client.go` | After GenerateResponse | Line 262 | ✅ Verified |
| `ai/providers/bedrock/client.go` | Existing StreamResponse | Lines 211-351 | ✅ Verified |
| `ai/providers/mock/provider.go` | After GenerateResponse | Line 113 | ✅ Verified |
| `ai/chain_client.go` | After GenerateResponse | Line 361 | ✅ Verified |
| `orchestration/interfaces.go` | After OrchestratorResponse | Line 67 | ✅ Verified |
| `orchestration/orchestrator.go` | After ProcessRequest | Line 537 | ✅ Verified |

**Note:** If code is modified before implementing streaming, line numbers may shift. Always verify exact insertion points by searching for the method/struct names mentioned.

### Step 1: Core Changes

#### Step 1a: Core Errors (`core/errors.go`)

**File:** `core/errors.go`
**Location:** After `ErrAIOperationFailed` (line 49)
**Action:** ADD new streaming error

```go
// ADD after line 49 (after ErrAIOperationFailed)

	// Streaming errors
	ErrStreamPartiallyCompleted = errors.New("stream partially completed before interruption")
```

**Why:** This error is used by the resilience module's `StreamErrorClassifier` to distinguish partial stream completion (not a circuit-breaking failure) from connection failures. Defining it in `core/errors.go` follows the framework's error handling pattern and allows cross-module usage without circular dependencies.

#### Step 1b: Core Interface (`core/interfaces.go`)

**File:** `core/interfaces.go`
**Location:** After `AIResponse` struct (around line 87)
**Action:** ADD new types

```go
// ADD after line 87 (after TokenUsage struct)

// StreamChunk represents a single chunk in a streaming response
type StreamChunk struct {
    Content      string                 `json:"content,omitempty"`
    Delta        bool                   `json:"delta"`
    Index        int                    `json:"index"`
    FinishReason string                 `json:"finish_reason,omitempty"`
    Model        string                 `json:"model,omitempty"`
    Usage        *TokenUsage            `json:"usage,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// StreamCallback is called for each chunk in a streaming response
type StreamCallback func(chunk StreamChunk) error

// StreamingAIClient extends AIClient with streaming support
type StreamingAIClient interface {
    AIClient
    StreamResponse(ctx context.Context, prompt string, options *AIOptions, callback StreamCallback) (*AIResponse, error)
    SupportsStreaming() bool
}
```

**Why:** Core interfaces must be defined in `core` module to avoid circular dependencies. All other modules import from `core`.

### Step 2: AI Module Re-exports (`ai/interfaces.go`)

**File:** `ai/interfaces.go`
**Action:** ADD re-exports after existing AIClient re-export

```go
// ADD after line 12

// Re-export streaming types for convenience
type (
    StreamChunk       = core.StreamChunk
    StreamCallback    = core.StreamCallback
    StreamingAIClient = core.StreamingAIClient
)
```

**Why:** Allows consumers to import from `ai` module instead of directly from `core`.

### Step 3: OpenAI Provider (`ai/providers/openai/client.go`)

**File:** `ai/providers/openai/client.go`
**Location:** After `GenerateResponse` method (after line 233)
**Action:** ADD two new methods

```go
// ADD after line 233 (after GenerateResponse)

// StreamResponse implements streaming for OpenAI API
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Implementation as shown in Provider Implementations section above
}

// SupportsStreaming returns true as OpenAI supports native streaming
func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required imports to add:**
```go
import (
    "bufio"    // ADD
    "strings"  // ADD if not present
)
```

**Why:** OpenAI's API supports SSE streaming natively. This implementation uses raw HTTP to match the existing client pattern.

### Step 4: Anthropic Provider (`ai/providers/anthropic/client.go`)

**File:** `ai/providers/anthropic/client.go`
**Location:** After `GenerateResponse` method (after line 247)
**Action:** ADD two new methods

```go
// ADD after line 247 (after GenerateResponse)

// StreamResponse implements streaming for Anthropic API
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Implementation as shown in Provider Implementations section above
}

// SupportsStreaming returns true as Anthropic supports native streaming
func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required imports to add:**
```go
import (
    "bufio"    // ADD
    "strings"  // ADD if not present
)
```

**Why:** Anthropic's Messages API supports streaming via SSE. Uses raw HTTP to match existing code pattern.

### Step 4b: Gemini Provider (`ai/providers/gemini/client.go`)

**File:** `ai/providers/gemini/client.go`
**Location:** After `GenerateResponse` method (after line 262)
**Action:** ADD two new methods

```go
// ADD after line 262 (after GenerateResponse)

// StreamResponse implements streaming for Gemini API
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Implementation as shown in Provider Implementations section above
}

// SupportsStreaming returns true as Gemini supports native streaming
func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required imports to add:**
```go
import (
    "bufio"    // ADD
    "strings"  // ADD if not present
)
```

**Why:** Gemini's API supports SSE streaming via the `streamGenerateContent` endpoint.

### Step 4c: Bedrock Provider (`ai/providers/bedrock/client.go`)

**File:** `ai/providers/bedrock/client.go`
**Location:** Replace existing `StreamResponse` method (lines 211-351)
**Action:** REFACTOR signature from channel-based to callback-based

**IMPORTANT BUILD CONSTRAINT:** This file has build tags that must be preserved:
```go
//go:build bedrock
// +build bedrock
```
The streaming changes only affect builds that include the `bedrock` tag (used in AWS deployments).

**Current signature (REMOVE lines 211-351):**
```go
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error
```

**New signature (REPLACE WITH):**
```go
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error)

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Required import additions:** (already present via AWS SDK, verify `strings` is added)
```go
import (
    "strings"  // ADD if not present - needed for content building
)
```

**Why:** Bedrock already has streaming but with incompatible signature. Refactoring to callback-based interface ensures consistency across all providers.

### Step 4d: Mock Provider (`ai/providers/mock/provider.go`)

**File:** `ai/providers/mock/provider.go`
**Location:** After `GenerateResponse` method (after line 113)
**Action:** ADD streaming support + new struct fields

**Required import additions:**
```go
import (
    "time"  // ADD - needed for StreamDelay field
)
```

```go
// ADD after line 113 (after GenerateResponse method, before SetResponses helper)

// StreamResponse returns a mock streaming response for testing
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Implementation as shown in Provider Implementations section above
}

func (c *Client) SupportsStreaming() bool {
    return true
}
```

**Also MODIFY Client struct (lines 52-60) to add streaming fields:**
```go
type Client struct {
    Config        *ai.AIConfig
    Responses     []string
    ResponseIndex int
    Error         error
    CallCount     int
    LastPrompt    string
    LastOptions   *core.AIOptions
    ChunkSize     int           // ADD: Size of chunks for streaming (default: 10)
    StreamDelay   time.Duration // ADD: Delay between chunks for realistic simulation
}
```

**Why:** Mock provider with streaming enables comprehensive unit testing without real API calls.

### Step 5: Chain Client (`ai/chain_client.go`)

**File:** `ai/chain_client.go`
**Location:** After `GenerateResponse` method (after line 361)
**Action:** ADD two new methods

```go
// ADD after line 361 (after GenerateResponse)

// StreamResponse implements streaming with failover support
func (c *ChainClient) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // Implementation as shown in Chain Client Streaming section above
}

// SupportsStreaming returns true if at least one provider supports streaming
func (c *ChainClient) SupportsStreaming() bool {
    for _, provider := range c.providers {
        if sp, ok := provider.(core.StreamingAIClient); ok && sp.SupportsStreaming() {
            return true
        }
    }
    return false
}
```

**Why:** Chain client must support streaming to enable failover during streamed responses.

### Step 6: Orchestrator Interface (`orchestration/interfaces.go`)

**File:** `orchestration/interfaces.go`
**Location:** After `OrchestratorResponse` struct (after line 67)
**Action:** ADD streaming response type

```go
// ADD after line 67

// StreamingOrchestratorResponse extends OrchestratorResponse with streaming metadata
type StreamingOrchestratorResponse struct {
    OrchestratorResponse
    ChunksDelivered int           `json:"chunks_delivered"`
    StreamDuration  time.Duration `json:"stream_duration"`
}
```

**Why:** Separate type for streaming results allows tracking streaming-specific metadata.

### Step 7: Orchestrator Implementation (`orchestration/orchestrator.go`)

**File:** `orchestration/orchestrator.go`
**Location:** After `ProcessRequest` method (after line 537)
**Action:** ADD `ProcessRequestStreaming` method

**Context:** ProcessRequest is defined at lines 360-537. Insert the new method right after line 537, before `generateExecutionPlan`.

```go
// ADD after line 537 (after ProcessRequest, before generateExecutionPlan)

// ProcessRequestStreaming processes a request with streaming response
func (o *AIOrchestrator) ProcessRequestStreaming(
    ctx context.Context,
    request string,
    metadata map[string]interface{},
    callback core.StreamCallback,
) (*StreamingOrchestratorResponse, error) {
    // Implementation as shown in Orchestrator Streaming section above
}
```

**Why:** New method enables streaming without modifying the existing `ProcessRequest` signature.

### Step 8: Travel Chat Agent (`examples/travel-chat-agent/chat_agent.go`)

**File:** `examples/travel-chat-agent/chat_agent.go`
**Location:** `ProcessWithStreaming` method (lines 202-296)
**Action:** MODIFY to use orchestrator streaming

**Current code (line 231):**
```go
result, err := orch.ProcessRequest(ctx, query, nil)
```

**New code:**
```go
// Check if orchestrator supports streaming
if streamingOrch, ok := orch.(interface {
    ProcessRequestStreaming(context.Context, string, map[string]interface{}, core.StreamCallback) (*orchestration.StreamingOrchestratorResponse, error)
}); ok {
    result, err := streamingOrch.ProcessRequestStreaming(ctx, query, nil, func(chunk core.StreamChunk) error {
        if chunk.Content != "" {
            callback.SendChunk(chunk.Content)
        }
        return nil
    })
    // ... handle result
} else {
    // Fallback to existing non-streaming path
    result, err := orch.ProcessRequest(ctx, query, nil)
    // ... existing chunking code
}
```

**Why:** Enables true streaming while maintaining backward compatibility.

### Step 9: Add Tests

**Files to create:**
- `ai/providers/openai/streaming_test.go`
- `ai/providers/anthropic/streaming_test.go`
- `ai/providers/gemini/streaming_test.go`
- `ai/providers/bedrock/streaming_test.go`
- `ai/providers/mock/streaming_test.go`
- `ai/chain_client_streaming_test.go`
- `orchestration/streaming_test.go`

**Why:** Comprehensive tests ensure streaming works correctly and handles edge cases across all providers.

### Verification Checklist

After implementation, verify:

**Build & Test:**
- [ ] `go build ./...` succeeds in all modules
- [ ] `go test ./...` passes in all modules
- [ ] Existing `GenerateResponse` tests still pass
- [ ] New streaming tests pass for each provider

**Provider-specific:**
- [ ] OpenAI streaming works with SSE
- [ ] Anthropic streaming works with SSE
- [ ] Gemini streaming works with `streamGenerateContent` endpoint
- [ ] Bedrock streaming works with `ConverseStream` API (refactored signature)
- [ ] Mock streaming works for unit tests

**Integration:**
- [ ] Travel-chat-agent works with streaming enabled
- [ ] Travel-chat-agent works with streaming disabled (fallback)
- [ ] Chain client failover works during streaming
- [ ] Context cancellation properly aborts streams
- [ ] Memory usage is reasonable during long streams

---

## Testing Strategy

### Unit Tests

```go
// Test streaming callback invocation
func TestStreamingCallbackInvocation(t *testing.T) {
    client := NewMockStreamingClient([]string{"Hello", " ", "World"})

    var chunks []string
    _, err := client.StreamResponse(ctx, "test", nil, func(chunk core.StreamChunk) error {
        chunks = append(chunks, chunk.Content)
        return nil
    })

    assert.NoError(t, err)
    assert.Equal(t, []string{"Hello", " ", "World"}, chunks)
}

// Test callback error handling
func TestStreamingCallbackError(t *testing.T) {
    client := NewMockStreamingClient([]string{"Hello", "World"})

    callbackErr := errors.New("callback failed")
    _, err := client.StreamResponse(ctx, "test", nil, func(chunk core.StreamChunk) error {
        if chunk.Content == "World" {
            return callbackErr
        }
        return nil
    })

    assert.ErrorIs(t, err, callbackErr)
}

// Test context cancellation
func TestStreamingContextCancellation(t *testing.T) {
    client := NewSlowMockStreamingClient()

    ctx, cancel := context.WithCancel(context.Background())

    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel()
    }()

    _, err := client.StreamResponse(ctx, "test", nil, func(chunk core.StreamChunk) error {
        return nil
    })

    assert.ErrorIs(t, err, context.Canceled)
}
```

### Integration Tests

```go
// Test full stack streaming
func TestEndToEndStreaming(t *testing.T) {
    // Start test server with real AI client
    agent := setupTestAgent(t)

    // Create SSE client
    var receivedChunks []string
    client := NewSSEClient("http://localhost:8095/chat/stream")

    err := client.Stream(context.Background(), ChatRequest{
        Message: "Hello",
    }, func(event SSEEvent) error {
        if event.Type == "chunk" {
            receivedChunks = append(receivedChunks, event.Data.Text)
        }
        return nil
    })

    assert.NoError(t, err)
    assert.Greater(t, len(receivedChunks), 1) // Multiple chunks received
}
```

### Mock Streaming Client for Testing

```go
// ai/providers/mock/streaming.go
type MockStreamingClient struct {
    chunks []string
    delay  time.Duration
}

func (m *MockStreamingClient) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    var fullContent strings.Builder

    for i, chunk := range m.chunks {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        if m.delay > 0 {
            time.Sleep(m.delay)
        }

        fullContent.WriteString(chunk)

        if err := callback(core.StreamChunk{
            Content: chunk,
            Delta:   true,
            Index:   i,
        }); err != nil {
            return nil, err
        }
    }

    return &core.AIResponse{
        Content: fullContent.String(),
        Model:   "mock",
    }, nil
}

func (m *MockStreamingClient) SupportsStreaming() bool {
    return true
}
```

---

## Performance Considerations

### Memory Usage

- **Buffering**: Avoid buffering entire response; stream through
- **Chunk aggregation**: Only aggregate for final response (logging/metrics)
- **Callback overhead**: Keep callback execution minimal

### Network Efficiency

- **Keep-alive connections**: Reuse HTTP connections where possible
- **Compression**: Enable gzip for SSE responses
- **Chunked encoding**: Use HTTP chunked transfer encoding

### Error Recovery

- **Partial failure**: If stream fails mid-way, client has partial response
- **Retry strategy**: Don't retry partial streams (complex state management)
- **Graceful degradation**: Fall back to non-streaming on errors

### Metrics to Track

```go
// Streaming-specific metrics
telemetry.Histogram("ai.stream.time_to_first_token_ms", timeToFirstToken)
telemetry.Histogram("ai.stream.total_duration_ms", totalDuration)
telemetry.Counter("ai.stream.chunks_delivered", chunksCount)
telemetry.Counter("ai.stream.errors", errorType)
telemetry.Gauge("ai.stream.active_streams", activeStreams)
```

---

## Timeline Estimate

| Phase | Description | Duration | Dependencies |
|-------|-------------|----------|--------------|
| 1 | Core Interface Design | 3 days | None |
| 2a | OpenAI Provider (new implementation) | 3 days | Phase 1 |
| 2b | Anthropic Provider (new implementation) | 3 days | Phase 1 |
| 2c | Gemini Provider (new implementation) | 2 days | Phase 1 |
| 2d | Bedrock Provider (**refactor existing**) | 2 days | Phase 1 |
| 2e | Mock Provider (new implementation) | 1 day | Phase 1 |
| 3 | Chain Client | 3 days | Phase 2 |
| 4 | Orchestrator | 5 days | Phase 3 |
| 5 | Example Updates | 2 days | Phase 4 |
| 6 | Testing & Documentation | 5 days | Phase 5 |

**Total Estimate: 4-6 weeks** (with parallel provider work)

**Note**: Phases 2a-2e can be done in parallel by different developers.

---

## Appendix A: SSE Handler Updates

The `travel-chat-agent` SSE handler needs minimal changes:

```go
// Current: Simulated streaming
func (t *TravelChatAgent) ProcessWithStreaming(ctx context.Context, sessionID, query string, callback StreamCallback) error {
    // ...
    result, err := orch.ProcessRequest(ctx, query, nil)  // Blocking
    // ...
    // Chunk complete response
    for i := 0; i < len(response); i += chunkSize {
        callback.SendChunk(response[i:end])
    }
}

// New: True streaming
func (t *TravelChatAgent) ProcessWithStreaming(ctx context.Context, sessionID, query string, callback StreamCallback) error {
    // ...
    result, err := orch.ProcessRequestStreaming(ctx, query, nil, func(chunk core.StreamChunk) error {
        if chunk.Content != "" {
            callback.SendChunk(chunk.Content)  // Forward immediately
        }
        if chunk.Metadata != nil {
            if status, ok := chunk.Metadata["status"].(string); ok {
                callback.SendStatus(chunk.Metadata["phase"].(string), status)
            }
        }
        return nil
    })
    // ...
}
```

---

## Appendix B: Client Detection Pattern

For gradual rollout, code can detect streaming support:

```go
func processWithBestMethod(ctx context.Context, ai core.AIClient, prompt string, callback func(string)) (*core.AIResponse, error) {
    // Try streaming first
    if streamingAI, ok := ai.(core.StreamingAIClient); ok && streamingAI.SupportsStreaming() {
        return streamingAI.StreamResponse(ctx, prompt, nil, func(chunk core.StreamChunk) error {
            callback(chunk.Content)
            return nil
        })
    }

    // Fall back to regular
    resp, err := ai.GenerateResponse(ctx, prompt, nil)
    if err != nil {
        return nil, err
    }

    // Simulate streaming
    for _, char := range resp.Content {
        callback(string(char))
    }

    return resp, nil
}
```

---

## Implementation Status

**Status: ✅ IMPLEMENTED** (as of 2026-01-07)

All core streaming functionality has been implemented and is working. The following sections document the implementation status and post-implementation enhancements.

### Completed Implementation Checklist

| Step | Component | Status | Notes |
|------|-----------|--------|-------|
| 1a | Core Errors (`core/errors.go`) | ✅ Done | `ErrStreamPartiallyCompleted` at line 52 |
| 1b | Core Interface (`core/interfaces.go`) | ✅ Done | `StreamChunk`, `StreamCallback`, `StreamingAIClient` at lines 89-112 |
| 2 | AI Re-exports (`ai/interfaces.go`) | ✅ Done | Type aliases at lines 14-20 |
| 3 | OpenAI Provider | ✅ Done | `StreamResponse` at lines 237-523 |
| 4 | Anthropic Provider | ✅ Done | `StreamResponse` at lines 251-538 |
| 4b | Gemini Provider | ✅ Done | `StreamResponse` at lines 266-535 |
| 4c | Bedrock Provider | ✅ Done | Refactored to callback-based at lines 210-432 |
| 4d | Mock Provider | ✅ Done | `StreamResponse` at lines 120-256 |
| 5 | Chain Client | ✅ Done | `StreamResponse` with failover at lines 363-520 |
| 6 | Orchestrator Interface | ✅ Done | `StreamingOrchestratorResponse` at lines 69-83 |
| 7 | Orchestrator Implementation | ✅ Done | `ProcessRequestStreaming` at lines 554-745 |
| 8 | Travel Chat Agent | ✅ Done | Uses `ProcessRequestStreaming` at lines 239-300 |

### Post-Implementation Enhancements (2026-01-07)

During code review, several issues were identified and fixed to improve the streaming implementation:

#### Issue 1: Missing Step Duration Tracking

**Problem:** Step completion events were sent with `durationMs: 0` because the original implementation didn't use the actual step durations from the orchestrator.

**Location:** `examples/travel-chat-agent/chat_agent.go` lines 271-286

**Fix:** Updated to use `StepResults` from `StreamingOrchestratorResponse`:
```go
// Before (always 0 duration):
for i, agentName := range agentsInvolved {
    callback.SendStep(fmt.Sprintf("step_%d", i+1), agentName, true, 0)
}

// After (actual durations from StepResults):
if len(result.StepResults) > 0 {
    for i, stepResult := range result.StepResults {
        callback.SendStep(
            fmt.Sprintf("step_%d", i+1),
            stepResult.AgentName,
            stepResult.Success,
            stepResult.Duration.Milliseconds(),
        )
    }
} else {
    // Fallback for simulated streaming
    for i, agentName := range agentsInvolved {
        callback.SendStep(fmt.Sprintf("step_%d", i+1), agentName, true, 0)
    }
}
```

#### Issue 2: Missing Usage Stats in Response

**Problem:** Token usage statistics from AI synthesis were not exposed to consumers.

**Location:** `orchestration/interfaces.go` lines 79-82

**Fix:** Added `Usage` field to `StreamingOrchestratorResponse`:
```go
type StreamingOrchestratorResponse struct {
    OrchestratorResponse
    // ... existing fields ...

    // Enhanced tracking fields (ADDED)
    StepResults  []StepResult     `json:"step_results,omitempty"`
    Usage        *core.TokenUsage `json:"usage,omitempty"`
    FinishReason string           `json:"finish_reason,omitempty"`
}
```

**Location:** `orchestration/orchestrator.go` line 741

**Fix:** Populated `Usage` field from AI response:
```go
response := &StreamingOrchestratorResponse{
    // ... existing fields ...
    Usage: &aiResponse.Usage,  // ADDED
}
```

#### Issue 3: Missing Finish Reason Forwarding

**Problem:** The `FinishReason` from AI providers (e.g., "stop", "length") was not captured or forwarded to consumers.

**Location:** `orchestration/orchestrator.go` lines 682-692, 742

**Fix:** Capture finish reason from streaming callback and include in response:
```go
var finishReason string
streamCallback := func(chunk core.StreamChunk) error {
    // ... existing code ...
    if chunk.FinishReason != "" {
        finishReason = chunk.FinishReason  // ADDED
    }
    return callback(chunk)
}

// In response construction:
response := &StreamingOrchestratorResponse{
    // ... existing fields ...
    FinishReason: finishReason,  // ADDED
}
```

#### Issue 4: SSE Handler Missing Event Types

**Problem:** The SSE handler didn't have methods to send usage stats or finish reason events to clients.

**Location:** `examples/travel-chat-agent/sse_handler.go` lines 13-22, 93-109

**Fix:** Extended `StreamCallback` interface and implemented new methods:
```go
// Extended interface (lines 13-22):
type StreamCallback interface {
    // ... existing methods ...
    SendUsage(promptTokens, completionTokens, totalTokens int)  // ADDED
    SendFinish(reason string)                                    // ADDED
}

// New implementations (lines 93-109):
func (c *SSECallback) SendUsage(promptTokens, completionTokens, totalTokens int) {
    c.sendEvent("usage", map[string]interface{}{
        "prompt_tokens":     promptTokens,
        "completion_tokens": completionTokens,
        "total_tokens":      totalTokens,
    })
}

func (c *SSECallback) SendFinish(reason string) {
    c.sendEvent("finish", map[string]interface{}{
        "reason": reason,
    })
}
```

#### Issue 5: Chat Agent Not Using Enhanced Fields

**Problem:** The chat agent wasn't forwarding the new usage and finish reason data to SSE clients.

**Location:** `examples/travel-chat-agent/chat_agent.go` lines 288-300

**Fix:** Added code to send usage and finish reason events:
```go
// Send usage stats if available (ADDED)
if result.Usage != nil {
    callback.SendUsage(
        result.Usage.PromptTokens,
        result.Usage.CompletionTokens,
        result.Usage.TotalTokens,
    )
}

// Send finish reason if available (ADDED)
if result.FinishReason != "" {
    callback.SendFinish(result.FinishReason)
}
```

#### Issue 6: Real-Time Tool Progress Events

**Problem:** Tool/step progress was only shown after the AI response started streaming, rather than as each tool was actually being executed. This created a poor user experience where users saw all tool execution steps appear simultaneously.

**Root Cause:** The original implementation only sent step events after `ProcessRequestStreaming` completed, using the aggregated `StepResults` from the response. There was no mechanism to emit step progress events in real-time as tools executed.

**Solution:** Implemented a context-based callback mechanism that allows per-request step callbacks to be injected, enabling real-time tool progress events during execution.

**Location:** `orchestration/interfaces.go` (new context helpers)

**Fix:** Added context key and helper functions for per-request step callbacks:
```go
// stepCallbackKey is the context key for per-request step callbacks
type stepCallbackKey struct{}

// WithStepCallback returns a new context with the step callback attached.
// This allows callers to receive real-time step completion events during
// plan execution, enabling immediate UI updates as tools execute.
func WithStepCallback(ctx context.Context, callback StepCompleteCallback) context.Context {
    return context.WithValue(ctx, stepCallbackKey{}, callback)
}

// GetStepCallback retrieves the step callback from context, if present.
// Returns nil if no callback was set.
func GetStepCallback(ctx context.Context) StepCompleteCallback {
    if cb, ok := ctx.Value(stepCallbackKey{}).(StepCompleteCallback); ok {
        return cb
    }
    return nil
}
```

**Location:** `orchestration/executor.go` (callback invocation)

**Fix:** Modified executor to check for context-level callbacks in addition to executor-level callbacks:
```go
// Invoke step completion callbacks (outside lock to avoid blocking)
// Check both executor-level and context-level callbacks
if e.onStepComplete != nil {
    e.onStepComplete(stepIndex, len(plan.Steps), s, stepResult)
}
// Also check for per-request callback from context
if ctxCallback := GetStepCallback(ctx); ctxCallback != nil {
    ctxCallback(stepIndex, len(plan.Steps), s, stepResult)
}
```

**Location:** `examples/travel-chat-agent/chat_agent.go` (usage example)

**Fix:** Added per-request step callback to emit real-time tool events:
```go
// Add per-request step callback to context for real-time tool progress
ctx = orchestration.WithStepCallback(ctx, func(stepIndex, totalSteps int, step orchestration.RoutingStep, stepResult orchestration.StepResult) {
    callback.SendStep(
        fmt.Sprintf("step_%d", stepIndex+1),
        step.AgentName,
        stepResult.Success,
        stepResult.Duration.Milliseconds(),
    )
})

// Call ProcessRequestStreaming with the enhanced context
result, err := orch.ProcessRequestStreaming(ctx, query, metadata, streamCallback)
```

**Why Context-Based Callbacks?**

Using Go's `context.Context` for per-request callbacks provides several advantages:

1. **Per-request isolation:** Each request gets its own callback without affecting other concurrent requests
2. **No executor reconfiguration:** The executor doesn't need to be reconfigured per-request; the callback travels with the context
3. **Thread-safety:** Context values are immutable and safe for concurrent access
4. **Opt-in behavior:** Callers that don't need real-time events simply don't set a callback
5. **Composability:** Works alongside existing executor-level callbacks for telemetry/logging

**Result:** Users now see tool progress events in real-time as each tool executes, rather than all at once after execution completes.

### New SSE Event Types

After the enhancements, SSE clients now receive these additional event types:

| Event | Description | Payload |
|-------|-------------|---------|
| `usage` | Token usage statistics | `{"prompt_tokens": N, "completion_tokens": N, "total_tokens": N}` |
| `finish` | Why streaming stopped | `{"reason": "stop"}` |

### Files Modified in Enhancement

| File | Changes |
|------|---------|
| `orchestration/interfaces.go` | Added `StepResults`, `Usage`, `FinishReason` fields to `StreamingOrchestratorResponse`; Added `WithStepCallback()` and `GetStepCallback()` context helpers for per-request step callbacks |
| `orchestration/orchestrator.go` | Capture finish reason, populate new fields in response |
| `orchestration/executor.go` | Check for context-level step callbacks in addition to executor-level callbacks |
| `examples/travel-chat-agent/sse_handler.go` | Added `SendUsage()` and `SendFinish()` methods |
| `examples/travel-chat-agent/chat_agent.go` | Use `StepResults` for durations, forward usage and finish reason; Use `WithStepCallback` for real-time tool progress events |

---

## Conclusion

~~Implementing~~ True streaming support ~~in GoMind will~~ has been implemented and significantly improves user experience for chat-based applications. The implementation is:

1. **Backward compatible**: Existing code using `GenerateResponse()` continues to work
2. **Incrementally adoptable**: Each component can be updated independently
3. **Provider-agnostic**: Uniform interface across all AI providers (OpenAI, Anthropic, Gemini, Bedrock)
4. **Well-tested**: Tests pass for all modules
5. **Observable**: Rich metadata including step durations, token usage, and finish reasons

~~The investment of 4-6 weeks will result in~~ The framework now provides a modern, streaming-capable AI framework suitable for production chat applications.
