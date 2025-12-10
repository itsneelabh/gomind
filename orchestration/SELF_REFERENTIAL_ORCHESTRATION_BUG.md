# Self-Referential Orchestration Bug

**Status:** IDENTIFIED (Not Yet Fixed)
**Severity:** Medium
**Discovered:** December 2025
**Module:** orchestration/catalog.go, orchestration/capability_provider.go
**Related:** examples/agent-with-orchestration/research_agent.go

## Summary

The orchestrator includes its own `orchestrate_natural` capability in the service catalog that is sent to the LLM for planning. This causes the LLM to potentially include recursive self-calls in its execution plans, leading to 400 errors and infinite recursion attempts.

## Symptoms

When analyzing Jaeger traces for complex orchestration requests:

| Symptom | Description |
|---------|-------------|
| Recursive spans | `travel-research-orchestration → travel-research-orchestration` calls |
| 400 Bad Request | Error: "request field is required" |
| High error count | 20+ errors in a single trace |
| Failed self-calls | Orchestrator calling itself with malformed parameters |

### Example Error in Logs

```
POST /orchestrate/natural 400 Bad Request
{"error": "request field is required"}
```

## Root Cause Analysis

### The Problem

The orchestrator agent (`travel-research-agent`) registers ALL its capabilities with the discovery service, including `orchestrate_natural`. When the orchestrator builds a prompt for the LLM, it queries the catalog for available agents/tools. The catalog returns ALL registered services - including the orchestrator itself.

The LLM sees `orchestrate_natural` as an available capability and may include it in the execution plan, causing recursive self-calls.

### Flow Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        Bug Flow Sequence                                  │
└──────────────────────────────────────────────────────────────────────────┘

1. TravelResearchAgent.registerCapabilities()
   └── Registers "orchestrate_natural" capability
       Location: research_agent.go:452-484

2. Agent registers with Redis Discovery
   └── All capabilities including orchestrate_natural stored in Redis

3. Orchestrator.ProcessNaturalRequest() is called
   └── Location: orchestrator.go:577

4. capabilityProvider.GetCapabilities()
   └── Calls catalog.FormatForLLM()
       Location: capability_provider.go:40

5. AgentCatalog.FormatForLLM()
   └── Iterates over ALL agents (including itself)
       Location: catalog.go:450-479

   OUTPUT INCLUDES:
   ┌────────────────────────────────────────────────────────────────┐
   │ Agent: travel-research-agent (ID: xyz)                        │
   │   - Capability: orchestrate_natural                           │
   │     Description: Process natural language travel requests...  │
   │     Parameters:                                                │
   │       - request: string (required)                            │
   └────────────────────────────────────────────────────────────────┘

6. LLM receives prompt with orchestrate_natural in catalog
   └── LLM generates plan that MAY include orchestrate_natural calls

7. SmartExecutor.executeStep() calls travel-research-agent/orchestrate/natural
   └── Creates RECURSIVE orchestration request

8. ERROR: "request field is required"
   └── Parameters mismatch - LLM generated wrong format for self-call
