# GoMind Orchestration Module Architecture

**Version**: 1.0
**Purpose**: Comprehensive architectural documentation for the orchestration module
**Audience**: Core contributors, module developers, system architects, LLM-based coding agents

---

## Table of Contents

1. [Overview](#overview)
2. [Design Philosophy](#design-philosophy)
3. [Core Architecture](#core-architecture)
4. [Dependency Injection Pattern](#dependency-injection-pattern)
5. [Component Architecture](#component-architecture)
6. [Execution Models](#execution-models)
7. [Integration Patterns](#integration-patterns)
8. [Capability Provider Architecture](#capability-provider-architecture)
9. [Resilience & Fault Tolerance](#resilience--fault-tolerance)
10. [Performance & Scalability](#performance--scalability)
11. [Production Deployment](#production-deployment)
12. [Common Patterns & Examples](#common-patterns--examples)
13. [Troubleshooting Guide](#troubleshooting-guide)
14. [Future Considerations](#future-considerations)

---

## Overview

The orchestration module provides multi-agent coordination with AI-driven orchestration and declarative workflows. It acts as the conductor of the GoMind framework, coordinating multiple agents and tools to accomplish complex tasks.

### Key Capabilities

1. **AI-Driven Orchestration**: Natural language request processing with intelligent routing
2. **Workflow Engine**: Declarative, DAG-based workflow execution
3. **Hybrid Mode**: Combines AI flexibility with workflow predictability
4. **Dynamic Discovery**: Runtime discovery and routing of components
5. **Parallel Execution**: Automatic parallelization based on dependencies
6. **Resilient Design**: Built-in retry, circuit breaker, and fallback mechanisms

### Architectural Position

```
┌──────────────────────────────────────────┐
│            Applications                   │
│  (Wire together modules and components)   │
└──────────────┬───────────────────────────┘
               │
    ┌──────────▼───────────┐
    │    Orchestration      │
    │                       │
    │  • AI Orchestrator    │
    │  • Workflow Engine    │
    │  • Smart Executor     │
    └──────────┬───────────┘
               │
    ┌──────────▼───────────┐
    │      Core Module      │
    │                       │
    │  • Interfaces         │
    │  • Base Types         │
    │  • Discovery           │
    └───────────────────────┘
```

---

## Design Philosophy

### 1. Interface-Based Dependency Injection

**The Principle**: The orchestration module uses interface-based dependencies for most optional modules. Per [FRAMEWORK_DESIGN_PRINCIPLES.md](../FRAMEWORK_DESIGN_PRINCIPLES.md), the valid dependencies are:

- `orchestration` → `core` (required)
- `orchestration` → `telemetry` (allowed for observability)

```go
// ❌ NEVER DO THIS - Would create circular dependency
import "github.com/itsneelabh/gomind/ai"
import "github.com/itsneelabh/gomind/resilience"

// ✅ ALLOWED - Per FRAMEWORK_DESIGN_PRINCIPLES.md
import "github.com/itsneelabh/gomind/core"
import "github.com/itsneelabh/gomind/telemetry"  // For observability

type AIOrchestrator struct {
    aiClient    core.AIClient    // Interface - injected by application
    discovery   core.Discovery   // Interface - injected by application
}
```

**Rationale**:
1. **Prevents circular dependencies**: `ai` module cannot be imported (would create cycle)
2. **Enables testing**: Can use mocks without importing real modules
3. **Maintains modularity**: Can swap implementations without code changes
4. **Follows SOLID principles**: Dependency Inversion Principle
5. **Telemetry exception**: `telemetry` is allowed because it provides observability infrastructure that all modules need, and it doesn't create circular dependencies

### 2. Explicit Configuration Over Magic

**The Principle**: Configuration is explicit and predictable, with intelligent defaults.

```go
// Explicit dependency injection
deps := OrchestratorDependencies{
    Discovery: discovery,  // Required
    AIClient:  aiClient,   // Required
    Logger:    logger,     // Optional - will create default if nil
    Telemetry: telemetry,  // Optional - will work without it
}

orchestrator, err := CreateOrchestrator(config, deps)
```

### 3. Progressive Enhancement

**The Principle**: Start simple, add complexity as needed.

```go
// Level 1: Zero configuration
orchestrator := CreateSimpleOrchestrator(discovery, aiClient)

// Level 2: With configuration
config := DefaultConfig()
config.CacheEnabled = true
orchestrator := NewAIOrchestrator(config, discovery, aiClient)

// Level 3: Full production setup
deps := OrchestratorDependencies{
    Discovery:      discovery,
    AIClient:       aiClient,
    CircuitBreaker: cb,
    Logger:         logger,
    Telemetry:      telemetry,
}
orchestrator, _ := CreateOrchestrator(config, deps)
```

### 4. Fail-Safe Defaults

**The Principle**: The system should degrade gracefully when optional components are unavailable.

```go
// If telemetry is nil, use NoOp implementation
if o.telemetry == nil {
    o.telemetry = &core.NoOpTelemetry{}
}

// If capability service fails, fall back to default provider
if err != nil && o.config.EnableFallback {
    return o.defaultProvider.GetCapabilities(ctx)
}
```

---

## Core Architecture

### Dependency Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│     core     │     │     core     │     │     core     │
│              │     │              │     │              │
│   Defines:   │     │   Defines:   │     │   Defines:   │
│ - AIClient   │     │ - Discovery  │     │ - Telemetry  │
│ - Logger     │     │ - Registry   │     │ - Span       │
└──────▲───────┘     └──────▲───────┘     └──────▲───────┘
       │                    │                    │
       ├────────────────────┼────────────────────┤
       │                    │                    │
┌──────┴───────┐     ┌──────┴───────┐     ┌──────┴───────┐
│      ai      │     │orchestration │     │  telemetry   │
│              │     │              │     │              │
│ Implements:  │     │    Uses:     │     │ Implements:  │
│ - AIClient   │     │ - AIClient   │     │ - Telemetry  │
└──────────────┘     │ - Discovery  │     └──────────────┘
                     │ - Telemetry  │
                     │ - Logger     │
                     └──────────────┘

Note: orchestration imports core and telemetry only (per FRAMEWORK_DESIGN_PRINCIPLES.md)
```

### Module Dependencies

```go
// orchestration/go.mod
module github.com/itsneelabh/gomind/orchestration

require (
    github.com/itsneelabh/gomind/core v0.1.0
    github.com/itsneelabh/gomind/telemetry v0.1.0  // Allowed for observability
    // NO direct imports of ai, resilience, or ui modules
)
```

---

## Dependency Injection Pattern

### Why Not Import AI Module Directly?

The orchestration module needs AI capabilities for intelligent routing, but it does NOT import the `ai` module. This is a **critical architectural decision**.

#### The Problem It Solves

```go
// If orchestration imported ai directly:
// orchestration → ai
// ai → orchestration (for orchestrating AI workflows)
// CIRCULAR DEPENDENCY! ❌
```

#### The Solution: Interface-Based DI

```go
// orchestration/orchestrator.go
type AIOrchestrator struct {
    aiClient core.AIClient  // Interface from core
}

func NewAIOrchestrator(config *Config, discovery core.Discovery, aiClient core.AIClient) *AIOrchestrator {
    return &AIOrchestrator{
        aiClient: aiClient,  // Injected, not created
    }
}
```

#### Application Wiring

```go
// main.go - Application layer wires everything together
import (
    "github.com/itsneelabh/gomind/ai"
    "github.com/itsneelabh/gomind/orchestration"
)

func main() {
    // App creates AI client
    aiClient, _ := ai.NewClient(
        ai.WithProvider("openai"),
        ai.WithAPIKey(apiKey),
    )

    // App injects it into orchestrator
    orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

    // Orchestration has AI capabilities without importing ai module!
}
```

### Benefits of This Pattern

1. **Testability**
```go
func TestOrchestrator(t *testing.T) {
    // Use mock instead of real AI
    aiClient := &MockAIClient{
        GenerateResponseFunc: func(ctx context.Context, prompt string, opts *AIOptions) (*AIResponse, error) {
            return &AIResponse{Content: "mock response"}, nil
        },
    }

    orchestrator := NewAIOrchestrator(config, discovery, aiClient)
    // Test without real AI calls or importing ai module
}
```

2. **Provider Flexibility**
```go
// Today: OpenAI
aiClient := ai.NewClient(ai.WithProvider("openai"))

// Tomorrow: Custom implementation
aiClient := company.InternalLLMClient()

// Orchestration code unchanged!
orchestrator := orchestration.NewAIOrchestrator(config, discovery, aiClient)
```

3. **Clean Architecture**
- Clear separation of concerns
- No circular dependencies possible
- Each module has single responsibility
- Dependencies flow in one direction: toward core

---

## Component Architecture

### Core Components

```
┌─────────────────────────────────────────────────────┐
│                   AIOrchestrator                     │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
│  │   Component  │  │    Smart     │  │    AI    │  │
│  │    Catalog   │  │   Executor   │  │Synthesizer│ │
│  └──────────────┘  └──────────────┘  └──────────┘  │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
│  │   Routing    │  │  Capability  │  │  Metrics │  │
│  │     Cache    │  │   Provider   │  │  Tracker │  │
│  └──────────────┘  └──────────────┘  └──────────┘  │
└─────────────────────────────────────────────────────┘
```

#### Component Catalog
- **Purpose**: Maintains registry of available agents and tools
- **Updates**: Refreshes every 10 seconds via discovery
- **Data**: Component names, capabilities, health status, endpoints

#### Smart Executor
- **Purpose**: Executes tool/agent calls with parallelization
- **Features**: Retry logic, timeout handling, error aggregation
- **Optimization**: Automatically detects independent steps for parallel execution

#### AI Synthesizer
- **Purpose**: Combines multiple responses into coherent answer
- **Strategies**: LLM-based, template-based, or simple concatenation
- **Context**: Maintains conversation context for coherent synthesis

#### Routing Cache
- **Purpose**: Caches routing decisions to reduce LLM calls
- **Types**: Time-based (TTL) or LRU (Least Recently Used)
- **Benefit**: Reduces latency and cost for repeated requests

#### Capability Provider
- **Purpose**: Provides component capabilities to LLM for routing
- **Types**: Default (all capabilities) or Service (filtered/semantic search)
- **Scaling**: Critical for 100s-1000s of agents

---

## Execution Models

### 1. AI-Driven Orchestration

```
Request → Understanding → Discovery → Planning → Execution → Synthesis → Response
```

#### Request Processing Pipeline

```go
func (o *AIOrchestrator) ProcessRequest(ctx context.Context, request string) (*Response, error) {
    // 1. Understanding: Extract intent
    intent := o.extractIntent(request)

    // 2. Discovery: Find available components
    components := o.catalog.GetComponents()

    // 3. Planning: Generate execution plan
    plan, err := o.generateExecutionPlan(ctx, request)

    // 4. Execution: Run plan with parallelization
    results, err := o.executor.Execute(ctx, plan)

    // 5. Synthesis: Combine results
    response := o.synthesizer.Synthesize(results)

    return response, nil
}
```

#### LLM Prompt Structure

```go
func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) string {
    prompt := fmt.Sprintf(`
You are an intelligent orchestrator. Given a user request and available components,
create an execution plan.

User Request: %s

Available Components:
%s

Create a JSON execution plan with the following structure:
{
  "steps": [
    {
      "name": "step_name",
      "component": "component_name",
      "action": "action_to_perform",
      "inputs": {},
      "depends_on": []
    }
  ]
}

Rules:
1. Steps without dependencies can run in parallel
2. Use the most appropriate component for each task
3. Ensure data flows correctly between steps
4. Minimize total execution time
`, request, o.formatComponentCapabilities())

    return prompt
}
```

### 2. Workflow Engine

```
Workflow → Parse → Build DAG → Schedule → Execute → Collect → Response
```

#### DAG Execution

```go
type WorkflowEngine struct {
    discovery   core.Discovery
    scheduler   *DAGScheduler
    executor    *StepExecutor
    state       *StateManager
}

func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflow *WorkflowDef, inputs map[string]interface{}) (*ExecutionResult, error) {
    // 1. Parse workflow and build DAG
    dag := e.buildDAG(workflow)

    // 2. Initialize execution state
    state := e.state.Initialize(workflow.Name, inputs)

    // 3. Schedule and execute steps
    for !dag.IsComplete() {
        // Get ready steps (no pending dependencies)
        readySteps := dag.GetReadySteps()

        // Execute in parallel
        results := e.executeParallel(ctx, readySteps, state)

        // Update state and DAG
        state.UpdateSteps(results)
        dag.MarkComplete(readySteps)
    }

    return state.GetResult(), nil
}
```

#### Variable Substitution

```go
func (e *WorkflowEngine) substituteVariables(template string, state *ExecutionState) string {
    // ${inputs.fieldName} - Input parameters
    // ${steps.stepName.output} - Step outputs
    // ${steps.stepName.output.field} - Specific fields

    return variableRegex.ReplaceAllStringFunc(template, func(match string) string {
        path := extractPath(match)
        value := state.GetValue(path)
        return fmt.Sprintf("%v", value)
    })
}
```

### 3. Hybrid Mode

Combines AI flexibility with workflow predictability:

```yaml
# Workflow with AI-driven steps
name: hybrid-analysis
steps:
  - name: understand-request
    type: ai-routing  # AI decides which components
    prompt: "Analyze the user's request and determine data sources needed"

  - name: gather-data
    type: workflow    # Fixed workflow steps
    parallel:
      - tool: market-data
      - tool: news-feed

  - name: analyze
    type: ai-routing  # AI chooses analysis approach
    inputs: ${steps.gather-data.output}
```

---

## Integration Patterns

### Pattern 1: Tool Integration

Tools are passive components that respond to requests:

```go
// Tool implementation
type WeatherTool struct {
    *core.BaseTool
    apiClient *WeatherAPIClient
}

func (t *WeatherTool) GetCapabilities() []core.Capability {
    return []core.Capability{
        {
            Name:        "get_weather",
            Description: "Get current weather for a location",
            Parameters: map[string]interface{}{
                "location": "string",
            },
        },
    }
}

// Orchestration uses the tool
plan := &RoutingPlan{
    Steps: []PlanStep{
        {
            Name:      "weather",
            Component: "weather-tool",
            Action:    "get_weather",
            Inputs:    map[string]interface{}{"location": "NYC"},
        },
    },
}
```

### Pattern 2: Agent Integration

Agents are active components that can orchestrate others:

```go
// Agent can discover and coordinate
type ResearchAgent struct {
    *core.BaseAgent
    discovery core.Discovery
}

func (a *ResearchAgent) ProcessRequest(ctx context.Context, request string) (*Response, error) {
    // Agent can discover other components
    tools, _ := a.discovery.FindByCapability(ctx, "data_gathering")

    // Agent orchestrates multiple tools
    for _, tool := range tools {
        // Call tools in sequence or parallel
    }

    return response, nil
}

// Orchestration delegates to agent
plan := &RoutingPlan{
    Steps: []PlanStep{
        {
            Name:      "research",
            Component: "research-agent",
            Action:    "comprehensive_analysis",
            Inputs:    map[string]interface{}{"topic": "Tesla"},
        },
    },
}
```

### Pattern 3: Service Integration

External services via HTTP/gRPC:

```go
// Capability service integration
type ServiceCapabilityProvider struct {
    endpoint string
    client   *http.Client
}

func (p *ServiceCapabilityProvider) GetCapabilities(ctx context.Context, request string) ([]Capability, error) {
    // Query external service for relevant capabilities
    resp, err := p.client.Post(p.endpoint+"/search", "application/json",
        bytes.NewBuffer([]byte(`{"query": "`+request+`"}`)))

    // Return filtered capabilities
    return capabilities, nil
}
```

---

## Capability Provider Architecture

### Scaling Challenge

At scale (100s-1000s of agents), sending all capabilities to LLM causes:
- Token limit overflow
- Increased costs
- Slower responses

### Solution: Capability Provider Pattern

```
┌─────────────────────────────────────────────────┐
│                  Orchestrator                    │
│                                                  │
│  ┌────────────┐        ┌────────────────────┐   │
│  │   Request  │───────▶│ Capability Provider│   │
│  └────────────┘        └────────────────────┘   │
│                               │                  │
│                               ▼                  │
│                    ┌─────────────────────┐      │
│                    │  Filtered/Relevant  │      │
│                    │    Capabilities     │      │
│                    └─────────────────────┘      │
│                               │                  │
│                               ▼                  │
│                    ┌─────────────────────┐      │
│                    │      LLM Router     │      │
│                    └─────────────────────┘      │
└─────────────────────────────────────────────────┘
```

### Provider Types

#### 1. Default Provider (< 200 agents)
```go
type DefaultCapabilityProvider struct {
    catalog *AgentCatalog
}

func (p *DefaultCapabilityProvider) GetCapabilities(ctx context.Context, request string) ([]Capability, error) {
    // Return ALL capabilities
    return p.catalog.GetAllCapabilities(), nil
}
```

#### 2. Service Provider (100s-1000s agents)
```go
type ServiceCapabilityProvider struct {
    endpoint       string
    topK           int
    threshold      float64
    circuitBreaker core.CircuitBreaker
}

func (p *ServiceCapabilityProvider) GetCapabilities(ctx context.Context, request string) ([]Capability, error) {
    // Use circuit breaker for resilience
    result, err := p.circuitBreaker.Execute(func() (interface{}, error) {
        // Semantic search for relevant capabilities
        return p.queryService(ctx, request, p.topK, p.threshold)
    })

    if err != nil && p.fallback != nil {
        // Fall back to default provider
        return p.fallback.GetCapabilities(ctx, request)
    }

    return result.([]Capability), nil
}
```

### Auto-Configuration

```go
func (c *OrchestratorConfig) AutoConfigure() {
    // Check environment for capability service
    if endpoint := os.Getenv("GOMIND_CAPABILITY_SERVICE_URL"); endpoint != "" {
        c.CapabilityProviderType = "service"
        c.CapabilityService.Endpoint = endpoint

        // Smart defaults
        if c.CapabilityService.TopK == 0 {
            c.CapabilityService.TopK = 20
        }
        if c.CapabilityService.Threshold == 0 {
            c.CapabilityService.Threshold = 0.7
        }

        // Enable fallback for production resilience
        c.EnableFallback = true
    }
}
```

---

## Resilience & Fault Tolerance

### Design Philosophy & Goals

The resilience architecture for the orchestration module's capability provider system is designed to handle external service failures gracefully while maintaining framework architectural principles.

#### Design Goals

1. **Framework Compliance**: Respect module dependency rules (orchestration → core + telemetry only)
2. **Extensibility**: Allow sophisticated resilience patterns without hard dependencies
3. **Progressive Enhancement**: Work with zero configuration, enhance when needed
4. **Production Ready**: Support best-practice resilience requirements
5. **Pattern Consistency**: Follow established patterns from other modules (UI)

### Three-Layer Resilience Architecture

The design provides three layers of resilience, each building on the previous:

#### Layer 1: Simple Built-in Resilience (Always Active)
- 3 retries with exponential backoff
- Simple failure tracking (5 failures → 30s cooldown)
- Timeout protection (30s default)
- No external dependencies
- Works out of the box with zero configuration

#### Layer 2: Circuit Breaker (Optional, Injected)
- Full circuit breaker pattern (closed/open/half-open states)
- Sliding window metrics
- Configurable thresholds and recovery
- Provided by application, not framework
- Injected via dependency injection

#### Layer 3: Fallback Provider (Configurable)
- Falls back to DefaultCapabilityProvider on failure
- Ensures system continues working
- Enabled by default with service provider
- Graceful degradation under failures

### Dependency Injection Pattern for Resilience

Following the UI module's proven pattern, we use dependency injection for optional resilience features:

```go
// ServiceCapabilityConfig accepts optional dependencies
type ServiceCapabilityConfig struct {
    // Required configuration
    Endpoint  string
    TopK      int
    Threshold float64
    Timeout   time.Duration

    // Optional dependencies (injected)
    CircuitBreaker   core.CircuitBreaker  `json:"-"`
    Logger           core.Logger          `json:"-"`
    Telemetry        core.Telemetry       `json:"-"`
    FallbackProvider CapabilityProvider   `json:"-"`
}
```

### Application Usage Patterns

#### Pattern 1: Simple Usage (Development)
```go
// Zero configuration - uses built-in Layer 1 resilience
deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,
    AIClient:  aiClient,
}
orchestrator, _ := orchestration.CreateOrchestrator(nil, deps)
```

#### Pattern 2: Production Usage (With Circuit Breaker)
```go
// Application creates and injects circuit breaker
cb, _ := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:             "capability-service",
    ErrorThreshold:   0.5,
    VolumeThreshold:  10,
    SleepWindow:      30 * time.Second,
})

deps := orchestration.OrchestratorDependencies{
    Discovery:      discovery,
    AIClient:       aiClient,
    CircuitBreaker: cb,  // Inject sophisticated circuit breaker
    Logger:         logger,  // Optional: Structured logging
}

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

#### Pattern 3: Service Mesh (Kubernetes)
```yaml
# Let Istio handle circuit breaking at network level
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: capability-service
spec:
  host: capability-service
  trafficPolicy:
    outlierDetection:
      consecutiveErrors: 5
      interval: 30s
      baseEjectionTime: 30s
```

### Module Dependencies for Resilience

#### What Orchestration Module Provides
- CapabilityProvider interface
- ServiceCapabilityProvider with simple resilience (Layer 1)
- Injection points for optional dependencies
- Fallback mechanisms (Layer 3)

#### What Application Provides
- Circuit breaker implementation (using resilience module)
- Logger implementation
- Telemetry implementation
- Configuration and tuning parameters

#### Dependency Flow for Resilience
```
Application Code
    ├── imports orchestration (for orchestrator)
    ├── imports resilience (for circuit breaker)
    └── injects circuit breaker into orchestrator

Orchestration Module
    ├── imports core (for interfaces)
    ├── imports telemetry (for observability)
    └── accepts core.CircuitBreaker (no resilience import!)

Core Module
    └── defines CircuitBreaker interface

Resilience Module
    ├── imports core
    └── implements CircuitBreaker interface
```

### Benefits of This Design

#### For Framework Maintainers
- **Clean Architecture**: No dependency violations
- **Consistent Patterns**: Same as UI module
- **Testable**: All dependencies mockable
- **Extensible**: Clear injection points

#### For Application Developers
- **Progressive Enhancement**: Start simple, add resilience as needed
- **Flexibility**: Choose any circuit breaker implementation
- **Production Ready**: Inject sophisticated patterns
- **Service Mesh Compatible**: Works with Istio/Linkerd

#### For Operations
- **Observable**: Metrics through telemetry interface
- **Configurable**: All parameters tunable
- **Graceful Degradation**: Multiple fallback layers
- **Fast Recovery**: Automatic recovery testing

### Migration Path

#### From Current Implementation
1. Current simple resilience remains as Layer 1
2. Add injection points for optional dependencies
3. No breaking changes to existing code

#### For Applications
```go
// Stage 1: Use as-is (current implementation works)
orchestrator := orchestration.CreateSimpleOrchestrator(discovery, aiClient)

// Stage 2: Add logging and telemetry
deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,
    AIClient:  aiClient,
    Logger:    logger,
    Telemetry: telemetry,
}
orchestrator, _ := orchestration.CreateOrchestrator(nil, deps)

// Stage 3: Add circuit breaker for production
deps.CircuitBreaker = resilience.NewCircuitBreaker(config)
orchestrator, _ = orchestration.CreateOrchestrator(config, deps)
```

### Design Rationale

#### Why Not Import Resilience Directly?
- **Framework Rule**: Orchestration can only import core + telemetry
- **Separation of Concerns**: Framework provides capability, apps choose implementation
- **Flexibility**: Apps might use different circuit breaker libraries
- **Testability**: Can test orchestration without resilience module

#### Why Follow UI Module Pattern?
- **Proven Pattern**: Already working successfully in production
- **Consistency**: Same pattern across framework
- **Developer Familiarity**: Learn once, apply everywhere
- **Maintenance**: Single pattern to maintain and document

#### Why Three Layers of Resilience?
- **Defense in Depth**: Multiple failure handling strategies
- **Progressive Enhancement**: Each layer adds protection
- **Flexibility**: Choose appropriate level for use case
- **No Vendor Lock-in**: Can use framework, custom, or service mesh resilience

### Implementation Examples

#### Service Provider with Three-Layer Resilience
```go
func (s *ServiceCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    // Layer 2: Use injected circuit breaker if provided
    if s.circuitBreaker != nil {
        var result string
        err := s.circuitBreaker.Execute(ctx, func() error {
            var err error
            result, err = s.queryExternalService(ctx, request, metadata)
            return err
        })

        if err != nil {
            // Layer 3: Try fallback
            if s.fallback != nil {
                return s.fallback.GetCapabilities(ctx, request, metadata)
            }
            return "", err
        }
        return result, nil
    }

    // Layer 1: Use simple built-in resilience
    return s.getCapabilitiesWithSimpleResilience(ctx, request, metadata)
}
```

#### Circuit Breaker Integration
```go
type ResilientOrchestrator struct {
    *AIOrchestrator
    circuitBreakers map[string]core.CircuitBreaker
}

func (o *ResilientOrchestrator) callComponent(ctx context.Context, component string, request interface{}) (interface{}, error) {
    cb, exists := o.circuitBreakers[component]
    if !exists {
        cb = o.createCircuitBreaker(component)
        o.circuitBreakers[component] = cb
    }

    return cb.Execute(func() (interface{}, error) {
        return o.executeComponentCall(ctx, component, request)
    })
}
```

#### Retry Mechanisms
```go
type RetryConfig struct {
    MaxAttempts int
    InitialDelay time.Duration
    BackoffFactor float64
    MaxDelay time.Duration
}

func (e *SmartExecutor) executeWithRetry(ctx context.Context, step PlanStep, config RetryConfig) (interface{}, error) {
    var lastErr error
    delay := config.InitialDelay

    for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
        result, err := e.execute(ctx, step)
        if err == nil {
            return result, nil
        }

        lastErr = err
        if attempt < config.MaxAttempts {
            time.Sleep(delay)
            delay = time.Duration(float64(delay) * config.BackoffFactor)
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        }
    }

    return nil, fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
```

#### Timeout Management
```go
func (e *SmartExecutor) executeWithTimeout(ctx context.Context, step PlanStep, timeout time.Duration) (interface{}, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    resultChan := make(chan interface{}, 1)
    errorChan := make(chan error, 1)

    go func() {
        result, err := e.execute(ctx, step)
        if err != nil {
            errorChan <- err
        } else {
            resultChan <- result
        }
    }()

    select {
    case result := <-resultChan:
        return result, nil
    case err := <-errorChan:
        return nil, err
    case <-ctx.Done():
        return nil, fmt.Errorf("step %s timed out after %v", step.Name, timeout)
    }
}
```

#### Graceful Degradation
```go
func (o *AIOrchestrator) ProcessRequestWithDegradation(ctx context.Context, request string) (*Response, error) {
    // Try AI routing
    response, err := o.ProcessRequest(ctx, request)
    if err == nil {
        return response, nil
    }

    // Fall back to cached response
    if cached := o.cache.Get(request); cached != nil {
        return cached.(*Response), nil
    }

    // Fall back to basic routing
    if basicResponse := o.basicRouter.Route(request); basicResponse != nil {
        return basicResponse, nil
    }

    // Return helpful error
    return &Response{
        Status: "degraded",
        Content: "Service temporarily limited. Please try again or use specific commands.",
    }, nil
}
```

#### Layer 4: Semantic Retry (Contextual Re-Resolution)

Beyond the three-layer resilience architecture for capability services, the execution layer has its own advanced error recovery: **Semantic Retry**.

**When Layer 3 Error Analysis says "cannot fix"**, Semantic Retry provides one more chance by using the full execution trajectory:

```go
// ExecutionContext captures everything needed for semantic retry
type ExecutionContext struct {
    UserQuery       string                 // Original user intent
    SourceData      map[string]interface{} // Data from dependent steps
    StepID          string                 // Current step being executed
    Capability      *EnhancedCapability    // Target capability schema
    AttemptedParams map[string]interface{} // What we tried
    ErrorResponse   string                 // What went wrong
    HTTPStatus      int                    // Error status code
}

// ContextualReResolver computes corrected parameters
type ContextualReResolver struct {
    aiClient core.AIClient
    logger   core.Logger
}

func (r *ContextualReResolver) ReResolve(ctx context.Context, execCtx *ExecutionContext) (*ReResolutionResult, error) {
    // LLM analyzes full context and computes corrected parameters
    // Returns: {ShouldRetry: true, CorrectedParameters: {...}, Analysis: "..."}
}
```

**Example scenario:**
```
User: "Sell 100 Tesla shares and convert proceeds to EUR"
Step 1: Returns {price: 468.285}
Step 2: Fails with "amount must be > 0" (amount: 0)

Layer 4 computes: 100 × 468.285 = 46828.5
Retries with corrected parameters → SUCCESS
```

**Configuration:**
```go
config := DefaultConfig()
config.SemanticRetry.Enabled = true    // Default: true
config.SemanticRetry.MaxAttempts = 2   // Default: 2
```

**Environment Variables:**
```bash
GOMIND_SEMANTIC_RETRY_ENABLED=true
GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS=2
```

For detailed design, see [SEMANTIC_RETRY_DESIGN.md](./notes/SEMANTIC_RETRY_DESIGN.md).

### Implementation Checklist

When implementing resilience patterns:

- [ ] Update ServiceCapabilityConfig with optional dependencies
- [ ] Refactor ServiceCapabilityProvider to use injected circuit breaker
- [ ] Create OrchestratorDependencies struct
- [ ] Update factory functions to accept dependencies
- [ ] Add WithCircuitBreaker option function
- [ ] Write unit tests with mock circuit breaker
- [ ] Write integration tests with real circuit breaker
- [ ] Update documentation with usage examples
- [ ] Create migration guide for existing users
- [ ] Add telemetry metrics for resilience monitoring
- [ ] Configure health checks to report degraded state
- [ ] Document service mesh integration patterns

---

## Performance & Scalability

### Optimization Strategies

#### 1. Parallel Execution
```go
func (e *SmartExecutor) ExecuteParallel(ctx context.Context, steps []PlanStep) map[string]interface{} {
    results := make(map[string]interface{})
    resultChan := make(chan struct{name string; result interface{}}, len(steps))

    var wg sync.WaitGroup
    for _, step := range steps {
        wg.Add(1)
        go func(s PlanStep) {
            defer wg.Done()
            result, _ := e.execute(ctx, s)
            resultChan <- struct{name string; result interface{}}{s.Name, result}
        }(step)
    }

    go func() {
        wg.Wait()
        close(resultChan)
    }()

    for r := range resultChan {
        results[r.name] = r.result
    }

    return results
}
```

#### 2. Intelligent Caching
```go
type RoutingCache struct {
    cache *lru.Cache
    ttl   time.Duration
}

func (c *RoutingCache) GetOrCompute(key string, compute func() (*RoutingPlan, error)) (*RoutingPlan, error) {
    // Check cache
    if cached, ok := c.cache.Get(key); ok {
        entry := cached.(*cacheEntry)
        if time.Since(entry.timestamp) < c.ttl {
            return entry.plan, nil
        }
        c.cache.Remove(key)
    }

    // Compute and cache
    plan, err := compute()
    if err == nil {
        c.cache.Add(key, &cacheEntry{
            plan:      plan,
            timestamp: time.Now(),
        })
    }

    return plan, err
}
```

#### 3. Connection Pooling
```go
type ComponentConnPool struct {
    pools map[string]*ConnectionPool
    mu    sync.RWMutex
}

func (p *ComponentConnPool) GetConnection(component string) (*Connection, error) {
    p.mu.RLock()
    pool, exists := p.pools[component]
    p.mu.RUnlock()

    if !exists {
        p.mu.Lock()
        pool = NewConnectionPool(component, 10) // Max 10 connections
        p.pools[component] = pool
        p.mu.Unlock()
    }

    return pool.Get()
}
```

### Metrics & Monitoring

```go
type OrchestratorMetrics struct {
    TotalRequests        int64
    SuccessfulRequests   int64
    FailedRequests       int64
    AverageLatency       time.Duration
    ComponentCalls       map[string]int64
    CacheHitRate         float64
    ParallelExecutions   int64
}

func (o *AIOrchestrator) recordMetrics(start time.Time, success bool, components []string) {
    duration := time.Since(start)

    o.metricsMutex.Lock()
    defer o.metricsMutex.Unlock()

    o.metrics.TotalRequests++
    if success {
        o.metrics.SuccessfulRequests++
    } else {
        o.metrics.FailedRequests++
    }

    // Update average latency
    o.metrics.AverageLatency = time.Duration(
        (int64(o.metrics.AverageLatency)*(o.metrics.TotalRequests-1) + int64(duration)) / o.metrics.TotalRequests,
    )

    // Track component usage
    for _, comp := range components {
        o.metrics.ComponentCalls[comp]++
    }

    // Emit metrics if telemetry available
    if o.telemetry != nil {
        o.telemetry.RecordMetric("orchestrator.request.duration", duration.Seconds(), map[string]string{
            "success": fmt.Sprintf("%v", success),
        })
    }
}
```

---

## Production Deployment

### Kubernetes Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchestrator
  namespace: gomind-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: orchestrator
  template:
    metadata:
      labels:
        app: orchestrator
    spec:
      containers:
      - name: orchestrator
        image: gomind/orchestrator:latest
        env:
        - name: REDIS_URL
          value: "redis://redis:6379"
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-secrets
              key: openai-key
        - name: GOMIND_CAPABILITY_SERVICE_URL
          value: "http://capability-service:8080"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4318"
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

### Environment Variables

```bash
# Required
export REDIS_URL="redis://redis:6379"              # Discovery service
export OPENAI_API_KEY="sk-..."                     # AI provider

# Optional - Capability Service (for scale)
export GOMIND_CAPABILITY_SERVICE_URL="http://capability-service:8080"
export GOMIND_CAPABILITY_TOP_K="50"
export GOMIND_CAPABILITY_THRESHOLD="0.75"

# Optional - Telemetry
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
export OTEL_SERVICE_NAME="orchestrator"

# Optional - Configuration
export GOMIND_ORCHESTRATOR_CACHE_ENABLED="true"
export GOMIND_ORCHESTRATOR_CACHE_TTL="5m"
export GOMIND_ORCHESTRATOR_MAX_CONCURRENCY="10"
export GOMIND_ORCHESTRATOR_STEP_TIMEOUT="30s"
```

### Health Checks

```go
func (o *AIOrchestrator) HealthCheck() HealthStatus {
    status := HealthStatus{
        Status: "healthy",
        Checks: make(map[string]bool),
    }

    // Check discovery connection
    if err := o.discovery.Ping(); err != nil {
        status.Checks["discovery"] = false
        status.Status = "degraded"
    } else {
        status.Checks["discovery"] = true
    }

    // Check AI client
    if o.aiClient != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if _, err := o.aiClient.GenerateResponse(ctx, "test", nil); err != nil {
            status.Checks["ai"] = false
            status.Status = "degraded"
        } else {
            status.Checks["ai"] = true
        }
    }

    // Check capability service if configured
    if o.capabilityProvider != nil {
        if _, err := o.capabilityProvider.GetCapabilities(context.Background(), "test"); err != nil {
            status.Checks["capabilities"] = false
            // Don't degrade if fallback is enabled
            if !o.config.EnableFallback {
                status.Status = "degraded"
            }
        } else {
            status.Checks["capabilities"] = true
        }
    }

    return status
}
```

---

## Common Patterns & Examples

### Pattern 1: Simple Q&A System

```go
func createQAOrchestrator() *orchestration.AIOrchestrator {
    // Minimal setup for Q&A
    discovery := core.NewLocalDiscovery()
    aiClient, _ := ai.NewClient()

    return orchestration.CreateSimpleOrchestrator(discovery, aiClient)
}

func handleQuestion(orchestrator *orchestration.AIOrchestrator, question string) string {
    response, err := orchestrator.ProcessRequest(
        context.Background(),
        question,
        nil,
    )

    if err != nil {
        return "Sorry, I couldn't process that question."
    }

    return response.Response
}
```

### Pattern 2: Multi-Tool Analysis

```go
func analyzeCompany(orchestrator *orchestration.AIOrchestrator, company string) (*Analysis, error) {
    request := fmt.Sprintf("Provide comprehensive analysis of %s including financials, news, sentiment, and technical indicators", company)

    response, err := orchestrator.ProcessRequest(
        context.Background(),
        request,
        map[string]interface{}{
            "priority": "high",
            "cache":    true,
        },
    )

    if err != nil {
        return nil, err
    }

    // Parse structured response
    var analysis Analysis
    if err := json.Unmarshal([]byte(response.Response), &analysis); err != nil {
        return nil, err
    }

    return &analysis, nil
}
```

### Pattern 3: Workflow-Based ETL

```yaml
name: etl-pipeline
inputs:
  source:
    type: string
    required: true
  destination:
    type: string
    required: true

steps:
  - name: extract
    tool: data-extractor
    action: extract
    inputs:
      source: ${inputs.source}
    retry:
      max_attempts: 3

  - name: validate
    tool: data-validator
    action: validate
    inputs:
      data: ${steps.extract.output}
    depends_on: [extract]

  - name: transform
    agent: transformation-agent
    action: transform
    inputs:
      data: ${steps.validate.output}
      rules: ${inputs.transform_rules}
    depends_on: [validate]

  - name: load
    tool: data-loader
    action: load
    inputs:
      data: ${steps.transform.output}
      destination: ${inputs.destination}
    depends_on: [transform]

outputs:
  records_processed: ${steps.load.output.count}
  status: ${steps.load.output.status}
```

### Pattern 4: Hybrid Intelligence

```go
func hybridOrchestration(orchestrator *orchestration.AIOrchestrator) {
    // Use AI for exploration
    explorationResponse, _ := orchestrator.ProcessRequest(
        context.Background(),
        "What data sources should I use for Tesla analysis?",
        nil,
    )

    // Extract patterns from AI response
    patterns := extractPatterns(explorationResponse)

    // Create workflow from patterns
    workflow := createWorkflowFromPatterns(patterns)

    // Execute workflow for production
    stateStore := orchestration.NewRedisStateStore(orchestrator.discovery)
    engine := orchestration.NewWorkflowEngine(orchestrator.discovery, stateStore, logger)
    result, _ := engine.ExecuteWorkflow(
        context.Background(),
        workflow,
        map[string]interface{}{"company": "TSLA"},
    )
}
```

---

## Troubleshooting Guide

### Common Issues and Solutions

#### Issue 1: AI Client Not Responding

**Symptoms**: Orchestrator fails with "AI client not configured" or timeouts

**Diagnosis**:
```go
// Check AI client availability
if orchestrator.aiClient == nil {
    log.Error("AI client is nil")
}

// Test AI client directly
response, err := aiClient.GenerateResponse(ctx, "test", nil)
if err != nil {
    log.Errorf("AI client error: %v", err)
}
```

**Solutions**:
1. Verify AI API key is set correctly
2. Check network connectivity to AI service
3. Ensure AI client is properly injected into orchestrator
4. Verify AI service rate limits aren't exceeded

#### Issue 2: Discovery Service Failures

**Symptoms**: Cannot find tools/agents, "no components available"

**Diagnosis**:
```go
// Check discovery connection
components, err := discovery.Discover(ctx, core.DiscoveryFilter{})
if err != nil {
    log.Errorf("Discovery error: %v", err)
}
log.Infof("Found %d components", len(components))
```

**Solutions**:
1. Verify Redis is running and accessible
2. Check that components are registering correctly
3. Ensure discovery refresh is happening (every 10s by default)
4. Verify network policies allow discovery traffic

#### Issue 3: Workflow Execution Hangs

**Symptoms**: Workflow starts but never completes

**Diagnosis**:
```go
// Enable debug logging
config.LogLevel = "debug"

// Add step monitoring
workflow.OnStepComplete = func(step string, duration time.Duration) {
    log.Infof("Step %s completed in %v", step, duration)
}
```

**Solutions**:
1. Check for circular dependencies in workflow
2. Verify all required inputs are provided
3. Ensure component timeouts are configured
4. Check for deadlocks in parallel execution

#### Issue 4: High Memory Usage

**Symptoms**: Orchestrator memory grows unbounded

**Diagnosis**:
```go
// Monitor metrics
metrics := orchestrator.GetMetrics()
log.Infof("Cache size: %d entries", metrics.CacheSize)
log.Infof("History size: %d records", len(orchestrator.GetExecutionHistory()))
```

**Solutions**:
1. Configure cache with appropriate TTL and size limits
2. Limit execution history buffer size
3. Ensure proper cleanup of completed executions
4. Check for goroutine leaks in parallel execution

#### Issue 5: Capability Service Overload

**Symptoms**: Slow responses when using service-based capability provider

**Diagnosis**:
```bash
# Check capability service health
curl http://capability-service:8080/health

# Monitor response times
time curl -X POST http://capability-service:8080/search \
  -H "Content-Type: application/json" \
  -d '{"query": "test", "top_k": 20}'
```

**Solutions**:
1. Increase capability service resources
2. Tune TopK parameter (reduce from 50 to 20)
3. Increase threshold to filter more aggressively
4. Enable fallback to default provider
5. Add caching layer for capability queries

---

## Future Considerations

### Potential Enhancements

1. **Streaming Response Support**
```go
type StreamingOrchestrator interface {
    ProcessRequestStream(ctx context.Context, request string) (<-chan ResponseChunk, error)
}
```

2. **Distributed Workflow Execution**
```go
type DistributedEngine struct {
    coordinator *ConsistentHash
    workers     map[string]*WorkerNode
}
```

3. **Visual Workflow Designer**
- Web-based UI for creating workflows
- Drag-and-drop component composition
- Real-time execution visualization

4. **Advanced Routing Strategies**
- Cost-based routing (minimize API costs)
- Load-based routing (distribute work evenly)
- Capability scoring (best fit selection)

5. **Event-Driven Orchestration**
```go
type EventOrchestrator struct {
    *AIOrchestrator
    eventBus *EventBus
}

func (o *EventOrchestrator) OnEvent(event Event) {
    // Trigger orchestration based on events
}
```

6. **Workflow Versioning**
```yaml
name: analysis-workflow
version: 2.0
compatible_with: ">=1.5"
migration:
  from: 1.0
  steps:
    - rename: old_step -> new_step
    - add: validation_step
```

### Areas for Research

1. **Predictive Caching**: Use ML to predict and pre-cache likely requests
2. **Adaptive Parallelization**: Dynamically adjust concurrency based on system load
3. **Semantic Workflow Discovery**: Find similar workflows based on intent
4. **Federated Orchestration**: Coordinate across multiple orchestrator instances
5. **Zero-Shot Planning**: Generate workflows for unseen request patterns

---

## Summary

The orchestration module is the brain of the GoMind framework, coordinating tools and agents to accomplish complex tasks. Its architecture emphasizes:

1. **Clean Separation**: Interface-based dependencies prevent coupling
2. **Progressive Enhancement**: Start simple, add complexity as needed
3. **Production Readiness**: Built-in resilience, monitoring, and scaling
4. **Flexibility**: Support for both AI-driven and workflow-based orchestration
5. **Performance**: Automatic parallelization, caching, and optimization

The module follows the framework's design principles religiously, ensuring that it remains modular, testable, and maintainable while providing powerful orchestration capabilities.

Remember: **The orchestrator never imports other modules directly** - it only depends on core interfaces. This is not a limitation but a strength that enables true modularity and flexibility.