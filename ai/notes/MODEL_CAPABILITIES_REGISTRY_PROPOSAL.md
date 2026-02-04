# Model Capabilities Registry Proposal

**Date**: 2026-02-03
**Status**: Approved (Phased Implementation)
**Author**: AI Assistant
**Related Issue**: OpenAI GPT-5 / Reasoning Model API Compatibility

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
   - [Immediate Issue](#immediate-issue)
   - [Verified API Behavior (2026-02-03)](#verified-api-behavior-2026-02-03)
   - [Architectural Concern](#architectural-concern)
3. [Implementation Strategy](#implementation-strategy)
   - [Decision: Phased Approach](#decision-phased-approach)
   - [Why Not Full Registry Now?](#why-not-full-registry-now)
   - [When to Upgrade to Registry Pattern](#when-to-upgrade-to-registry-pattern)
4. [CURRENT: Simple Reasoning Model Detection (Phase 1)](#current-simple-reasoning-model-detection-phase-1)
   - [Implementation](#implementation)
   - [Files Changed](#files-changed)
   - [Unit Tests](#unit-tests-phase-1)
   - [Effort Estimate](#effort-estimate-phase-1)
5. [FUTURE: Model Capabilities Registry Pattern (Phase 2+)](#future-model-capabilities-registry-pattern-phase-2)
   - [Design Principles](#design-principles)
   - [Data Structure](#data-structure)
   - [Capability Lookup Function](#capability-lookup-function)
     - [Interface for Testability (5.1)](#interface-for-testability-51)
     - [Deterministic Longest-Prefix-Match (5.2)](#deterministic-longest-prefix-match-52)
     - [Startup Validation (5.3)](#startup-validation-53)
     - [Complete Lookup Implementation](#complete-lookup-implementation)
     - [Example: Longest-Prefix-Match in Action](#example-longest-prefix-match-in-action)
   - [Request Builder Integration](#request-builder-integration)
   - [Updated GenerateResponse Method](#updated-generateresponse-method)
   - [Updated StreamResponse Method](#updated-streamresponse-method)
   - [File Structure](#file-structure)
   - [Testing Strategy](#testing-strategy-registry)
6. [BUG FIX: Reasoning Models Return Empty Content](#bug-fix-reasoning-models-return-empty-content)
   - [Bug Description](#bug-description-reasoning)
   - [Root Cause](#root-cause-reasoning)
   - [Implementation](#implementation-reasoning)
   - [Files Changed](#files-changed-reasoning)
   - [Verification](#verification-reasoning)
7. [BUG FIX: Missing Distributed Trace Correlation](#bug-fix-missing-distributed-trace-correlation-in-ai-provider-logs)
   - [Bug Description](#bug-description)
   - [Root Cause](#root-cause)
   - [Impact](#impact)
   - [Audit Results](#audit-results)
   - [Fix: Files Requiring Updates](#fix-files-requiring-updates)
8. [BUG FIX: Hardcoded HTTP Timeout Causing Reasoning Model Failures](#bug-fix-hardcoded-http-timeout-causing-reasoning-model-failures)
   - [Issue Summary](#issue-summary)
   - [Root Cause Analysis](#root-cause-analysis)
   - [Production Evidence](#production-evidence)
   - [Affected Files](#affected-files)
   - [Proposed Fix](#proposed-fix)
   - [Implementation Plan](#implementation-plan)
9. [IMPROVEMENT: Context-Aware Logging for Reasoning Model Debug](#improvement-context-aware-logging-for-reasoning-model-debug)
   - [Overview](#overview)
   - [Changes Made](#changes-made)
   - [Logging Field Standards](#logging-field-standards)
   - [Trace Correlation Details](#trace-correlation-details)
   - [Filtering Reasoning Model Logs](#filtering-reasoning-model-logs)
10. [Multi-Provider Research (Reference)](#multi-provider-research-reference)
   - [Research Findings (2026-02-03)](#research-findings-2026-02-03---comprehensive-multi-provider-analysis)
   - [OpenAI Reasoning Models](#openai-reasoning-models-o1-o3-o4-mini-gpt-5-family)
   - [DeepSeek Reasoning Models](#deepseek-reasoning-models-r1-deepseek-reasoner)
   - [xAI Grok Models](#xai-grok-models-grok-3-grok-4)
   - [AWS Bedrock](#aws-bedrock-parameter-format-differences)
   - [Anthropic Claude Models](#anthropic-claude-model-capabilities)
   - [Google Gemini Models](#google-gemini-model-capabilities)
10. [References](#references)
11. [Appendix: Verified API Test Results](#appendix-verified-api-test-results)

---

## Executive Summary

This document addresses **OpenAI reasoning model API compatibility** issues where GPT-5, o1, o3, and o4 models reject standard parameters (`max_tokens`, `temperature`).

### Implementation Decision

After analysis, we're taking a **phased approach**:

| Phase | What | When | Effort |
|-------|------|------|--------|
| **Phase 1 (NOW)** | Simple `isReasoningModel()` function | Immediate | ~30 lines |
| **Phase 2 (FUTURE)** | Full Registry Pattern | When complexity increases | ~500+ lines |

### Why This Approach?

**Current State**: All OpenAI reasoning models have the **same restrictions** (reject `max_tokens`, `temperature`). A simple prefix check is sufficient.

**Future State**: When models have **varied restrictions** or we need **runtime configuration**, we'll upgrade to the registry pattern documented in this proposal.

### Current Provider Status

| Provider | Has Breaking Issues? | Needs Fix Now? | Notes |
|----------|---------------------|----------------|-------|
| **OpenAI** | ✅ Yes | ✅ **YES** | GPT-5/o1/o3/o4 reject params |
| **Anthropic** | ⚠️ Theoretical | ❌ No | Current client doesn't send `top_p` |
| **Gemini** | ⚠️ Future | ❌ No | Thinking features not implemented |
| **DeepSeek** | ✅ Yes | ❌ No | Params accepted but ignored (no error) |

### Additional Finding

During investigation, an audit revealed ~53 logging calls across AI providers don't use `WithContext` variants, breaking distributed tracing correlation. This is documented for future fix.

---

## Problem Statement

### Immediate Issue

The OpenAI provider in `ai/providers/openai/client.go` sends parameters that OpenAI reasoning models (GPT-5, o1, o3, o4) reject:

```go
// Current code (lines 106-111) - FAILS for reasoning models
reqBody := map[string]interface{}{
    "model":       options.Model,
    "messages":    messages,
    "temperature": options.Temperature,  // ❌ ERROR: only value 1 supported
    "max_tokens":  options.MaxTokens,    // ❌ ERROR: use max_completion_tokens
}
```

### Verified API Behavior (2026-02-03)

| Parameter               | OpenAI GPT-5/o-series | DeepSeek-reasoner  |
| ----------------------- | --------------------- | ------------------ |
| `max_tokens`            | **ERROR**             | Works              |
| `max_completion_tokens` | **Required**          | Not used           |
| `temperature` ≠ 1       | **ERROR**             | Ignored (no error) |
| `stream_options`        | Works                 | Works              |

### Architectural Concern

A naive fix using if-else chains will become unmaintainable **if** different models have different restrictions:

```go
// ❌ ANTI-PATTERN: Spaghetti code (only bad if restrictions vary per model)
func buildRequestBody(...) {
    if isOpenAIReasoning(model) {
        // OpenAI reasoning params
    } else if isDeepSeekReasoning(model) {
        // DeepSeek reasoning params
    } else if isGroqSpecialModel(model) {
        // Groq params
    } else if isXAIReasoning(model) {
        // xAI params
    }
    // ... endless growth as providers evolve
}
```

**However**: Currently, all OpenAI reasoning models have the **same restrictions**. A simple approach is sufficient for now.

---

## Implementation Strategy

### Decision: Phased Approach

After careful analysis, we've decided on a **pragmatic phased approach**:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        IMPLEMENTATION TIMELINE                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  NOW                           FUTURE (when needed)                     │
│   │                                   │                                 │
│   ▼                                   ▼                                 │
│  ┌──────────────────────┐    ┌──────────────────────────────────────┐  │
│  │  Phase 1: Simple     │    │  Phase 2: Registry Pattern           │  │
│  │  isReasoningModel()  │───▶│  - Per-model capability flags        │  │
│  │  ~30 lines           │    │  - Provider overrides                │  │
│  │                      │    │  - Runtime configuration             │  │
│  └──────────────────────┘    └──────────────────────────────────────┘  │
│                                                                         │
│  Trigger: All reasoning       Trigger: Models have DIFFERENT           │
│  models have SAME             restrictions, or need runtime config     │
│  restrictions                                                           │
└─────────────────────────────────────────────────────────────────────────┘
```

### Why Not Full Registry Now?

| Factor | Simple Approach | Registry Pattern |
|--------|-----------------|------------------|
| **Lines of Code** | ~30 | ~500+ |
| **Time to Implement** | Quick | Longer |
| **Complexity** | Low | Medium |
| **Current Need** | ✅ Sufficient | Over-engineered |
| **Flexibility** | Low | High |
| **Maintenance** | Low | Higher (registry updates) |

**Key Insight**: All OpenAI reasoning models (GPT-5, o1, o3, o4) have **identical restrictions**:
- Reject `max_tokens` → use `max_completion_tokens`
- Reject `temperature` (only value `1` allowed) → omit parameter

A simple prefix check handles this perfectly.

### When to Upgrade to Registry Pattern

Upgrade to the full registry pattern when ANY of these become true:

| Trigger | Example |
|---------|---------|
| Different models have different restrictions | GPT-5 allows streaming, o1 doesn't |
| Need per-model capability flags (5+) | `NoLogprobs`, `NoTopP`, `FixedTemperature`, etc. |
| Need runtime configuration | Environment variable overrides |
| Need provider-specific overrides | Same model behaves differently on Groq vs OpenAI |
| Multiple providers have breaking issues | Anthropic starts rejecting params |

---

## CURRENT: Simple Reasoning Model Detection (Phase 1)

> **Status**: ✅ IMPLEMENT NOW
>
> **Effort**: ~30 lines of code
>
> **Fixes**: OpenAI GPT-5, o1, o3, o4 parameter rejection

### Implementation

Create a simple helper function and integrate it into the request building:

```go
// ai/providers/openai/reasoning.go

package openai

import "strings"

// OpenAI reasoning model prefixes - all have same parameter restrictions:
// - Reject max_tokens (use max_completion_tokens instead)
// - Reject temperature != 1 (omit parameter entirely)
var reasoningModelPrefixes = []string{
    "gpt-5",    // GPT-5 family (gpt-5, gpt-5-mini, gpt-5-turbo, etc.)
    "o1",       // o1 family (o1, o1-preview, o1-mini, etc.)
    "o3",       // o3 family
    "o4",       // o4 family
}

// IsReasoningModel returns true if the model is an OpenAI reasoning model
// that requires special parameter handling.
//
// Reasoning models:
// - Reject 'max_tokens' parameter (use 'max_completion_tokens' instead)
// - Reject 'temperature' values other than 1 (omit parameter entirely)
// - May have limited streaming support (o1-preview, o1-mini)
func IsReasoningModel(model string) bool {
    modelLower := strings.ToLower(model)
    for _, prefix := range reasoningModelPrefixes {
        if strings.HasPrefix(modelLower, prefix) {
            return true
        }
    }
    return false
}
```

Update the client to use this helper:

```go
// ai/providers/openai/client.go - Update GenerateResponse and StreamResponse

func (c *Client) buildRequestBody(model string, messages []map[string]string,
                                   options *core.AIOptions, streaming bool) map[string]interface{} {
    reqBody := map[string]interface{}{
        "model":    model,
        "messages": messages,
    }

    // Handle reasoning model parameter restrictions
    if IsReasoningModel(model) {
        // Reasoning models require max_completion_tokens instead of max_tokens
        reqBody["max_completion_tokens"] = options.MaxTokens
        // Temperature is omitted - reasoning models only support default (1)

        if c.Logger != nil {
            c.Logger.Debug("Using reasoning model parameters", map[string]interface{}{
                "model":                     model,
                "using_max_completion_tokens": true,
                "temperature_omitted":        true,
            })
        }
    } else {
        // Standard models use traditional parameters
        reqBody["max_tokens"] = options.MaxTokens
        reqBody["temperature"] = options.Temperature
    }

    // Streaming configuration
    if streaming {
        reqBody["stream"] = true
        reqBody["stream_options"] = map[string]interface{}{
            "include_usage": true,
        }
    }

    return reqBody
}
```

### Files Changed

```
ai/providers/openai/
├── reasoning.go        # NEW: IsReasoningModel() helper (~25 lines)
├── reasoning_test.go   # NEW: Unit tests (~40 lines)
└── client.go           # MODIFIED: Use buildRequestBody() helper
```

### Unit Tests (Phase 1)

```go
// ai/providers/openai/reasoning_test.go

package openai

import "testing"

func TestIsReasoningModel(t *testing.T) {
    tests := []struct {
        model    string
        expected bool
    }{
        // Reasoning models (should return true)
        {"gpt-5", true},
        {"gpt-5-mini", true},
        {"gpt-5-mini-2025-08-07", true},
        {"GPT-5-TURBO", true},  // Case insensitive
        {"o1", true},
        {"o1-preview", true},
        {"o1-preview-2024-09-12", true},
        {"o1-mini", true},
        {"o3", true},
        {"o3-mini", true},
        {"o4", true},
        {"o4-mini", true},

        // Standard models (should return false)
        {"gpt-4", false},
        {"gpt-4o", false},
        {"gpt-4o-mini", false},
        {"gpt-4-turbo", false},
        {"gpt-3.5-turbo", false},
        {"deepseek-chat", false},
        {"deepseek-reasoner", false},  // DeepSeek accepts params, just ignores
        {"llama-3.3-70b", false},
        {"claude-3-opus", false},  // Wrong provider anyway
        {"unknown-model", false},
    }

    for _, tt := range tests {
        t.Run(tt.model, func(t *testing.T) {
            got := IsReasoningModel(tt.model)
            if got != tt.expected {
                t.Errorf("IsReasoningModel(%q) = %v, want %v", tt.model, got, tt.expected)
            }
        })
    }
}

func TestBuildRequestBody_ReasoningModel(t *testing.T) {
    client := &Client{providerAlias: "openai"}

    // Test reasoning model
    reqBody := client.buildRequestBody("gpt-5-mini",
        []map[string]string{{"role": "user", "content": "test"}},
        &core.AIOptions{MaxTokens: 100, Temperature: 0.7},
        false)

    // Should use max_completion_tokens, not max_tokens
    if _, ok := reqBody["max_completion_tokens"]; !ok {
        t.Error("Expected max_completion_tokens for reasoning model")
    }
    if _, ok := reqBody["max_tokens"]; ok {
        t.Error("Did not expect max_tokens for reasoning model")
    }
    // Temperature should be omitted
    if _, ok := reqBody["temperature"]; ok {
        t.Error("Did not expect temperature for reasoning model")
    }
}

func TestBuildRequestBody_StandardModel(t *testing.T) {
    client := &Client{providerAlias: "openai"}

    // Test standard model
    reqBody := client.buildRequestBody("gpt-4o",
        []map[string]string{{"role": "user", "content": "test"}},
        &core.AIOptions{MaxTokens: 100, Temperature: 0.7},
        false)

    // Should use max_tokens, not max_completion_tokens
    if _, ok := reqBody["max_tokens"]; !ok {
        t.Error("Expected max_tokens for standard model")
    }
    if _, ok := reqBody["max_completion_tokens"]; ok {
        t.Error("Did not expect max_completion_tokens for standard model")
    }
    // Temperature should be included
    if _, ok := reqBody["temperature"]; !ok {
        t.Error("Expected temperature for standard model")
    }
}
```

### Effort Estimate (Phase 1)

| Task | Lines | Time |
|------|-------|------|
| Create `reasoning.go` | ~25 | Quick |
| Create `reasoning_test.go` | ~60 | Quick |
| Update `client.go` (buildRequestBody) | ~20 | Quick |
| Update `GenerateResponse` to use helper | ~5 | Quick |
| Update `StreamResponse` to use helper | ~5 | Quick |
| **Total** | **~115** | **Quick** |

---

## FUTURE: Model Capabilities Registry Pattern (Phase 2+)

> **Status**: 📋 DOCUMENTED FOR FUTURE USE
>
> **When to Implement**: When triggers in [When to Upgrade](#when-to-upgrade-to-registry-pattern) are met
>
> **Effort**: ~500+ lines of code

This section documents the full registry pattern for future reference when the simple approach becomes insufficient.

### Design Principles

1. **Declarative over Imperative**: Define model rules as data, not code logic
2. **Single Source of Truth**: One registry for all model parameter rules
3. **Prefix Matching**: Support model families (e.g., "gpt-5" matches "gpt-5-mini-2025-08-07")
4. **Provider Scoping**: Rules can be global or provider-specific
5. **Safe Defaults**: Unknown models use standard OpenAI-compatible parameters

### Data Structure

```go
// ai/providers/openai/capabilities.go

package openai

// ModelCapabilities defines parameter handling rules for specific models
type ModelCapabilities struct {
    // Token limit parameter
    UseMaxCompletionTokens bool // Use max_completion_tokens instead of max_tokens

    // Temperature handling
    OmitTemperature bool     // Don't send temperature parameter at all
    FixedTemperature *float64 // If set, only this value is allowed (omit if different)

    // Streaming support
    NoStreaming      bool // Model doesn't support streaming at all
    NoStreamOptions  bool // Don't send stream_options even when streaming

    // Other restrictions
    NoLogprobs          bool // Don't send logprobs/top_logprobs
    NoFrequencyPenalty  bool // Don't send frequency_penalty
    NoPresencePenalty   bool // Don't send presence_penalty
    NoTopP              bool // Don't send top_p

    // Informational (for logging/debugging)
    IsReasoningModel bool   // Model uses chain-of-thought reasoning
    Notes            string // Human-readable notes about restrictions
}

// modelCapabilitiesRegistry maps model prefixes to their capabilities
// Models not listed use standard OpenAI-compatible parameters
var modelCapabilitiesRegistry = map[string]ModelCapabilities{
    // ===================
    // OpenAI Reasoning Models (Verified 2026-02-03)
    // ===================
    // These models REJECT max_tokens and temperature != 1

    "gpt-5": {
        UseMaxCompletionTokens: true,
        OmitTemperature:        true,
        IsReasoningModel:       true,
        Notes:                  "GPT-5 family rejects max_tokens, temperature must be 1",
    },
    "o1": {
        UseMaxCompletionTokens: true,
        OmitTemperature:        true,
        NoStreaming:            true,  // o1 doesn't support streaming
        IsReasoningModel:       true,
        Notes:                  "o1 family: no streaming, rejects max_tokens/temperature",
    },
    "o3": {
        UseMaxCompletionTokens: true,
        OmitTemperature:        true,
        IsReasoningModel:       true,
        Notes:                  "o3 family rejects max_tokens, temperature must be 1",
    },
    "o4": {
        UseMaxCompletionTokens: true,
        OmitTemperature:        true,
        IsReasoningModel:       true,
        Notes:                  "o4 family rejects max_tokens, temperature must be 1",
    },

    // ===================
    // DeepSeek Models (Verified 2026-02-03 from official docs)
    // ===================
    // DeepSeek-reasoner ACCEPTS standard params but IGNORES some silently
    // No special handling needed - standard params work fine
    // Documenting for completeness:
    //
    // "deepseek-reasoner": {
    //     // Uses max_tokens (standard)
    //     // temperature, top_p, penalties: accepted but ignored
    //     // logprobs: triggers error
    //     NoLogprobs:       true,
    //     IsReasoningModel: true,
    //     Notes:            "Accepts standard params, ignores temp/top_p/penalties",
    // },

    // ===================
    // Standard Models (not listed - use defaults)
    // ===================
    // gpt-4, gpt-4o, gpt-4o-mini, gpt-4-turbo: standard params
    // deepseek-chat: standard params
    // llama-*, mistral-*, etc: standard params
}

// providerCapabilityOverrides allows provider-specific overrides
// Use when a model has different behavior on different providers
var providerCapabilityOverrides = map[string]map[string]ModelCapabilities{
    // Example: If Groq ever has special handling for a model
    // "openai.groq": {
    //     "llama-4-reasoning": {OmitTemperature: true},
    // },
}
```

### Capability Lookup Function

#### Interface for Testability (5.1)

Define an interface to enable mocking in tests and support future extensibility:

```go
// CapabilityProvider abstracts capability lookup for testing and extensibility
type CapabilityProvider interface {
    GetCapabilities(providerAlias, model string) ModelCapabilities
}

// DefaultCapabilityProvider implements CapabilityProvider using the in-memory registry
type DefaultCapabilityProvider struct{}

// Ensure DefaultCapabilityProvider implements the interface
var _ CapabilityProvider = (*DefaultCapabilityProvider)(nil)
```

This enables:
- **Unit testing**: Mock the provider to test specific capability scenarios
- **Future extensibility**: Swap implementations (e.g., load from config file, remote registry)
- **Dependency injection**: Pass provider to Client for easier testing

#### Deterministic Longest-Prefix-Match (5.2)

Go maps have no guaranteed iteration order. Without sorting, the prefix "o1" might be matched before "o1-preview", causing incorrect behavior. We use `sync.Once` to sort prefixes once at first lookup:

```go
import (
    "sort"
    "strings"
    "sync"
)

var (
    // sortedPrefixes holds prefixes sorted by length (longest first)
    // This ensures "o1-preview" matches before "o1"
    sortedPrefixes     []string
    sortedPrefixesOnce sync.Once

    // sortedProviderPrefixes holds per-provider sorted prefixes
    sortedProviderPrefixes     map[string][]string
    sortedProviderPrefixesOnce sync.Once
)

// initSortedPrefixes initializes the sorted prefix list (called once via sync.Once)
func initSortedPrefixes() {
    sortedPrefixes = make([]string, 0, len(modelCapabilitiesRegistry))
    for prefix := range modelCapabilitiesRegistry {
        sortedPrefixes = append(sortedPrefixes, prefix)
    }
    // Sort by length descending - longer prefixes first for longest-prefix-match
    sort.Slice(sortedPrefixes, func(i, j int) bool {
        return len(sortedPrefixes[i]) > len(sortedPrefixes[j])
    })
}

// initSortedProviderPrefixes initializes per-provider sorted prefix lists
func initSortedProviderPrefixes() {
    sortedProviderPrefixes = make(map[string][]string)
    for provider, overrides := range providerCapabilityOverrides {
        prefixes := make([]string, 0, len(overrides))
        for prefix := range overrides {
            prefixes = append(prefixes, prefix)
        }
        sort.Slice(prefixes, func(i, j int) bool {
            return len(prefixes[i]) > len(prefixes[j])
        })
        sortedProviderPrefixes[provider] = prefixes
    }
}
```

**Why sync.Once?**
- Thread-safe initialization (important for concurrent requests)
- Zero overhead after first call (just an atomic load)
- Lazy initialization - no startup cost if registry isn't used

#### Startup Validation (5.3)

Detect configuration errors early by validating prefix relationships at startup:

```go
import "log"

func init() {
    // Validate registry: detect shadowing prefixes
    // e.g., if both "o1" and "o1-preview" exist, warn that order matters
    for p1 := range modelCapabilitiesRegistry {
        for p2 := range modelCapabilitiesRegistry {
            if p1 != p2 && strings.HasPrefix(p1, p2) {
                // p2 is a prefix of p1, meaning p2 could shadow p1 in naive iteration
                // With sorted longest-prefix-match, this is handled correctly,
                // but we warn in case of unintended overlap
                log.Printf("MODEL_CAPS_REGISTRY: Note - prefix %q is a prefix of %q (handled by longest-prefix-match)", p2, p1)
            }
        }
    }

    // Same validation for provider overrides
    for provider, overrides := range providerCapabilityOverrides {
        for p1 := range overrides {
            for p2 := range overrides {
                if p1 != p2 && strings.HasPrefix(p1, p2) {
                    log.Printf("MODEL_CAPS_REGISTRY: Note - provider %q: prefix %q is a prefix of %q", provider, p2, p1)
                }
            }
        }
    }
}
```

**Benefits**:
- Catches misconfiguration at startup (fail-fast)
- Documents intentional prefix relationships via logs
- No runtime overhead (runs once at init)

#### Complete Lookup Implementation

```go
// GetModelCapabilities returns the capabilities for a model, using longest-prefix matching.
// Falls back to empty capabilities (standard params) if no match found.
func (p *DefaultCapabilityProvider) GetCapabilities(providerAlias, model string) ModelCapabilities {
    modelLower := strings.ToLower(model)

    // Initialize sorted prefixes on first call (thread-safe)
    sortedPrefixesOnce.Do(initSortedPrefixes)
    sortedProviderPrefixesOnce.Do(initSortedProviderPrefixes)

    // Check provider-specific overrides first (longest prefix wins)
    if prefixes, ok := sortedProviderPrefixes[providerAlias]; ok {
        for _, prefix := range prefixes {
            if strings.HasPrefix(modelLower, prefix) {
                return providerCapabilityOverrides[providerAlias][prefix]
            }
        }
    }

    // Check global registry (only for vanilla OpenAI)
    // Other providers use standard params unless explicitly overridden
    if providerAlias == "openai" || providerAlias == "" {
        for _, prefix := range sortedPrefixes {
            if strings.HasPrefix(modelLower, prefix) {
                return modelCapabilitiesRegistry[prefix]
            }
        }
    }

    // Default: standard OpenAI-compatible parameters
    return ModelCapabilities{}
}

// Package-level convenience function using default provider
var defaultProvider = &DefaultCapabilityProvider{}

func GetModelCapabilities(providerAlias, model string) ModelCapabilities {
    return defaultProvider.GetCapabilities(providerAlias, model)
}

// Helper for checking if model is a reasoning model (for logging/telemetry)
func IsReasoningModel(providerAlias, model string) bool {
    return GetModelCapabilities(providerAlias, model).IsReasoningModel
}

// Helper for checking streaming support
func SupportsStreaming(providerAlias, model string) bool {
    return !GetModelCapabilities(providerAlias, model).NoStreaming
}
```

#### Example: Longest-Prefix-Match in Action

```
Registry entries: "o1", "o1-preview", "o1-mini"
Sorted order: ["o1-preview", "o1-mini", "o1"]  // longest first

Query: "o1-preview-2024-09-12"
  - Check "o1-preview" → ✅ MATCH (returns o1-preview capabilities)

Query: "o1-mini"
  - Check "o1-preview" → ❌ no match
  - Check "o1-mini" → ✅ MATCH (returns o1-mini capabilities)

Query: "o1"
  - Check "o1-preview" → ❌ no match
  - Check "o1-mini" → ❌ no match
  - Check "o1" → ✅ MATCH (returns o1 capabilities)
```

### Request Builder Integration

> **Note**: This is a simplified version showing the core logic. See **Phase 2: DEBUG Logging Implementation** for the full version with proper observability following `ai/providers/base.go` conventions.

```go
// buildRequestBody creates the appropriate request body based on model capabilities
// SIMPLIFIED VERSION - see Phase 2 for full implementation with logging
func (c *Client) buildRequestBody(ctx context.Context, model string, messages []map[string]string,
                                   options *core.AIOptions, streaming bool) map[string]interface{} {

    caps := GetModelCapabilities(c.providerAlias, model)

    reqBody := map[string]interface{}{
        "model":    model,
        "messages": messages,
    }

    // Token limit parameter
    if caps.UseMaxCompletionTokens {
        reqBody["max_completion_tokens"] = options.MaxTokens
    } else {
        reqBody["max_tokens"] = options.MaxTokens
    }

    // Temperature (omit for reasoning models that don't support it)
    if !caps.OmitTemperature {
        reqBody["temperature"] = options.Temperature
    }

    // Streaming
    if streaming {
        reqBody["stream"] = true
        if !caps.NoStreamOptions {
            reqBody["stream_options"] = map[string]interface{}{
                "include_usage": true,
            }
        }
    }

    return reqBody
}
```

### Updated GenerateResponse Method

```go
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
    // ... existing setup code ...

    // Build messages
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

    // Build request body using capabilities registry (ctx enables trace correlation)
    reqBody := c.buildRequestBody(ctx, options.Model, messages, options, false)

    jsonData, err := json.Marshal(reqBody)
    // ... rest of method unchanged ...
}
```

### Updated StreamResponse Method

```go
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // ... existing setup code ...

    // Check if model supports streaming
    if !SupportsStreaming(c.providerAlias, options.Model) {
        if c.Logger != nil {
            c.Logger.Info("Model does not support streaming, using non-streaming fallback", map[string]interface{}{
                "model":    options.Model,
                "provider": c.providerAlias,
            })
        }
        return c.GenerateResponse(ctx, prompt, options)
    }

    // Build messages
    messages := []map[string]string{}
    // ... message building ...

    // Build request body using capabilities registry (ctx enables trace correlation)
    reqBody := c.buildRequestBody(ctx, options.Model, messages, options, true)

    jsonData, err := json.Marshal(reqBody)
    // ... rest of method unchanged ...
}
```

### File Structure (Future Registry Pattern)

```
ai/providers/openai/
├── client.go            # Uses buildRequestBody()
├── capabilities.go      # FUTURE: ModelCapabilities registry and lookup
├── capabilities_test.go # FUTURE: Tests for capability lookup
├── reasoning.go         # CURRENT: Simple IsReasoningModel() helper
├── reasoning_test.go    # CURRENT: Tests for reasoning detection
├── models.go            # Existing model aliases (unchanged)
├── factory.go           # Unchanged
└── ...
```

### Testing Strategy (Registry)

See the Unit Tests section below for comprehensive test cases that should be implemented when upgrading to the registry pattern.

---

## BUG FIX: Reasoning Models Return Empty Content

> **Status**: ✅ IMPLEMENTED
>
> **Severity**: Critical
>
> **Discovered**: 2026-02-03 during production testing with GPT-5-mini
>
> **Fixed In**: `ai/providers/openai/reasoning.go`, `ai/providers/openai/models.go`, `ai/providers/openai/client.go`

### Bug Description (Reasoning)

OpenAI reasoning models (GPT-5, o1, o3, o4) were returning **empty or truncated content** when used with the gomind framework. Requests would succeed (HTTP 200) but the response content was empty or incomplete.

**Symptoms**:
- AI responses returned empty `Content` field
- Plan generation produced incomplete or empty plans
- No API errors - requests appeared successful
- Standard models (gpt-4, gpt-4o) worked fine with same prompts

### Root Cause (Reasoning)

**Three interconnected issues** caused this failure:

#### Issue 1: Wrong Token Limit Parameter

OpenAI reasoning models **reject** the standard `max_tokens` parameter and require `max_completion_tokens` instead:

```go
// ❌ REJECTED by reasoning models
{"model": "gpt-5-mini", "max_tokens": 2000, ...}
// Error: "Unsupported parameter: 'max_tokens' is not supported with this model"

// ✅ REQUIRED for reasoning models
{"model": "gpt-5-mini", "max_completion_tokens": 2000, ...}
```

#### Issue 2: Unsupported Temperature Parameter

Reasoning models **reject** non-default temperature values:

```go
// ❌ REJECTED by reasoning models
{"model": "gpt-5-mini", "temperature": 0.7, ...}
// Error: "Unsupported value: 'temperature' does not support 0.7 with this model"

// ✅ Must omit temperature entirely (defaults to 1)
{"model": "gpt-5-mini", ...}  // No temperature field
```

#### Issue 3: Token Exhaustion on Internal Reasoning

**Critical discovery**: In "thinking mode", reasoning models (GPT-5, o1, o3, o4) count their internal chain-of-thought reasoning tokens against `max_completion_tokens`, but **these reasoning tokens are NOT returned as part of the response**. This creates a hidden token consumption problem:

- The model uses tokens for internal reasoning (not visible in response)
- The model uses remaining tokens for actual output (visible in response)
- If reasoning exhausts the token limit, the visible output is empty or truncated

**Key insight**: The reasoning tokens are counted towards output tokens but are not sent in the response. Without a token multiplier, users get empty responses because all tokens were consumed by invisible reasoning.

```
Standard Model Token Budget:
┌────────────────────────────────┐
│        Output Content          │  ← All 2000 tokens for visible output
└────────────────────────────────┘

Reasoning Model Token Budget (without fix):
┌─────────────────────────────────────────────┐
│  Internal Reasoning (counted but NOT sent)  │  ← Uses 1800+ tokens
├─────────────────────────────────────────────┤
│  Visible Output (empty!)                    │  ← Only 200 tokens left → empty!
└─────────────────────────────────────────────┘

Reasoning Model Token Budget (with 5x multiplier fix):
┌─────────────────────────────────────────────┐
│  Internal Reasoning (counted but NOT sent)  │  ← ~4000 tokens for thinking
├─────────────────────────────────────────────┤
│  Visible Output Content                     │  ← ~6000 tokens for response
└─────────────────────────────────────────────┘
(10000 total tokens from 2000 * 5x multiplier)
```

#### Issue 4: Response Content Field Location

Reasoning models may return content in a different JSON field (`reasoning_content` instead of `content`):

```json
// Standard model response
{"choices": [{"message": {"role": "assistant", "content": "Hello!"}}]}

// Reasoning model response (may use reasoning_content)
{"choices": [{"message": {"role": "assistant", "reasoning_content": "Hello!"}}]}
```

### Implementation (Reasoning)

#### 1. Reasoning Model Detection (`reasoning.go`)

```go
// OpenAI reasoning model prefixes - all have same parameter restrictions
var reasoningModelPrefixes = []string{
    "gpt-5",    // GPT-5 family
    "o1",       // o1 family
    "o3",       // o3 family
    "o4",       // o4 family
}

// IsReasoningModel returns true if model requires special handling
func IsReasoningModel(model string) bool {
    modelLower := strings.ToLower(model)
    for _, prefix := range reasoningModelPrefixes {
        if strings.HasPrefix(modelLower, prefix) {
            return true
        }
    }
    return false
}
```

#### 2. Configurable Token Multiplier

The token multiplier is configurable with a default of **5x**. This can be overridden by agents:

```go
// Default multiplier (5x) - provides adequate tokens for reasoning + output
const DefaultReasoningTokenMultiplier = 5

// Example: If caller requests 2000 tokens, reasoning models get 2000 * 5 = 10000,
// ensuring sufficient tokens for both internal reasoning (~4000) and visible output (~6000).
```

**Configuration Options:**

```go
// Single provider client - use default 5x multiplier
client, _ := ai.NewClient()

// Single provider client - custom multiplier for cost optimization
client, _ := ai.NewClient(
    ai.WithReasoningTokenMultiplier(3),  // 3x multiplier (lower cost, may truncate)
)

// Chain client - use default 5x multiplier
chainClient, _ := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
)

// Chain client - custom multiplier
chainClient, _ := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainReasoningTokenMultiplier(4),  // 4x multiplier
)
```

**Why 5x default?**
- Reasoning models can use 60-80% of tokens for internal chain-of-thought
- 5x ensures ~20% of requested tokens available for visible output even in worst case
- Agents can reduce this for cost optimization if responses are simpler

#### 3. Request Body Builder (`reasoning.go`)

```go
func buildRequestBody(model string, messages []map[string]string,
                      maxTokens int, temperature float32, streaming bool) map[string]interface{} {
    reqBody := map[string]interface{}{
        "model":    model,
        "messages": messages,
    }

    if IsReasoningModel(model) {
        // Reasoning models require max_completion_tokens instead of max_tokens
        // Apply multiplier for internal reasoning + output token budget
        adjustedMaxTokens := maxTokens * ReasoningModelTokenMultiplier
        reqBody["max_completion_tokens"] = adjustedMaxTokens
        // Temperature is intentionally omitted for reasoning models
    } else {
        // Standard models use traditional parameters
        reqBody["max_tokens"] = maxTokens
        reqBody["temperature"] = temperature
    }

    if streaming {
        reqBody["stream"] = true
        reqBody["stream_options"] = map[string]interface{}{
            "include_usage": true,
        }
    }

    return reqBody
}
```

#### 4. Response Model Update (`models.go`)

Added `ReasoningContent` field to handle reasoning model responses:

```go
// Message represents a chat message
// For reasoning models (GPT-5, o1, o3, o4), content may be in ReasoningContent field
type Message struct {
    Role             string `json:"role"`
    Content          string `json:"content"`
    ReasoningContent string `json:"reasoning_content,omitempty"` // GPT-5/o-series
}

// StreamDelta represents the delta content in a streaming chunk
type StreamDelta struct {
    Role             string `json:"role,omitempty"`
    Content          string `json:"content,omitempty"`
    ReasoningContent string `json:"reasoning_content,omitempty"` // GPT-5/o-series
}
```

#### 5. Content Extraction in Client (`client.go`)

```go
// In GenerateResponse - extract content from either field
content := apiResp.Choices[0].Message.Content
if content == "" && apiResp.Choices[0].Message.ReasoningContent != "" {
    content = apiResp.Choices[0].Message.ReasoningContent
}

// In StreamResponse - extract chunk content from either field
chunk := choice.Delta.Content
if chunk == "" && choice.Delta.ReasoningContent != "" {
    chunk = choice.Delta.ReasoningContent
}
```

### Files Changed (Reasoning)

| File | Changes |
|------|---------|
| `ai/providers/openai/reasoning.go` | **NEW**: `IsReasoningModel()`, `ReasoningModelTokenMultiplier`, `buildRequestBody()` |
| `ai/providers/openai/reasoning_test.go` | **NEW**: Unit tests for all reasoning model functions |
| `ai/providers/openai/models.go` | **MODIFIED**: Added `ReasoningContent` field to `Message` and `StreamDelta` |
| `ai/providers/openai/client.go` | **MODIFIED**: Use `buildRequestBody()`, handle `ReasoningContent` extraction |

### Verification (Reasoning)

#### Unit Tests Added

```go
func TestIsReasoningModel(t *testing.T) {
    // Reasoning models (should return true)
    {"gpt-5", true},
    {"gpt-5-mini", true},
    {"gpt-5-mini-2025-08-07", true},
    {"o1", true}, {"o1-preview", true}, {"o1-mini", true},
    {"o3", true}, {"o3-mini", true},
    {"o4", true}, {"o4-mini", true},

    // Standard models (should return false)
    {"gpt-4", false}, {"gpt-4o", false}, {"gpt-4o-mini", false},
}

func TestBuildRequestBody_ReasoningModel(t *testing.T) {
    // Verify max_completion_tokens is used with multiplier
    // Verify temperature is omitted
    // Verify max_tokens is NOT present
}

func TestBuildRequestBody_StandardModel(t *testing.T) {
    // Verify max_tokens is used
    // Verify temperature is included
    // Verify max_completion_tokens is NOT present
}

func TestMessage_ReasoningContent(t *testing.T) {
    // Verify content extraction from ReasoningContent field
}
```

#### Production Verification

Tested with `travel-chat-agent` example using GPT-5-mini:

| Metric | Before Fix | After Fix |
|--------|------------|-----------|
| Plan generation | ❌ Empty/truncated | ✅ Complete plans |
| Token usage | N/A (request failed) | ~4000-6000 tokens |
| Response content | Empty string | Full response |
| Streaming | ❌ Empty chunks | ✅ Valid chunks |

### Key Design Decisions

1. **4x Token Multiplier**: Chosen based on empirical testing. Complex prompts like orchestration plans need significant internal reasoning. 4x provides adequate budget for most use cases while staying within model limits.

2. **Prefix-based Detection**: Simple and maintainable. All OpenAI reasoning models follow the `gpt-5`, `o1`, `o3`, `o4` naming pattern.

3. **Fallback Content Extraction**: Check both `Content` and `ReasoningContent` fields to handle API variations gracefully.

4. **No User-facing API Change**: The fix is transparent to framework users. Existing code using `ai.NewClient()` automatically benefits.

---

## BUG FIX: Missing Distributed Trace Correlation in AI Provider Logs

> **Status**: 📋 DOCUMENTED FOR FUTURE FIX
>
> **Severity**: Medium
>
> **Discovered**: 2026-02-03 during Model Capabilities Registry investigation

### Bug Description

Most logging calls across all AI providers don't use `WithContext` even though `ctx` is available in the method signature. This breaks distributed tracing correlation between Jaeger spans and application logs.

### Root Cause

When the AI providers were implemented, logging used non-context methods:

```go
c.Logger.Error("message", fields)  // ❌ No trace context
```

Instead of context-aware methods:

```go
c.Logger.ErrorWithContext(ctx, "message", fields)  // ✅ Includes trace_id, span_id
```

### Impact

| Scenario      | Jaeger Traces             | Logs                    | Correlation         |
| ------------- | ------------------------- | ----------------------- | ------------------- |
| **Current**   | ✅ Spans linked correctly | ❌ No trace_id/span_id  | ❌ Cannot correlate |
| **After Fix** | ✅ Spans linked correctly | ✅ Has trace_id/span_id | ✅ Full correlation |

### Audit Results

| File                  | Without Context | With Context | Has `ctx` Available |
| --------------------- | --------------- | ------------ | ------------------- |
| `base.go`             | 4 calls         | 8 calls      | Partial             |
| `openai/client.go`    | 12 calls        | 0 calls      | ✅ Yes              |
| `anthropic/client.go` | 13 calls        | 0 calls      | ✅ Yes              |
| `gemini/client.go`    | 14 calls        | 0 calls      | ✅ Yes              |
| `bedrock/client.go`   | 10 calls        | 0 calls      | ✅ Yes              |

**Total**: ~53 logging calls need migration to `WithContext` variants

### Fix: Files Requiring Updates

**1. base.go** - Update methods that have access to ctx:

```go
// LogRequest currently doesn't take ctx - needs signature change
func (b *BaseClient) LogRequest(ctx context.Context, provider, model, prompt string) {
    b.Logger.InfoWithContext(ctx, "AI request initiated", map[string]interface{}{...})
}
```

**2. All provider client.go files** - Change all logging calls:

```go
// Before:
c.Logger.Error("OpenAI request failed", map[string]interface{}{...})

// After:
c.Logger.ErrorWithContext(ctx, "OpenAI request failed", map[string]interface{}{...})
```

---

## BUG FIX: Hardcoded HTTP Timeout Causing Reasoning Model Failures

> **Status**: ✅ IMPLEMENTED (2026-02-04)
>
> **Severity**: High
>
> **Discovered**: 2026-02-04 during production testing with GPT-5-mini
>
> **Related Issue**: Reasoning models require longer response times for chain-of-thought processing
>
> **Fixed**: Default timeout changed to 180 seconds across all providers

### Issue Summary

All AI provider clients hardcode a **30-second HTTP timeout**, ignoring the configurable `AIConfig.Timeout`. This causes requests to reasoning models (GPT-5, o1, o3, o4) to timeout during plan generation, as these models take longer to respond due to internal chain-of-thought processing.

**Symptoms**:
- Requests timeout after exactly 30 seconds
- Error: `context deadline exceeded (Client.Timeout exceeded while awaiting headers)`
- Cloudflare returns `400 Bad Request` on retry (stale connection)
- Intermittent failures even after token multiplier fix was applied

### Root Cause Analysis

**Problem**: The HTTP client timeout is hardcoded in each provider's `NewClient()` function:

```go
// BEFORE (caused 30s timeout):
// ai/providers/openai/client.go:32
base := providers.NewBaseClient(30*time.Second, logger)  // ❌ HARDCODED

// AFTER (fixed - 180s default):
// ai/providers/openai/client.go:32
base := providers.NewBaseClient(180*time.Second, logger)  // ✅ 3 minutes for reasoning models

// Same fix applied to all providers:
// ai/providers/anthropic/client.go:38 - now 180*time.Second
// ai/providers/gemini/client.go:36 - now 180*time.Second
// ai/providers/bedrock/client.go:33 - now 180*time.Second
```

**Fixed Behavior**: All providers now use 180-second default, configurable via `ai.WithTimeout()`:

```go
// ai/client.go:16 - Updated default
Timeout: 180 * time.Second,  // 3 minutes default for reasoning models
// Users can still override with ai.WithTimeout(300*time.Second) if needed
```

### Production Evidence

From `travel-chat-agent-986886cc-r4cmc.log` (2026-02-04):

```json
// Request starts at 03:51:52
{"timestamp":"2026-02-04T03:51:52Z","operation":"ai_request","model":"gpt-5-mini-2025-08-07","prompt_length":11582}

// Timeout exactly 30 seconds later at 03:52:22
{"timestamp":"2026-02-04T03:52:22Z","operation":"ai_request_retry_wait","error":"context deadline exceeded (Client.Timeout exceeded while awaiting headers)"}

// Cloudflare returns 400 on retry
{"timestamp":"2026-02-04T03:52:23Z","operation":"ai_chain_abort","error":"OpenAI API error: invalid request - <html>...<h1>400 Bad Request</h1>...<center>cloudflare</center>..."}
```

**Key Observation**: The successful request `orch-1770176848795362426` completed in ~52 seconds total (plan generation ~7s, execution ~1.5s, synthesis ~17s). Reasoning models need more than 30 seconds for complex prompts.

### Affected Files

#### Production Code (✅ FIXED)

| File | Line | Fixed Code |
|------|------|------------|
| `ai/client.go` | 16 | `Timeout: 180 * time.Second` ✅ |
| `ai/providers/openai/client.go` | 32 | `providers.NewBaseClient(180*time.Second, logger)` ✅ |
| `ai/providers/anthropic/client.go` | 38 | `providers.NewBaseClient(180*time.Second, logger)` ✅ |
| `ai/providers/gemini/client.go` | 36 | `providers.NewBaseClient(180*time.Second, logger)` ✅ |
| `ai/providers/bedrock/client.go` | 33 | `providers.NewBaseClient(180*time.Second, logger)` ✅ |

#### Chain Client (✅ ENHANCED)

Added `WithChainTimeout` option for explicit timeout configuration in chain clients:

| File | Line | Change |
|------|------|--------|
| `ai/chain_client.go` | 597 | Added `Timeout time.Duration` to `ChainConfig` ✅ |
| `ai/chain_client.go` | 629-637 | Added `WithChainTimeout(timeout time.Duration)` function ✅ |
| `ai/chain_client.go` | 83-86 | Apply timeout when creating provider clients ✅ |

**Example usage** (from `examples/travel-chat-agent/chat_agent.go`):
```go
chainClient, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainLogger(agent.Logger),
    ai.WithChainTimeout(240*time.Second), // Extended timeout for reasoning models
)
```

#### Factory Files (no changes needed)

The factories already apply `config.Timeout` via the `BaseClient.HTTPClient.Timeout` setting after client creation. The fix was to update the default value in each provider's `NewClient()` function.

#### Test Code (update for consistency)

| File | Lines |
|------|-------|
| `ai/config_edge_cases_test.go` | 143 |
| `ai/config_test.go` | 84 |
| `ai/providers/base_test.go` | 58, 99, 248, 280, 336, 398, 453, 498, 540, 643 |

#### Documentation (update for accuracy)

| File | Lines |
|------|-------|
| `ai/README.md` | 369 |
| `ai/ARCHITECTURE.md` | 381, 665 |

### Proposed Fix

#### 1. Update Default Timeout (ai/client.go)

```go
// Change from:
Timeout: 30 * time.Second,

// To:
Timeout: 180 * time.Second,  // 3 minutes - reasoning models need longer
```

**Rationale**: 180 seconds (3 minutes) provides adequate time for:
- Reasoning model chain-of-thought processing
- Complex plan generation prompts
- Network variability
- While still failing reasonably fast for actual connectivity issues

#### 2. Update Provider NewClient Signatures

Each provider's `NewClient()` needs to accept timeout as a parameter:

```go
// ai/providers/openai/client.go

// Change from:
func NewClient(apiKey, baseURL, providerAlias string, logger core.Logger) *Client {
    base := providers.NewBaseClient(30*time.Second, logger)
    // ...
}

// To:
func NewClient(apiKey, baseURL, providerAlias string, timeout time.Duration, logger core.Logger) *Client {
    if timeout == 0 {
        timeout = 180 * time.Second  // Default fallback
    }
    base := providers.NewBaseClient(timeout, logger)
    // ...
}
```

Apply same pattern to:
- `ai/providers/anthropic/client.go`
- `ai/providers/gemini/client.go`
- `ai/providers/bedrock/client.go`

#### 3. Update Factories to Pass Timeout

```go
// ai/providers/openai/factory.go

func (f *Factory) Create(config *ai.AIConfig) core.AIClient {
    // ...
    return NewClient(apiKey, baseURL, providerAlias, config.Timeout, config.Logger)
}
```

### Implementation Plan

| Step | File(s) | Change | Risk |
|------|---------|--------|------|
| 1 | `ai/client.go` | Change default from 30s → 180s | Low |
| 2 | `ai/providers/openai/client.go` | Add `timeout` parameter to `NewClient()` | Low |
| 3 | `ai/providers/openai/factory.go` | Pass `config.Timeout` to `NewClient()` | Low |
| 4 | `ai/providers/anthropic/client.go` | Add `timeout` parameter to `NewClient()` | Low |
| 5 | `ai/providers/anthropic/factory.go` | Pass `config.Timeout` to `NewClient()` | Low |
| 6 | `ai/providers/gemini/client.go` | Add `timeout` parameter to `NewClient()` | Low |
| 7 | `ai/providers/gemini/factory.go` | Pass `config.Timeout` to `NewClient()` | Low |
| 8 | `ai/providers/bedrock/client.go` | Add `timeout` parameter to `NewClient()` | Low |
| 9 | `ai/providers/bedrock/factory.go` | Pass `config.Timeout` to `NewClient()` | Low |
| 10 | Update tests | Adjust test expectations for new default | Low |
| 11 | Update docs | Update README.md, ARCHITECTURE.md | Low |

### User-Facing API (After Fix)

```go
// Single Provider Client
// ----------------------

// Uses default 180s timeout (sufficient for reasoning models)
client, _ := ai.NewClient()

// Custom timeout for specific use cases
client, _ := ai.NewClient(
    ai.WithTimeout(300 * time.Second),  // 5 minutes for very complex tasks
)

// Short timeout for latency-sensitive applications
client, _ := ai.NewClient(
    ai.WithTimeout(60 * time.Second),  // 1 minute
)

// Chain Client (with failover)
// ----------------------------

// Uses default 180s timeout from NewClient defaults
chainClient, _ := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainLogger(logger),
)

// Custom timeout for reasoning models (recommended: 240s for GPT-5, o1, o3, o4)
chainClient, _ := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainLogger(logger),
    ai.WithChainTimeout(240 * time.Second),  // 4 minutes for reasoning models
)
```

**Note**: `WithChainTimeout` was added in [ai/chain_client.go](../chain_client.go) to allow
explicit timeout configuration for chain clients. If not specified, chain clients inherit
the default 180s timeout from `NewClient()`.

### Benefits

| Aspect | Before Fix | After Fix |
|--------|------------|-----------|
| Default timeout | 30s (hardcoded, ignored) | 180s (configurable) |
| Reasoning model support | ❌ Timeouts on complex prompts | ✅ Works reliably |
| User configurability | ❌ `ai.WithTimeout()` ignored | ✅ Fully configurable |
| Provider consistency | ❌ Each provider hardcodes | ✅ All use config |

### Testing After Fix

1. **Unit Tests**: Verify timeout is passed from config to HTTP client
2. **Integration Tests**: Test GPT-5-mini with complex prompts (>30s response time)
3. **Regression Tests**: Verify standard models still work with new default

---

## IMPROVEMENT: Context-Aware Logging for Reasoning Model Debug

> **Status**: ✅ IMPLEMENTED (2026-02-04)
>
> **Severity**: Medium
>
> **Related Issue**: Distributed tracing correlation for reasoning model debugging

### Overview

**All loggers** in `ai/providers/openai/client.go` have been updated to use context-aware logging methods (`DebugWithContext`, `ErrorWithContext`) instead of basic `Debug`/`Error` calls. This enables proper distributed tracing correlation in production.

### Why This Matters

Without context-aware logging:
- Debug logs for reasoning models appear in the console but have **no trace_id or span_id**
- You cannot correlate reasoning model parameter adjustments with Jaeger spans
- Debugging reasoning model issues in production requires manual timestamp correlation

With context-aware logging:
- All reasoning model debug logs include `trace_id` and `span_id`
- You can filter logs in Grafana Loki by trace ID to see the full request flow
- Reasoning model issues can be debugged alongside their telemetry spans

### Changes Made

#### Debug Loggers (Reasoning Model Specific)

| Location | Before | After |
|----------|--------|-------|
| `client.go:123-131` | `c.Logger.Debug("Using reasoning model parameters", ...)` | `c.Logger.DebugWithContext(ctx, ...)` |
| `client.go:236-244` | `c.Logger.Debug("Raw OpenAI response for reasoning model", ...)` | `c.Logger.DebugWithContext(ctx, ...)` |
| `client.go:263-273` | `c.Logger.Debug("Parsed message fields for reasoning model", ...)` | `c.Logger.DebugWithContext(ctx, ...)` |
| `client.go:365-373` | `c.Logger.Debug("Using reasoning model parameters for streaming", ...)` | `c.Logger.DebugWithContext(ctx, ...)` |
| `client.go:510` | `c.Logger.Debug("OpenAI stream - failed to parse chunk", ...)` | `c.Logger.DebugWithContext(ctx, ...)` |

#### Error Loggers (All Error Paths)

| Location | Operation | Description |
|----------|-----------|-------------|
| `client.go:76` | `ai_request_error` | API key not configured |
| `client.go:136` | `ai_request_error` | Request marshal error |
| `client.go:151` | `ai_request_error` | HTTP request creation error |
| `client.go:169` | `ai_request_error` | Request execution error |
| `client.go:187` | `ai_request_error` | Response read error |
| `client.go:201` | `ai_request_error` | API error response |
| `client.go:218` | `ai_request_error` | Response parse error |
| `client.go:248` | `ai_request_error` | Empty response |
| `client.go:318` | `ai_stream_error` | Streaming API key not configured |
| `client.go:378` | `ai_stream_error` | Streaming marshal error |
| `client.go:393` | `ai_stream_error` | Streaming request creation error |
| `client.go:412` | `ai_stream_error` | Streaming execution error |
| `client.go:430` | `ai_stream_error` | Streaming API error |

**Total: 18 loggers updated to use context-aware methods**

### Logging Field Standards

All reasoning model debug logs follow the standard field naming from [LOGGING_IMPLEMENTATION_GUIDE.md](../../docs/LOGGING_IMPLEMENTATION_GUIDE.md):

| Field | Required | Description |
|-------|----------|-------------|
| `operation` | **YES** | Unique operation identifier (e.g., `ai_request_params`, `ai_raw_response_debug`) |
| `provider` | **YES** | Always `"openai"` |
| `model` | **YES** | The model being used (e.g., `"gpt-5-mini"`) |

### Example Log Output (JSON Format)

```json
{
  "timestamp": "2026-02-04T10:00:00Z",
  "level": "DEBUG",
  "component": "framework/ai",
  "message": "Using reasoning model parameters",
  "operation": "ai_request_params",
  "provider": "openai",
  "model": "gpt-5-mini",
  "using_max_completion_tokens": true,
  "temperature_omitted": true,
  "token_multiplier": 5,
  "request_id": "req-1738670400123456789",
  "trace_id": "abc123def456789012345678901234",
  "span_id": "1234567890abcdef"
}
```

### Filtering Reasoning Model Logs

```bash
# Filter by operation to see all reasoning model parameter adjustments
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.operation == "ai_request_params")'

# Filter by request_id to see full request flow
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.request_id == "req-1738670400123456789")'

# Filter by trace_id to correlate with Jaeger spans
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.trace_id == "abc123def456789012345678901234")'

# Show reasoning model debug logs with response details
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.operation | startswith("ai_") and contains("debug"))'

# Show all AI provider errors with trace correlation
kubectl logs -n gomind-examples -l app=my-agent | \
  jq 'select(.operation | endswith("_error")) | {msg: .message, op: .operation, req: .request_id, trace: .trace_id}'
```

### Trace Correlation Details

When using `WithContext` methods, the `ProductionLogger` automatically extracts and includes the following fields from context baggage (via `telemetry.GetBaggage(ctx)`):

| Field | Source | Description |
|-------|--------|-------------|
| `request_id` | Context baggage | Unique request identifier (set by orchestration) |
| `trace_id` | Context baggage | OpenTelemetry trace ID for Jaeger correlation |
| `span_id` | Context baggage | OpenTelemetry span ID for precise span location |

This enables:
- **Log filtering by request**: `jq 'select(.request_id == "req-12345")'`
- **Trace correlation**: Link logs to Jaeger spans via `trace_id`
- **End-to-end debugging**: See full request flow from orchestration through AI provider

### Remaining Work

The OpenAI provider now has **all 18 loggers** using context-aware methods. The broader issue documented in section 7 ("BUG FIX: Missing Distributed Trace Correlation") may still apply to other AI providers (Anthropic, Gemini, Bedrock). Those will be addressed as needed.

---

## Multi-Provider Research (Reference)

> **Status**: 📚 REFERENCE DOCUMENTATION
>
> **Purpose**: Document known API restrictions across providers for future implementation

This section contains comprehensive research on model-specific API parameter restrictions across all major AI providers. Use this as reference when:
- Implementing registry pattern for other providers
- Debugging API errors with specific models
- Planning new provider integrations

### Research Findings (2026-02-03) - Comprehensive Multi-Provider Analysis

| Provider | Has Breaking Restrictions? | Primary Issue | Severity | Needs Registry? |
|----------|---------------------------|---------------|----------|-----------------|
| **OpenAI** | ✅ Yes | Reasoning models reject `max_tokens`, `temperature`, `top_p` | Critical | ✅ Yes (Phase 1 done) |
| **Anthropic** | ✅ Yes | Claude 4+ rejects `temperature` + `top_p` together | High | ⚠️ Future (current client safe) |
| **Gemini** | ✅ Yes | Model-version-specific `thinkingLevel` vs `thinkingBudget` | High | ⚠️ Future |
| **DeepSeek** | ✅ Yes | Reasoning mode disables `temperature`, `top_p`, `logprobs` | High | ❌ No (params ignored, no error) |
| **xAI Grok** | ✅ Yes | Grok 4 rejects `reasoning_effort`, `presence_penalty`, `frequency_penalty` | Medium | ⚠️ Future |
| **Groq** | ⚠️ Minor | Standard OpenAI-compatible params work; rate limits vary by model | Low | ❌ Optional |
| **AWS Bedrock** | ✅ Yes | Different parameter format (`thinking` vs `reasoningConfig`) | High | ⚠️ Future |

### OpenAI Reasoning Models (o1, o3, o4-mini, GPT-5 family)

| Parameter | Standard Models | Reasoning Models | Error |
|-----------|----------------|------------------|-------|
| `max_tokens` | ✅ Supported | ❌ **REJECTED** | `Unsupported parameter: 'max_tokens' is not supported with this model` |
| `max_completion_tokens` | ✅ Supported | ✅ **Required** | - |
| `temperature` | ✅ Supported | ❌ **REJECTED** | `Unsupported value: 'temperature' does not support X with this model` |
| `top_p` | ✅ Supported | ❌ **REJECTED** | Same as temperature |
| `presence_penalty` | ✅ Supported | ❌ **REJECTED** | Unsupported parameter |
| `frequency_penalty` | ✅ Supported | ❌ **REJECTED** | Unsupported parameter |
| `logprobs` | ✅ Supported | ❌ **REJECTED** | Unsupported parameter |
| `stream` | ✅ Supported | ⚠️ **Limited** | o1/o3 require verification for streaming |

**GPT-5 `reasoning_effort` Parameter:**

| Model | Supported Values | Default |
|-------|-----------------|---------|
| GPT-5 | `minimal`, `low`, `medium`, `high` | `medium` |
| GPT-5.1 | `none`, `low`, `medium`, `high` | `none` |
| GPT-5.2 | `none`, `low`, `medium`, `high` | `none` |
| GPT-5-Codex | `low`, `medium`, `high`, `xhigh` | - |
| GPT-5-Pro | `high` only | `high` |

### DeepSeek Reasoning Models (R1, deepseek-reasoner)

| Parameter | Chat Mode | Reasoning/Thinking Mode | Notes |
|-----------|-----------|------------------------|-------|
| `max_tokens` | ✅ Supported | ✅ Supported (limit 8K optimal) | Quality degrades above 8K tokens |
| `temperature` | ✅ Supported (0-2) | ❌ **NOT SUPPORTED** | Silently ignored or rejected |
| `top_p` | ✅ Supported | ❌ **NOT SUPPORTED** | Silently ignored |
| `presence_penalty` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| `frequency_penalty` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| `logprobs` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| System prompt | ✅ Supported | ✅ Supported (R1-0528+) | Earlier versions required workaround |

**Note**: DeepSeek accepts standard parameters but ignores them - no error is returned. No fix needed.

### xAI Grok Models (Grok 3, Grok 4)

| Parameter | Grok 3 | Grok 4 / Grok 4.1 Fast | Error |
|-----------|--------|------------------------|-------|
| `reasoning_effort` | ✅ `low`, `high` | ❌ **REJECTED** | Returns error if provided |
| `presence_penalty` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| `frequency_penalty` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| `stop` | ✅ Supported | ❌ **NOT SUPPORTED** | - |
| `max_completion_tokens` | ✅ Supported | ✅ Required (high values) | Low values cause no response |
| `temperature` | ✅ Supported | ✅ Supported | - |

**Key Notes:**
- Grok 4 reasoning is always-on and cannot be disabled
- Grok 4 requires high `max_completion_tokens` for complex problems

### AWS Bedrock Parameter Format Differences

AWS Bedrock uses a **different parameter format** than the direct Anthropic API for extended thinking:

| Feature | Direct Anthropic API | AWS Bedrock API |
|---------|---------------------|-----------------|
| Extended Thinking | `thinking: {type: "enabled", budget_tokens: N}` | Same format, but via `additionalModelRequestFields` |
| Knowledge Bases | N/A | Uses `reasoningConfig` with different structure |
| Timeout | N/A | 60 minutes for Claude 3.7+/4 models (SDK default is 1 min) |

### Anthropic Claude Model Capabilities

> **Current gomind Status**: ✅ COMPATIBLE
>
> The current `anthropic/client.go` only sends `temperature`, NOT `top_p`. This means Claude 4+ models work fine without any changes.

**Breaking Changes (for reference):**

| Issue | Models Affected | Error Message |
|-------|-----------------|---------------|
| **temperature + top_p conflict** | Claude 4.1 Opus, Claude 4.5 Sonnet/Opus, Claude 3.5 Haiku | `temperature and top_p cannot both be specified for this model` |
| **Extended thinking + temperature** | All thinking-enabled models | Thinking isn't compatible with temperature/top_p/top_k modifications |
| **budget_tokens > max_tokens** | All thinking-enabled models | `max_tokens must be greater than thinking.budget_tokens` |

**When to Implement Registry for Anthropic:**
- When adding `top_p` parameter support
- When implementing extended thinking features

### Google Gemini Model Capabilities

> **Current gomind Status**: ⚠️ THINKING FEATURES NOT IMPLEMENTED
>
> The Gemini thinking parameter differences only matter if/when thinking features are added.

**Parameter Compatibility Matrix:**

| Parameter | Gemini 2.5 Flash | Gemini 2.5 Pro | Gemini 3 Flash | Gemini 3 Pro |
|-----------|-----------------|----------------|----------------|--------------|
| `thinkingBudget` | ✅ Supported | ✅ Supported | ⚠️ Backwards compat only | ⚠️ Backwards compat only |
| `thinkingLevel` | ❌ **ERROR** | ❌ **ERROR** | ✅ **Recommended** | ✅ **Recommended** |
| Disable thinking | ✅ `budget: 0` | ❌ **Cannot disable** | ❌ **Cannot disable** | ❌ **Cannot disable** |

**When to Implement Registry for Gemini:**
- When implementing thinking/reasoning features

---

## FUTURE Registry Pattern: Additional Sections

### DEBUG Logging Implementation (Future)

Following the patterns in `base.go:LogResponse()` and `client.go`, add DEBUG logging to `buildRequestBody()` using `WithContext` variants for distributed tracing support:

```go
// buildRequestBody creates the appropriate request body based on model capabilities
// ctx is required for distributed tracing correlation
func (c *Client) buildRequestBody(ctx context.Context, model string, messages []map[string]string,
                                   options *core.AIOptions, streaming bool) map[string]interface{} {

    caps := GetModelCapabilities(c.providerAlias, model)

    // Log capability detection at DEBUG level with context for trace correlation
    // Follows base.go:LogResponse() pattern (line 312) using WithContext
    if c.Logger != nil && caps.IsReasoningModel {
        c.Logger.DebugWithContext(ctx, "Reasoning model detected, adjusting request parameters", map[string]interface{}{
            "operation":                  "ai_capability_detection",
            "provider":                   c.providerAlias,
            "model":                      model,
            "use_max_completion_tokens":  caps.UseMaxCompletionTokens,
            "omit_temperature":           caps.OmitTemperature,
            "no_streaming":               caps.NoStreaming,
            "notes":                      caps.Notes,
        })
    }

    reqBody := map[string]interface{}{
        "model":    model,
        "messages": messages,
    }

    // Token limit parameter
    if caps.UseMaxCompletionTokens {
        reqBody["max_completion_tokens"] = options.MaxTokens

        // DEBUG: Log parameter substitution with context
        if c.Logger != nil {
            c.Logger.DebugWithContext(ctx, "Using max_completion_tokens instead of max_tokens", map[string]interface{}{
                "operation":              "ai_param_adjustment",
                "provider":               c.providerAlias,
                "model":                  model,
                "param_used":             "max_completion_tokens",
                "value":                  options.MaxTokens,
            })
        }
    } else {
        reqBody["max_tokens"] = options.MaxTokens
    }

    // Temperature (omit for reasoning models that don't support it)
    if !caps.OmitTemperature {
        reqBody["temperature"] = options.Temperature
    } else if c.Logger != nil {
        c.Logger.DebugWithContext(ctx, "Omitting temperature parameter for reasoning model", map[string]interface{}{
            "operation":             "ai_param_adjustment",
            "provider":              c.providerAlias,
            "model":                 model,
            "param_omitted":         "temperature",
            "original_value":        options.Temperature,
        })
    }

    // Streaming
    if streaming {
        reqBody["stream"] = true
        if !caps.NoStreamOptions {
            reqBody["stream_options"] = map[string]interface{}{
                "include_usage": true,
            }
        }
    }

    return reqBody
}
```

#### Log Level Guidelines (per ai/ARCHITECTURE.md)

| Level     | Use Case                                        | Example                                                   |
| --------- | ----------------------------------------------- | --------------------------------------------------------- |
| **DEBUG** | Capability detection, parameter adjustments     | "Reasoning model detected", "Using max_completion_tokens" |
| **INFO**  | Request initiation, response completion         | Handled by `LogRequest()`, `LogResponse()` in base.go     |
| **WARN**  | Fallback to non-streaming for unsupported model | See StreamResponse example below                          |
| **ERROR** | API errors, configuration errors                | Existing patterns in client.go                            |

#### StreamResponse Logging Update

```go
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, callback core.StreamCallback) (*core.AIResponse, error) {
    // ... existing setup code ...

    // Check if model supports streaming
    if !SupportsStreaming(c.providerAlias, options.Model) {
        if c.Logger != nil {
            c.Logger.WarnWithContext(ctx, "Model does not support streaming, using non-streaming fallback", map[string]interface{}{
                "operation":       "ai_streaming_fallback",
                "provider":        c.providerAlias,
                "model":           options.Model,
                "reason":          "model_no_streaming_support",
                "is_reasoning":    IsReasoningModel(c.providerAlias, options.Model),
            })
        }
        return c.GenerateResponse(ctx, prompt, options)
    }
    // ... rest of method ...
}
```

#### Telemetry Metrics (Span Attributes)

Add span attributes for reasoning model tracking in `GenerateResponse()` and `StreamResponse()`:

```go
// After capability lookup in GenerateResponse()
caps := GetModelCapabilities(c.providerAlias, options.Model)
span.SetAttribute("ai.is_reasoning_model", caps.IsReasoningModel)
span.SetAttribute("ai.use_max_completion_tokens", caps.UseMaxCompletionTokens)
span.SetAttribute("ai.omit_temperature", caps.OmitTemperature)
```

This enables:

- Filtering Jaeger traces by reasoning model usage
- Grafana dashboards showing reasoning vs. standard model distribution
- Debugging parameter adjustment behavior in production

### Phase 4: Future Enhancements (Optional)

Allow runtime override of capabilities via environment variables:

```bash
# Force a model to use max_completion_tokens
GOMIND_MODEL_CAPS_MYMODEL_USE_MAX_COMPLETION_TOKENS=true
```

---

## Testing Strategy

### Unit Tests

#### Registry Lookup Tests

```go
func TestGetModelCapabilities(t *testing.T) {
    tests := []struct {
        name          string
        provider      string
        model         string
        wantMaxComp   bool
        wantOmitTemp  bool
        wantNoStream  bool
    }{
        // OpenAI reasoning models
        {"gpt-5-mini", "openai", "gpt-5-mini-2025-08-07", true, true, false},
        {"gpt-5", "openai", "gpt-5", true, true, false},
        {"o1-preview", "openai", "o1-preview", true, true, true},
        {"o1-mini", "openai", "o1-mini", true, true, true},
        {"o3", "openai", "o3", true, true, false},
        {"o3-mini", "openai", "o3-mini", true, true, false},
        {"o4-mini", "openai", "o4-mini", true, true, false},

        // OpenAI standard models (no special handling)
        {"gpt-4", "openai", "gpt-4", false, false, false},
        {"gpt-4o", "openai", "gpt-4o", false, false, false},
        {"gpt-4o-mini", "openai", "gpt-4o-mini", false, false, false},

        // DeepSeek (no special handling - standard params work)
        {"deepseek-chat", "openai.deepseek", "deepseek-chat", false, false, false},
        {"deepseek-reasoner", "openai.deepseek", "deepseek-reasoner", false, false, false},

        // Groq (no special handling)
        {"groq-llama", "openai.groq", "llama-3.3-70b-versatile", false, false, false},

        // Unknown model (safe defaults)
        {"unknown", "openai", "some-future-model", false, false, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            caps := GetModelCapabilities(tt.provider, tt.model)

            if caps.UseMaxCompletionTokens != tt.wantMaxComp {
                t.Errorf("UseMaxCompletionTokens = %v, want %v",
                    caps.UseMaxCompletionTokens, tt.wantMaxComp)
            }
            if caps.OmitTemperature != tt.wantOmitTemp {
                t.Errorf("OmitTemperature = %v, want %v",
                    caps.OmitTemperature, tt.wantOmitTemp)
            }
            if caps.NoStreaming != tt.wantNoStream {
                t.Errorf("NoStreaming = %v, want %v",
                    caps.NoStreaming, tt.wantNoStream)
            }
        })
    }
}
```

#### Longest-Prefix-Match Tests (5.2)

Verify that longer prefixes are matched before shorter ones:

```go
func TestLongestPrefixMatch(t *testing.T) {
    // These tests verify the sorted longest-prefix-match behavior
    tests := []struct {
        name           string
        model          string
        expectNoStream bool // o1-preview and o1-mini should have NoStreaming=true, o1 base should too
    }{
        // Specific versions should match their specific prefix
        {"o1-preview exact", "o1-preview", true},
        {"o1-preview with version", "o1-preview-2024-09-12", true},
        {"o1-mini exact", "o1-mini", true},
        {"o1-mini with version", "o1-mini-2024-09-12", true},

        // Base model should fall back to "o1" prefix
        {"o1 base", "o1", true},
        {"o1 with version", "o1-2024-12-17", true},

        // Ensure gpt-4o doesn't accidentally match gpt-4 rules
        {"gpt-4o should not match gpt-4", "gpt-4o", false},
        {"gpt-4o-mini", "gpt-4o-mini", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            caps := GetModelCapabilities("openai", tt.model)
            if caps.NoStreaming != tt.expectNoStream {
                t.Errorf("NoStreaming for %q = %v, want %v",
                    tt.model, caps.NoStreaming, tt.expectNoStream)
            }
        })
    }
}
```

#### Interface Mocking Tests (5.1)

The `CapabilityProvider` interface enables clean mocking in client tests:

```go
// MockCapabilityProvider for testing specific scenarios
type MockCapabilityProvider struct {
    capabilities ModelCapabilities
}

func (m *MockCapabilityProvider) GetCapabilities(providerAlias, model string) ModelCapabilities {
    return m.capabilities
}

func TestClientWithMockedCapabilities(t *testing.T) {
    // Create a mock that returns specific capabilities
    mockProvider := &MockCapabilityProvider{
        capabilities: ModelCapabilities{
            UseMaxCompletionTokens: true,
            OmitTemperature:        true,
            IsReasoningModel:       true,
        },
    }

    // Create client with mocked provider (requires Client to accept provider)
    client := &Client{
        capabilityProvider: mockProvider,
        // ... other fields
    }

    // Test that client uses the mocked capabilities
    reqBody := client.buildRequestBody(context.Background(), "test-model",
        []map[string]string{}, &core.AIOptions{MaxTokens: 100}, false)

    // Verify max_completion_tokens is used
    if _, ok := reqBody["max_completion_tokens"]; !ok {
        t.Error("Expected max_completion_tokens in request body")
    }
    if _, ok := reqBody["max_tokens"]; ok {
        t.Error("Did not expect max_tokens in request body")
    }
    if _, ok := reqBody["temperature"]; ok {
        t.Error("Did not expect temperature in request body (should be omitted)")
    }
}
```

#### Request Body Tests

```go
func TestBuildRequestBody(t *testing.T) {
    // Test that request bodies are built correctly for different model types
    tests := []struct {
        name             string
        model            string
        streaming        bool
        wantMaxCompToken bool // expect max_completion_tokens instead of max_tokens
        wantNoTemp       bool // expect temperature to be absent
        wantStream       bool // expect stream=true
    }{
        {"gpt-5 non-streaming", "gpt-5-mini", false, true, true, false},
        {"gpt-5 streaming", "gpt-5-mini", true, true, true, true},
        {"gpt-4 non-streaming", "gpt-4o", false, false, false, false},
        {"gpt-4 streaming", "gpt-4o", true, false, false, true},
        {"o1 non-streaming", "o1-preview", false, true, true, false},
    }

    client := &Client{providerAlias: "openai"}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            reqBody := client.buildRequestBody(
                context.Background(),
                tt.model,
                []map[string]string{{"role": "user", "content": "test"}},
                &core.AIOptions{MaxTokens: 100, Temperature: 0.7},
                tt.streaming,
            )

            _, hasMaxComp := reqBody["max_completion_tokens"]
            _, hasMaxTokens := reqBody["max_tokens"]
            _, hasTemp := reqBody["temperature"]
            _, hasStream := reqBody["stream"]

            if hasMaxComp != tt.wantMaxCompToken {
                t.Errorf("max_completion_tokens present = %v, want %v", hasMaxComp, tt.wantMaxCompToken)
            }
            if hasMaxTokens == tt.wantMaxCompToken {
                t.Errorf("max_tokens present = %v, should be opposite of max_completion_tokens", hasMaxTokens)
            }
            if hasTemp == tt.wantNoTemp {
                t.Errorf("temperature present = %v, should be opposite of wantNoTemp", hasTemp)
            }
            if hasStream != tt.wantStream {
                t.Errorf("stream present = %v, want %v", hasStream, tt.wantStream)
            }
        })
    }
}
```

### Integration Tests

```go
func TestOpenAIReasoningModelIntegration(t *testing.T) {
    if os.Getenv("OPENAI_API_KEY") == "" {
        t.Skip("OPENAI_API_KEY not set")
    }

    client := NewClient(os.Getenv("OPENAI_API_KEY"), "", "openai", nil)

    // Test GPT-5 model works with registry-based params
    resp, err := client.GenerateResponse(context.Background(), "Say hello", &core.AIOptions{
        Model:     "gpt-5-mini-2025-08-07",
        MaxTokens: 100,
    })

    if err != nil {
        t.Fatalf("GPT-5 request failed: %v", err)
    }
    if resp.Content == "" {
        t.Error("Expected non-empty response")
    }
}
```

---

## Benefits

| Aspect                  | Spaghetti Code               | Registry Pattern                          |
| ----------------------- | ---------------------------- | ----------------------------------------- |
| **Adding new model**    | Edit multiple if-else chains | Add one entry to registry                 |
| **Understanding rules** | Read through code logic      | Look at declarative config                |
| **Testing**             | Complex mocking              | Interface enables clean mocking (5.1)     |
| **Provider changes**    | Code changes scattered       | Single registry update                    |
| **Documentation**       | Separate from code           | Config IS the docs                        |
| **Code review**         | Review logic changes         | Review data changes                       |
| **Rollback**            | Revert code changes          | Remove registry entry                     |
| **Determinism**         | Map iteration order varies   | Sorted longest-prefix-match (5.2)         |
| **Fail-fast**           | Errors at runtime            | Startup validation catches issues (5.3)   |

### Design Improvements Incorporated

This proposal incorporates the following best practices validated against Go design patterns:

1. **Interface for Testability (5.1)**: `CapabilityProvider` interface enables dependency injection and mocking in unit tests without requiring real registry lookups.

2. **Deterministic Longest-Prefix-Match (5.2)**: Sorted prefixes with `sync.Once` initialization ensures "o1-preview" always matches before "o1", regardless of Go map iteration order.

3. **Startup Validation (5.3)**: `init()` function detects shadowing prefixes at application startup, enabling fail-fast for configuration errors.

---

## Future Extensibility

### Adding New Provider Rules

```go
// Example: xAI releases a reasoning model with restrictions
"grok-5-think": {
    UseMaxCompletionTokens: true,
    OmitTemperature:        true,
    IsReasoningModel:       true,
    Notes:                  "xAI Grok 5 thinking model",
},
```

### Provider-Specific Overrides

```go
// Example: Groq hosts a model that behaves differently than on OpenAI
providerCapabilityOverrides = map[string]map[string]ModelCapabilities{
    "openai.groq": {
        "llama-4-reasoning": {
            OmitTemperature:  true,
            IsReasoningModel: true,
        },
    },
}
```

### Runtime Configuration (Future)

```go
// Load additional capabilities from config file or environment
func init() {
    if configPath := os.Getenv("GOMIND_MODEL_CAPS_CONFIG"); configPath != "" {
        loadCapabilitiesFromFile(configPath)
    }
}
```

### Extending to All AI Providers

> **Note**: Detailed multi-provider research is documented in [Multi-Provider Research (Reference)](#multi-provider-research-reference) section above.

When upgrading to the registry pattern for other providers, refer to:
- [OpenAI Reasoning Models](#openai-reasoning-models-o1-o3-o4-mini-gpt-5-family) - Parameter restrictions
- [Anthropic Claude Models](#anthropic-claude-model-capabilities) - temperature + top_p conflict
- [Google Gemini Models](#google-gemini-model-capabilities) - thinkingBudget vs thinkingLevel
- [AWS Bedrock](#aws-bedrock-parameter-format-differences) - Different parameter formats

#### Anthropic Claude Model Capabilities (Future Registry)

> **Current gomind Status**: ✅ No changes needed - current client doesn't send `top_p`
>
> **When to Implement**: When adding `top_p` support or extended thinking features

**Proposed Anthropic Capabilities Structure:**

```go
// ai/providers/anthropic/capabilities.go

package anthropic

// AnthropicModelCapabilities defines parameter handling rules for Claude models
type AnthropicModelCapabilities struct {
    // Sampling parameter conflict (Claude 4+)
    RejectTopPWithTemperature bool // Cannot use both temperature AND top_p

    // Extended thinking support
    SupportsExtendedThinking  bool // Model supports thinking.budget_tokens
    ThinkingDisablesTemp      bool // When thinking enabled, temperature must be omitted
    ThinkingDisablesTopP      bool // When thinking enabled, top_p must be omitted
    ThinkingDisablesTopK      bool // When thinking enabled, top_k must be omitted

    // Model capabilities
    SupportsInterleaved       bool // Supports interleaved thinking (Claude 4+ only)
    MaxThinkingBudget         int  // Maximum budget_tokens value

    // Informational
    IsReasoningModel          bool
    Notes                     string
}

// Registry for Anthropic models
var anthropicModelCapabilitiesRegistry = map[string]AnthropicModelCapabilities{
    // Claude 4.5 family (August 2025+)
    "claude-opus-4.5": {
        RejectTopPWithTemperature: true,
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        SupportsInterleaved:       true,
        IsReasoningModel:          true,
        Notes:                     "Claude Opus 4.5 - rejects temp+top_p, supports extended thinking",
    },
    "claude-sonnet-4.5": {
        RejectTopPWithTemperature: true,
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        SupportsInterleaved:       true,
        Notes:                     "Claude Sonnet 4.5 - rejects temp+top_p",
    },
    "claude-opus-4.1": {
        RejectTopPWithTemperature: true,
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        Notes:                     "Claude Opus 4.1 - first model to reject temp+top_p",
    },
    "claude-opus-4": {
        RejectTopPWithTemperature: true,
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        SupportsInterleaved:       true,
        Notes:                     "Claude Opus 4",
    },
    "claude-sonnet-4": {
        RejectTopPWithTemperature: true,
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        SupportsInterleaved:       true,
        Notes:                     "Claude Sonnet 4",
    },

    // Claude 3.7 family (supports thinking but allows temp+top_p)
    "claude-3.7": {
        RejectTopPWithTemperature: false, // Still allows both
        SupportsExtendedThinking:  true,
        ThinkingDisablesTemp:      true,
        Notes:                     "Claude 3.7 - supports thinking, allows temp+top_p",
    },

    // Claude 3.5 Haiku (has the restriction)
    "claude-3.5-haiku": {
        RejectTopPWithTemperature: true,
        Notes:                     "Claude 3.5 Haiku - rejects temp+top_p",
    },

    // Older models (no restrictions) - not listed, use defaults
}
```

**Request Builder Integration for Anthropic:**

```go
// buildRequestBody for Anthropic client
func (c *Client) buildRequestBody(ctx context.Context, model string, messages []Message,
                                   options *core.AIOptions) map[string]interface{} {

    caps := GetAnthropicModelCapabilities(model)

    reqBody := map[string]interface{}{
        "model":      model,
        "messages":   messages,
        "max_tokens": options.MaxTokens,
    }

    // Handle temperature + top_p conflict (Claude 4+)
    if caps.RejectTopPWithTemperature {
        // Only include temperature, never top_p for these models
        if options.Temperature != 0 {
            reqBody["temperature"] = options.Temperature
        }
        // Explicitly DO NOT add top_p even if provided

        if c.Logger != nil && options.TopP != 0 {
            c.Logger.WarnWithContext(ctx, "Ignoring top_p parameter for Claude 4+ model", map[string]interface{}{
                "operation":   "anthropic_param_adjustment",
                "model":       model,
                "reason":      "model_rejects_temp_and_top_p_together",
                "top_p_value": options.TopP,
            })
        }
    } else {
        // Older models - can use both
        if options.Temperature != 0 {
            reqBody["temperature"] = options.Temperature
        }
        if options.TopP != 0 {
            reqBody["top_p"] = options.TopP
        }
    }

    // Handle extended thinking
    if options.EnableThinking && caps.SupportsExtendedThinking {
        thinking := map[string]interface{}{
            "type":         "enabled",
            "budget_tokens": options.ThinkingBudget,
        }
        reqBody["thinking"] = thinking

        // Remove temperature if thinking disables it
        if caps.ThinkingDisablesTemp {
            delete(reqBody, "temperature")
        }
    }

    return reqBody
}
```

#### Google Gemini Model Capabilities (Future Registry)

> **Current gomind Status**: ⚠️ Thinking features not implemented
>
> **When to Implement**: When adding Gemini thinking/reasoning features

**Proposed Gemini Capabilities Structure:**

```go
// ai/providers/gemini/capabilities.go

package gemini

// GeminiModelCapabilities defines parameter handling rules for Gemini models
type GeminiModelCapabilities struct {
    // Thinking parameter type (mutually exclusive)
    UseThinkingBudget     bool     // Gemini 2.5 - use thinkingBudget (token count)
    UseThinkingLevel      bool     // Gemini 3+ - use thinkingLevel (LOW/HIGH)
    ThinkingLevelOptions  []string // Valid options: ["LOW", "HIGH"] for 3 Pro, more for others

    // Thinking behavior
    ThinkingAlwaysOn      bool // Gemini 2.5 Pro - cannot disable thinking
    ThinkingCanBeDisabled bool // Model allows thinking to be turned off

    // Model deprecation
    IsDeprecated          bool
    DeprecationDate       string // e.g., "2026-03-03"
    RecommendedReplacement string

    // Informational
    IsReasoningModel      bool
    Notes                 string
}

// Registry for Gemini models
var geminiModelCapabilitiesRegistry = map[string]GeminiModelCapabilities{
    // Gemini 3.x family (2026+) - uses thinkingLevel
    "gemini-3-pro": {
        UseThinkingLevel:      true,
        ThinkingLevelOptions:  []string{"LOW", "HIGH"}, // Only LOW or HIGH
        ThinkingCanBeDisabled: true,
        IsReasoningModel:      true,
        Notes:                 "Gemini 3 Pro - uses thinkingLevel (LOW/HIGH only)",
    },
    "gemini-3-flash": {
        UseThinkingLevel:      true,
        ThinkingLevelOptions:  []string{"MINIMAL", "LOW", "MEDIUM", "HIGH"},
        ThinkingCanBeDisabled: true,
        Notes:                 "Gemini 3 Flash - uses thinkingLevel with all options",
    },

    // Gemini 2.5 family - uses thinkingBudget
    "gemini-2.5-pro": {
        UseThinkingBudget:     true,
        ThinkingAlwaysOn:      true,  // Cannot disable thinking
        ThinkingCanBeDisabled: false,
        IsReasoningModel:      true,
        Notes:                 "Gemini 2.5 Pro - uses thinkingBudget, thinking always on",
    },
    "gemini-2.5-flash": {
        UseThinkingBudget:     true,
        ThinkingCanBeDisabled: true,
        Notes:                 "Gemini 2.5 Flash - uses thinkingBudget",
    },
    "gemini-2.5-flash-lite": {
        UseThinkingBudget:     true,
        ThinkingCanBeDisabled: true,
        IsDeprecated:          true,
        DeprecationDate:       "2026-03-31",
        RecommendedReplacement: "gemini-2.5-flash",
        Notes:                 "Deprecated - use gemini-2.5-flash",
    },

    // Gemini 2.0 family (being retired)
    "gemini-2.0-flash": {
        IsDeprecated:          true,
        DeprecationDate:       "2026-03-03",
        RecommendedReplacement: "gemini-2.5-flash",
        Notes:                 "Deprecated - use gemini-2.5-flash or newer",
    },
    "gemini-2.0-flash-lite": {
        IsDeprecated:          true,
        DeprecationDate:       "2026-03-31",
        RecommendedReplacement: "gemini-2.5-flash-lite",
        Notes:                 "Deprecated",
    },

    // Older models - no thinking support, use defaults
}
```

**Request Builder Integration for Gemini:**

```go
// buildGenerationConfig for Gemini client
func (c *Client) buildGenerationConfig(ctx context.Context, model string,
                                        options *core.AIOptions) map[string]interface{} {

    caps := GetGeminiModelCapabilities(model)

    config := map[string]interface{}{
        "maxOutputTokens": options.MaxTokens,
        "temperature":     options.Temperature,
    }

    // Warn about deprecated models
    if caps.IsDeprecated && c.Logger != nil {
        c.Logger.WarnWithContext(ctx, "Using deprecated Gemini model", map[string]interface{}{
            "operation":      "gemini_deprecation_warning",
            "model":          model,
            "deprecation_date": caps.DeprecationDate,
            "recommended":    caps.RecommendedReplacement,
        })
    }

    // Handle thinking configuration
    if options.EnableThinking {
        thinkingConfig := map[string]interface{}{}

        if caps.UseThinkingBudget {
            // Gemini 2.5 - use thinkingBudget
            if caps.ThinkingAlwaysOn {
                // Cannot set budget to 0 for 2.5 Pro
                if options.ThinkingBudget == 0 {
                    options.ThinkingBudget = -1 // Dynamic budgeting
                }
            }
            thinkingConfig["thinkingBudget"] = options.ThinkingBudget

            if c.Logger != nil {
                c.Logger.DebugWithContext(ctx, "Using thinkingBudget for Gemini 2.5 model", map[string]interface{}{
                    "operation":       "gemini_thinking_config",
                    "model":           model,
                    "thinking_budget": options.ThinkingBudget,
                })
            }

        } else if caps.UseThinkingLevel {
            // Gemini 3+ - use thinkingLevel
            level := mapEffortToThinkingLevel(options.ThinkingEffort, caps.ThinkingLevelOptions)
            thinkingConfig["thinkingLevel"] = level

            if c.Logger != nil {
                c.Logger.DebugWithContext(ctx, "Using thinkingLevel for Gemini 3+ model", map[string]interface{}{
                    "operation":      "gemini_thinking_config",
                    "model":          model,
                    "thinking_level": level,
                    "valid_options":  caps.ThinkingLevelOptions,
                })
            }
        }

        if len(thinkingConfig) > 0 {
            config["thinkingConfig"] = thinkingConfig
        }
    }

    return config
}

// mapEffortToThinkingLevel converts effort string to valid thinkingLevel
func mapEffortToThinkingLevel(effort string, validOptions []string) string {
    // Map common effort values to Gemini thinkingLevel
    effortMap := map[string]string{
        "low":    "LOW",
        "medium": "MEDIUM",
        "high":   "HIGH",
        "minimal": "MINIMAL",
    }

    if level, ok := effortMap[strings.ToLower(effort)]; ok {
        // Verify it's valid for this model
        for _, opt := range validOptions {
            if opt == level {
                return level
            }
        }
        // Fall back to first valid option
        if len(validOptions) > 0 {
            return validOptions[0]
        }
    }

    return "LOW" // Default
}
```

#### Implementation Strategy (Future Registry)

When ready to implement the full registry pattern:

**Registry Phase 1**: Implement OpenAI registry in `ai/providers/openai/capabilities.go`

**Registry Phase 2**: Extend to Anthropic
- Create `ai/providers/anthropic/capabilities.go`
- Update `anthropic/client.go` to use registry
- Primary focus: `RejectTopPWithTemperature` flag

**Registry Phase 3**: Extend to Gemini
- Create `ai/providers/gemini/capabilities.go`
- Update `gemini/client.go` to use registry
- Primary focus: `UseThinkingBudget` vs `UseThinkingLevel`

**Shared Infrastructure (Optional)**:

If significant overlap emerges, consider extracting common patterns to a shared package:

```
ai/providers/
├── capabilities/           # Shared infrastructure (optional)
│   ├── interfaces.go       # Common CapabilityProvider interface
│   └── registry.go         # Shared registry utilities
├── openai/
│   └── capabilities.go     # OpenAI-specific
├── anthropic/
│   └── capabilities.go     # Anthropic-specific
└── gemini/
    └── capabilities.go     # Gemini-specific
```

---

## Risks and Mitigations

### Phase 1 (Simple Approach) Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| New reasoning model prefix not covered | Low | API errors | Update `reasoningModelPrefixes` list |
| False positive (non-reasoning model matched) | Very Low | Wrong params used | Use specific prefixes, add tests |

### Phase 2+ (Registry Pattern) Risks

| Risk                         | Likelihood | Impact                                   | Mitigation                                     |
| ---------------------------- | ---------- | ---------------------------------------- | ---------------------------------------------- |
| Prefix matching is too broad | Low        | Model incorrectly matched                | Use specific prefixes, add tests               |
| New model not in registry    | Medium     | Falls back to standard params (may fail) | Monitor errors, update registry                |
| Registry becomes stale       | Medium     | Outdated rules                           | Document update process, link to provider docs |
| Performance overhead         | Very Low   | Negligible                               | Map lookup is O(n) where n is small            |

---

## Decision

**Approved**: Phased implementation approach.

### Phase 1 (NOW) - Simple Approach

Implement `IsReasoningModel()` helper function:
- ~30 lines of code
- Fixes immediate GPT-5/o1/o3/o4 compatibility issue
- Simple, maintainable, easy to test

### Phase 2+ (FUTURE) - Registry Pattern

Upgrade to full registry pattern when:
- Different models have different restrictions
- Need 5+ capability flags per model
- Need runtime configuration
- Multiple providers have breaking issues

### Why This Decision?

| Consideration | Simple Approach | Registry Pattern |
|---------------|-----------------|------------------|
| Solves immediate problem | ✅ Yes | ✅ Yes |
| Implementation effort | ~30 lines | ~500+ lines |
| Current complexity needs | ✅ Sufficient | Over-engineered |
| Future-proofed | ⚠️ Limited | ✅ Yes |
| YAGNI principle | ✅ Follows | ❌ Violates |

The simple approach follows the YAGNI (You Aren't Gonna Need It) principle while keeping the registry pattern documented for future use.

---

## Documentation Requirements

> **Status**: ✅ COMPLETED (2026-02-04)
>
> **Purpose**: Track what needs to be added to official documentation files

This section lists the user-facing configuration options that have been documented in the official guides.

### Files Updated

| File | Change | Status |
|------|--------|--------|
| `core/config.go:144` | Updated `GOMIND_AI_TIMEOUT` default from `30s` to `180s` | ✅ Done |
| `docs/ENVIRONMENT_VARIABLES_GUIDE.md:279` | Updated timeout default to `180s` with reasoning model note | ✅ Done |
| `docs/AI_PROVIDERS_SETUP_GUIDE.md` | Added "Advanced Configuration" section (~100 lines) with Request Timeouts and Reasoning Model Support | ✅ Done |
| `docs/API_REFERENCE.md` | Added "Client Configuration Options" section (~70 lines) with `WithTimeout`, `WithReasoningTokenMultiplier`, and options table | ✅ Done |

### Documentation Items for ENVIRONMENT_VARIABLES_GUIDE.md

**Line 279 update** - Change:
```markdown
| `GOMIND_AI_TIMEOUT` | `30s` | Struct Tag Only | Request timeout | [core/config.go:144](../core/config.go#L144) |
```

To:
```markdown
| `GOMIND_AI_TIMEOUT` | `180s` | Struct Tag Only | Request timeout (increased for reasoning model support) | [core/config.go:144](../core/config.go#L144) |
| `GOMIND_AI_REASONING_TOKEN_MULTIPLIER` | `5` | Struct Tag Only | Token multiplier for reasoning models (GPT-5, o1, o3, o4) | [ai/providers/openai/reasoning.go:45](../ai/providers/openai/reasoning.go#L45) |
```

**Add explanation after the AI Configuration Variables table:**
```markdown
> **Note on Reasoning Models**: OpenAI reasoning models (GPT-5, o1, o3, o4) require special handling:
> - **Timeout**: These models take longer due to internal chain-of-thought processing. The default 180s accommodates most use cases.
> - **Token Multiplier**: Reasoning models count internal reasoning tokens against `max_completion_tokens` but don't return them. The 5x multiplier ensures sufficient tokens for both reasoning and visible output.
```

### Documentation Items for AI_PROVIDERS_SETUP_GUIDE.md

Add a new section **"Advanced Configuration Options"** covering:

#### 1. Timeout Configuration

```go
// Single Client - Custom Timeout
// Default timeout is 180 seconds (3 minutes), sufficient for reasoning models
client, err := ai.NewClient(
    ai.WithTimeout(300 * time.Second),  // 5 minutes for very complex tasks
)

// Single Client - Shorter timeout for latency-sensitive applications
client, err := ai.NewClient(
    ai.WithTimeout(60 * time.Second),   // 1 minute
)

// Chain Client - Custom Timeout
chainClient, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainTimeout(240 * time.Second),  // 4 minutes for reasoning models
)
```

**When to adjust timeout:**
- Reasoning models (GPT-5, o1, o3, o4) often need 60-120 seconds for complex prompts
- Default 180s is sufficient for most use cases
- Increase for orchestration/plan generation tasks
- Decrease for simple, latency-sensitive applications

#### 2. Reasoning Token Multiplier Configuration

```go
// Single Client - Default 5x multiplier (recommended)
client, err := ai.NewClient()  // Uses default 5x for reasoning models

// Single Client - Custom multiplier for cost optimization
client, err := ai.NewClient(
    ai.WithReasoningTokenMultiplier(3),  // 3x multiplier (lower cost, may truncate)
)

// Single Client - Higher multiplier for complex reasoning
client, err := ai.NewClient(
    ai.WithReasoningTokenMultiplier(8),  // 8x multiplier for complex tasks
)

// Chain Client - Custom multiplier
chainClient, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainReasoningTokenMultiplier(4),  // 4x multiplier
)
```

**Why reasoning models need a token multiplier:**

Reasoning models (GPT-5, o1, o3, o4) count internal chain-of-thought tokens against `max_completion_tokens`, but these reasoning tokens are NOT returned in the response. Without a multiplier, complex prompts exhaust all tokens on internal reasoning, leaving nothing for visible output.

```
Token Budget (without multiplier):
┌────────────────────────────────────┐
│  Internal Reasoning (invisible)    │  ← Uses ~80% of tokens
├────────────────────────────────────┤
│  Visible Output (truncated/empty!) │  ← Only ~20% remains
└────────────────────────────────────┘

Token Budget (with 5x multiplier):
┌────────────────────────────────────┐
│  Internal Reasoning (invisible)    │  ← ~4000 tokens for thinking
├────────────────────────────────────┤
│  Visible Output (complete!)        │  ← ~6000 tokens for response
└────────────────────────────────────┘
(10000 total from 2000 * 5x multiplier)
```

**When to adjust the multiplier:**
- Default 5x works for most orchestration and complex tasks
- Reduce to 3x for simpler prompts to save costs
- Increase to 8x+ for very complex reasoning tasks

### Documentation Items for API_REFERENCE.md

Add an **"AI Module Configuration"** section under a new or existing configuration reference:

#### AIConfig Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithTimeout(d time.Duration)` | `time.Duration` | `180s` | HTTP timeout for AI API requests. Reasoning models may need longer timeouts. |
| `WithReasoningTokenMultiplier(n int)` | `int` | `5` | Token multiplier for reasoning models (GPT-5, o1, o3, o4). Applied to `max_completion_tokens`. |
| `WithModel(m string)` | `string` | (auto) | Model name or alias (e.g., "smart", "fast", "gpt-4o") |
| `WithProviderAlias(a string)` | `string` | (auto) | Provider alias (e.g., "openai", "openai.deepseek", "anthropic") |
| `WithMaxTokens(n int)` | `int` | `4096` | Default max tokens for responses |
| `WithTemperature(t float32)` | `float32` | `0.7` | Default temperature for responses |
| `WithAPIKey(k string)` | `string` | (env) | Override API key from environment |
| `WithLogger(l core.Logger)` | `core.Logger` | `nil` | Logger for AI operations |
| `WithTelemetry(t core.Telemetry)` | `core.Telemetry` | `nil` | Telemetry for distributed tracing |

#### ChainConfig Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `WithProviderChain(providers ...string)` | `[]string` | - | Ordered list of provider aliases for failover |
| `WithChainTimeout(d time.Duration)` | `time.Duration` | `180s` | HTTP timeout applied to all providers in chain |
| `WithChainReasoningTokenMultiplier(n int)` | `int` | `5` | Token multiplier for reasoning models, applied to all providers |
| `WithChainLogger(l core.Logger)` | `core.Logger` | `nil` | Logger for chain operations |
| `WithChainTelemetry(t core.Telemetry)` | `core.Telemetry` | `nil` | Telemetry for distributed tracing |

#### Environment Variables

| Variable | Description |
|----------|-------------|
| `GOMIND_AI_TIMEOUT` | Default AI request timeout (e.g., "180s", "5m") |
| `GOMIND_AI_MODEL` | Default model alias |
| `GOMIND_AI_PROVIDER` | Default provider alias |

### Recommended Documentation Structure

**In AI_PROVIDERS_SETUP_GUIDE.md**, add after "Troubleshooting Common Issues":

```markdown
## Advanced Configuration

### Request Timeouts

By default, all AI providers use a 180-second (3 minute) timeout...
[content as above]

### Reasoning Model Support

When using OpenAI reasoning models (GPT-5, o1, o3, o4), the framework automatically...
[content as above]
```

**In API_REFERENCE.md**, add a new section:

```markdown
## AI Module

### Configuration Options

The `ai` package provides flexible configuration for AI clients...
[table of options as above]
```

---

## References

**OpenAI**:
- [OpenAI API Documentation - Chat Completions](https://platform.openai.com/docs/api-reference/chat)
- [DeepSeek API Documentation - Reasoning Model](https://api-docs.deepseek.com/guides/reasoning_model)

**Anthropic**:
- [Migrating to Claude 4.5 - Claude Docs](https://docs.claude.com/en/docs/about-claude/models/migrating-to-claude-4)
- [Building with Extended Thinking - Claude Docs](https://platform.claude.com/docs/en/build-with-claude/extended-thinking)
- [GitHub: temperature + top_p conflict issues](https://github.com/sst/opencode/issues/1644)

**Google Gemini**:
- [Gemini Thinking Documentation](https://ai.google.dev/gemini-api/docs/thinking)
- [Vertex AI Thinking Documentation](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/thinking)
- [GitHub: thinkingLevel not supported](https://github.com/google-gemini/gemini-cli/issues/13857)

**Verified API behavior tests**: 2026-02-03

---

## Appendix: Verified API Test Results

### OpenAI GPT-5-mini (2026-02-03)

```bash
# FAILS: max_tokens
curl -X POST "https://api.openai.com/v1/chat/completions" \
  -d '{"model": "gpt-5-mini-2025-08-07", "messages": [...], "max_tokens": 500}'
# Error: "Unsupported parameter: 'max_tokens' is not supported with this model.
#         Use 'max_completion_tokens' instead."

# FAILS: temperature != 1
curl -X POST "https://api.openai.com/v1/chat/completions" \
  -d '{"model": "gpt-5-mini-2025-08-07", "messages": [...], "temperature": 0.7, "max_completion_tokens": 500}'
# Error: "Unsupported value: 'temperature' does not support 0.7 with this model.
#         Only the default (1) value is supported."

# WORKS: Correct parameters
curl -X POST "https://api.openai.com/v1/chat/completions" \
  -d '{"model": "gpt-5-mini-2025-08-07", "messages": [...], "max_completion_tokens": 500}'
# Success: Returns completion with reasoning_tokens in usage

# WORKS: Streaming with stream_options
curl -X POST "https://api.openai.com/v1/chat/completions" \
  -d '{"model": "gpt-5-mini-2025-08-07", "messages": [...], "max_completion_tokens": 500, "stream": true, "stream_options": {"include_usage": true}}'
# Success: Returns SSE stream with usage in final chunk
```

### DeepSeek-reasoner (From Official Docs)

- `max_tokens`: **Works** (standard parameter)
- `temperature`, `top_p`, `presence_penalty`, `frequency_penalty`: **Accepted but ignored**
- `logprobs`, `top_logprobs`: **Triggers error**
- Streaming: **Fully supported**

Source: https://api-docs.deepseek.com/guides/reasoning_model
