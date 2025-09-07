# GoMind Core Module

Welcome to the foundation of intelligent agent systems! This guide will walk you through everything step-by-step, like a friendly mentor sitting right next to you. ☕

## 🎯 What Is This Module and Why Should You Care?

Let me explain this in the simplest way possible.

### The Coffee Shop Analogy

Imagine you're running a coffee shop. You have different workers:
- **The Barista** makes coffee
- **The Cashier** takes orders  
- **The Baker** makes pastries
- **The Cleaner** keeps things tidy

Now, imagine if these workers could:
1. **Find each other automatically** (like having each other's phone numbers)
2. **Remember important things** (like regular customers' favorite drinks)
3. **Handle problems gracefully** (if the coffee machine breaks, they know what to do)
4. **Work together seamlessly** (cashier tells barista what to make)

**That's exactly what the GoMind Core module does for your code!** It helps you build intelligent components that can work independently or together.

### The Two Types of Components: Tools and Agents

In GoMind, we have two fundamental building blocks:

#### 🔧 Tools (Passive Components)
Think of **Tools** like the appliances in your kitchen:
- A **toaster** toasts bread (doesn't make coffee)
- A **blender** blends ingredients (doesn't cook them)
- A **microwave** heats food (doesn't wash dishes)

Tools in GoMind:
- **Do ONE thing well** (like Unix commands: `ls`, `grep`, `sort`)
- **Register themselves** ("I'm a calculator, I can add and multiply")
- **Respond to requests** (process input, return output)
- **Are stateless** (don't maintain conversation context)
- **CANNOT discover or call other components** (they're passive)

```go
// Tools are created with NewTool()
calculator := core.NewTool("calculator")
```

#### 🤖 Agents (Active Orchestrators)
Think of **Agents** like the human workers who USE the tools:
- A **chef** uses multiple kitchen tools to create a meal
- A **barista** uses the coffee machine, grinder, and steamer
- A **manager** coordinates multiple workers

Agents in GoMind:
- **Can discover both tools and other agents**
- **Orchestrate complex workflows**
- **Make intelligent decisions** (often using AI)
- **Coordinate multiple components**
- **Maintain context and state**

```go
// Agents are created with NewBaseAgent()
orchestrator := core.NewBaseAgent("orchestrator")
```

### The Critical Architectural Rule

⚠️ **IMPORTANT** ⚠️  
**Tools are passive - they NEVER call or discover other components.**  
**Agents are active - they CAN discover and orchestrate everything.**

This separation ensures:
- Clean architecture
- Predictable behavior
- Easy testing
- Clear responsibility boundaries

### Anti-Pattern vs Correct Pattern

**❌ WRONG: Tool trying to discover:**
```go
// Tools should NEVER do this!
func (t *BadTool) Handler(w http.ResponseWriter, r *http.Request) {
    // Compile error! Tools don't have Discovery
    other, _ := t.Discovery.FindByCapability("something") // WON'T COMPILE!
}
```

**✅ RIGHT: Tool just does its job:**
```go
func (t *GoodTool) Handler(w http.ResponseWriter, r *http.Request) {
    // Process input, return output
    result := t.calculate(input)
    json.NewEncoder(w).Encode(result)
}
```

**✅ RIGHT: Agent orchestrating tools:**
```go
func (a *SmartAgent) Orchestrate(ctx context.Context) {
    // Agents CAN discover
    tools, _ := a.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
    })
    // ... coordinate the tools
}
```

### The Magic of Discovery (Redis Registry)

How do components find each other? Through a **registry** - think of it like a company directory.

**Redis** is our registry. It keeps track of:
- Which tools and agents are running
- Where they're located (their addresses)
- What they can do (their capabilities)
- Whether they're healthy (still working)

But here's the key difference:
- **Tools** can only REGISTER themselves ("I exist!")
- **Agents** can both REGISTER and DISCOVER ("Who's available?")

### Why This Matters for AI Applications

With the rise of AI and Large Language Models (LLMs), we need:
- **Discoverable tools** that AI can find and use
- **Intelligent agents** that can orchestrate tools
- **Clear boundaries** between passive tools and active orchestrators
- **Scalable architecture** where each component does one thing well

## 🎨 What's Included (and What's Not)

### ✅ Core Module Includes:
- **Component framework** - Both Tool and Agent base implementations
- **Discovery system** - Registry for tools, Discovery for agents
- **HTTP server** - Automatic server setup with health checks
- **Memory interface** - For state storage (in-memory by default)
- **Configuration** - Environment-based config with validation
- **Kubernetes support** - Automatic Service DNS resolution
- **Error handling** - Comprehensive error types and recovery

### ❌ NOT Included (Bring Your Own):
- **AI/LLM integration** - Add via the `ai` module
- **Workflow orchestration** - Add via the `orchestration` module  
- **Distributed tracing** - Add via the `telemetry` module
- **Circuit breakers** - Add via the `resilience` module
- **Actual business logic** - That's your job! 😊

### 🤖 Adding AI Support (Optional)

Want AI-powered components? It's easy!

```go
// AI-enhanced Tool (passive, but uses AI internally)
translator := ai.NewAITool("translator", apiKey)

// AI-powered Agent (can orchestrate AND use AI)
assistant := ai.NewAIAgent("assistant", apiKey)
```

## 🚀 Quick Start: Your First Components

### Prerequisites
- Go 1.21 or later
- Basic Go knowledge (packages, functions, structs)
- Redis (optional, for discovery between components)

### Installation

```bash
go get github.com/itsneelabh/gomind/core
```

### Example 1: Creating a Tool (Passive Component)

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create a tool (passive component)
    calculator := core.NewTool("calculator")
    
    // Register what it can do
    calculator.RegisterCapability(core.Capability{
        Name:        "add",
        Description: "Adds two numbers",
        Endpoint:    "/add",
        Handler: func(w http.ResponseWriter, r *http.Request) {
            // Parse input
            var input struct {
                A float64 `json:"a"`
                B float64 `json:"b"`
            }
            json.NewDecoder(r.Body).Decode(&input)
            
            // Calculate (tools just do their job)
            result := input.A + input.B
            
            // Return result
            json.NewEncoder(w).Encode(map[string]float64{
                "result": result,
            })
        },
    })
    
    // Initialize and start
    ctx := context.Background()
    calculator.Initialize(ctx)
    calculator.Start(ctx, 8080)
    
    // Tool is now running at http://localhost:8080
    // It CANNOT discover or call other components
    select {} // Keep running
}
```

### Example 2: Creating an Agent (Active Orchestrator)

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/itsneelabh/gomind/core"
)

func main() {
    // Create an agent (active orchestrator)
    orchestrator := core.NewBaseAgent("orchestrator")
    
    // Configure with discovery capability
    framework, _ := core.NewFramework(orchestrator,
        core.WithRedisURL("redis://localhost:6379"),
        core.WithDiscovery(true),
    )
    
    // Initialize
    ctx := context.Background()
    framework.Initialize(ctx)
    
    // Agents CAN discover components
    go func() {
        // Find all available tools
        tools, _ := orchestrator.Discover(ctx, core.DiscoveryFilter{
            Type: core.ComponentTypeTool,
        })
        
        fmt.Printf("Found %d tools\n", len(tools))
        for _, tool := range tools {
            fmt.Printf("- %s at %s:%d\n", tool.Name, tool.Address, tool.Port)
        }
        
        // Find other agents
        agents, _ := orchestrator.Discover(ctx, core.DiscoveryFilter{
            Type: core.ComponentTypeAgent,
        })
        
        fmt.Printf("Found %d agents\n", len(agents))
    }()
    
    // Start the agent
    framework.Run(ctx)
}
```

## 📚 Understanding Component Registration and Discovery

### How Tools Register Themselves

Tools announce their existence but can't look for others:

```go
tool := core.NewTool("weather-tool")

// Tools only get Registry (not Discovery)
// They can register but not discover
framework, _ := core.NewFramework(tool,
    core.WithRedisURL("redis://localhost:6379"),
    core.WithDiscovery(true), // Enables registration
)

// The tool is now in the registry
// Other agents can find it, but it can't find others
```

### How Agents Discover Components

Agents can find both tools and other agents:

```go
agent := core.NewBaseAgent("coordinator")

// Agents get full Discovery interface
// Find tools for specific tasks
weatherTools, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeTool,
    Capabilities: []string{"weather_forecast"},
})

// Find other agents for delegation
aiAgents, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeAgent,
    Capabilities: []string{"natural_language_processing"},
})
```

## 🏗️ Architecture Patterns

### Pattern 1: Tool Collection with Agent Coordinator

```
┌─────────────────────────────────────────┐
│           Orchestrator Agent            │
│         (Discovers & Coordinates)       │
└────────────────┬────────────────────────┘
                 │ Discovers
    ┌────────────┼────────────┬───────────┐
    ▼            ▼            ▼           ▼
┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│Calculator││ Weather  ││Database ││Translator│
│  Tool    ││  Tool    ││  Tool   ││  Tool    │
└─────────┘ └─────────┘ └─────────┘ └─────────┘
```

### Pattern 2: Hierarchical Agents

```
┌─────────────────────────────────────────┐
│          Master Agent (AI-Powered)      │
└────────────────┬────────────────────────┘
                 │ Discovers & Delegates
    ┌────────────┼────────────┐
    ▼            ▼            ▼
┌─────────┐ ┌─────────┐ ┌─────────┐
│  Data   ││Analytics ││ Report   │
│  Agent  ││  Agent   ││  Agent   │
└────┬────┘ └────┬────┘ └────┬────┘
     │           │           │
  [Tools]    [Tools]     [Tools]
```

## 🚀 Advanced Features

### Kubernetes Support

Both Tools and Agents work seamlessly in Kubernetes:

```go
// Configuration automatically detects Kubernetes
config := core.DefaultConfig()
config.Kubernetes.ServiceName = "calculator-tool"
config.Kubernetes.ServicePort = 80

tool := core.NewToolWithConfig(config)
// Registers as: calculator-tool.namespace.svc.cluster.local:80
```

### Component Filtering

Agents can filter discoveries by multiple criteria:

```go
// Find specific tools
mathTools, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Type:         core.ComponentTypeTool,
    Capabilities: []string{"add", "multiply"},
    Name:         "calculator",
})

// Find agents in specific namespace
productionAgents, _ := agent.Discover(ctx, core.DiscoveryFilter{
    Type: core.ComponentTypeAgent,
    Metadata: map[string]interface{}{
        "environment": "production",
    },
})
```

## 🎓 Best Practices

### 1. Choose the Right Component Type

**Use a Tool when:**
- Building a single-purpose function
- Creating a stateless processor
- Implementing a pure calculation or transformation
- Building something that responds to requests

**Use an Agent when:**
- Orchestrating multiple components
- Making decisions based on discovery
- Implementing workflows
- Building something that initiates actions

### 2. Keep Tools Simple

```go
// ✅ GOOD: Tool does one thing
type CalculatorTool struct {
    *core.BaseTool
}

func (c *CalculatorTool) Add(a, b float64) float64 {
    return a + b
}

// ❌ BAD: Tool trying to do too much
type BadTool struct {
    *core.BaseTool
}

func (b *BadTool) ProcessOrder() {
    // Trying to calculate, save to DB, send email...
    // This should be an Agent orchestrating multiple tools!
}
```

### 3. Use Agents for Coordination

```go
// ✅ GOOD: Agent orchestrating tools
type OrderAgent struct {
    *core.BaseAgent
}

func (o *OrderAgent) ProcessOrder(ctx context.Context, order Order) error {
    // Find calculator tool
    calc, _ := o.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
        Name: "calculator",
    })
    // Call it for tax calculation
    
    // Find database tool
    db, _ := o.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
        Name: "database",
    })
    // Call it to save order
    
    // Find email tool
    email, _ := o.Discover(ctx, core.DiscoveryFilter{
        Type: core.ComponentTypeTool,
        Name: "email",
    })
    // Call it to send confirmation
    
    return nil
}
```

## 🎯 Common Patterns and Solutions

### Pattern: AI-Enhanced Tool
```go
// A tool that uses AI internally but doesn't orchestrate
translator := ai.NewAITool("translator", apiKey)
translator.RegisterCapability(core.Capability{
    Name: "translate",
    Description: "Translates text using AI",
    Handler: translateHandler,
})
// Can use AI but can't discover other components
```

### Pattern: Workflow Agent
```go
// An agent that orchestrates a complex workflow
workflow := core.NewBaseAgent("workflow-engine")
// Can discover and coordinate multiple tools and agents
```

### Pattern: Gateway Agent
```go
// An agent that acts as a gateway to multiple tools
gateway := core.NewBaseAgent("api-gateway")
// Discovers available tools and routes requests
```

## 🔍 Debugging and Monitoring

### Check Component Registration

```bash
# See what's registered in Redis
redis-cli KEYS "gomind:*"

# Check specific component
redis-cli GET "gomind:services:calculator-tool-abc123"
```

### Component Health Checks

All components automatically provide health endpoints:
- Tools: `http://localhost:8080/health`
- Agents: `http://localhost:8090/health`

## 📊 Performance Considerations

### Tools are Lightweight
- Minimal overhead (~5MB binary)
- No discovery overhead
- Fast startup
- Perfect for serverless/FaaS

### Agents are Feature-Rich
- Full discovery capability (~10MB binary)
- Can coordinate complex workflows
- Suitable for long-running orchestrators
- Ideal for AI-powered coordination

## 🎉 Summary

The GoMind Core module provides two fundamental building blocks:

1. **Tools** - Passive components that do one thing well
2. **Agents** - Active orchestrators that discover and coordinate

This clear separation enables:
- Clean, maintainable architecture
- Predictable component behavior
- Easy testing and debugging
- Scalable AI-powered systems

Remember: **Tools work, Agents think!**

## 📚 Next Steps

- Explore the [AI Module](../ai/README.md) for AI-enhanced components
- Learn about [Orchestration](../orchestration/README.md) for complex workflows
- Add [Telemetry](../telemetry/README.md) for observability
- Implement [Resilience](../resilience/README.md) for production readiness

Happy building! 🚀