```

### Code Evidence

#### 1. Capability Registration (research_agent.go:452-484)

```go
// registerCapabilities sets up all orchestration-related capabilities
func (t *TravelResearchAgent) registerCapabilities() {
    // Track registered capabilities for debug logging
    registeredCaps := []string{}

    // Capability 1: Natural language orchestration
    t.RegisterCapability(core.Capability{
        Name:        "orchestrate_natural",  // <-- THIS IS THE PROBLEM
        Description: "Process natural language travel requests using AI-powered orchestration",
        Endpoint:    "/orchestrate/natural",
        InputTypes:  []string{"json", "text"},
        OutputTypes: []string{"json"},
        Handler:     t.handleNaturalOrchestration,
        InputSummary: &core.SchemaSummary{
            RequiredFields: []core.FieldHint{
                {
                    Name:        "request",
                    Type:        "string",
                    Example:     "I'm planning a trip to Tokyo...",
                    Description: "Natural language travel research request",
                },
            },
            // ...
        },
    })
    registeredCaps = append(registeredCaps, "orchestrate_natural")
    // ...
}
```

#### 2. Catalog Gets All Services (catalog.go:414-417)

```go
// getAllServices gets all services from discovery using the new Discover API
func (c *AgentCatalog) getAllServices(ctx context.Context) ([]*core.ServiceInfo, error) {
    // Use the new Discover method with empty filter to get all services
    return c.discovery.Discover(ctx, core.DiscoveryFilter{})  // <-- NO FILTERING
}
```

#### 3. FormatForLLM Includes Everything (catalog.go:450-479)

```go
// FormatForLLM formats the catalog for LLM consumption.
func (c *AgentCatalog) FormatForLLM() string {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var output string
    output = "Available Agents and Capabilities:\n\n"

    for id, agent := range c.agents {  // <-- Iterates ALL agents
        output += fmt.Sprintf("Agent: %s (ID: %s)\n", agent.Registration.Name, id)
        output += fmt.Sprintf("  Address: http://%s:%d\n", agent.Registration.Address, agent.Registration.Port)

        for _, cap := range agent.Capabilities {  // <-- ALL capabilities
            output += fmt.Sprintf("  - Capability: %s\n", cap.Name)
            // No filtering - orchestrate_natural is included
        }
    }
    return output
}
```

#### 4. Capability Provider Uses Full Catalog (capability_provider.go:38-41)

```go
// GetCapabilities returns all agents/tools formatted for LLM
func (d *DefaultCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    // Use existing catalog.FormatForLLM() method
    return d.catalog.FormatForLLM(), nil  // <-- NO EXCLUSIONS
}
```

### Why This Causes 400 Errors

When the LLM generates a plan with `orchestrate_natural`, it creates parameters like:

```json
{
  "step_id": "step-5",
  "agent_name": "travel-research-agent",
  "metadata": {
    "capability": "orchestrate_natural",
    "parameters": {
      "topic": "travel planning to Paris"  // WRONG - should be "request"
    }
  }
}
```

The executor calls `/orchestrate/natural` with these parameters, but the endpoint expects:

```json
{
  "request": "I'm planning a trip to Paris..."
}
```

This mismatch causes the 400 "request field is required" error.

---

## Proposed Fixes

### Option 1: Framework-Level Self-Exclusion (Recommended)

**Location:** `orchestration/capability_provider.go` and `orchestration/orchestrator.go`

**Approach:** Pass the orchestrator's own service name/ID to the capability provider, which filters it out.

```go
// orchestrator.go - Add self-identification
type OrchestratorConfig struct {
    // ... existing fields ...
    SelfServiceName string  // NEW: Identify self for filtering
}

// capability_provider.go - Add exclusion support
type DefaultCapabilityProvider struct {
    catalog         *AgentCatalog
    excludeServices []string  // NEW: Services to exclude from LLM prompt
}

func NewDefaultCapabilityProviderWithExclusions(catalog *AgentCatalog, exclude []string) *DefaultCapabilityProvider {
    return &DefaultCapabilityProvider{
        catalog:         catalog,
        excludeServices: exclude,
    }
}

func (d *DefaultCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    return d.catalog.FormatForLLMWithExclusions(d.excludeServices), nil
}
```

**Pros:**
- Generic solution that works for any orchestrator
- No changes required in example applications
- Prevents all orchestrators from self-referencing
- Follows GoMind's design principle of framework-level solutions

**Cons:**
- Requires framework change
- Need to propagate service identity through the stack

---

### Option 2: Capability Metadata Filtering

**Location:** `core/agent.go`, `orchestration/catalog.go`

**Approach:** Add metadata to capabilities to mark them as "internal" or "orchestration-type":

```go
// core/agent.go - Add Internal flag
type Capability struct {
    Name         string            `json:"name"`
    Description  string            `json:"description"`
    Endpoint     string            `json:"endpoint"`
    // ... existing fields ...
    Internal     bool              `json:"internal,omitempty"` // NEW: Exclude from LLM catalog
}

