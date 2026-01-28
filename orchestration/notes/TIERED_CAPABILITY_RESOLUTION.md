# Tiered Capability Resolution: LLM Token Cost Optimization

## Table of Contents

- [Implementation Status](#implementation-status)
- [Problem Statement](#problem-statement)
  - [Current Implementation](#current-implementation)
- [Proposed Solution: Tiered Capability Resolution](#proposed-solution-tiered-capability-resolution)
  - [Architecture Overview](#architecture-overview)
  - [Token Savings Analysis](#token-savings-analysis)
  - [Break-Even Analysis](#break-even-analysis)
- [Detailed Implementation Plan](#detailed-implementation-plan)
  - [Phase 1: New Interfaces and Types](#phase-1-new-interfaces-and-types)
  - [Phase 2: Implement TieredCapabilityProvider](#phase-2-implement-tieredcapabilityprovider)
  - [Phase 3: Extend AgentCatalog](#phase-3-extend-agentcatalog)
  - [Phase 4: Configuration Integration](#phase-4-configuration-integration)
  - [Phase 5: Metrics and Observability](#phase-5-metrics-and-observability)
- [Usage Examples](#usage-examples)
- [Migration Guide](#migration-guide)
- [Alternative Approaches Considered](#alternative-approaches-considered)
- [Future Enhancements](#future-enhancements)
- [Files to Create/Modify](#files-to-createmodify)
- [Acceptance Criteria](#acceptance-criteria)
- [Research-Backed Analysis (Updated January 2026)](#research-backed-analysis-updated-january-2026)
  - [Your Current Deployment Analysis](#your-current-deployment-analysis)
  - [Key Research Finding #1: RAG-MCP Framework](#key-research-finding-1-rag-mcp-framework-may-2025)
  - [Key Research Finding #2: MCP Hierarchical Tool Management](#key-research-finding-2-mcp-hierarchical-tool-management-november-2025)
  - [Key Research Finding #3: Guided-Structured Templates](#key-research-finding-3-guided-structured-templates-september-2025)
  - [Key Research Finding #4: "Less is More"](#key-research-finding-4-less-is-more---tool-count-degradation)
  - [Key Research Finding #5: Input Tokens and Latency](#key-research-finding-5-input-tokens-and-latency)
  - [Key Research Finding #6: Fast Model Benchmarks](#key-research-finding-6-fast-model-benchmarks-january-2026)
  - [Key Research Finding #7: LLM-Based Agents Survey](#key-research-finding-7-llm-based-agents-survey-june-2025)
  - [Latency Impact Analysis](#latency-impact-analysis)
  - [Research-Backed Recommendations](#research-backed-recommendations)
  - [Why Two-Step Beats Single-Step (Summary)](#why-two-step-beats-single-step-summary)
  - [Industry Adoption (2025)](#industry-adoption-2025)
- [Developer Customization: Orchestration Persona & Agent Specialization](#developer-customization-orchestration-persona--agent-specialization)
  - [Research: How Major Frameworks Solve This](#research-how-major-frameworks-solve-this)
  - [Current GoMind Customization Mechanisms](#current-gomind-customization-mechanisms)
  - [Recommended Enhancement: Keep It Simple](#recommended-enhancement-keep-it-simple)
  - [Usage Examples](#usage-examples-1)
  - [Why This Approach?](#why-this-approach)
  - [What About Agent Specialization?](#what-about-agent-specialization)
- [Framework Design Principles Compliance](#framework-design-principles-compliance)
- [Orchestration ARCHITECTURE.md Compliance](#orchestration-architecturemd-compliance)
- [Core Module Design Principles Compliance](#core-module-design-principles-compliance)
- [AI Module ARCHITECTURE.md Compliance](#ai-module-architecturemd-compliance)
- [Telemetry Module ARCHITECTURE.md Compliance](#telemetry-module-architecturemd-compliance)
- [Distributed Tracing Guide Compliance](#distributed-tracing-guide-compliance)
- [Logging Implementation Guide Compliance](#logging-implementation-guide-compliance)
- [LLM Debug Payload Design Compliance](#llm-debug-payload-design-compliance)
- [Files to Create/Modify - Complete Summary](#files-to-createmodify---complete-summary)
- [References](#references)

---

## Implementation Status

> **Last Updated**: January 2026

| Phase | Description | Status |
|-------|-------------|--------|
| **Phase 1** | New Interfaces and Types (`CapabilitySummary`, `Summary` field) | ✅ **Completed** |
| **Phase 2** | Implement `TieredCapabilityProvider` | ✅ **Completed** |
| **Phase 3** | Extend `AgentCatalog` (`GetCapabilitySummaries`, `FormatToolsForLLM`) | ✅ **Completed** |
| **Phase 4** | Configuration Integration (`EnableTieredResolution`, factory wiring) | ✅ **Completed** |
| **Phase 5** | Metrics and Observability | ✅ **Completed** |
| **Developer Customization** | `SystemInstructions` field for orchestration persona | ✅ **Completed** |

### Implementation Files

| File | Status | Notes |
|------|--------|-------|
| `orchestration/tiered_capability_provider.go` | ✅ Created | Core tiered resolution logic |
| `orchestration/tiered_capability_provider_test.go` | ✅ Created | Comprehensive unit tests |
| `orchestration/catalog.go` | ✅ Modified | Added `Summary`, `GetCapabilitySummaries()`, `FormatToolsForLLM()` |
| `orchestration/interfaces.go` | ✅ Modified | Added `EnableTieredResolution`, `TieredCapabilityConfig` |
| `orchestration/factory.go` | ✅ Modified | Wired up `TieredCapabilityProvider` |
| `orchestration/prompt_builder.go` | ✅ Modified | Added `SystemInstructions` field |
| `orchestration/default_prompt_builder.go` | ✅ Modified | Added `buildPersonaSection()` method |
| `orchestration/prompt_builder_test.go` | ✅ Modified | Added `SystemInstructions` tests |

---

## Problem Statement

The orchestration module currently sends **all tool/agent capabilities** to the LLM during plan generation. This approach has significant cost implications:

| Tool Count | Capability Tokens (approx) | Total Plan Gen Tokens |
|------------|---------------------------|----------------------|
| 10 tools   | ~2,000                    | ~5,000               |
| 25 tools   | ~5,000                    | ~8,000               |
| 50 tools   | ~10,000                   | ~13,000              |
| 100 tools  | ~20,000                   | ~23,000              |

**Cost scales linearly with tool count**, making large deployments expensive.

### Current Implementation

The `DefaultCapabilityProvider` in [`orchestration/capability_provider.go`](../capability_provider.go) sends all capabilities:

```go
// GetCapabilities returns all agents/tools formatted for LLM
func (d *DefaultCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    return d.catalog.FormatForLLM(), nil  // Returns EVERYTHING
}
```

The `AgentCatalog.FormatForLLM()` in [`orchestration/catalog.go`](../catalog.go) formats each capability with:
- Agent name, ID, address
- Capability name and description
- All parameters with types, required flags, descriptions
- Return type information

This produces verbose output like:
```
Agent: weather-tool (ID: weather-tool-abc123)
  Address: http://10.0.0.5:8080
  - Capability: get_weather
    Description: Get current weather conditions for a location
    Parameters:
      - location: string (required) - City name or coordinates
      - units: string - Temperature units (celsius, fahrenheit)
    Returns: object - Weather data including temperature, conditions, humidity
```

---

## Proposed Solution: Tiered Capability Resolution

A **3-tier approach** that dramatically reduces token usage while maintaining orchestration accuracy.

> **Default Behavior**: Tiered resolution is **enabled by default** for optimal token usage. Set `GOMIND_TIERED_RESOLUTION_ENABLED=false` to disable if needed for edge cases (e.g., deployments with < 15 tools).

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      User Request                                │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tier 1: Tool Selection (Lightweight)                           │
│  ─────────────────────────────────────                          │
│  • Send only: tool names + 1-sentence descriptions              │
│  • ~50-100 tokens per tool (vs 200-500 for full schema)         │
│  • Use fast/cheap model (e.g., gemini-2.0-flash, gpt-4o-mini)   │
│  • Output: JSON array of needed tool names                      │
│  • Token cost: ~1,000-2,000 total                               │
└───────────────────────────┬─────────────────────────────────────┘
                            │ ["weather-tool", "currency-tool"]
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tier 2: Schema Retrieval (Targeted)                            │
│  ────────────────────────────────────                           │
│  • Fetch full capability schemas ONLY for selected tools        │
│  • No LLM call - just catalog lookup                            │
│  • Token cost: 0 (local operation)                              │
└───────────────────────────┬─────────────────────────────────────┘
                            │ Full schemas for 3 tools
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Tier 3: Plan Generation (Existing Logic)                       │
│  ────────────────────────────────────────                       │
│  • Generate execution plan with full parameter details          │
│  • Same as current implementation                               │
│  • Token cost: ~3,000-4,000 (reduced from ~13,000)              │
└─────────────────────────────────────────────────────────────────┘
```

### Token Savings Analysis

**Scenario: 50 tools, user needs 3 for their request**

| Approach | Tier 1 Tokens | Tier 3 Tokens | Total | Savings |
|----------|--------------|---------------|-------|---------|
| Current (all tools) | N/A | ~13,000 | ~13,000 | - |
| Tiered | ~2,500 | ~4,000 | ~6,500 | **50%** |

**Scenario: 100 tools, user needs 4 for their request**

| Approach | Tier 1 Tokens | Tier 3 Tokens | Total | Savings |
|----------|--------------|---------------|-------|---------|
| Current (all tools) | N/A | ~23,000 | ~23,000 | - |
| Tiered | ~4,000 | ~4,500 | ~8,500 | **63%** |

### Break-Even Analysis

The tiered approach adds one LLM call (~1,500-2,500 tokens). It's beneficial when:

```
Total_tools × tokens_per_full_schema >
    (Total_tools × tokens_per_summary) + (Selected_tools × tokens_per_full_schema) + overhead
```

**Rule of thumb**: Beneficial when `Total_tools > 15-20` tools.

---

## Detailed Implementation Plan

### Phase 1: New Interfaces and Types

#### 1.1 Add `CapabilitySummary` Type

**File**: `orchestration/catalog.go`
**Location**: After line 78 (after `Example` struct)

Add this new type:

```go
// CapabilitySummary is a lightweight representation of a capability for Tier 1 selection.
// It contains only the essential information needed for an LLM to decide if a tool
// is relevant to a user's request, without the full parameter schemas.
type CapabilitySummary struct {
    // AgentName is the service name (used in execution plans)
    AgentName string `json:"agent_name"`

    // CapabilityName is the specific capability identifier
    CapabilityName string `json:"capability_name"`

    // Summary is a 1-2 sentence description of what this capability does
    // This should be concise but informative enough for tool selection
    Summary string `json:"summary"`

    // Tags are optional categorization labels (e.g., "weather", "finance", "location")
    Tags []string `json:"tags,omitempty"`
}
```

#### 1.2 Extend `EnhancedCapability` with Summary Field

**File**: `orchestration/catalog.go`
**Location**: Lines 40-54 (`EnhancedCapability` struct)
**Insert after**: Line 53 (after `Internal bool` field)

Modify the struct to add `Summary` field and `GetSummary()` method:

```go
type EnhancedCapability struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Endpoint    string      `json:"endpoint"`
    Parameters  []Parameter `json:"parameters"`
    Returns     ReturnType  `json:"returns"`
    Tags        []string    `json:"tags"`
    Examples    []Example   `json:"examples,omitempty"`
    Internal    bool        `json:"internal,omitempty"`

    // Summary is a pre-computed 1-2 sentence description for Tier 1 selection.
    // If empty, GetSummary() auto-generates from Description.
    // Tools can set this explicitly for better selection accuracy.
    Summary string `json:"summary,omitempty"`
}

// GetSummary returns the summary for Tier 1 selection.
// If Summary is explicitly set, returns it. Otherwise, auto-generates
// from the first 1-2 sentences of Description.
func (c *EnhancedCapability) GetSummary() string {
    if c.Summary != "" {
        return c.Summary
    }
    return extractFirstSentences(c.Description, 2)
}
```

#### 1.3 Add `TieredCapabilityProvider` Interface

**File**: `orchestration/tiered_capability_provider.go` **(NEW FILE)**

Create this new file with the following content:

```go
package orchestration

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/itsneelabh/gomind/core"
    "github.com/itsneelabh/gomind/telemetry"
)

// TieredCapabilityProvider implements a two-phase capability resolution strategy
// that significantly reduces LLM token usage for large tool deployments.
//
// Research basis:
// - RAG-MCP (May 2025): 74.8% token reduction, 62.1% faster, 3.2x accuracy
// - Less is More (Nov 2024): Accuracy degrades beyond ~20 tools
// - Guided-Structured Templates (Sept 2025): 3-12% improvement with structured prompts
//
// Phase 1 (Tier 1): Send lightweight summaries to LLM for tool selection
// Phase 2 (Tier 2): Retrieve full schemas only for selected tools
//
// This approach reduces token usage by 50-75% for deployments with 20+ tools.
type TieredCapabilityProvider struct {
    catalog  *AgentCatalog
    aiClient core.AIClient

    // MinToolsForTiering is the minimum tool count to trigger tiered resolution.
    // Below this threshold, sends all tools directly (simpler, one LLM call).
    // Research shows degradation starts at ~20 tools (Less is More, Nov 2024)
    // Default: 20
    MinToolsForTiering int

    // Logger for observability
    logger core.Logger

    // Telemetry for metrics
    telemetry core.Telemetry

    // LLM Debug Store integration (per LLM_DEBUG_PAYLOAD_DESIGN.md)
    debugStore LLMDebugStore     // For recording LLM interactions
    debugWg    sync.WaitGroup    // Tracks in-flight debug recordings for graceful shutdown
    debugSeqID atomic.Uint64     // For generating unique fallback IDs when TraceID is empty
}

// TieredCapabilityConfig holds configuration for the tiered provider
type TieredCapabilityConfig struct {
    // MinToolsForTiering threshold (default: 20)
    // Precedence: Explicit config → GOMIND_TIERED_MIN_TOOLS → 20
    // Research: "Less is More" (Nov 2024) shows degradation at ~20 tools
    MinToolsForTiering int `json:"min_tools_for_tiering"`
}
```

### Phase 2: Implement `TieredCapabilityProvider`

#### 2.1 Constructor

```go
// Environment variable constant for tiered resolution
const (
    // EnvTieredMinTools overrides the minimum tool count to trigger tiering.
    // Example: GOMIND_TIERED_MIN_TOOLS=15
    EnvTieredMinTools = "GOMIND_TIERED_MIN_TOOLS"
)

// NewTieredCapabilityProvider creates a provider with intelligent tiered resolution.
// Configuration precedence: Explicit config → GOMIND_TIERED_MIN_TOOLS → 20
// Both tiers use the AI client's default model for simplicity.
func NewTieredCapabilityProvider(
    catalog *AgentCatalog,
    aiClient core.AIClient,
    config *TieredCapabilityConfig,
) *TieredCapabilityProvider {
    if config == nil {
        config = &TieredCapabilityConfig{}
    }

    // Resolve MinToolsForTiering with environment variable fallback
    // Precedence: Explicit config → GOMIND_TIERED_MIN_TOOLS → 20
    minTools := config.MinToolsForTiering
    if minTools == 0 {
        if envVal := os.Getenv(EnvTieredMinTools); envVal != "" {
            if parsed, err := strconv.Atoi(envVal); err == nil && parsed > 0 {
                minTools = parsed
            }
        }
    }
    if minTools == 0 {
        minTools = 20 // Research-backed default
    }

    return &TieredCapabilityProvider{
        catalog:            catalog,
        aiClient:           aiClient,
        MinToolsForTiering: minTools,
    }
}

// SetLogger sets the logger for observability
func (t *TieredCapabilityProvider) SetLogger(logger core.Logger) {
    if logger != nil {
        if cal, ok := logger.(core.ComponentAwareLogger); ok {
            t.logger = cal.WithComponent("framework/orchestration/tiered")
        } else {
            t.logger = logger
        }
    }
}

// SetTelemetry sets the telemetry provider for metrics
func (t *TieredCapabilityProvider) SetTelemetry(telemetry core.Telemetry) {
    t.telemetry = telemetry
}

// SetLLMDebugStore sets the debug store for recording LLM interactions.
// Per LLM_DEBUG_PAYLOAD_DESIGN.md, this enables recording of tiered_selection calls.
func (t *TieredCapabilityProvider) SetLLMDebugStore(store LLMDebugStore) {
    t.debugStore = store
}

// GetLLMDebugStore returns the debug store (for testing/inspection)
func (t *TieredCapabilityProvider) GetLLMDebugStore() LLMDebugStore {
    return t.debugStore
}

// recordDebugInteraction stores an LLM interaction for debugging.
// Uses WaitGroup to ensure graceful shutdown waits for pending recordings.
// Per LLM_DEBUG_PAYLOAD_DESIGN.md section 4.6 Lifecycle Management.
func (t *TieredCapabilityProvider) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if t.debugStore == nil {
        return
    }

    // GetTraceContext is nil-safe, returns empty TraceID if no span
    tc := telemetry.GetTraceContext(ctx)
    requestID := tc.TraceID
    if requestID == "" {
        // Generate unique fallback ID using atomic counter (collision-safe)
        seq := t.debugSeqID.Add(1)
        requestID = fmt.Sprintf("tiered-no-trace-%d-%d", time.Now().Unix(), seq)
    }

    t.debugWg.Add(1)
    go func() {
        defer t.debugWg.Done()

        // Use background context to allow completion even if request ctx is cancelled
        recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        if err := t.debugStore.RecordInteraction(recordCtx, requestID, interaction); err != nil {
            if t.logger != nil {
                t.logger.Warn("Failed to record tiered_selection debug interaction", map[string]interface{}{
                    "operation":   "llm_debug_record",
                    "request_id":  requestID,
                    "type":        interaction.Type,
                    "error":       err.Error(),
                })
            }
        }
    }()
}

// Shutdown waits for pending debug recordings with a timeout.
// Should be called during graceful shutdown to ensure no data loss.
func (t *TieredCapabilityProvider) Shutdown(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        t.debugWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        if t.logger != nil {
            t.logger.Warn("TieredCapabilityProvider shutdown timeout: some debug recordings may be lost", map[string]interface{}{
                "operation": "tiered_provider_shutdown",
            })
        }
        return fmt.Errorf("tiered provider shutdown timeout: %w", ctx.Err())
    }
}
```

#### 2.2 Main `GetCapabilities` Method

```go
// GetCapabilities implements CapabilityProvider with tiered resolution.
// It automatically chooses between direct (all tools) and tiered based on tool count.
func (t *TieredCapabilityProvider) GetCapabilities(
    ctx context.Context,
    request string,
    metadata map[string]interface{},
) (string, error) {
    // Get all capability summaries
    summaries := t.catalog.GetCapabilitySummaries()

    // Check if tiering is beneficial
    if len(summaries) < t.MinToolsForTiering {
        // Below threshold - use direct approach (simpler, one LLM call)
        if t.logger != nil {
            t.logger.DebugWithContext(ctx, "Below tiering threshold, using direct approach", map[string]interface{}{
                "tool_count": len(summaries),
                "threshold":  t.MinToolsForTiering,
            })
        }
        return t.catalog.FormatForLLM(), nil
    }

    // Tier 1: Select relevant tools using lightweight summaries
    selectedTools, err := t.selectRelevantTools(ctx, request, summaries)
    if err != nil {
        // Fallback to direct approach on selection failure
        if t.logger != nil {
            t.logger.WarnWithContext(ctx, "Tool selection failed, falling back to direct approach", map[string]interface{}{
                "error": err.Error(),
            })
        }
        return t.catalog.FormatForLLM(), nil
    }

    if t.logger != nil {
        t.logger.InfoWithContext(ctx, "Tier 1 tool selection complete", map[string]interface{}{
            "total_tools":    len(summaries),
            "selected_tools": selectedTools,
            "reduction":      fmt.Sprintf("%.1f%%", (1-float64(len(selectedTools))/float64(len(summaries)))*100),
        })
    }

    // Tier 2: Get full schemas for selected tools only
    return t.catalog.FormatToolsForLLM(selectedTools), nil
}
```

#### 2.3 Tier 1: Tool Selection

```go
// selectRelevantTools uses an LLM call to identify which tools are needed.
// Uses structured prompting (Guided-Structured Templates, Sept 2025) and validates
// results to filter hallucinated tools (RAG-MCP, May 2025).
// Records LLM interaction to debug store per LLM_DEBUG_PAYLOAD_DESIGN.md.
// Uses AI client's default model - cost savings come from reduced token counts.
func (t *TieredCapabilityProvider) selectRelevantTools(
    ctx context.Context,
    request string,
    summaries []CapabilitySummary,
) ([]string, error) {
    // Build the selection prompt using structured template
    prompt := t.buildSelectionPrompt(summaries, request)

    // Use deterministic settings for tool selection
    options := &core.AIOptions{
        Temperature: 0.0,   // Deterministic selection
        MaxTokens:   500,   // Small output (just a list)
    }
    // Uses AI client's default model - no override needed

    // Capture timing for LLM debug recording
    llmStartTime := time.Now()

    // Make the LLM call
    response, err := t.aiClient.GenerateResponse(ctx, prompt, options)
    llmDuration := time.Since(llmStartTime)

    // LLM Debug: Record interaction (success or failure)
    // Per LLM_DEBUG_PAYLOAD_DESIGN.md - this is the 7th recording site: "tiered_selection"
    if err != nil {
        // Record failed selection attempt
        t.recordDebugInteraction(ctx, LLMInteraction{
            Type:        "tiered_selection",
            Timestamp:   llmStartTime,
            DurationMs:  llmDuration.Milliseconds(),
            Prompt:      prompt,
            Temperature: 0.0,
            MaxTokens:   500,
            Success:     false,
            Error:       err.Error(),
            Attempt:     1,
        })
        return nil, fmt.Errorf("tool selection failed: %w", err)
    }

    // Record successful selection
    t.recordDebugInteraction(ctx, LLMInteraction{
        Type:             "tiered_selection",
        Timestamp:        llmStartTime,
        DurationMs:       llmDuration.Milliseconds(),
        Prompt:           prompt,
        Temperature:      0.0,
        MaxTokens:        500,
        Model:            response.Model,
        Provider:         response.Provider,
        Response:         response.Content,
        PromptTokens:     response.Usage.PromptTokens,
        CompletionTokens: response.Usage.CompletionTokens,
        TotalTokens:      response.Usage.TotalTokens,
        Success:          true,
        Attempt:          1,
    })

    // Parse the response
    selectedTools, err := t.parseToolSelection(response.Content)
    if err != nil {
        return nil, err
    }

    // Validate and filter to prevent hallucinated tool names
    // RAG-MCP research: "model often picks the wrong one or makes up fake tools"
    validatedTools := t.validateAndFilterTools(ctx, selectedTools, summaries)

    if len(validatedTools) == 0 {
        return nil, fmt.Errorf("no valid tools after filtering (all selections were hallucinated)")
    }

    return validatedTools, nil
}

// buildSelectionPrompt creates the Tier 1 prompt with tool summaries.
// Uses structured template approach based on "Guided-Structured Templates" research (Sept 2025)
// which shows 3-12% accuracy improvement over free-form prompts.
func (t *TieredCapabilityProvider) buildSelectionPrompt(
    summaries []CapabilitySummary,
    request string,
) string {
    var sb strings.Builder

    // Structured template following research recommendations:
    // identification → relevancy decision → dependency analysis → selection
    sb.WriteString(`You are a tool selector. Follow this structured process to select tools.

## STEP 1: TASK IDENTIFICATION
Analyze what the user wants to accomplish. Break down the request into discrete sub-tasks.

## STEP 2: AVAILABLE TOOLS
`)

    // Format each tool summary (compact format)
    for _, s := range summaries {
        sb.WriteString(fmt.Sprintf("- %s/%s: %s\n", s.AgentName, s.CapabilityName, s.Summary))
    }

    sb.WriteString(fmt.Sprintf(`
## STEP 3: USER REQUEST
%s

## STEP 4: STRUCTURED SELECTION PROCESS

Think through these questions (but only output the final JSON):

A. PRIMARY TOOLS: Which tools directly address the user's explicit requests?
   - What information is the user explicitly asking for?
   - Which tools provide that information?

B. DATA DEPENDENCY TOOLS: What intermediate data is needed?
   - Do any selected tools require input from other tools?
   - Example: Weather tools often need coordinates → need geocoding tool
   - Example: Currency conversion needs currency codes → need country-info tool

C. COMPLETENESS CHECK: Review each part of the request
   - Is every aspect of the user's request covered?
   - Are all data dependencies satisfied?

## OUTPUT FORMAT
Return ONLY a JSON array of tool identifiers. Format: "agent_name/capability_name"
Example: ["stock-service/stock_quote", "country-info-tool/get_country_info", "currency-tool/convert_currency"]

JSON array (no explanation):
`, request))

    return sb.String()
}

// parseToolSelection extracts tool names from the LLM response.
func (t *TieredCapabilityProvider) parseToolSelection(response string) ([]string, error) {
    // Clean up response (handle markdown wrapping)
    response = strings.TrimSpace(response)
    response = strings.TrimPrefix(response, "```json")
    response = strings.TrimPrefix(response, "```")
    response = strings.TrimSuffix(response, "```")
    response = strings.TrimSpace(response)

    // Parse JSON array
    var tools []string
    if err := json.Unmarshal([]byte(response), &tools); err != nil {
        return nil, fmt.Errorf("failed to parse tool selection: %w (response: %s)", err, response)
    }

    if len(tools) == 0 {
        return nil, fmt.Errorf("no tools selected")
    }

    return tools, nil
}

// validateAndFilterTools verifies selected tools exist in the catalog.
// Returns only valid tools and logs warnings for invalid selections.
// This prevents hallucinated tool names (a known issue per RAG-MCP research).
func (t *TieredCapabilityProvider) validateAndFilterTools(
    ctx context.Context,
    selectedTools []string,
    summaries []CapabilitySummary,
) []string {
    // Build lookup set of valid tool IDs
    validTools := make(map[string]bool)
    for _, s := range summaries {
        toolID := fmt.Sprintf("%s/%s", s.AgentName, s.CapabilityName)
        validTools[toolID] = true
    }

    // Filter to only valid tools
    var filtered []string
    var invalid []string
    for _, tool := range selectedTools {
        if validTools[tool] {
            filtered = append(filtered, tool)
        } else {
            invalid = append(invalid, tool)
        }
    }

    // Log any hallucinated tools (research shows this is common with many tools)
    if len(invalid) > 0 && t.logger != nil {
        t.logger.WarnWithContext(ctx, "LLM selected non-existent tools (hallucination)", map[string]interface{}{
            "invalid_tools": invalid,
            "valid_count":   len(filtered),
        })
    }

    return filtered
}
```

### Phase 3: Extend `AgentCatalog`

**File**: `orchestration/catalog.go`

Add these methods after the existing `FormatForLLM()` method (after line 520):

#### 3.1 Get Summaries

```go
// GetCapabilitySummaries returns lightweight summaries of all capabilities.
// This is used by TieredCapabilityProvider for Tier 1 tool selection.
func (c *AgentCatalog) GetCapabilitySummaries() []CapabilitySummary {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var summaries []CapabilitySummary

    for _, agent := range c.agents {
        for _, cap := range agent.Capabilities {
            // Skip internal capabilities
            if cap.Internal {
                continue
            }

            summaries = append(summaries, CapabilitySummary{
                AgentName:      agent.Registration.Name,
                CapabilityName: cap.Name,
                Summary:        cap.GetSummary(),
                Tags:           cap.Tags,
            })
        }
    }

    return summaries
}

// GetToolCount returns the total number of public capabilities.
// Used to determine if tiered resolution should be used.
func (c *AgentCatalog) GetToolCount() int {
    c.mu.RLock()
    defer c.mu.RUnlock()

    count := 0
    for _, agent := range c.agents {
        for _, cap := range agent.Capabilities {
            if !cap.Internal {
                count++
            }
        }
    }
    return count
}
```

#### 3.2 Format Selected Tools

```go
// FormatToolsForLLM formats only the specified tools for LLM consumption.
// Tool identifiers are in format "agent_name/capability_name".
// This is used by TieredCapabilityProvider for Tier 2 schema retrieval.
func (c *AgentCatalog) FormatToolsForLLM(toolIDs []string) string {
    c.mu.RLock()
    defer c.mu.RUnlock()

    // Build lookup set for O(1) checking
    toolSet := make(map[string]bool)
    for _, id := range toolIDs {
        toolSet[id] = true
    }

    var output strings.Builder
    output.WriteString("Available Agents and Capabilities:\n\n")

    for id, agent := range c.agents {
        // Collect capabilities that match the selection
        var matchingCaps []EnhancedCapability
        for _, cap := range agent.Capabilities {
            if cap.Internal {
                continue
            }

            toolID := fmt.Sprintf("%s/%s", agent.Registration.Name, cap.Name)
            if toolSet[toolID] {
                matchingCaps = append(matchingCaps, cap)
            }
        }

        // Skip agents with no matching capabilities
        if len(matchingCaps) == 0 {
            continue
        }

        // Format this agent and its matching capabilities
        output.WriteString(fmt.Sprintf("Agent: %s (ID: %s)\n", agent.Registration.Name, id))
        output.WriteString(fmt.Sprintf("  Address: http://%s:%d\n", agent.Registration.Address, agent.Registration.Port))

        for _, cap := range matchingCaps {
            output.WriteString(fmt.Sprintf("  - Capability: %s\n", cap.Name))
            output.WriteString(fmt.Sprintf("    Description: %s\n", cap.Description))

            if len(cap.Parameters) > 0 {
                output.WriteString("    Parameters:\n")
                for _, param := range cap.Parameters {
                    required := ""
                    if param.Required {
                        required = " (required)"
                    }
                    output.WriteString(fmt.Sprintf("      - %s: %s%s - %s\n",
                        param.Name, param.Type, required, param.Description))
                }
            }

            if cap.Returns.Type != "" {
                output.WriteString(fmt.Sprintf("    Returns: %s - %s\n",
                    cap.Returns.Type, cap.Returns.Description))
            }
        }
        output.WriteString("\n")
    }

    return output.String()
}
```

#### 3.3 Helper Function

```go
// extractFirstSentences extracts the first N sentences from text.
// Used for auto-generating capability summaries.
func extractFirstSentences(text string, n int) string {
    if text == "" {
        return ""
    }

    // Simple sentence detection (handles ., !, ?)
    sentences := 0
    for i, r := range text {
        if r == '.' || r == '!' || r == '?' {
            sentences++
            if sentences >= n {
                return strings.TrimSpace(text[:i+1])
            }
        }
    }

    // Text has fewer sentences than requested
    return strings.TrimSpace(text)
}
```

### Phase 4: Configuration Integration

#### Environment Variables

The tiered resolution feature supports configuration via environment variables.

| Environment Variable | Type | Default | Acceptable Values | Description |
|---------------------|------|---------|-------------------|-------------|
| `GOMIND_TIERED_RESOLUTION_ENABLED` | bool | `true` | `true`, `false` (case-insensitive) | Enable/disable tiered capability resolution. Set to `false` for deployments with < 15 tools. |
| `GOMIND_TIERED_MIN_TOOLS` | int | `20` | Positive integer (recommended: 15-50) | Minimum tool count to trigger tiered resolution. Below this threshold, all tools are sent directly. Research suggests 20 is optimal. |

**Why these defaults?**
- **`GOMIND_TIERED_RESOLUTION_ENABLED=true`**: Research shows tiered resolution improves accuracy and reduces costs for most deployments
- **`GOMIND_TIERED_MIN_TOOLS=20`**: Based on "Less is More" (Nov 2024) research showing LLM accuracy degradation starts at ~20 tools

**Model Selection**: Both tiers use the AI client's default model. Cost savings come from **reduced token counts** (tool summaries vs full schemas), not from using different models.

**Configuration Precedence** (per FRAMEWORK_DESIGN_PRINCIPLES.md):
```
Explicit Code Config → Environment Variable → Default
```

**Example: Kubernetes ConfigMap**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: travel-agent-config
data:
  GOMIND_TIERED_RESOLUTION_ENABLED: "true"
  GOMIND_TIERED_MIN_TOOLS: "15"
```

**Example: Local Development**
```bash
export GOMIND_TIERED_RESOLUTION_ENABLED=true
export GOMIND_TIERED_MIN_TOOLS=20
```

**Example: Disable for Small Deployments**
```bash
# For deployments with < 15 tools, disable tiered resolution
export GOMIND_TIERED_RESOLUTION_ENABLED=false
```

#### 4.1 Extend `OrchestratorConfig`

**File**: `orchestration/interfaces.go`
**Location**: Lines 259-306 (`OrchestratorConfig` struct)
**Insert after**: Line 295 (after `SemanticRetry` field)

Add these fields to `OrchestratorConfig`:

```go
// orchestration/interfaces.go - Add after line 295 (inside OrchestratorConfig)

    // Tiered Capability Resolution (token optimization)
    // When enabled, uses a two-phase approach to reduce LLM token usage:
    // Phase 1: Send lightweight tool summaries for selection
    // Phase 2: Send full schemas only for selected tools
    EnableTieredResolution bool                   `json:"enable_tiered_resolution"`
    TieredResolution       TieredCapabilityConfig `json:"tiered_resolution,omitempty"`
```

**Also add** `TieredCapabilityConfig` struct after line 325 (after `SemanticRetryConfig`):

```go
// orchestration/interfaces.go - Add after line 325 (after SemanticRetryConfig struct)

// TieredCapabilityConfig holds configuration for tiered capability resolution
type TieredCapabilityConfig struct {
    // MinToolsForTiering is the minimum tool count to trigger tiered resolution.
    // Below this threshold, sends all tools directly (simpler, one LLM call).
    // Default: 20 | Env: GOMIND_TIERED_MIN_TOOLS
    MinToolsForTiering int `json:"min_tools_for_tiering,omitempty"`
}
```

#### 4.2 Update `DefaultConfig()`

```go
func DefaultConfig() *OrchestratorConfig {
    config := &OrchestratorConfig{
        // ... existing defaults ...

        // Tiered Resolution defaults (enabled by default for token optimization)
        // Research: "Less is More" (Nov 2024) shows degradation at ~20 tools
        EnableTieredResolution: true,
        TieredResolution: TieredCapabilityConfig{
            MinToolsForTiering: 20,
        },
    }

    // Environment variable configuration (allows disabling in edge cases)
    if enabled := os.Getenv("GOMIND_TIERED_RESOLUTION_ENABLED"); enabled != "" {
        config.EnableTieredResolution = strings.ToLower(enabled) == "true"
    }
    if minTools := os.Getenv("GOMIND_TIERED_MIN_TOOLS"); minTools != "" {
        if val, err := strconv.Atoi(minTools); err == nil && val > 0 {
            config.TieredResolution.MinToolsForTiering = val
        }
    }

    return config
}
```

#### 4.3 Update Factory

**File**: `orchestration/factory.go`
**Location**: `CreateOrchestrator` function
**Insert after**: Line 158 (after prompt builder initialization, before error analyzer setup)

Add this capability provider initialization block:

```go
// orchestration/factory.go - Add after line 158 (after prompt builder setup)
// Insert this block BEFORE the error analyzer setup (line 160)

    // Initialize TieredCapabilityProvider if enabled (token optimization)
    // Priority: 1) Tiered (if enabled), 2) Service (if configured), 3) Default
    var tieredProvider *TieredCapabilityProvider
    if config.EnableTieredResolution {
        tieredProvider = NewTieredCapabilityProvider(
            orchestrator.catalog,
            deps.AIClient,
            &config.TieredResolution,
        )
        tieredProvider.SetLogger(deps.Logger)
        tieredProvider.SetTelemetry(deps.Telemetry)
        orchestrator.SetCapabilityProvider(tieredProvider)

        factoryLogger.Info("Using TieredCapabilityProvider for token optimization", map[string]interface{}{
            "operation":  "capability_provider_initialization",
            "min_tools":  config.TieredResolution.MinToolsForTiering,
            "enabled":    true,
        })
    } else {
        factoryLogger.Debug("TieredCapabilityProvider disabled, using default capability provider", map[string]interface{}{
            "operation": "capability_provider_initialization",
            "enabled":   false,
        })
    }
```

**Also update** the LLM Debug Store section (around line 204) to propagate the store to TieredCapabilityProvider:

```go
// orchestration/factory.go - Update LLM Debug Store section (after line 204)
// Add after: orchestrator.SetLLMDebugStore(config.LLMDebugStore)

        orchestrator.SetLLMDebugStore(config.LLMDebugStore)

        // Propagate LLM debug store to TieredCapabilityProvider for tiered_selection recording
        if tieredProvider != nil {
            tieredProvider.SetLLMDebugStore(config.LLMDebugStore)
        }
    }
```

**Complete integration context** (showing where the new code fits):

```go
// orchestration/factory.go - CreateOrchestrator function

func CreateOrchestrator(config *OrchestratorConfig, deps OrchestratorDependencies) (*AIOrchestrator, error) {
    // ... existing setup code (lines 36-158) ...

    // === NEW: TieredCapabilityProvider initialization ===
    var tieredProvider *TieredCapabilityProvider
    if config.EnableTieredResolution {
        tieredProvider = NewTieredCapabilityProvider(
            orchestrator.catalog,
            deps.AIClient,
            &config.TieredResolution,
        )
        tieredProvider.SetLogger(deps.Logger)
        tieredProvider.SetTelemetry(deps.Telemetry)
        orchestrator.SetCapabilityProvider(tieredProvider)

        factoryLogger.Info("Using TieredCapabilityProvider for token optimization", map[string]interface{}{
            "operation": "capability_provider_initialization",
            "min_tools": config.TieredResolution.MinToolsForTiering,
            "enabled":   true,
        })
    }
    // === END NEW ===

    // Configure LLM-based error analyzer if enabled (existing code - line 160)
    if deps.EnableErrorAnalyzer && deps.AIClient != nil {
        // ... existing error analyzer code ...
    }

    // Initialize LLM Debug Store if enabled (existing code - line 170)
    if config.LLMDebug.Enabled {
        // ... existing debug store setup ...

        orchestrator.SetLLMDebugStore(config.LLMDebugStore)

        // === NEW: Propagate to TieredCapabilityProvider ===
        if tieredProvider != nil {
            tieredProvider.SetLLMDebugStore(config.LLMDebugStore)
        }
        // === END NEW ===
    }

    // ... rest of existing code ...
}
```

#### 4.4 Add Option Function

```go
// WithTieredResolution enables tiered capability resolution for token optimization.
// This is recommended for deployments with 20+ tools.
// Both tiers use the AI client's default model for simplicity.
func WithTieredResolution(enabled bool) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.EnableTieredResolution = enabled
    }
}
```

### Phase 5: Metrics and Observability

Add telemetry calls to track the effectiveness of tiered resolution:

```go
// In TieredCapabilityProvider.GetCapabilities():
if t.telemetry != nil {
    t.telemetry.RecordMetric("orchestrator.tiered.tool_selection", 1, map[string]string{
        "total_tools":    strconv.Itoa(len(summaries)),
        "selected_tools": strconv.Itoa(len(selectedTools)),
    })

    // Record token savings estimate
    savedTokens := (len(summaries) - len(selectedTools)) * 200 // ~200 tokens per full schema
    t.telemetry.RecordMetric("orchestrator.tiered.tokens_saved", float64(savedTokens), nil)
}
```

---

## Usage Examples

### Basic Usage

```go
// Enable tiered resolution via environment
// GOMIND_TIERED_RESOLUTION_ENABLED=true

orchestrator, err := orchestration.CreateOrchestrator(nil, deps)
```

### Programmatic Configuration

```go
config := orchestration.DefaultConfig()
config.EnableTieredResolution = true
config.TieredResolution = orchestration.TieredCapabilityConfig{
    MinToolsForTiering: 20,  // Custom threshold
}

orchestrator, err := orchestration.CreateOrchestrator(config, deps)
```

### Using Option Functions

```go
orchestrator, err := orchestration.CreateOrchestratorWithOptions(deps,
    orchestration.WithTieredResolution(true),
    orchestration.WithTelemetry(true),
)
```

---

## Migration Guide

### For Existing Deployments

1. **Enabled by default** - Tiered resolution is now enabled by default for optimal token usage
2. **Disable if needed** - Set `GOMIND_TIERED_RESOLUTION_ENABLED=false` for edge cases
3. **Monitor** - Check `orchestrator.tiered.tokens_saved` metric

### Recommended Settings by Deployment Size

| Tool Count | Recommended Setting |
|------------|---------------------|
| < 15       | `EnableTieredResolution: false` (disable via env var if needed) |
| 15-50      | `EnableTieredResolution: true` (default) |
| 50-100     | `EnableTieredResolution: true` (default) + Service provider backup |
| 100+       | `EnableTieredResolution: true` (default) + Consider ServiceCapabilityProvider |

---

## Alternative Approaches Considered

### 1. Pure Vector Search (RAG)

**Approach**: Use embeddings to find semantically similar tools.

**Pros**:
- No extra LLM call
- Fast retrieval

**Cons**:
- Requires vector DB infrastructure
- May miss tools needed for intermediate steps
- Semantic similarity != task relevance

**Verdict**: Good for filtering, but LLM is better for multi-step reasoning.

### 2. Hierarchical Categories

**Approach**: Group tools by category, select category first, then tools.

```
Categories: [Finance, Weather, Location]
Finance: [stock-tool, currency-tool]
Location: [geocoding-tool, country-info-tool]
```

**Pros**:
- Reduces search space
- Works without LLM for simple cases

**Cons**:
- Requires manual categorization
- Tools may fit multiple categories
- Still needs tool selection within category

**Verdict**: Can combine with tiered approach as optimization.

### 3. Capability Compression

**Approach**: Use abbreviated schemas:
```
weather-tool: loc(str,req) -> {temp:num, cond:str}
```

**Pros**:
- Significant token reduction

**Cons**:
- LLMs may misinterpret compressed formats
- Loses semantic richness
- Harder to debug

**Verdict**: Too risky for production use.

---

## Future Enhancements

### 1. Adaptive Tiering

Automatically adjust `MinToolsForTiering` based on:
- Historical selection accuracy
- Token cost trends
- Error rates

### 2. Selection Caching

Cache tool selections for similar requests:
```go
type SelectionCache struct {
    cache map[string][]string  // requestHash -> selectedTools
    ttl   time.Duration
}
```

### 3. Hybrid RAG + LLM

Use vector search for initial filtering (top 30), then LLM for final selection (top 5-10).

---

## Files to Create/Modify

| Module/File | Action | Status | Description |
|-------------|--------|--------|-------------|
| `orchestration/tiered_capability_provider.go` | **Create** | ✅ Done | New TieredCapabilityProvider with fallback resilience (graceful fallback to `FormatForLLM()` on failure), context cancellation check, wrapped errors with context, `truncateRequest()` helper |
| `orchestration/tiered_capability_provider_test.go` | **Create** | ✅ Done | Unit tests including fallback behavior, hallucination filtering, error wrapping, and config precedence tests |
| `orchestration/catalog.go` | Modify | ✅ Done | Add `Summary` field to `EnhancedCapability`, `CapabilitySummary` type, `GetCapabilitySummaries()`, `FormatToolsForLLM()`, `GetToolCount()` |
| `orchestration/interfaces.go` | Modify | ✅ Done | Add `EnableTieredResolution` bool, `TieredResolution TieredCapabilityConfig` field, `TieredCapabilityConfig` struct, update `DefaultConfig()`, document GOMIND_* env var rationale |
| `orchestration/factory.go` | Modify | ✅ Done | Wire up TieredCapabilityProvider when `EnableTieredResolution` is true, propagate LLMDebugStore |
| `orchestration/prompt_builder.go` | Modify | ✅ Done | Add `SystemInstructions` field to `PromptConfig` |
| `orchestration/default_prompt_builder.go` | Modify | ✅ Done | Add `buildPersonaSection()`, update `BuildPlanningPrompt()` |
| `orchestration/prompt_builder_test.go` | Modify | ✅ Done | Add `TestBuildPersonaSection` tests |
| `orchestration/catalog_test.go` | Modify | ✅ Done | Add tests for new catalog methods |

---

## Acceptance Criteria

1. **Token Reduction**: 40-70% reduction in planning tokens for deployments with 50+ tools
2. **Accuracy**: Tool selection accuracy > 95% (no missing required tools)
3. **Latency**: Additional latency < 500ms (Tier 1 uses AI client's default model)
4. **Enabled by Default**: Tiered resolution enabled by default for optimal performance; can be disabled via env var if needed
5. **Observability**: Metrics for selection accuracy and token savings

---

## Framework Design Principles Compliance

This section validates the implementation against [FRAMEWORK_DESIGN_PRINCIPLES.md](../../FRAMEWORK_DESIGN_PRINCIPLES.md).

### ✅ Compliant

| Principle | Status | Notes |
|-----------|--------|-------|
| **Interface-First Design** | ✅ | `TieredCapabilityProvider` implements existing `CapabilityProvider` interface |
| **Module Architecture** | ✅ | `orchestration` → `core` + `telemetry` is valid dependency |
| **Telemetry Pattern** | ✅ | Always checks `if t.telemetry != nil`, uses NoOp when unavailable |
| **Dependency Injection** | ✅ | `SetLogger()` and `SetTelemetry()` follow established patterns |
| **Enabled by Default** | ✅ | Enabled by default for optimal token usage; can be disabled via env var for edge cases |
| **Environment-Aware Config** | ✅ | `GOMIND_TIERED_*` environment variables follow precedence rules |
| **Smart Defaults** | ✅ | `MinToolsForTiering: 20` based on research findings |

### ⚠️ Required Fixes

#### 1. Missing Circuit Breaker Protection

**Principle**: "External API calls must be protected by circuit breakers" (Error Handling Principles)

**Issue**: The Tier 1 LLM call in `selectRelevantTools()` lacks circuit breaker protection.

**Fix**: Add circuit breaker to `TieredCapabilityProvider`:

```go
// orchestration/tiered_capability_provider.go - Add to struct
type TieredCapabilityProvider struct {
    // ... existing fields ...
    circuitBreaker core.CircuitBreaker  // Optional: sophisticated resilience
}

// Add setter
func (t *TieredCapabilityProvider) SetCircuitBreaker(cb core.CircuitBreaker) {
    t.circuitBreaker = cb
}

// Modify selectRelevantTools to use circuit breaker
func (t *TieredCapabilityProvider) selectRelevantTools(...) ([]string, error) {
    // ... existing code ...

    // Wrap LLM call with circuit breaker if available
    var response *core.AIResponse
    var err error

    if t.circuitBreaker != nil {
        err = t.circuitBreaker.Execute(ctx, func() error {
            response, err = t.aiClient.GenerateResponse(ctx, prompt, options)
            return err
        })
    } else {
        response, err = t.aiClient.GenerateResponse(ctx, prompt, options)
    }

    if err != nil {
        return nil, fmt.Errorf("tool selection failed: %w", err)
    }
    // ... rest of code ...
}
```

**Factory Update** (`orchestration/factory.go`):
```go
if config.EnableTieredResolution {
    tieredProvider := NewTieredCapabilityProvider(...)
    tieredProvider.SetLogger(deps.Logger)
    tieredProvider.SetTelemetry(deps.Telemetry)
    tieredProvider.SetCircuitBreaker(deps.CircuitBreaker)  // ADD THIS
    // ...
}
```

#### 2. Simple Configuration in `WithTieredResolution`

**Principle**: "WithXXX() Option Functions - Auto-configure related settings when intent is clear" (Configuration System Rules)

**Design Decision**: Both tiers use the AI client's default model for simplicity. Cost savings come from reduced token counts (tool summaries vs full schemas), not from using different models.

**Implementation**:

```go
// orchestration/factory.go

// WithTieredResolution enables tiered capability resolution for token optimization.
// Both tiers use the AI client's default model for simplicity.
func WithTieredResolution(enabled bool) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.EnableTieredResolution = enabled
    }
}
```

#### 3. Context Cancellation Handling

**Principle**: "All components must handle context cancellation" (Component Lifecycle Rules)

**Issue**: The `selectRelevantTools` implementation should explicitly check context before LLM call.

**Fix**: Add context check:

```go
func (t *TieredCapabilityProvider) selectRelevantTools(
    ctx context.Context,
    request string,
    summaries []CapabilitySummary,
) ([]string, error) {
    // Check context before expensive LLM call
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Build the selection prompt...
    // (rest of existing code)
}
```

### Summary of Required Changes

| File | Change | Principle |
|------|--------|-----------|
| `orchestration/tiered_capability_provider.go` | Add `circuitBreaker` field and `SetCircuitBreaker()` | Error Handling |
| `orchestration/tiered_capability_provider.go` | Add context cancellation check | Component Lifecycle |
| `orchestration/factory.go` | Pass `deps.CircuitBreaker` to tiered provider | Error Handling |
| `orchestration/factory.go` | Document smart model selection in `WithTieredResolution` | Configuration Rules |

---

## Orchestration ARCHITECTURE.md Compliance

This section validates the implementation against [orchestration/ARCHITECTURE.md](../ARCHITECTURE.md).

### ✅ Compliant

| Architectural Pattern | Status | Notes |
|----------------------|--------|-------|
| **Interface-Based DI** | ✅ | Only imports `core` package; uses `core.AIClient`, `core.Logger`, `core.Telemetry` |
| **Module Dependencies** | ✅ | `orchestration` → `core` only (no `ai` or `resilience` imports) |
| **Explicit Configuration** | ✅ | `TieredCapabilityConfig` struct with clear fields |
| **Fail-Safe Defaults** | ✅ | Falls back to `FormatForLLM()` on selection failure |
| **Capability Provider Pattern** | ✅ | Implements `CapabilityProvider` interface |
| **Environment Variables** | ✅ | `GOMIND_TIERED_*` variables follow precedence rules |
| **SetLogger/SetTelemetry** | ✅ | Follows established dependency injection patterns |

### ⚠️ Required Fixes

#### 1. Fallback Resilience (Design Decision)

**Architecture Requirement** (ARCHITECTURE.md lines 700-705):
> Layer 1: Simple Built-in Resilience (Always Active)

**Design Decision**: Instead of retry logic, TieredCapabilityProvider uses **fallback resilience**:

- On Tier 1 failure → gracefully fallback to `FormatForLLM()` (direct approach)
- This is **safe and functional** - it just uses more tokens but still works
- No retry needed because fallback is always available
- The AI client itself may already have retry logic at a lower level

This approach is simpler, faster to recover, and provides equivalent reliability.

```go
// orchestration/tiered_capability_provider.go - GetCapabilities()

// Tier 1: Select relevant tools using lightweight summaries
selectedTools, err := t.selectRelevantTools(ctx, request, summaries)
if err != nil {
    // Fallback to direct approach on selection failure (resilience via fallback)
    if t.logger != nil {
        t.logger.WarnWithContext(ctx, "Tool selection failed, falling back to direct approach", map[string]interface{}{
            "error": err.Error(),
        })
    }
    return t.catalog.FormatForLLM(), nil  // Safe fallback - always works
}
```

#### 2. Missing Provider Priority Chain Integration

**Architecture Requirement** (ARCHITECTURE.md lines 620-654):
> Provider Types:
> 1. Default Provider (< 200 agents)
> 2. Service Provider (100s-1000s agents)

**Issue**: The factory code doesn't show how `TieredCapabilityProvider` integrates with the existing provider priority chain (Service → Default).

**Fix**: Update factory to show complete priority chain:

```go
// orchestration/factory.go - Complete provider initialization logic

    // Initialize capability provider based on configuration
    // Priority: 1) Tiered (if enabled), 2) Service (if configured), 3) Default
    if config.EnableTieredResolution {
        tieredProvider := NewTieredCapabilityProvider(
            orchestrator.catalog,
            deps.AIClient,
            &config.TieredResolution,
        )
        tieredProvider.SetLogger(deps.Logger)
        tieredProvider.SetTelemetry(deps.Telemetry)
        tieredProvider.SetCircuitBreaker(deps.CircuitBreaker)
        orchestrator.SetCapabilityProvider(tieredProvider)

        factoryLogger.Info("Using TieredCapabilityProvider for token optimization", map[string]interface{}{
            "operation": "capability_provider_initialization",
            "min_tools": config.TieredResolution.MinToolsForTiering,
        })
    } else if config.CapabilityProviderType == "service" {
        // Existing ServiceCapabilityProvider logic...
        // (unchanged)
    } else {
        // Default provider (unchanged)
    }
```

#### 3. Progressive Enhancement Levels Not Explicit

**Architecture Requirement** (ARCHITECTURE.md lines 116-138):
> Progressive Enhancement:
> - Level 1: Zero configuration
> - Level 2: With configuration
> - Level 3: Full production setup

**Issue**: The design doesn't explicitly document the three usage levels for TieredCapabilityProvider.

**Fix**: Add explicit usage levels to the "Usage Examples" section:

```go
// Level 1: Zero configuration (enabled by default - optimal token usage)
orchestrator, err := orchestration.CreateOrchestrator(nil, deps)

// Level 2: Environment configuration (disable if needed for edge cases)
// GOMIND_TIERED_RESOLUTION_ENABLED=false
orchestrator, err := orchestration.CreateOrchestrator(nil, deps)

// Level 3: Full production setup with circuit breaker
cb, _ := resilience.NewCircuitBreaker(&resilience.CircuitBreakerConfig{
    Name:           "tiered-selection",
    ErrorThreshold: 0.5,
    SleepWindow:    30 * time.Second,
})

deps := orchestration.OrchestratorDependencies{
    Discovery:      discovery,
    AIClient:       aiClient,
    Logger:         logger,
    Telemetry:      telemetry,
    CircuitBreaker: cb,  // Injected for production resilience
}

config := orchestration.DefaultConfig()
config.EnableTieredResolution = true

orchestrator, err := orchestration.CreateOrchestrator(config, deps)
```

### Summary of ARCHITECTURE.md Required Changes

| File | Change | Architectural Pattern |
|------|--------|----------------------|
| `orchestration/tiered_capability_provider.go` | Fallback resilience via `FormatForLLM()` | Graceful Degradation |
| `orchestration/factory.go` | Document complete provider priority chain | Capability Provider Pattern |
| Design doc "Usage Examples" | Add explicit Level 1/2/3 examples | Progressive Enhancement |

### Test Requirements for Architecture Compliance

```go
func TestTieredCapabilityProvider_FallbackResilience(t *testing.T) {
    // Test: Falls back to FormatForLLM() on Tier 1 failure
    // Test: Fallback is logged with error details
    // Test: Fallback returns valid capability string
    // Test: Context cancellation is respected
}

func TestTieredCapabilityProvider_HallucinationFiltering(t *testing.T) {
    // Test: Invalid tool names are filtered out
    // Test: Warning logged for hallucinated tools
    // Test: Empty selection after filtering returns error
}
```

---

## Core Module Design Principles Compliance

This section validates the implementation against [core/CORE_DESIGN_PRINCIPLES.md](../../core/CORE_DESIGN_PRINCIPLES.md).

### ✅ Compliant

| Principle | Status | Notes |
|-----------|--------|-------|
| **Interface-First Architecture** | ✅ | Uses `core.AIClient`, `core.Logger`, `core.Telemetry`, `core.CircuitBreaker` interfaces |
| **Zero Framework Dependencies** | ✅ | N/A for orchestration (applies to core only); orchestration → core is valid |
| **Minimal Interface Principle** | ✅ | `CapabilitySummary` has 4 fields, `TieredCapabilityConfig` has 2 fields |
| **Context-First Parameter** | ✅ | All methods follow `func(ctx context.Context, ...)` pattern |
| **Error as Last Return** | ✅ | All methods return `(..., error)` |
| **Graceful Degradation** | ✅ | Falls back to `FormatForLLM()` when selection fails or below threshold |
| **Option Function Pattern** | ✅ | `WithTieredResolution(enabled, model)` follows framework conventions |

### ⚠️ Required Fixes

#### 1. Environment Variable Precedence

**Core Principle** (CORE_DESIGN_PRINCIPLES.md lines 147-151):
> Configuration Priority:
> 1. Explicit function options (highest)
> 2. Standard environment variables (`REDIS_URL`, `OPENAI_API_KEY`)
> 3. GoMind-specific variables (`GOMIND_REDIS_URL`, etc.)
> 4. Sensible defaults (lowest)

**Issue**: The design uses `GOMIND_TIERED_*` variables but doesn't document whether there are standard equivalents.

**Fix**: Update `DefaultConfig()` to follow precedence:

```go
// orchestration/interfaces.go - Update DefaultConfig()

func DefaultConfig() *OrchestratorConfig {
    config := &OrchestratorConfig{
        // ... existing fields ...
    }

    // Tiered resolution: No standard env var equivalent exists, so GOMIND_* is correct
    // Document: GOMIND_TIERED_RESOLUTION_ENABLED has no standard equivalent
    // because "tiered resolution" is a GoMind-specific feature
    if enabled := os.Getenv("GOMIND_TIERED_RESOLUTION_ENABLED"); enabled != "" {
        config.EnableTieredResolution = strings.ToLower(enabled) == "true"
    }

    return config
}
```

**Documentation**: Add comment explaining that `GOMIND_TIERED_*` is correct because there's no industry-standard equivalent.

#### 2. Wrapped Errors with Context

**Core Principle** (CORE_DESIGN_PRINCIPLES.md lines 293-294):
> Use wrapped errors for context:
> `return fmt.Errorf("failed to register service %s: %w", info.Name, err)`

**Issue**: Some error returns in `selectRelevantTools` may not include sufficient context.

**Fix**: Ensure all errors are wrapped with operation context:

```go
// orchestration/tiered_capability_provider.go

func (t *TieredCapabilityProvider) selectRelevantTools(
    ctx context.Context,
    request string,
    summaries []CapabilitySummary,
) ([]string, error) {
    // ... existing code ...

    // ✅ Good: Wrapped with context
    if err != nil {
        return nil, fmt.Errorf("tiered tool selection failed for request %q: %w",
            truncateRequest(request, 50), err)
    }

    // Parse JSON response
    var tools []string
    if err := json.Unmarshal([]byte(response.Content), &tools); err != nil {
        // ✅ Good: Include what we tried to parse
        return nil, fmt.Errorf("failed to parse tool selection response as JSON array: %w", err)
    }

    return tools, nil
}

// Helper to truncate request for error messages
func truncateRequest(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
```

#### 3. Option Function Validation

**Core Principle** (CORE_DESIGN_PRINCIPLES.md lines 358-371):
> Option functions with intelligence including validation:
> ```go
> func WithPort(port int) Option {
>     return func(c *Config) error {
>         if port <= 0 || port > 65535 {
>             return fmt.Errorf("invalid port %d", port)
>         }
>         c.Port = port
>         return nil
>     }
> }
> ```

**Design Decision**: Both tiers use the AI client's default model for simplicity. No validation needed since there's no model parameter.

```go
// orchestration/factory.go

// WithTieredResolution enables tiered capability resolution for token optimization.
// Both tiers use the AI client's default model for simplicity.
// Cost savings come from reduced token counts (tool summaries vs full schemas).
func WithTieredResolution(enabled bool) OrchestratorOption {
    return func(c *OrchestratorConfig) {
        c.EnableTieredResolution = enabled
    }
}
```

### Summary of CORE_DESIGN_PRINCIPLES Required Changes

| File | Change | Core Principle |
|------|--------|----------------|
| `orchestration/interfaces.go` | Document GOMIND_* precedence rationale | Configuration Intelligence |
| `orchestration/tiered_capability_provider.go` | Wrap all errors with operation context | Error Handling |
| `orchestration/tiered_capability_provider.go` | Add `truncateRequest()` helper | Error Handling |

### Test Requirements for Core Principles Compliance

```go
func TestTieredCapabilityProvider_ErrorWrapping(t *testing.T) {
    // Test: Errors include operation context
    // Test: Errors include request snippet
    // Test: Errors wrap underlying error with %w
    // Test: errors.Is() works for wrapped errors
}

func TestTieredCapabilityProvider_ConfigPrecedence(t *testing.T) {
    // Test: Explicit config beats environment
    // Test: GOMIND_TIERED_* variables are loaded
    // Test: Defaults applied when no config provided
}

func TestWithTieredResolution_Validation(t *testing.T) {
    // Test: Empty model is accepted (uses default)
    // Test: Fast models are accepted without warning
    // Test: Expensive models trigger warning (if logging available)
}
```

---

## Test Specifications

### TieredCapabilityProvider Tests (`orchestration/tiered_capability_provider_test.go`)

```go
func TestTieredCapabilityProvider_BelowThreshold(t *testing.T) {
    // Setup: Create catalog with 15 tools (below default threshold of 20)
    // Expected: Should return all tools via FormatForLLM() without Tier 1 call
    // Verify: No LLM call made, full catalog returned
}

func TestTieredCapabilityProvider_AboveThreshold(t *testing.T) {
    // Setup: Create catalog with 30 tools, mock AIClient
    // Expected: Should make Tier 1 selection call, return filtered tools
    // Verify: LLM called with summaries, FormatToolsForLLM() called with selection
}

func TestTieredCapabilityProvider_HallucinationFiltering(t *testing.T) {
    // Setup: Mock AIClient returns ["real-tool/cap1", "fake-tool/fake_cap"]
    // Expected: Should filter out "fake-tool/fake_cap"
    // Verify: Warning logged, only valid tools returned
}

func TestTieredCapabilityProvider_FallbackOnError(t *testing.T) {
    // Setup: Mock AIClient returns error
    // Expected: Should fall back to FormatForLLM() (all tools)
    // Verify: Warning logged, graceful degradation
}

func TestTieredCapabilityProvider_EmptySelection(t *testing.T) {
    // Setup: Mock AIClient returns []
    // Expected: Should fall back to FormatForLLM()
    // Verify: All hallucinations filtered results in fallback
}

func TestTieredCapabilityProvider_CustomThreshold(t *testing.T) {
    // Setup: Config with MinToolsForTiering = 10
    // Expected: 15 tools should trigger tiering
    // Verify: Tier 1 call made
}
```

### AgentCatalog Extension Tests (`orchestration/catalog_test.go` additions)

```go
func TestAgentCatalog_GetCapabilitySummaries(t *testing.T) {
    // Verify: Returns summaries for all non-internal capabilities
    // Verify: Internal capabilities excluded
    // Verify: Summary field used if set, otherwise auto-generated from Description
}

func TestAgentCatalog_FormatToolsForLLM(t *testing.T) {
    // Verify: Only requested tools formatted
    // Verify: Full parameter schemas included
    // Verify: Unknown tool IDs silently ignored
}

func TestAgentCatalog_GetToolCount(t *testing.T) {
    // Verify: Returns count of non-internal capabilities
}

func TestEnhancedCapability_GetSummary(t *testing.T) {
    // Test: Returns explicit Summary when set
    // Test: Auto-generates from first 2 sentences of Description
    // Test: Handles Description with < 2 sentences
}

func TestExtractFirstSentences(t *testing.T) {
    tests := []struct{
        text     string
        n        int
        expected string
    }{
        {"First. Second. Third.", 2, "First. Second."},
        {"Only one sentence", 2, "Only one sentence"},
        {"First! Second? Third.", 2, "First! Second?"},
        {"", 2, ""},
    }
}
```

---

## Research-Backed Analysis (Updated January 2026)

This section presents findings from peer-reviewed research and industry benchmarks to validate the tiered approach. Research spans from late 2024 through 2025.

### Your Current Deployment Analysis

Based on the `travel-chat-agent` prompt ([monolith-prompt.txt](../../monolith-prompt.txt)):

| Metric | Value |
|--------|-------|
| Total Agents | 11 |
| Total Capabilities | ~23 |
| Capability Section | ~190 lines, ~7,500 characters |
| Estimated Capability Tokens | ~2,000-2,500 |
| Full Prompt Tokens | ~3,500-4,000 |

**Current status**: With 23 capabilities, you're at the threshold where tiered resolution becomes beneficial.

---

### Key Research Finding #1: RAG-MCP Framework (May 2025)

**Source**: [RAG-MCP: Mitigating Prompt Bloat in LLM Tool Selection](https://arxiv.org/html/2505.03275v1) (May 2025)

This is the most directly relevant peer-reviewed research, addressing prompt bloat in MCP tool selection:

| Metric | All-Tools Baseline | RAG-Based Selection | Improvement |
|--------|-------------------|---------------------|-------------|
| **Accuracy** | 13.62% | 43.13% | **3.2x better** |
| **Prompt Tokens** | 2,133 | 1,084 | **49% reduction** |
| **Token Usage** | Baseline | RAG-MCP | **74.8% reduction** |
| **Response Time** | Baseline | RAG-MCP | **62.1% faster** |

Key insights from the paper:
> "By injecting only the single most relevant MCP schema, the model avoids the distraction caused by irrelevant tool descriptions, resulting in clearer decision boundaries."

> "The dramatic reduction in prompt tokens allows the model to allocate more of its context window to reasoning about the task itself rather than parsing extraneous metadata."

> "When faced with hundreds of similar tools, the model often picks the wrong one or makes up fake tools (hallucinations)."

**Context**: The rapid growth of MCP repositories (4,400+ servers on mcp.so as of April 2025) underscores the need for scalable discovery mechanisms.

---

### Key Research Finding #2: MCP Hierarchical Tool Management (November 2025)

**Source**: [MCP Discussion #532 - Hierarchical Tool Management](https://github.com/orgs/modelcontextprotocol/discussions/532)

The official MCP community has identified the same scaling challenges:

> "At ~400-500 tokens per tool definition, 50 tools consume 20,000-25,000 tokens of precious context space."

**Problems with flat tool lists**:
- Context window saturation
- Degraded LLM performance on tool selection
- No dynamic loading capability
- Poor usability at scale

**Proposed solutions in MCP 2025-11-25 spec**:
- `tools/categories` and `tools/discover` for browsing without loading
- `tools/load` and `tools/unload` for dynamic management
- Enhanced metadata (category, namespace, latency estimates)

**Real consequence**: Many MCP servers artificially limit tool offerings or create multiple specialized servers.

---

### Key Research Finding #3: Guided-Structured Templates (September 2025)

**Source**: [Improving LLMs Function Calling via Guided-Structured Templates](https://arxiv.org/html/2509.18076v1) (ArXiv, September 2025)

This ICLR-track research shows how structured reasoning improves tool calling:

| Approach | BFCLv2 Score | Improvement |
|----------|-------------|-------------|
| Free-form CoT | 78.83 | Baseline |
| Template Prompting | 80.26 | **+1.8%** |
| Template Fine-tuning | +1.0-1.3 | **3-12% relative** |

Key insight:
> "Free-form CoT is insufficient and sometimes counterproductive for structured function-calling tasks."

The template enforces: identification → relevancy decision → documentation examination → parameter extraction → type conversion → drafting → revalidation.

**Implication for tiered approach**: Tier 1 can use structured templates for more accurate tool selection.

---

### Key Research Finding #4: "Less is More" - Tool Count Degradation

**Source**: [Less is More: Optimizing Function Calling for LLM Execution](https://arxiv.org/html/2411.15399v1) (November 2024)

This research quantified how LLM accuracy degrades as tool count increases:

| Tool Count | Success Rate | Observation |
|------------|-------------|-------------|
| 19 tools | **100%** | Model selects correctly |
| 46 tools | **0%** | Complete failure |

The paper states:
> "Even though Llama3.1-8b has a 16K context window that can fit all tools, it fails to select the correct one. This occurs because of the large number of available options confusing the LLM."

**Results of dynamic tool filtering**:

| Model | Before | After | Execution Time |
|-------|--------|-------|----------------|
| Hermes2-Pro-8b | Low | ~71% | **80% reduction** |
| Llama3.1-8b | 0% | 44.2% | **72% reduction** |
| Qwen2-7b | Low | 68% | **70% reduction** |

**Key insight**: Beyond ~20 tools, accuracy degrades significantly. Filtering tools **improves both accuracy AND latency**.

---

### Key Research Finding #5: Input Tokens and Latency

**Source**: [Glean Research - How Input Token Count Impacts LLM Latency](https://www.glean.com/blog/glean-input-token-llm-latency) (December 2024)

> "For every additional input token, the P95 TTFT increases by ~0.24ms and the average TTFT increases by ~0.20ms."

**Implications for Tiered Approach**:

| Scenario | Token Difference | TTFT Impact (P95) |
|----------|-----------------|-------------------|
| 50 tools → 5 tools | -9,000 tokens | **-2,160ms faster** |
| 100 tools → 6 tools | -18,800 tokens | **-4,512ms faster** |
| 23 tools → 6 tools | -3,400 tokens | **-816ms faster** |

The research also found:
> "Splitting a complicated 3000 token prompt into three parallel 1000 token prompts results in a reduction in the TTFT by 480ms."

---

### Key Research Finding #6: Fast Model Benchmarks (January 2026)

**Source**: [Vellum LLM Leaderboard](https://www.vellum.ai/llm-leaderboard) (Updated January 2026)

**Latest TTFT (Time to First Token) Benchmarks**:

| Model | TTFT | Best For |
|-------|------|----------|
| **Nova Micro** | 0.30s | Fastest overall |
| **Llama 3.1 8B** | 0.32s | Open source speed |
| **Llama 4 Scout** | 0.33s | New generation |
| **Gemini 2.0 Flash** | 0.34s | Best for tool selection |
| **GPT-4o mini** | 0.35s | OpenAI fastest |
| **Gemini 2.5 Flash** | 0.35s | Latest Gemini |
| **Claude 3.5 Haiku** | 0.88s | Anthropic fast tier |

**Throughput (Tokens/Second)**:
- Gemini 2.5 Flash: **372 TPS** (fastest)
- Gemini Flash: **250 TPS**
- Claude Haiku: **165 TPS**
- GPT-4o mini: **80 TPS**

**2025 Pattern for Production Systems**:
> "A 'planner' model (GPT-5.2 / Gemini 3 Deep Think / Claude Sonnet 4.5) for hard reasoning and tool orchestration, and one or more 'executor' models (Haiku 4.5, DeepSeek-V3.2, Qwen3-30B) for high-volume, low-latency calls."

**Recommendation for Tier 1**: Use **Gemini 2.0 Flash** (0.34s TTFT, 250+ TPS) or **GPT-4o mini** (0.35s TTFT) for tool selection.

---

### Key Research Finding #7: LLM-Based Agents Survey (June 2025)

**Source**: [LLM-Based Agents for Tool Learning: A Survey](https://link.springer.com/article/10.1007/s41019-025-00296-9) (Springer, June 2025)

This comprehensive survey identifies the **three components of tool use**:
1. Determining **whether** to use a tool
2. Selecting **which** tool to use
3. Understanding **how** to effectively employ the tool

**Multi-agent decomposition pattern**:
> "One proposed approach decomposes the general multi-agent framework into three distinct roles: a **planner**, a **caller**, and a **summarizer**, with each role instantiated by a single LLM agent."

This aligns with our tiered approach:
- **Tier 1 (Planner/Selector)**: Fast model selects relevant tools
- **Tier 3 (Caller)**: Main model generates execution plan with full schemas
- **Synthesis (Summarizer)**: Combines results

---

### Latency Impact Analysis

#### Theoretical Model

Using the Glean research formula: `TTFT_increase = tokens × 0.24ms`

**Current Approach (23 capabilities)**:
```
Capability tokens: ~2,500
Prompt overhead:   ~1,500
Total:             ~4,000 tokens
Estimated TTFT:    960ms + base latency (~500ms) = ~1,460ms
```

**Tiered Approach (select 6 capabilities)**:

```
Tier 1 (using Gemini 2.0 Flash):
  Summary tokens:    ~1,200 (23 caps × ~50 tokens each)
  Request overhead:  ~300
  Total:             ~1,500 tokens
  Fast model TTFT:   ~340ms (Gemini 2.0 Flash benchmark)

Tier 3:
  Selected caps:     ~600 (6 caps × ~100 tokens each)
  Prompt overhead:   ~1,500
  Total:             ~2,100 tokens
  Estimated TTFT:    504ms + base latency (~500ms) = ~1,004ms

Total: 340ms + 1,004ms = ~1,344ms
```

**Net Impact**: **~116ms faster** with tiered approach (even with extra LLM call)

**With RAG-MCP optimizations** (74.8% token reduction, 62.1% faster):
```
Expected total: ~550ms (62% faster than current)
```

#### Scaling Analysis

| Tool Count | Current TTFT | Tiered TTFT | Difference |
|------------|-------------|-------------|------------|
| 23 tools | ~1,460ms | ~1,344ms | **116ms faster** |
| 50 tools | ~2,900ms | ~1,450ms | **1,450ms faster** |
| 100 tools | ~5,400ms | ~1,550ms | **3,850ms faster** |

#### RAG-MCP Benchmark Comparison

From the May 2025 research:

| Metric | All Tools | RAG-MCP | Your Tiered |
|--------|-----------|---------|-------------|
| Token Reduction | Baseline | 74.8% | ~50-60% |
| Response Time | Baseline | 62.1% faster | ~40-50% faster |
| Accuracy | 13.62% | 43.13% | ~40% (estimated) |

---

### Research-Backed Recommendations

#### For Your Current Deployment (23 capabilities)

Based on the 2025 research:

1. **You are at the threshold** where tiered resolution becomes beneficial
2. The "Less is More" research shows degradation starts around 20 tools
3. RAG-MCP shows 3x accuracy improvement even at moderate tool counts
4. MCP community confirms 50 tools = 20,000-25,000 tokens of context saturation

**Recommendation**: **Enable tiered resolution** with the following settings:

```go
config.EnableTieredResolution = true
config.TieredResolution = TieredCapabilityConfig{
    MinToolsForTiering: 20,  // Just below your tool count
}
```

#### Model Selection for Tier 1 (Updated January 2026)

| Priority | Model | TTFT | TPS | Rationale |
|----------|-------|------|-----|-----------|
| **Latency-critical** | Nova Micro | 0.30s | - | Fastest TTFT |
| **Balanced (Recommended)** | Gemini 2.0 Flash | 0.34s | 250 | Best speed + quality |
| **OpenAI ecosystem** | GPT-4o mini | 0.35s | 80 | Good accuracy |
| **Throughput-critical** | Gemini 2.5 Flash | 0.35s | 372 | Highest TPS |
| **Anthropic ecosystem** | Claude 3.5 Haiku | 0.88s | 165 | Higher latency but good accuracy |

**2025 Production Pattern**:
> Use a fast "selector" model (Gemini Flash, GPT-4o mini) for Tier 1, and your main "reasoning" model (Claude Sonnet, GPT-4o) for Tier 3 plan generation.

#### Threshold Recommendations (Research-Based)

| Tool Count | Recommendation | Research Support |
|------------|----------------|------------------|
| < 15 | Direct (no tiering) | Low complexity, single call optimal |
| 15-25 | **Enable tiering** | "Less is More" shows degradation >20 |
| 25-50 | **Tiering required** | RAG-MCP: 3x accuracy, 62% faster |
| 50-100 | Tiering + caching | MCP Discussion: 20-25K token saturation |
| 100+ | Tiering + RAG hybrid | RAG-MCP + hierarchical tool management |

---

### Why Two-Step Beats Single-Step (Summary)

| Factor | Single Prompt (All Tools) | Tiered (Selection + Plan) |
|--------|---------------------------|---------------------------|
| **Accuracy** | 13.62% (RAG-MCP baseline) | 43.13% (3.2x better) |
| **Token Usage** | Baseline | 74.8% reduction (RAG-MCP) |
| **Response Time** | Baseline | 62.1% faster (RAG-MCP) |
| **TTFT at 50 tools** | ~2,900ms | ~1,450ms (50% faster) |
| **Context for Reasoning** | Diluted by tool metadata | Focused on task |
| **Scaling** | Degrades beyond 20 tools | Maintains accuracy |
| **Hallucination Risk** | High with many similar tools | Reduced with filtering |

The 2025 research conclusively shows that **presenting fewer, relevant tools to the LLM improves both accuracy and speed**. The overhead of an additional fast-model call is more than offset by:

1. Reduced prompt size in the main planning call
2. Clearer decision boundaries for tool selection
3. More context available for task reasoning
4. Reduced hallucination of non-existent tools (RAG-MCP finding)

### Industry Adoption (2025)

The tiered/hierarchical approach is now **industry standard**:

- **MCP 2025-11-25 spec** added `tools/discover`, `tools/load`, `tools/unload` for dynamic tool management
- **OpenAI adopted MCP** (March 2025), recognizing the need for scalable tool discovery
- **4,400+ MCP servers** on mcp.so (April 2025) - flat tool lists are unsustainable
- **Production pattern**: Planner model + Executor model architecture widely adopted

---

## Developer Customization: Orchestration Persona & Agent Specialization

This section addresses how developers can provide additional context to improve LLM decision-making.

### Research: Industry Best Practices

From [Microsoft's AI Agent Design Patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns):
> "The system prompt shapes the agent's behavior by defining its core task, persona, and operations."

From [Google Cloud's Agentic AI Patterns](https://docs.cloud.google.com/architecture/choose-design-pattern-agentic-ai-system):
> "A single-agent system uses an AI model, a defined set of tools, and a comprehensive system prompt to autonomously handle a user request."

**Key Insight**: Major frameworks use **one primary field** for customization - a single system prompt that defines the orchestrator's core behavioral context.

### Current GoMind Customization Mechanisms

GoMind already provides robust customization:

| Layer | Mechanism | Purpose |
|-------|-----------|---------|
| **1** | `PromptConfig.Domain` | Domain context (healthcare, finance, legal) |
| **2** | `PromptConfig.CustomInstructions` | Array of behavioral rules |
| **3** | `PromptConfig.AdditionalTypeRules` | Parameter type guidance |
| **4** | `PromptConfig.TemplateFile` | Full prompt template override |
| **5** | `OrchestratorDependencies.PromptBuilder` | Complete custom implementation |
| **6** | `PromptInput.Metadata` | Per-request context |

**Environment Variables**:
- `GOMIND_PROMPT_DOMAIN`
- `GOMIND_PROMPT_CUSTOM_INSTRUCTIONS`
- `GOMIND_PROMPT_TEMPLATE_FILE`
- `GOMIND_PROMPT_TYPE_RULES`

### What's Missing

The current system lacks a **single, prominent field for system-level instructions** (the orchestrator's "persona"). The existing `CustomInstructions` array is good for rules, but there's no dedicated field for the core behavioral context.

### Recommended Enhancement: Keep It Simple

Following industry best practices, add **one field**:

#### Add `SystemInstructions` to `PromptConfig`

```go
type PromptConfig struct {
    // ... existing fields ...

    // SystemInstructions defines the orchestrator's core behavioral context.
    // This is prepended to the planning prompt.
    //
    // Example: "You are a travel planning assistant. Always check weather
    // before recommending outdoor activities. Prefer real-time data sources."
    SystemInstructions string `json:"system_instructions,omitempty"`
}
```

#### Prompt Integration

```go
// In default_prompt_builder.go
func (d *DefaultPromptBuilder) BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error) {
    // Prepend system instructions if configured
    var systemContext string
    if d.config.SystemInstructions != "" {
        systemContext = fmt.Sprintf("SYSTEM CONTEXT:\n%s\n\n", d.config.SystemInstructions)
    }

    prompt := fmt.Sprintf(`%sYou are an AI orchestrator managing a multi-agent system.

%s

User Request: %s
// ... rest of existing prompt ...
`, systemContext, input.CapabilityInfo, input.Request)
}
```

### Usage Examples

#### Simple Usage

```go
config := orchestration.DefaultConfig()
config.PromptConfig.SystemInstructions = `You are a travel planning assistant.
Always check weather before recommending outdoor activities.
Prefer real-time data sources over cached data.
Convert prices to user's preferred currency.`

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

#### Combined with Existing Features

```go
config := orchestration.DefaultConfig()

// Core persona (NEW - one field, like other frameworks)
config.PromptConfig.SystemInstructions = "You are a helpful travel assistant."

// Domain context (EXISTING)
config.PromptConfig.Domain = "travel"

// Specific rules (EXISTING - array format)
config.PromptConfig.CustomInstructions = []string{
    "Prefer real-time data sources",
    "Always check weather before outdoor recommendations",
}
```

### Why This Approach?

| Aspect | Benefit |
|--------|---------|
| **Industry Alignment** | Matches established patterns for system prompts |
| **Simplicity** | One field instead of many nested structs |
| **Backward Compatible** | New optional field, existing code unchanged |
| **Discoverable** | Developers will find it intuitive |
| **Composable** | Works with existing `Domain` and `CustomInstructions` |

### What About Agent Specialization?

For per-tool/agent hints, the existing `EnhancedCapability.Description` field is sufficient. The description is already included in the LLM prompt. If more context is needed, developers can:

1. **Enrich descriptions** in their tool registration
2. **Use `CustomInstructions`** for tool preference rules
3. **Implement custom `PromptBuilder`** for full control

This follows the "start simple, add complexity as needed" principle from [Google Cloud's guidance](https://docs.cloud.google.com/architecture/choose-design-pattern-agentic-ai-system).

### Implementation Details

#### Change 1: Add `SystemInstructions` field to `PromptConfig`

**File**: `orchestration/prompt_builder.go`
**Location**: Lines 84-117 (inside `PromptConfig` struct)
**Insert after**: Line 116 (after `IncludeAntiPatterns` field)

```go
// orchestration/prompt_builder.go - Add after line 116

	// SystemInstructions defines the orchestrator's core behavioral context.
	// This is prepended to the planning prompt.
	//
	// Example: "You are a travel planning assistant. Always check weather
	// before recommending outdoor activities. Prefer real-time data sources."
	SystemInstructions string `json:"system_instructions,omitempty"`
```

#### Change 2: Integrate system instructions in `BuildPlanningPrompt()`

**File**: `orchestration/default_prompt_builder.go`
**Location**: Lines 175-330 (`BuildPlanningPrompt` method)

**Design Decision**: Avoid duplicate "You are..." statements.

When `SystemInstructions` is provided:
- Developer's persona becomes the **primary identity** ("You are a travel specialist...")
- Orchestrator role becomes a **functional description** ("As an AI orchestrator, you manage...")

This ensures a single, clear identity for the LLM.

**Step 2a**: Add helper method after line 393 (after `buildDomainSection`):

```go
// orchestration/default_prompt_builder.go - Add after line 393

// buildPersonaSection generates the persona statement.
// If SystemInstructions is configured, uses it as the primary identity.
// Otherwise, uses the default orchestrator identity.
func (d *DefaultPromptBuilder) buildPersonaSection() string {
	if d.config.SystemInstructions != "" {
		// Developer's persona is primary identity, orchestrator role is functional
		return fmt.Sprintf(`%s

As an AI orchestrator, you manage a multi-agent system to fulfill user requests.`,
			d.config.SystemInstructions)
	}
	// Default identity when no custom instructions provided
	return "You are an AI orchestrator managing a multi-agent system."
}
```

**Step 2b**: Modify `BuildPlanningPrompt` at line 196-204 to call the new method:

```go
// orchestration/default_prompt_builder.go - Add after line 203 (after domainSection)

	// Build persona section (handles SystemInstructions vs default)
	personaSection := d.buildPersonaSection()
```

**Step 2c**: Modify the `fmt.Sprintf` at line 206 to use the persona section:

```go
// orchestration/default_prompt_builder.go - Modify line 206

	// Change from:
	prompt := fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

User Request: %s
...

	// To:
	prompt := fmt.Sprintf(`%s

%s

User Request: %s
...
```

And add `personaSection` as the first argument at line 278:

```go
// orchestration/default_prompt_builder.go - Modify line 278

	// Change from:
		input.CapabilityInfo,
	// To:
		personaSection,
		input.CapabilityInfo,
```

**Example Output Without SystemInstructions (unchanged behavior)**:
```
You are an AI orchestrator managing a multi-agent system.

Available Agents and Capabilities:
...
```

**Example Output With SystemInstructions (unified persona)**:
```
You are a travel planning specialist with expertise in multi-destination trips.
Always check weather before recommending outdoor activities.

As an AI orchestrator, you manage a multi-agent system to fulfill user requests.

Available Agents and Capabilities:
...
```

This approach:
1. **Single identity**: One "You are..." statement
2. **Clear role**: Orchestrator function is subordinate to identity
3. **Backward compatible**: Default behavior unchanged

### Summary of Changes

| Module/File | Line(s) | Change | Status |
|-------------|---------|--------|--------|
| `orchestration/prompt_builder.go` | 116-117 | Add `SystemInstructions string` field to `PromptConfig` | ✅ Done |
| `orchestration/default_prompt_builder.go` | 393+ | Add `buildPersonaSection()` method | ✅ Done |
| `orchestration/default_prompt_builder.go` | 203-204 | Call `buildPersonaSection()` | ✅ Done |
| `orchestration/default_prompt_builder.go` | 206, 278 | Replace hardcoded persona with `personaSection` | ✅ Done |

**Total: ~20 lines of code** — simple, aligned with major frameworks, and avoids duplicate identities. ✅ **Implemented**

### Complete Format String Change (Line 206)

For clarity, here is the exact modification required at line 206 of `default_prompt_builder.go`:

**BEFORE** (current code):
```go
prompt := fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

User Request: %s

Create an execution plan...
```

**AFTER** (with personaSection):
```go
prompt := fmt.Sprintf(`%s

%s

User Request: %s

Create an execution plan...
```

The first `%s` is now `personaSection` (which includes the orchestrator role text), and `input.CapabilityInfo` shifts to the second `%s`.

### Test Requirements

**Unit Tests for SystemInstructions** (add to `orchestration/default_prompt_builder_test.go`):

```go
func TestBuildPersonaSection(t *testing.T) {
    tests := []struct {
        name               string
        systemInstructions string
        wantContains       string
        wantNotContains    string
    }{
        {
            name:               "default_persona_when_empty",
            systemInstructions: "",
            wantContains:       "You are an AI orchestrator managing a multi-agent system",
            wantNotContains:    "As an AI orchestrator",
        },
        {
            name:               "custom_persona_with_orchestrator_role",
            systemInstructions: "You are a travel planning specialist.",
            wantContains:       "You are a travel planning specialist",
            wantNotContains:    "",
        },
        {
            name:               "custom_persona_includes_orchestrator_function",
            systemInstructions: "You are a travel planning specialist.",
            wantContains:       "As an AI orchestrator, you manage a multi-agent system",
            wantNotContains:    "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            config := &PromptConfig{SystemInstructions: tt.systemInstructions}
            builder, _ := NewDefaultPromptBuilder(config)

            result := builder.buildPersonaSection()

            if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
                t.Errorf("expected result to contain %q, got %q", tt.wantContains, result)
            }
            if tt.wantNotContains != "" && strings.Contains(result, tt.wantNotContains) {
                t.Errorf("expected result NOT to contain %q, got %q", tt.wantNotContains, result)
            }
        })
    }
}
```

### Observability Enhancement

Add telemetry/logging to `BuildPlanningPrompt` after line 193:

```go
// Add span attribute for custom persona usage
if d.telemetry != nil && span != nil {
    span.SetAttribute("has_custom_persona", d.config.SystemInstructions != "")
}
```

---

## AI Module ARCHITECTURE.md Compliance

This section validates the implementation against [ai/ARCHITECTURE.md](../../ai/ARCHITECTURE.md) **focusing on code implementation patterns**.

### ✅ Compliant

| Pattern | Status | Notes |
|---------|--------|-------|
| **Configuration Hierarchy** | ✅ | Follows Explicit → Provider-specific → GOMIND_* → Defaults precedence |
| **Logger Propagation** | ✅ | Uses `SetLogger()` with `WithComponent("framework/orchestration")` pattern |
| **Factory Pattern** | ✅ | Uses dependency injection via factory, not direct instantiation |
| **Interface-Based AI Client** | ✅ | Accepts `core.AIClient` interface, not concrete provider |

### ⚠️ Required Code Fixes

#### 1. Missing StartSpan Helper Method

**Reference**: `ai/providers/base.go` lines 74-82

The AI module's `BaseClient` provides a `StartSpan()` helper that returns `core.NoOpSpan{}` when telemetry is nil. The TieredCapabilityProvider should follow this pattern instead of checking `t.telemetry != nil` everywhere.

**Current Code** (missing helper):
```go
// Scattered nil checks throughout
if t.telemetry != nil {
    ctx, span = t.telemetry.StartSpan(ctx, "...")
}
```

**Fix**: Add StartSpan helper method to TieredCapabilityProvider:

```go
// orchestration/tiered_capability_provider.go

// StartSpan starts a distributed tracing span if telemetry is configured.
// Returns the updated context and a span. If telemetry is nil, returns NoOpSpan.
// Caller is responsible for calling span.End() when the operation completes.
func (t *TieredCapabilityProvider) StartSpan(ctx context.Context, name string) (context.Context, core.Span) {
    if t.telemetry != nil {
        return t.telemetry.StartSpan(ctx, name)
    }
    return ctx, &core.NoOpSpan{}
}

// Usage in GetCapabilities:
func (t *TieredCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    ctx, span := t.StartSpan(ctx, "orchestrator.tiered_capability_resolution")
    defer span.End()

    span.SetAttribute("total_tools", t.catalog.GetToolCount())
    span.SetAttribute("threshold", t.MinToolsForTiering)

    // ... rest of implementation - no more nil checks for span
}
```

#### 2. Add Span Attributes for LLM Selection

**Reference**: `ai/providers/base.go` lines 84-150

The AI module adds span attributes for LLM call instrumentation. The Tier 1 selection call should follow this pattern.

**Fix**: Add span attributes in `selectRelevantTools()`:

```go
// orchestration/tiered_capability_provider.go - selectRelevantTools()

// Create span for the selection call
ctx, span := t.StartSpan(ctx, "tiered.tool_selection")
defer span.End()

span.SetAttribute("tool_count", len(summaries))
span.SetAttribute("request_length", len(request))

startTime := time.Now()
response, err := t.aiClient.GenerateResponse(ctx, prompt, options)
duration := time.Since(startTime)

span.SetAttribute("duration_ms", duration.Milliseconds())

if err != nil {
    span.SetAttribute("status", "error")
    span.SetAttribute("error", err.Error())
    return nil, err
}

span.SetAttribute("status", "success")
span.SetAttribute("selected_tools", len(selectedTools))
```

#### 3. Align with BaseClient Telemetry Pattern

**Reference**: `ai/providers/base.go` lines 97-100

The AI module uses **both** the injected telemetry interface AND the global singleton for metrics:

```go
// ai/providers/base.go line 98-100
telemetry.Counter("ai.request.total",
    "module", telemetry.ModuleAI,
)
```

**Issue**: The TieredCapabilityProvider only uses `t.telemetry.RecordMetric()` which requires nil checks. For consistency and to leverage unified_metrics patterns, also use global `telemetry.*` functions.

**Fix**: Use both patterns consistently:

```go
// orchestration/tiered_capability_provider.go

import "github.com/itsneelabh/gomind/telemetry"

// Use global singleton (no nil check needed, NoOp if not initialized):
telemetry.Counter("orchestrator.tiered.selections.total",
    "module", telemetry.ModuleOrchestration,
    "status", status,
)

// For histograms/complex metrics - use injected telemetry:
if t.telemetry != nil {
    t.telemetry.RecordMetric("orchestrator.tiered.selection.duration_ms",
        float64(duration.Milliseconds()),
        map[string]string{"status": status})
}
```

### Summary of AI ARCHITECTURE.md Required Code Changes

| File | Change | AI Architecture Pattern |
|------|--------|-------------------------|
| `orchestration/tiered_capability_provider.go` | Add `StartSpan()` helper method returning `core.NoOpSpan{}` | BaseClient.StartSpan pattern |
| `orchestration/tiered_capability_provider.go` | Import `telemetry` module, use global counters | BaseClient metrics pattern |

---

## Telemetry Module ARCHITECTURE.md Compliance

This section validates the implementation against [telemetry/ARCHITECTURE.md](../../telemetry/ARCHITECTURE.md) **focusing on code implementation patterns**.

### ✅ Compliant

| Pattern | Status | Notes |
|---------|--------|-------|
| **No Telemetry in Core** | ✅ | Telemetry is optional, injected via `SetTelemetry()` |

### ⚠️ Required Code Fixes

#### 1. Use Global Singleton Pattern (Like AI Module)

**Reference**: `ai/providers/base.go` lines 97-100, `telemetry/unified_metrics.go` lines 78-89

The AI module uses **global singleton functions** (`telemetry.Counter()`, `telemetry.Histogram()`) which require no nil checks and are NoOp when telemetry isn't initialized. The design currently uses `t.telemetry.RecordMetric()` which requires scattered nil checks.

**Current Code** (requires nil checks everywhere):
```go
if t.telemetry != nil {
    t.telemetry.RecordMetric("...", value, labels)
}
```

**Fix**: Use global singleton pattern like the AI module:

```go
// orchestration/tiered_capability_provider.go

import "github.com/itsneelabh/gomind/telemetry"

// No nil checks needed - NoOp if telemetry not initialized
telemetry.Counter("orchestrator.tiered.selections.total",
    "module", telemetry.ModuleOrchestration,
    "status", "success",
)

telemetry.Histogram("orchestrator.tiered.selection.duration_ms",
    durationMs,
    "module", telemetry.ModuleOrchestration,
)
```

#### 2. Use Unified Metrics Helpers for AI Calls

**Reference**: `telemetry/unified_metrics.go` lines 161-188 (`RecordAIRequest`)

The telemetry module provides `RecordAIRequest()` helper specifically for AI API calls. The Tier 1 LLM selection call should use this for consistency.

**Fix**: Use unified metrics helper:

```go
// After LLM selection call completes
telemetry.RecordAIRequest(
    telemetry.ModuleOrchestration,  // Module constant
    "tiered_selection",              // Operation name
    float64(tier1Duration.Milliseconds()),
    status,  // "success" or "error"
)
```

#### 3. High Cardinality Label Prevention

**Reference**: `telemetry/ARCHITECTURE.md` lines 1090-1113

**Issue**: Exact tool counts as labels create cardinality explosion.

**Fix**: Add `bucketize()` helper:

```go
// orchestration/tiered_capability_provider.go

// bucketize converts a count to a range bucket for low-cardinality metrics.
func bucketize(count int) string {
    switch {
    case count == 0:
        return "0"
    case count <= 5:
        return "1-5"
    case count <= 10:
        return "6-10"
    case count <= 20:
        return "11-20"
    case count <= 50:
        return "21-50"
    case count <= 100:
        return "51-100"
    default:
        return "100+"
    }
}

// Usage - LOW cardinality (7 possible values per label)
telemetry.Counter("orchestrator.tiered.selections.total",
    "module", telemetry.ModuleOrchestration,
    "total_tools_bucket", bucketize(len(summaries)),       // "21-50" not "35"
    "selected_tools_bucket", bucketize(len(selectedTools)), // "1-5" not "4"
)
```

#### 4. Metric Naming for OTEL Type Mapping

**Reference**: `telemetry/ARCHITECTURE.md` lines 432-458

Use naming conventions for automatic OTEL instrument type mapping:

```go
// Counter metrics (contain "total", "count", "errors")
"orchestrator.tiered.selections.total"           // ✅ Counter
"orchestrator.tiered.tokens_saved.count"         // ✅ Counter
"orchestrator.tiered.fallbacks.total"            // ✅ Counter

// Histogram metrics (contain "duration", "latency")
"orchestrator.tiered.selection.duration_ms"      // ✅ Histogram
```

#### 5. Complete Implementation Following All Patterns

```go
// orchestration/tiered_capability_provider.go

import "github.com/itsneelabh/gomind/telemetry"

func (t *TieredCapabilityProvider) GetCapabilities(ctx context.Context, request string, metadata map[string]interface{}) (string, error) {
    // Span via StartSpan helper (handles nil telemetry - see AI compliance section)
    ctx, span := t.StartSpan(ctx, "orchestrator.tiered_capability_resolution")
    defer span.End()

    startTime := time.Now()
    summaries := t.catalog.GetCapabilitySummaries()

    span.SetAttribute("total_tools", len(summaries))
    span.SetAttribute("threshold", t.MinToolsForTiering)

    // Check if tiering is needed
    if len(summaries) < t.MinToolsForTiering {
        span.SetAttribute("tiering_skipped", true)
        return t.catalog.FormatForLLM(), nil
    }

    // Tier 1: Select relevant tools
    selectedTools, err := t.selectRelevantTools(ctx, request, summaries)
    tier1Duration := time.Since(startTime)

    status := "success"
    if err != nil {
        status = "fallback"
        span.SetAttribute("fallback_reason", err.Error())

        // Global singleton - no nil check needed
        telemetry.Counter("orchestrator.tiered.fallbacks.total",
            "module", telemetry.ModuleOrchestration,
            "reason", "selection_failed",
        )
        return t.catalog.FormatForLLM(), nil
    }

    // Metrics via global singleton pattern
    telemetry.Counter("orchestrator.tiered.selections.total",
        "module", telemetry.ModuleOrchestration,
        "total_tools_bucket", bucketize(len(summaries)),
        "selected_tools_bucket", bucketize(len(selectedTools)),
        "status", status,
    )

    telemetry.Histogram("orchestrator.tiered.selection.duration_ms",
        float64(tier1Duration.Milliseconds()),
        "module", telemetry.ModuleOrchestration,
    )

    // Use unified AI metrics helper
    telemetry.RecordAIRequest(
        telemetry.ModuleOrchestration,
        "tiered_selection",
        float64(tier1Duration.Milliseconds()),
        status,
    )

    // Token savings
    savedTokens := (len(summaries) - len(selectedTools)) * 200
    telemetry.Histogram("orchestrator.tiered.tokens_saved_amount",
        float64(savedTokens),
        "module", telemetry.ModuleOrchestration,
    )

    span.SetAttribute("selected_tools_count", len(selectedTools))
    span.SetAttribute("tier1_duration_ms", tier1Duration.Milliseconds())
    span.SetAttribute("tokens_saved", savedTokens)

    return t.catalog.FormatToolsForLLM(selectedTools), nil
}
```

### Summary of Telemetry ARCHITECTURE.md Required Code Changes

| File | Change | Pattern |
|------|--------|---------|
| `orchestration/tiered_capability_provider.go` | Import `telemetry` module | Global singleton access |
| `orchestration/tiered_capability_provider.go` | Use `telemetry.Counter()`, `telemetry.Histogram()` | No nil checks |
| `orchestration/tiered_capability_provider.go` | Use `telemetry.RecordAIRequest()` | Unified metrics |
| `orchestration/tiered_capability_provider.go` | Add `bucketize()` helper | Cardinality prevention |
| `orchestration/tiered_capability_provider.go` | Metric names with `*.total`, `*.duration_ms` | OTEL type mapping |

### Test Requirements

```go
func TestBucketize(t *testing.T) {
    tests := []struct {
        input    int
        expected string
    }{
        {0, "0"},
        {3, "1-5"},
        {10, "6-10"},
        {25, "21-50"},
        {100, "51-100"},
        {150, "100+"},
    }
    for _, tt := range tests {
        if got := bucketize(tt.input); got != tt.expected {
            t.Errorf("bucketize(%d) = %q, want %q", tt.input, got, tt.expected)
        }
    }
}
```

---

## Files to Create/Modify - Complete Summary

This table consolidates all required changes from the compliance reviews, **focusing on code implementation patterns**.

| Module/File | Action | Status | Implementation Requirements |
|-------------|--------|--------|----------------------------|
| `orchestration/tiered_capability_provider.go` | **Create** | ✅ Done | **Core Implementation**: Implement `CapabilityProvider` interface with two-tier resolution. **Fallback Resilience**: On Tier 1 failure, gracefully fallback to `FormatForLLM()` (no retry needed - fallback is safe and functional). **Context Cancellation**: Check context before LLM call. **Tracing**: `StartSpan()` helper returning `core.NoOpSpan{}` when nil, `telemetry.AddSpanEvent()` before/after LLM calls, `telemetry.RecordSpanError()` for errors, include trace_id in error messages. **Metrics**: Import `telemetry` module, use global singleton (`telemetry.Counter()`, `telemetry.Histogram()`), use `telemetry.RecordAIRequest()` helper, `bucketize()` for low-cardinality labels, naming conventions (`*.total`, `*.duration_ms`). **Logging**: Standard field names (`operation`, `status`, `duration_ms`, `error`, `error_type`), log both success and failure paths, add package documentation about initialization order. **LLM Debug Store**: Add `debugStore`, `debugWg`, `debugSeqID` fields; `SetLLMDebugStore()` method; `recordDebugInteraction()` helper; record `tiered_selection` interactions; `Shutdown()` for graceful shutdown. **Errors**: Wrap with context using `fmt.Errorf(...: %w)`, add `truncateRequest()` helper. |
| `orchestration/tiered_capability_provider_test.go` | **Create** | ✅ Done | Tests for: fallback behavior on Tier 1 failure, `StartSpan()` helper, span events emission, `RecordSpanError()` calls, metric emission via global singleton, `bucketize()` function, standard log field names, LLM debug recording with `tiered_selection` type, `Shutdown()` graceful termination, error wrapping, config precedence, hallucination filtering |
| `orchestration/interfaces.go` | Modify | ✅ Done | Add `EnableTieredResolution` bool field, add `TieredResolution TieredCapabilityConfig` field, add `TieredCapabilityConfig` struct, document GOMIND_* env var precedence rationale |
| `orchestration/factory.go` | Modify | ✅ Done | Wire up `TieredCapabilityProvider` when `EnableTieredResolution` is true, include `operation` field in factory logs, propagate `LLMDebugStore` to TieredCapabilityProvider |
| `orchestration/catalog.go` | Modify | ✅ Done | Add `Summary` field to `EnhancedCapability`, add `GetSummary()` method, add `GetCapabilitySummaries()`, `FormatToolsForLLM()`, `GetToolCount()` methods, add `extractFirstSentences()` helper |

### Key Code Patterns to Follow

| Pattern | Reference | Implementation |
|---------|-----------|----------------|
| `StartSpan()` helper | `ai/providers/base.go:74-82` | Returns `core.NoOpSpan{}` when telemetry is nil |
| Fallback resilience | `GetCapabilities()` | On Tier 1 failure, fallback to `FormatForLLM()` - no retry needed |
| Global telemetry | `ai/providers/base.go:97-100` | Use `telemetry.Counter()` not `t.telemetry.RecordMetric()` |
| Unified AI metrics | `telemetry/unified_metrics.go:161-188` | Use `telemetry.RecordAIRequest()` for LLM calls |
| Logger propagation | `ai/providers/base.go:64-72` | `WithComponent("framework/orchestration")` |
| Cardinality prevention | `telemetry/ARCHITECTURE.md:1090-1113` | `bucketize()` for count-based labels |
| Span events for LLM | `docs/DISTRIBUTED_TRACING_GUIDE.md:1418-1441` | Use `telemetry.AddSpanEvent()` before/after LLM calls |
| Error recording on spans | `docs/DISTRIBUTED_TRACING_GUIDE.md:1403` | Use `telemetry.RecordSpanError(ctx, err)` for errors |
| Trace context in errors | `docs/DISTRIBUTED_TRACING_GUIDE.md:1470-1496` | Include trace_id in error messages for debugging |
| Standard log fields | `docs/LOGGING_IMPLEMENTATION_GUIDE.md:699-733` | Use `operation`, `status`, `error`, `duration_ms`, `error_type` |
| Log both paths | `docs/LOGGING_IMPLEMENTATION_GUIDE.md:1386-1424` | Always log success and failure outcomes |
| Initialization order | `docs/LOGGING_IMPLEMENTATION_GUIDE.md:1046-1065` | Document telemetry before agent creation requirement |
| LLM Debug Store integration | `LLM_DEBUG_PAYLOAD_DESIGN.md:4.1-4.6` | Add `debugStore` field, `SetLLMDebugStore()`, `recordDebugInteraction()` helper |
| Debug recording lifecycle | `LLM_DEBUG_PAYLOAD_DESIGN.md:4.6` | Use `sync.WaitGroup` for tracking, `Shutdown()` for graceful termination |
| Fallback request ID | `LLM_DEBUG_PAYLOAD_DESIGN.md:4.6` | Use `atomic.Uint64` counter when TraceID is empty |
| Recording site type | `LLM_DEBUG_PAYLOAD_DESIGN.md:4.1` | Use `"tiered_selection"` as the 7th recording site type |

---

## Distributed Tracing Guide Compliance

This section validates the implementation against [docs/DISTRIBUTED_TRACING_GUIDE.md](../../docs/DISTRIBUTED_TRACING_GUIDE.md) **focusing on code implementation patterns**.

### ✅ Compliant

| Pattern | Status | Notes |
|---------|--------|-------|
| **StartSpan() Helper** | ✅ | Uses pattern from `ai/providers/base.go:74-82` returning `core.NoOpSpan{}` |
| **span.SetAttribute()** | ✅ | Documented throughout for span attributes |
| **defer span.End()** | ✅ | Consistently defers span closure |
| **Fallback Resilience** | ✅ | On Tier 1 failure, fallback to `FormatForLLM()` with logged warning |
| **Context Propagation** | ✅ | `ctx` passed through all method calls |

### ⚠️ Required Code Fixes

#### 1. Add Span Events for LLM Selection

**Reference**: `docs/DISTRIBUTED_TRACING_GUIDE.md` lines 1418-1441 (LLM Telemetry Events)

The distributed tracing guide shows that LLM interactions should emit span events for debugging and cost analysis. The Tier 1 selection LLM call should emit events like the orchestration module does.

**Fix**: Add span events before and after the LLM selection call:

```go
// orchestration/tiered_capability_provider.go - selectRelevantTools()

import "github.com/itsneelabh/gomind/telemetry"

func (t *TieredCapabilityProvider) selectRelevantTools(
    ctx context.Context,
    request string,
    summaries []CapabilitySummary,
) ([]string, error) {
    prompt := t.buildSelectionPrompt(summaries, request)

    // Emit span event BEFORE LLM call (like orchestrator.go pattern)
    telemetry.AddSpanEvent(ctx, "llm.tier1_selection.request",
        "prompt_length", len(prompt),
        "tool_count", len(summaries),
    )

    startTime := time.Now()
    response, err := t.aiClient.GenerateResponse(ctx, prompt, options)
    duration := time.Since(startTime)

    if err != nil {
        // Use RecordSpanError for proper error recording
        telemetry.RecordSpanError(ctx, err)
        return nil, fmt.Errorf("tool selection failed: %w", err)
    }

    // Emit span event AFTER LLM call with response details
    telemetry.AddSpanEvent(ctx, "llm.tier1_selection.response",
        "response_length", len(response.Content),
        "duration_ms", duration.Milliseconds(),
        "prompt_tokens", response.PromptTokens,
        "completion_tokens", response.CompletionTokens,
    )

    // ... parse and validate ...
}
```

#### 2. Use RecordSpanError for Errors

**Reference**: `docs/DISTRIBUTED_TRACING_GUIDE.md` line 1403 (`telemetry.RecordSpanError`)

**Issue**: The design uses `span.SetAttribute("error", err.Error())` but doesn't use the proper `RecordSpanError()` function which marks the span as errored and makes it easy to find in Jaeger.

**Current Code**:
```go
span.SetAttribute("status", "failed")
span.SetAttribute("error", err.Error())
```

**Fix**: Use `RecordSpanError()` for proper error recording:

```go
import "github.com/itsneelabh/gomind/telemetry"

// When selection fails
span.SetAttribute("status", "failed")
telemetry.RecordSpanError(ctx, err)
```

#### 3. Add Trace Context to Error Responses

**Reference**: `docs/DISTRIBUTED_TRACING_GUIDE.md` lines 1470-1496 (Manual Trace ID Extraction)

For debugging, include trace ID in error responses so users can reference specific failures:

```go
// orchestration/tiered_capability_provider.go

func (t *TieredCapabilityProvider) GetCapabilities(
    ctx context.Context,
    request string,
    metadata map[string]interface{},
) (string, error) {
    ctx, span := t.StartSpan(ctx, "orchestrator.tiered_capability_resolution")
    defer span.End()

    // Get trace context for error messages
    tc := telemetry.GetTraceContext(ctx)

    // ... selection logic ...

    if err != nil {
        // Include trace_id in error for debugging
        if tc.TraceID != "" {
            return "", fmt.Errorf("tiered selection failed (trace_id=%s): %w", tc.TraceID, err)
        }
        return "", fmt.Errorf("tiered selection failed: %w", err)
    }
}
```

### Summary of Distributed Tracing Guide Required Code Changes

| File | Change | Pattern Reference |
|------|--------|-------------------|
| `orchestration/tiered_capability_provider.go` | Add `telemetry.AddSpanEvent()` before/after LLM call | LLM telemetry events pattern |
| `orchestration/tiered_capability_provider.go` | Use `telemetry.RecordSpanError()` for errors | Proper span error recording |
| `orchestration/tiered_capability_provider.go` | Include trace_id in error messages | Error debugging support |

---

## Logging Implementation Guide Compliance

This section validates the implementation against [docs/LOGGING_IMPLEMENTATION_GUIDE.md](../../docs/LOGGING_IMPLEMENTATION_GUIDE.md) **focusing on code implementation patterns**.

### ✅ Compliant

| Pattern | Status | Notes |
|---------|--------|-------|
| **WithContext Methods in Handlers** | ✅ | Uses `DebugWithContext`, `WarnWithContext`, `InfoWithContext` |
| **Component-Aware Logging** | ✅ | Uses `WithComponent("framework/orchestration/tiered")` |
| **Nil Logger Checks** | ✅ | Checks `if t.logger != nil` before logging |
| **Structured Fields** | ✅ | Uses `map[string]interface{}` for all log fields |

### ⚠️ Required Code Fixes

#### 1. Standardize Field Names

**Reference**: `docs/LOGGING_IMPLEMENTATION_GUIDE.md` lines 699-733 (Standard Field Names)

**Issue**: Some field names don't follow the standard naming convention.

| Current Field | Standard Name | Change Required |
|---------------|---------------|-----------------|
| `tool_count` | `total_count` or keep | Acceptable |
| `threshold` | `threshold` | OK |
| `reduction` | `reduction_percent` | Clarify unit |
| `invalid_tools` | `invalid_items` | Consider generic name |

**Fix**: Use standard field names from the guide:

```go
// Recommended standard fields
t.logger.InfoWithContext(ctx, "Tier 1 tool selection complete", map[string]interface{}{
    "operation":    "tiered_selection",         // Standard: operation name
    "status":       "success",                  // Standard: result status
    "duration_ms":  tier1Duration.Milliseconds(), // Standard: timing
    "total_count":  len(summaries),             // Count of input items
    "selected_count": len(selectedTools),       // Count of output items
    "reduction_percent": fmt.Sprintf("%.1f", reduction*100),
})

// For errors
t.logger.ErrorWithContext(ctx, "Tool selection failed", map[string]interface{}{
    "operation":   "tiered_selection",
    "error":       err.Error(),                 // Standard: error message
    "error_type":  "llm_selection_failure",     // Standard: error classification
    "duration_ms": duration.Milliseconds(),
})
```

#### 2. Log Both Success and Failure Paths

**Reference**: `docs/LOGGING_IMPLEMENTATION_GUIDE.md` lines 1386-1424 (Log Both Paths)

**Issue**: The `validateAndFilterTools()` function only logs warnings for invalid tools, not success cases.

**Fix**: Add success logging:

```go
func (t *TieredCapabilityProvider) validateAndFilterTools(
    ctx context.Context,
    selectedTools []string,
    summaries []CapabilitySummary,
) []string {
    // ... filtering logic ...

    if len(invalid) > 0 && t.logger != nil {
        t.logger.WarnWithContext(ctx, "LLM selected non-existent tools (hallucination)", map[string]interface{}{
            "operation":      "tool_validation",
            "invalid_count":  len(invalid),
            "invalid_tools":  invalid,
            "valid_count":    len(filtered),
        })
    }

    // Add success case logging
    if t.logger != nil {
        t.logger.DebugWithContext(ctx, "Tool validation complete", map[string]interface{}{
            "operation":      "tool_validation",
            "status":         "complete",
            "selected_count": len(selectedTools),
            "valid_count":    len(filtered),
            "hallucination_rate": fmt.Sprintf("%.1f%%", float64(len(invalid))/float64(len(selectedTools))*100),
        })
    }

    return filtered
}
```

#### 3. Factory Logging with Standard Fields

**Reference**: `docs/LOGGING_IMPLEMENTATION_GUIDE.md` lines 806-807, 1290-1291

**Issue**: Factory logs at lines 806 and 1290 use basic `Info()` method (correct for factory/startup), but should include standard fields.

**Current Code**:
```go
factoryLogger.Info("Using TieredCapabilityProvider for token optimization", map[string]interface{}{
    "min_tools": config.TieredResolution.MinToolsForTiering,
})
```

**Fix**: Include `operation` field for consistency:

```go
factoryLogger.Info("Using TieredCapabilityProvider for token optimization", map[string]interface{}{
    "operation": "orchestrator_initialization",  // Add operation field
    "min_tools": config.TieredResolution.MinToolsForTiering,
})
```

#### 4. Document Initialization Order

**Reference**: `docs/LOGGING_IMPLEMENTATION_GUIDE.md` lines 1046-1065 (Initialization Order)

**Requirement**: Add documentation note about telemetry initialization order:

```go
// orchestration/tiered_capability_provider.go - Add to package documentation

/*
INITIALIZATION ORDER REQUIREMENT:

Telemetry must be initialized BEFORE creating agents that use TieredCapabilityProvider.
This ensures:
1. The logger receives trace context for correlation
2. Metrics are properly recorded
3. Spans are visible in Jaeger

Example:
    func main() {
        // 1. Initialize telemetry FIRST
        initTelemetry("my-agent")

        // 2. Create agent (which may use TieredCapabilityProvider)
        agent, _ := NewMyAgent()

        // 3. Create and start Framework
        framework, _ := core.NewFramework(agent, ...)
        framework.Run(ctx)
    }

See: docs/LOGGING_IMPLEMENTATION_GUIDE.md section "AI Module Logger Propagation"
*/
```

### Summary of Logging Guide Required Code Changes

| File | Change | Pattern Reference |
|------|--------|-------------------|
| `orchestration/tiered_capability_provider.go` | Use standard field names (`operation`, `status`, `duration_ms`) | Standard field naming |
| `orchestration/tiered_capability_provider.go` | Add success logging to `validateAndFilterTools()` | Log both paths |
| `orchestration/factory.go` | Add `operation` field to factory logs | Standard field naming |
| `orchestration/tiered_capability_provider.go` | Add package documentation about initialization order | Initialization requirements |

---

## LLM Debug Payload Design Compliance

This section validates the implementation against [orchestration/notes/LLM_DEBUG_PAYLOAD_DESIGN.md](LLM_DEBUG_PAYLOAD_DESIGN.md) to ensure complete observability of LLM interactions.

### Context

The LLM Debug Payload Design defines 6 recording sites for LLM interactions:

| Existing Site | Component | Purpose |
|---------------|-----------|---------|
| `plan_generation` | Orchestrator | Initial plan generation |
| `correction` | Orchestrator | Plan correction after failures |
| `synthesis` | Synthesizer | Final response synthesis |
| `synthesis_streaming` | Orchestrator | Streaming synthesis |
| `micro_resolution` | MicroResolver | Micro-step resolution |
| `semantic_retry` | ContextualReResolver | Semantic retry |

### New Recording Site: `tiered_selection`

The TieredCapabilityProvider introduces a **7th LLM call site** that must be integrated with the LLM Debug Store for complete observability.

| New Site | Component | Purpose |
|----------|-----------|---------|
| `tiered_selection` | TieredCapabilityProvider | Tier 1 tool selection LLM call |

### ✅ Implementation (Included in This Design)

#### 1. Struct Fields (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.6)

```go
type TieredCapabilityProvider struct {
    // ... existing fields ...

    // LLM Debug Store integration
    debugStore LLMDebugStore     // For recording LLM interactions
    debugWg    sync.WaitGroup    // Tracks in-flight recordings for graceful shutdown
    debugSeqID atomic.Uint64     // For generating unique fallback IDs
}
```

#### 2. Setter Method (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.2-4.4)

```go
func (t *TieredCapabilityProvider) SetLLMDebugStore(store LLMDebugStore) {
    t.debugStore = store
}
```

#### 3. Recording Helper (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.1)

```go
func (t *TieredCapabilityProvider) recordDebugInteraction(ctx context.Context, interaction LLMInteraction) {
    if t.debugStore == nil {
        return
    }
    // Async recording with WaitGroup tracking
    // See implementation in Phase 2.1 Constructor section
}
```

#### 4. Recording in selectRelevantTools (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.1)

```go
// In selectRelevantTools() - after LLM call:
t.recordDebugInteraction(ctx, LLMInteraction{
    Type:             "tiered_selection",  // NEW 7th recording site
    Timestamp:        llmStartTime,
    DurationMs:       llmDuration.Milliseconds(),
    Prompt:           prompt,
    Temperature:      0.0,
    MaxTokens:        500,
    Model:            response.Model,
    Provider:         response.Provider,    // Per Phase 1f
    Response:         response.Content,
    PromptTokens:     response.Usage.PromptTokens,
    CompletionTokens: response.Usage.CompletionTokens,
    TotalTokens:      response.Usage.TotalTokens,
    Success:          true,
    Attempt:          1,
})
```

#### 5. Shutdown Method (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.6)

```go
func (t *TieredCapabilityProvider) Shutdown(ctx context.Context) error {
    // Wait for pending debug recordings with timeout
    // See implementation in Phase 2.1 Constructor section
}
```

#### 6. Factory Wiring (Per LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.5)

```go
// orchestration/factory.go - in CreateOrchestrator()
if config.LLMDebugStore != nil {
    // ... existing propagation ...

    // NEW: Propagate to TieredCapabilityProvider
    if tieredProvider, ok := orchestrator.capabilityProvider.(*TieredCapabilityProvider); ok {
        tieredProvider.SetLLMDebugStore(config.LLMDebugStore)
    }
}
```

### Updated Recording Sites Table

After this implementation, the complete list of LLM recording sites is:

| Site | Component | Description |
|------|-----------|-------------|
| `plan_generation` | Orchestrator | Initial plan generation from LLM |
| `correction` | Orchestrator | Plan correction after tool failures |
| `synthesis` | Synthesizer | Final response synthesis (non-streaming) |
| `synthesis_streaming` | Orchestrator | Final response synthesis (streaming) |
| `micro_resolution` | MicroResolver | Micro-step resolution |
| `semantic_retry` | ContextualReResolver | Semantic retry with full context |
| **`tiered_selection`** | **TieredCapabilityProvider** | **Tier 1 tool selection LLM call** |

### Summary of LLM Debug Design Required Code Changes

| File | Change | Pattern Reference |
|------|--------|-------------------|
| `orchestration/tiered_capability_provider.go` | Add `debugStore`, `debugWg`, `debugSeqID` fields | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.6 |
| `orchestration/tiered_capability_provider.go` | Add `SetLLMDebugStore()` method | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.2-4.4 |
| `orchestration/tiered_capability_provider.go` | Add `recordDebugInteraction()` helper | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.1 |
| `orchestration/tiered_capability_provider.go` | Record `tiered_selection` in `selectRelevantTools()` | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.1 |
| `orchestration/tiered_capability_provider.go` | Add `Shutdown()` method for graceful shutdown | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.6 |
| `orchestration/factory.go` | Propagate debug store to TieredCapabilityProvider | LLM_DEBUG_PAYLOAD_DESIGN.md Section 4.5 |

---

## References

### Primary Research (2025)

1. **RAG-MCP** - [Mitigating Prompt Bloat in LLM Tool Selection](https://arxiv.org/html/2505.03275v1) (May 2025) - Core research validating two-step approach
2. **Guided-Structured Templates** - [Improving LLMs Function Calling](https://arxiv.org/html/2509.18076v1) (September 2025) - 3-12% accuracy improvements
3. **LLM-Based Agents Survey** - [Tool Learning Survey](https://link.springer.com/article/10.1007/s41019-025-00296-9) (Springer, June 2025) - Planner/Caller/Summarizer pattern
4. **MCP Hierarchical Tools** - [GitHub Discussion #532](https://github.com/orgs/modelcontextprotocol/discussions/532) (November 2025) - Official MCP scaling solutions
5. **MCP 2025-11-25 Spec** - [Specification Update](https://modelcontextprotocol.io/specification/2025-11-25) - Dynamic tool loading

### Foundational Research (2024)

6. **Less is More** - [Optimizing Function Calling for LLM Execution](https://arxiv.org/html/2411.15399v1) (November 2024) - Tool count degradation study
7. **Glean Research** - [Input Token Count vs LLM Latency](https://www.glean.com/blog/glean-input-token-llm-latency) (December 2024) - 0.24ms/token formula

### Benchmarks (January 2026)

8. **Vellum LLM Leaderboard** - [Latest TTFT Benchmarks](https://www.vellum.ai/llm-leaderboard) - Model latency comparisons
9. **Artificial Analysis** - [LLM Leaderboard](https://artificialanalysis.ai/leaderboards/models) - Throughput and pricing
