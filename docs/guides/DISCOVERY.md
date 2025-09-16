# Agent and Tool Discovery Guide

This guide explains how AI agents and tools find each other in the GoMind framework. Whether you're building a math-solving agent that needs to find calculation tools, or a document processor that needs language analysis capabilities, this guide will help you understand how components discover and connect with each other.

Think of discovery as an intelligent "AI capability marketplace" where agents can find the right tools and collaborators for any task.

## Table of Contents
- [Key Terms and Concepts](#key-terms-and-concepts)
- [Overview](#overview)
- [How Agents and Tools Register](#how-agents-and-tools-register)
- [How Agents Discover Tools and Other Agents](#how-agents-discover-tools-and-other-agents)
- [Heartbeat and TTL Management](#heartbeat-and-ttl-management)
- [Multi-Pod Deployments](#multi-pod-deployments)
- [Understanding System Behavior](#understanding-system-behavior)

## Key Terms and Concepts

Before diving into how discovery works, let's understand the fundamental concepts:

### Agent vs Tool
- **Agent**: An intelligent component that can make decisions, orchestrate workflows, and coordinate with other agents. Think of an agent as the "brain" that plans and executes complex AI tasks.
  - *Example*: A document analysis agent that decides which tools to use for OCR, language detection, and sentiment analysis.
  
- **Tool**: A specialized component that performs specific AI tasks when requested. Tools are "workers" that excel at particular capabilities.
  - *Example*: A mathematical computation tool that solves equations, or an image recognition tool that identifies objects.

### Capability
A **capability** is a specific AI skill or function that an agent or tool can perform. It's like listing your skills on a resume - it tells others what you can do.

- *Examples*: 
  - "mathematical_computation" - can solve math problems
  - "natural_language_processing" - can understand and process text
  - "image_recognition" - can identify objects in images
  - "document_analysis" - can extract information from documents

### Agent and Tool Discovery
**Agent and tool discovery** is the process of AI components finding each other automatically. Instead of hardcoding addresses like "call the math tool at localhost:8080", agents can ask "find me any tool that can do mathematical computation" and get back all available options.

### TTL (Time To Live)
**TTL** is an expiration timer on information. It's like a parking meter - when time runs out, the information expires and gets cleaned up automatically. This prevents dead or crashed components from staying in the system forever.

### Heartbeat
A **heartbeat** is a periodic "I'm still alive and ready to work" signal that components send to prove they're healthy and available for AI tasks.

## Overview

Agent and tool discovery in GoMind works like an AI capability marketplace:
- **Registration**: Agents and tools "advertise their capabilities" by registering in Redis
- **Discovery**: Agents "shop for capabilities" to find the right tools and collaborators
- **Heartbeat**: Components "stay online" by sending periodic availability updates
- **TTL (Time To Live)**: "Capabilities expire" if components don't stay responsive

## How Agents and Tools Register

**What registration means**: When an agent or tool starts up, it essentially puts up a "business listing" in a shared directory, telling everyone what AI capabilities it offers and how to contact it.

**Why this matters**: Without registration, agents would have no way to find tools, and tools would sit unused. Registration makes the entire AI ecosystem discoverable and collaborative.

When an AI tool or agent starts up, it registers itself with detailed information about what it can do:

```go
ServiceInfo{
    ID:           "calculator-tool-a1b2c3d4",  // Unique identifier (like a business license number)
    Name:        "calculator-tool",             // Human-readable name (like a business name)
    Type:        "tool",                        // "tool" or "agent" (what kind of component this is)
    Address:     "localhost",                   // Where to reach it (like a street address)
    Port:        8080,                         // Which port (like an apartment number)
    Capabilities: []Capability{                // What AI skills it offers
        {Name: "mathematical_computation", Description: "Performs complex mathematical calculations"},
        {Name: "data_analysis", Description: "Analyzes numerical data and generates insights"},
        {Name: "formula_solving", Description: "Solves algebraic and calculus equations"},
    },
}
```

**How the system organizes this information**: Registration creates two types of entries in Redis - detailed individual records and fast lookup indexes.

### Individual Component Records
Each agent or tool gets its own detailed record with all the information needed to contact it:

```
Key: gomind:services:calculator-tool-a1b2c3d4
Value: {full component info with address, port, AI capabilities, health status, etc.}
TTL: 30 seconds (expires quickly to detect failures)
```

**Why individual records**: This is like having a detailed business card for each component. When an agent wants to call a specific tool, it needs the complete contact information.

### Capability Index Sets (for fast searches)
The system also creates shared "phone books" that group components by what they can do:

```
gomind:capabilities:mathematical_computation → {calculator-tool-a1b2c3d4, advanced-math-agent-x7y8z9, statistics-tool-m5n6p7...}
gomind:capabilities:data_analysis → {calculator-tool-a1b2c3d4, analytics-agent-q2w3e4, visualization-tool-r8t9u0...}
gomind:names:calculator-tool → {calculator-tool-a1b2c3d4, calculator-tool-b2c3d4...}
gomind:types:tool → {calculator-tool-a1b2c3d4, nlp-tool-e5f6g7, image-tool-h8i9j0...}
TTL: 60 seconds (longer expiration for stability)
```

**Why indexes**: Imagine you need mathematical computation - instead of checking every single component one by one, you can instantly look up "who can do math?" and get a list. It's like having specialized directories for "Math Experts", "Language Specialists", "Image Processors", etc.

**Real-world analogy**: Think of this like a business directory:
- **Individual records** = detailed business listings with address, phone, services
- **Capability indexes** = category pages like "Plumbers", "Electricians", "Accountants"

## How Agents Discover Tools and Other Agents

**What discovery solves**: Instead of hardcoding which tools to call, agents can dynamically find the best available components for any task. It's like having a smart assistant that knows everyone in your company and their specialties.

The system provides three main ways for agents to find the components they need:

### Find by AI Capability

**When to use**: When you know what task you need done, but don't care who does it. This is the most common discovery pattern.

**Real-world scenario**: Your document processing agent needs to analyze sentiment in customer feedback. It doesn't care which specific tool does this - it just needs someone who can perform "sentiment_analysis".

```go
// Find all tools and agents that can analyze sentiment
components, err := discovery.FindByCapability(ctx, "sentiment_analysis")
// Returns: [nlp-tool-x1y2z3, sentiment-agent-a4b5c6, text-analyzer-d7e8f9, ...]
```

**What happens behind the scenes**:
1. System looks up the capability index: `gomind:capabilities:sentiment_analysis`
2. Gets a list of component IDs that offer this capability
3. Fetches detailed information for each ID
4. Filters out any components that have expired (stopped sending heartbeats)
5. Returns healthy components that can handle your request

**Benefits**: Fast lookup, automatic load balancing (you get all available options), fault tolerance (dead components are filtered out).

### Find by Component Name

**When to use**: When you need a specific tool or agent by name, possibly across multiple instances.

**Real-world scenario**: Your orchestration agent specifically needs to work with the "document-parser" tool because it has custom integration logic for that particular component.

```go
// Find all instances of the document parser tool
components, err := discovery.FindService(ctx, "document-parser")
// Returns: [document-parser-a1b2c3d4, document-parser-e5f6g7h8, ...] (multiple instances)
```

**What this gives you**: All running instances of a specific named component. Useful when you have specific integration requirements or need to work with a particular tool's API.

### Complex Discovery with Multiple Criteria

**When to use**: When you have specific requirements about the type of component, multiple capabilities, or environmental constraints.

**Real-world scenario**: You're building a production AI pipeline and need a tool (not an agent) that can do both data analysis and statistical modeling, running the latest model version in the production environment.

```go
// Find production-ready tools with multiple capabilities
components, err := discovery.Discover(ctx, DiscoveryFilter{
    Type:         "tool",                                           // Only tools, not agents
    Capabilities: []string{"data_analysis", "statistical_modeling"}, // Must have both capabilities
    Metadata:     map[string]string{                               // Additional requirements
        "env": "production", 
        "model_version": "v2.1",
        "certified": "true",
    },
})
```

**What this enables**: Precise filtering for complex requirements. Perfect for production systems where you need specific guarantees about capabilities and environment.

### How Discovery Works Under the Hood

The discovery process follows these steps:
1. **Check Capability Indexes**: Look up component IDs from the relevant AI capability indexes (fast Redis set operations)
2. **Fetch Component Details**: Get full registration information for each matching component ID
3. **Filter Expired**: Skip any components whose main entry has expired (they stopped sending heartbeats)
4. **Apply Additional Filters**: Remove components that don't match type, metadata, or other criteria
5. **Return Results**: Provide list of healthy, available AI components ready to handle your requests

**Performance characteristics**:
- Capability-based queries are fastest (use indexes)
- Name-based queries are moderately fast (use indexes)
- Complex filtering with metadata is slower but more precise (requires fetching full details)

## Heartbeat and TTL Management

At the heart of GoMind's discovery system lies an elegant lease-based architecture. Understanding this design will give you insights into how distributed systems maintain consistency without complex coordination protocols.

### The Concept of Distributed Leases

Imagine you're managing a co-working space. When someone reserves a desk, you don't give them permanent ownership - you give them a **lease** that expires after a certain time. If they want to keep the desk, they must renew their lease before it expires. If they disappear without renewing, their desk becomes available to others.

This is exactly how GoMind's TTL (Time To Live) system works. Services don't permanently register themselves; they obtain renewable leases on their registry entries.

### Why Leases Are Brilliant

**The Fundamental Problem**: In distributed AI systems, agents can crash, tools can become unresponsive, networks can fail, and AI model containers can be terminated unexpectedly. How do you know if an AI component is actually available for collaboration or just having a bad moment?

**Traditional Approach**: Keep a permanent registry and hope AI components unregister themselves when they shut down. (Spoiler: they often don't!)

**GoMind's Approach**: Every AI component registration is a lease that expires automatically. Agents and tools prove they're alive and ready for AI tasks by continuously renewing their lease through heartbeats.

### The Two-Layer Lease Architecture

GoMind uses a sophisticated two-layer lease system that balances performance with reliability:

#### Layer 1: Component Identity Leases (30 seconds)
```go
// Each AI component has its own lease on its detailed information
Key: gomind:services:calculator-tool-a1b2c3d4
TTL: 30 seconds
Contents: {address, port, health status, AI capabilities, model versions, metadata...}
```

This is the "proof of AI readiness" - detailed information about a specific agent or tool instance. Short TTL means quick detection of component failures or model unresponsiveness.

#### Layer 2: AI Capability Index Leases (60 seconds) 
```go
// AI components share leases on capability memberships
Keys: 
  gomind:capabilities:mathematical_computation → {calculator-tool-a1b2c3d4, math-agent-x7y8z9, stats-tool-m5n6p7}
  gomind:capabilities:natural_language_processing → {nlp-agent-q2w3e4, translation-tool-r8t9u0}
  gomind:names:calculator-tool → {calculator-tool-a1b2c3d4, calculator-tool-b2c3d4}
  gomind:types:agent → {orchestration-agent-e5f6g7, analysis-agent-h8i9j0}
TTL: 60 seconds (2x component TTL)
```

These are the "fast AI capability lookups" - they tell you which agents and tools provide which AI capabilities without having to scan everything. Longer TTL provides stability for capability index structures.

### The Heartbeat Protocol

Every agent and tool runs a heartbeat goroutine that executes this elegant renewal process:

```go
// Every 15 seconds (TTL/2 for safety margin):
func (r *RedisRegistry) UpdateHealth(componentID string, status HealthStatus) {
    // 1. Renew my individual component lease
    componentKey := "gomind:services:calculator-tool-a1b2c3d4"  
    r.client.Set(ctx, serviceKey, updatedData, 30*time.Second)
    
    // 2. Renew all shared index leases I participate in
    r.refreshIndexSetTTLs(ctx, &info)
    //   - Extends gomind:capabilities:math to 60s
    //   - Extends gomind:names:calculator-tool to 60s
    //   - Extends gomind:types:tool to 60s
}
```

**Why This Design Is Elegant**:
- **Self-healing**: Dead components automatically disappear without manual cleanup
- **Failure detection**: Components that stop heartbeating are detected within 30 seconds
- **Performance**: Indices remain stable even during brief component restarts
- **No coordination required**: Each component independently manages its own leases

### Understanding the Timing Strategy

The framework uses carefully chosen timing intervals:

```
Heartbeat Interval: 15 seconds (TTL ÷ 2)
Component TTL:      30 seconds  
Index TTL:          60 seconds (2 × Component TTL)
```

**Why TTL ÷ 2 for heartbeats?**
This provides a safety margin. Even if one heartbeat is delayed or lost, there's still time for the next one before the lease expires.

**Why 2× TTL for indices?**
Index structures are shared across multiple components and more expensive to rebuild. The longer lease provides stability while still ensuring cleanup of abandoned indices.

**Example Timeline**:
```
T=0s:   Component starts  → Component: 30s TTL, Indices: 60s TTL
T=15s:  Heartbeat #1      → Component: 30s TTL, Indices: 60s TTL (renewed)  
T=30s:  Heartbeat #2      → Component: 30s TTL, Indices: 60s TTL (renewed)
T=45s:  Heartbeat #3      → Component: 30s TTL, Indices: 60s TTL (renewed)
T=60s:  Heartbeat #4      → Component: 30s TTL, Indices: 60s TTL (renewed)
...continues as long as component is healthy
```

If the component crashes at T=30s:
```
T=30s:  Component crashes → No more heartbeats
T=45s:                    → Component: 15s remaining, Indices: 45s remaining
T=60s:                    → Component: EXPIRED, Indices: 30s remaining  
T=75s:                    → Component: gone, Indices: 15s remaining
T=90s:                    → Everything cleaned up automatically
```

### The Beauty of Emergent Behavior

What makes this design truly elegant is how complex distributed behaviors emerge from simple local rules:

**Individual Component Rule**: "Renew my lease every 15 seconds"

**Emergent System Behaviors**:
- **Automatic failover**: When components die, others take over seamlessly  
- **Load balancing**: Discovery returns all healthy instances automatically
- **Self-healing**: No manual cleanup required, ever
- **Graceful degradation**: System continues working even during partial failures

This is distributed systems design at its finest - complex system-level behaviors achieved through simple, local actions that compose beautifully.

## Multi-Pod Deployments

**What are "pods" and why do you need multiple instances?**: In production AI systems, you often run multiple copies of the same agent or tool for reliability and performance. Think of it like having multiple cashiers at a store - if one is busy or breaks down, others can handle the work.

**The challenge**: How does the system handle multiple identical components? Each instance needs its own identity, but agents searching for capabilities should find all available instances.

**GoMind's elegant solution**: Each instance gets a unique identity, but they all share the same capability listings. This gives you automatic load balancing and fault tolerance without any complex coordination.

Let's see how this works in practice:

### Unique Identity Per Pod

Each pod gets its own unique component ID:

```bash
# Kubernetes starts 3 replicas:
kubectl scale deployment calculator-service --replicas=3
```

```go
// Each pod registers with a unique ID:
Pod 1: calculator-service-a1b2c3d4 (IP: 10.1.2.3, Port: 8080)
Pod 2: calculator-service-e5f6g7h8 (IP: 10.1.2.4, Port: 8080)  
Pod 3: calculator-service-i9j0k1l2 (IP: 10.1.2.5, Port: 8080)
```

### Redis Structure with Multiple Pods

**Individual Component Entries (one per pod):**
```
gomind:services:calculator-service-a1b2c3d4 → Pod 1 info
gomind:services:calculator-service-e5f6g7h8 → Pod 2 info
gomind:services:calculator-service-i9j0k1l2 → Pod 3 info
```

**Shared Index Sets (all pods in same lists):**
```
gomind:capabilities:calculation → {
    calculator-service-a1b2c3d4,  // Pod 1
    calculator-service-e5f6g7h8,  // Pod 2  
    calculator-service-i9j0k1l2   // Pod 3
}

gomind:names:calculator-service → {
    calculator-service-a1b2c3d4,  // Pod 1
    calculator-service-e5f6g7h8,  // Pod 2
    calculator-service-i9j0k1l2   // Pod 3
}
```

### Independent Heartbeats

Each pod runs its own heartbeat goroutine:

```
Pod 1: Every 15s → Refreshes calculator-service-a1b2c3d4 + index sets
Pod 2: Every 15s → Refreshes calculator-service-e5f6g7h8 + index sets  
Pod 3: Every 15s → Refreshes calculator-service-i9j0k1l2 + index sets
```

### Smart Coordination

Here's the clever part - multiple pods refreshing the same index sets:

```
Time     Pod 1 Heartbeat    Pod 2 Heartbeat    Pod 3 Heartbeat    Index Set TTL
T=0s     Sets TTL=60s       Sets TTL=60s       Sets TTL=60s       60s (latest)
T=15s    Sets TTL=60s       -                  -                  60s (Pod 1)
T=30s    -                  Sets TTL=60s       -                  60s (Pod 2)
T=45s    -                  -                  Sets TTL=60s       60s (Pod 3)
T=60s    Sets TTL=60s       -                  -                  60s (Pod 1)
```

**Key insight**: Redis `EXPIRE` command is "last writer wins" - whichever pod heartbeats most recently sets the TTL. This means index sets stay alive as long as **any pod is healthy**.

### Fault Tolerance Example

Let's say Pod 1 crashes:

```
T=0s     Pod 1: ❌ (died)    Pod 2: Sets TTL=60s    Pod 3: Sets TTL=60s    → 60s
T=15s    Pod 1: ❌          Pod 2: -               Pod 3: -               → 45s  
T=30s    Pod 1: ❌          Pod 2: Sets TTL=60s    Pod 3: -               → 60s ✅
T=45s    Pod 1: ❌          Pod 2: -               Pod 3: Sets TTL=60s    → 60s ✅
```

**Result:**
- Pod 1's component entry expires after 30s (no more heartbeats)
- Index sets stay alive because Pod 2 & 3 keep refreshing them
- Discovery still works and returns Pod 2 & 3
- Automatic load balancing and failover!

### Discovery with Multiple Pods

When clients discover services:

```go
// Find all calculator services
services, err := discovery.FindByCapability(ctx, "calculation")

// Returns all healthy pods:
// [
//   {ID: "calculator-service-e5f6g7h8", Address: "10.1.2.4", Port: 8080},  // Pod 2
//   {ID: "calculator-service-i9j0k1l2", Address: "10.1.2.5", Port: 8080}   // Pod 3
// ]
// Note: Pod 1 is missing because it crashed and expired

// Client can pick any pod:
selectedService := services[rand.Intn(len(services))]
response := callService(selectedService)
```

### Benefits of This Architecture

1. **Automatic Load Balancing**: Discovery returns all healthy pods, clients can choose
2. **Fault Tolerance**: Individual pod failures don't break discovery for remaining pods
3. **No Coordination Needed**: Each pod works independently, no complex synchronization
4. **Clean Cleanup**: When all pods die, everything expires automatically
5. **Scale Up/Down**: Adding/removing pods just works without configuration changes

## Understanding System Behavior

**Why this section matters**: When you start using the discovery system, you might notice behaviors that seem unusual if you're used to simpler systems. This section explains what's normal, what's by design, and what might need attention.

**The key insight**: GoMind prioritizes keeping your AI system running over having perfect information. It's better to have slightly outdated data than a completely failed system.

### The Graceful Degradation Principle

**What graceful degradation means**: Instead of the entire system crashing when something goes wrong, only the affected parts stop working while everything else continues normally.

**Real-world analogy**: If one elevator in a building breaks, people can still use the other elevators and the stairs. The building doesn't shut down completely.

**How GoMind applies this**: When agents or tools fail, discovery continues working with the remaining healthy components. The system gradually cleans up information about failed components instead of immediately removing everything.

### Discovery Behavior During Component Transitions

**What you might observe**: Sometimes discovery methods return slightly different results during rapid component startup/shutdown cycles.

**Why this happens**: The two-layer lease system means component identity leases (30s TTL) and index leases (60s TTL) can be in different states during transitions. This is by design - it provides stability during brief network hiccups while ensuring eventual consistency.

**Example scenario**:
```
T=0s:   Component registers     → Both layers: fully available
T=30s:  Component crashes       → Component lease expires, index lease remains
T=45s:  Discovery query         → Finds component ID in index, but component details expired
        Result: Component filtered out automatically (working as intended)
T=90s:  Index lease expires     → Complete cleanup, system returns to consistent state
```

This behavior demonstrates the system's resilience - it never returns unreachable components to clients, even during transitional states.

### Performance Characteristics You Should Expect

**Discovery Speed**: 
- Filtered queries (by capability, name, type) are very fast - they use index lookups
- Unfiltered queries are slower - they scan all component keys but are more resilient

**Memory Usage**:
- Redis memory grows gradually as components register and deregister
- This is normal behavior - dead component IDs accumulate in index sets but are filtered out during discovery
- The system prioritizes reliability over memory optimization

**Consistency Model**:
- GoMind uses **eventual consistency** - there may be brief moments where different discovery methods return slightly different results
- The system converges to consistency as leases expire and renew
- This trade-off enables high availability without complex distributed coordination

### Observing the Lease System in Action

You can observe the lease system's behavior using Redis CLI:

```bash
# Watch component leases being renewed every 15 seconds
redis-cli MONITOR | grep "gomind:services"

# Check current TTLs to understand lease states  
redis-cli TTL "gomind:services:your-component-id"
redis-cli TTL "gomind:capabilities:your-capability"

# Count components participating in each capability
redis-cli SCARD "gomind:capabilities:calculation"
```

**What healthy behavior looks like**:
- Component TTLs should consistently refresh to ~30 seconds
- Index TTLs should consistently refresh to ~60 seconds  
- Component counts in capabilities should reflect the actual number of healthy components

### When to Use Different Discovery Methods

**Choosing the right discovery method**: Each method is optimized for different use cases. Using the right one can improve both performance and reliability.

### `FindByCapability()` - The Workhorse Method

**Best for**: Day-to-day AI task coordination where you need a specific skill

**Use `FindByCapability()` when**:
- You know what AI task needs to be done ("I need sentiment analysis")
- You don't care which specific tool does it (any capable component is fine)
- You want the fastest possible results (uses optimized indexes)
- You want automatic load balancing (returns all available providers)

**Example scenarios**:
- Document processing agent needs OCR capability
- Chat bot needs language translation
- Analytics pipeline needs statistical modeling

### `FindService()` - The Targeted Method

**Best for**: When you have specific integration requirements or need to work with particular components

**Use `FindService()` when**:
- You've built custom integration with a specific named tool
- You need all instances of a particular component (for monitoring or coordination)
- You have business logic that depends on a specific tool's behavior
- You're implementing failover logic between named components

**Example scenarios**:
- Integration with a custom "company-specific-analyzer" tool
- Monitoring all instances of your "document-processor" service
- Coordinating work across multiple instances of the same agent

### `Discover()` with Complex Filters - The Precision Method

**Best for**: Production systems with strict requirements or complex deployment constraints

**Use complex `Discover()` when**:
- You have multiple criteria that must all be met
- You need components with specific environmental properties
- You're implementing production-grade reliability requirements
- You need to filter by metadata like version, certification, or deployment environment

**Example scenarios**:
- Production pipeline requiring certified, latest-version tools only
- Development environment needing debug-enabled tools
- Multi-tenant system filtering by customer or region

### `Discover()` with No Filters - The Debugging Method

**Best for**: System analysis, debugging, and operational visibility

**Use unfiltered `Discover()` when**:
- You're troubleshooting discovery issues
- You need to see the complete state of your AI system
- You want the most resilient discovery method (doesn't depend on indexes)
- You're building monitoring or administrative tools

**Example scenarios**:
- System health dashboard showing all components
- Debugging why a specific capability isn't being found
- Auditing what's currently running in your AI ecosystem

### Understanding "Expected" Anomalies

**Why some behaviors might seem odd**: If you're used to traditional databases or simple component lists, GoMind's distributed behavior might seem unusual at first. These "quirks" are actually intelligent design choices.

**What might seem like problems but are actually features working correctly**:

**Brief discovery inconsistencies during deployments**: 
- *What you see*: Different discovery methods return slightly different results for a few seconds
- *Why this happens*: Services are registering/deregistering, and the two-layer TTL system creates brief inconsistencies
- *Why this is good*: The system continues working during changes instead of locking up

**Gradual memory growth in Redis**: 
- *What you see*: Redis memory usage grows slowly over time, even after components shut down
- *Why this happens*: Dead component IDs remain in capability indexes until TTL expiration
- *Why this is good*: The system prioritizes availability and performance over perfect memory efficiency

**Different results from different discovery methods**: 
- *What you see*: `FindByCapability()` and `Discover()` might return different results
- *Why this happens*: Each method has different consistency guarantees and performance trade-offs
- *Why this is good*: You can choose the right balance of speed vs. completeness for your use case

**Services appearing in some queries but not others during failures**: 
- *What you see*: A component shows up in capability searches but not in detailed discovery
- *Why this happens*: The component's capability index entry hasn't expired yet, but its detailed record has
- *Why this is good*: The system protects you from calling dead components while maintaining fast capability lookups

### The Philosophy Behind the Design

GoMind's discovery system embodies several distributed systems principles:

1. **Availability over Consistency**: The system remains available even when parts fail
2. **Simple Local Rules**: Complex behaviors emerge from simple heartbeat logic
3. **No Single Point of Failure**: Each component manages its own state independently  
4. **Self-Healing**: Problems resolve automatically without human intervention
5. **Graceful Resource Usage**: The system balances performance, memory, and reliability

Understanding these principles helps you work **with** the system rather than fighting against its design. When you see behaviors that seem unusual, ask yourself: "How does this serve the goal of keeping agents and tools discoverable and available?"

Most of the time, you'll find that the system is making intelligent trade-offs to maintain overall system health.