// catalog.go - FormatForLLM filters internal capabilities
func (c *AgentCatalog) FormatForLLM() string {
    // ...
    for _, cap := range agent.Capabilities {
        if cap.Internal {
            continue  // Skip internal capabilities
        }
        // ...
    }
    // ...
}
```

**Pros:**
- Flexible - any capability can be marked internal
- Clear semantic meaning
- Could be extended to other use cases (admin-only, deprecated, etc.)
- Application-controlled behavior

**Cons:**
- Requires changes to `core.Capability` struct (core module change)
- Application developers must remember to set the flag

---

### Option 3: Example-Level Fix (Quick Fix)

**Location:** `examples/agent-with-orchestration/research_agent.go`

**Approach:** Don't register `orchestrate_natural` as a capability - it's an internal endpoint.

```go
// research_agent.go - Remove orchestration capability registration
func (t *TravelResearchAgent) registerCapabilities() {
    // DON'T register orchestrate_natural
    // It's handled internally via HTTP routes, not meant for LLM planning

    // Only register workflow execution
    t.RegisterCapability(core.Capability{
        Name:        "execute_workflow",
        // ...
    })
}
```

The `/orchestrate/natural` endpoint would still be exposed via HTTP routes but wouldn't appear in the capability catalog.

**Pros:**
- Simplest fix, no framework changes
- Immediate effect
- Clear separation of "LLM-visible" vs "HTTP-only" endpoints

**Cons:**
- Per-example fix, doesn't prevent issue in other orchestrators
- Developers might make the same mistake again
- Endpoint is still callable via HTTP, just hidden from LLM

---

### Option 4: Prompt-Level Instruction (Least Invasive)

**Location:** `orchestration/default_prompt_builder.go`

**Approach:** Add explicit instruction to LLM to not use orchestration capabilities:

```go
const orchestrationWarning = `
IMPORTANT: Do NOT use capabilities that:
- Have "orchestrate" in the name
- Are described as "orchestration" or "AI-powered orchestration"
- Would create recursive planning requests

These are internal capabilities, not tools for execution.
`
```

**Pros:**
- No code changes to capability system
- Quick to implement
- Non-breaking change

**Cons:**
- Relies on LLM following instructions (not guaranteed)
- Doesn't address the root cause
- May fail with different LLM models or edge cases
- Increases prompt size

---

## Recommended Solution

**Option 1 (Framework-Level Self-Exclusion)** combined with **Option 2 (Capability Metadata)** for maximum flexibility:

### Phase 1: Immediate Fix (Option 1)

Add `excludeServices` to capability providers:

| File | Change |
|------|--------|
| `orchestration/capability_provider.go` | Add `excludeServices []string` field and `FormatForLLMWithExclusions` method |
| `orchestration/orchestrator.go` | Configure self-exclusion via `OrchestratorConfig.SelfServiceName` |
| `orchestration/catalog.go` | Add `FormatForLLMWithExclusions(exclude []string)` method |

### Phase 2: Long-term Solution (Option 2)

Add `Internal` flag to capabilities:

| File | Change |
|------|--------|
| `core/agent.go` | Add `Internal bool` field to `Capability` struct |
| `orchestration/catalog.go` | Filter internal capabilities in `FormatForLLM` |
| Example applications | Mark orchestration capabilities as `Internal: true` |

This provides:
- Automatic self-exclusion for orchestrators
- Explicit control for developers who want to hide specific capabilities
- Clean separation between "tools for execution" and "internal endpoints"

---

## Implementation Plan

### Files to Modify

| File | Changes Required |
|------|-----------------|
| `orchestration/capability_provider.go` | Add exclusion support |
| `orchestration/orchestrator.go` | Pass self-service-name to provider |
| `orchestration/catalog.go` | Add `FormatForLLMWithExclusions` method |
| `core/agent.go` | Add `Internal` field to `Capability` (Phase 2) |
| `examples/agent-with-orchestration/research_agent.go` | Mark capability as internal (Phase 2) |

### Testing Plan

1. **Unit Tests**
   - Test `FormatForLLMWithExclusions` filters correctly
   - Test empty exclusion list returns all capabilities
   - Test multiple exclusions work correctly

2. **Integration Tests**
   - Deploy orchestrator with self-exclusion
   - Send complex natural language request
   - Verify no self-referential calls in Jaeger traces
   - Verify execution completes without 400 errors

3. **Manual Verification**
   - Check LLM prompt doesn't contain `orchestrate_natural`
   - Verify other capabilities are still visible

---

## Impact Assessment

### Affected
- All orchestrator agents that register orchestration capabilities
- Multi-step orchestrations where LLM might choose to delegate to orchestrator
- Complex requests that benefit from recursive planning

### Not Affected
- Single-step orchestrations
- Tool-only agents (weather-tool, geocoding-tool, etc.)
- Workflows that don't trigger LLM planning

### Risk Level
- **Low** - The fix is additive (filtering) and doesn't change existing behavior for non-orchestrator agents

---

## References

- [TEMPLATE_SUBSTITUTION_BUG.md](./TEMPLATE_SUBSTITUTION_BUG.md) - Related template syntax issue
- [catalog.go](./catalog.go) - Catalog implementation
- [capability_provider.go](./capability_provider.go) - Capability provider implementation
- [research_agent.go](../examples/agent-with-orchestration/research_agent.go) - Example orchestrator
