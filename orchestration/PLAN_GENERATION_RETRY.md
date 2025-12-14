# Plan Generation JSON Parse Error Handling

## Overview

This document describes handling JSON parse errors during LLM plan generation. **This is not a standalone solution** - it extends the existing LLM response handling infrastructure already present in the orchestration module.

**Existing Infrastructure (Already Handles):**
- Markdown formatting (`**bold**`, `*italic*`) → [INTELLIGENT_PARAMETER_BINDING.md § Markdown Stripping](./INTELLIGENT_PARAMETER_BINDING.md#markdown-stripping-from-llm-responses)
- Code block extraction (` ```json ... ``` `) → `cleanLLMResponse()` in [orchestrator.go:1042-1069](orchestrator.go#L1042-L1069)
- JSON boundary detection → `cleanLLMResponse()` finds `{` and `}`

**This Document Adds:**
- Arithmetic expression prevention (prompt-level) - NEW
- Retry mechanism with error feedback - PROPOSED

LLM responses are probabilistic - the same query may produce valid JSON one time and invalid JSON the next.

---

## Problem Statement

During plan generation, the LLM may produce invalid JSON due to:

1. **Arithmetic expressions**: `"amount": 100 * "{{step-1.response.price}}"` — ❌ **Cannot be cleaned** (this doc addresses)
2. **Markdown formatting**: `"city": "**Paris**"` or `"tool": "*weather*"` — ✅ **Already handled** by `stripMarkdownFromJSON()`
3. **Code block wrapping**: ` ```json { ... } ``` ` — ✅ **Already handled** by `cleanLLMResponse()`
4. **Malformed templates**: Missing quotes, trailing commas, etc. — ❌ **Cannot be cleaned**

These errors result in parse failures:

```json
{
    "error": "Orchestration failed",
    "details": "failed to generate execution plan: failed to parse plan JSON: invalid character '*' after object key:value pair",
    "status": 500
}
```

---

## Solution Architecture

The orchestration module uses a **multi-layer defense** strategy for LLM response handling:

```
┌─────────────────────────────────────────────────────────────────┐
│               LLM Response Cleaning Pipeline                     │
│                                                                  │
│  LLM Response: ```json {"city": "**Paris**", ...} ```           │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 1: Prompt Instructions (Prevention)                  │ │
│  │   Location: default_prompt_builder.go:266-275              │ │
│  │                                                             │ │
│  │   • "Do NOT use markdown formatting"                       │ │
│  │   • "Do NOT wrap in code fences"                           │ │
│  │   • "Do NOT use arithmetic expressions"  ← NEW             │ │
│  │   • "Start with { and end with }"                          │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 2: Programmatic Cleanup (Existing)                   │ │
│  │   Location: orchestrator.go:1042-1113                      │ │
│  │                                                             │ │
│  │   cleanLLMResponse():                                       │ │
│  │   • Extract JSON from code blocks (```json ... ```)        │ │
│  │   • Find JSON object boundaries ({ ... })                  │ │
│  │   • Strip intro/outro text                                 │ │
│  │                                                             │ │
│  │   stripMarkdownFromJSON():                                  │ │
│  │   • Strip bold: **text** → text                            │ │
│  │   • Strip italic: *text* → text                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│       │                                                          │
│       ▼                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Layer 3: JSON Parse (json.Unmarshal)                       │ │
│  │                                                             │ │
│  │   If parse fails after cleanup:                            │ │
│  │   • Arithmetic expressions cannot be cleaned               │ │
│  │   • Malformed syntax cannot be fixed programmatically      │ │
│  │   → Error returned to user (or retry if enabled)           │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## What Can Be Cleaned vs What Cannot

| Issue | Can Be Cleaned? | Solution | Status |
|-------|-----------------|----------|--------|
| `**bold**` in strings | ✅ Yes | `stripMarkdownFromJSON()` removes `**` | **Existing** |
| `*italic*` in strings | ✅ Yes | `stripMarkdownFromJSON()` removes `*` | **Existing** |
| ` ```json ... ``` ` wrapper | ✅ Yes | `cleanLLMResponse()` extracts JSON | **Existing** |
| Intro text before `{` | ✅ Yes | `cleanLLMResponse()` finds `{` | **Existing** |
| `"amount": 100 * price` | ❌ No | Prompt-level prevention | **Implemented** |
| Trailing commas | ❌ No | Requires retry with error feedback | Proposed |
| Missing quotes | ❌ No | Requires retry with error feedback | Proposed |

**Key Insight:** The first four issues (markdown-related) are **already handled** by the existing cleanup infrastructure in `orchestrator.go`. Arithmetic expressions like `100 * price` cannot be fixed programmatically because they represent **computation**, not malformed text. The `*` is not markdown - it's a multiplication operator that JSON doesn't support.

---

## Implemented: Arithmetic Expression Prevention

Added to the prompt instructions to prevent arithmetic expressions before they occur.

**Location:** [default_prompt_builder.go:273-274](default_prompt_builder.go#L273-L274)

```go
CRITICAL FORMAT RULES (applies to all LLM providers):
- You are a JSON API. Your ONLY output is a raw JSON object.
- Output ONLY valid JSON - no markdown, no code blocks, no backticks
- Do NOT use any text formatting: no ** (bold), no * (italic), no __ (underline)
- Do NOT wrap JSON in code fences (no triple backticks)
- Do NOT include any explanatory text before or after the JSON
- String values must be plain text without any markdown formatting
- Do NOT use arithmetic expressions in JSON values (e.g., "amount": 100 * price is INVALID)  ← NEW
- If calculations are needed, create separate steps or use literal values only                 ← NEW
- Start your response with { and end with } - nothing else
```

**Why prompt-level prevention:**
- Arithmetic expressions cannot be cleaned up post-hoc
- JSON fundamentally doesn't support expressions
- LLMs understand "don't do X" better when given explicit examples

---

## Existing Cleanup Infrastructure (Production-Ready)

The orchestration module already includes robust LLM response cleaning. **This infrastructure is fully documented in [INTELLIGENT_PARAMETER_BINDING.md § Markdown Stripping from LLM Responses](./INTELLIGENT_PARAMETER_BINDING.md#markdown-stripping-from-llm-responses)** (lines 554-777).

### cleanLLMResponse() - [orchestrator.go:1042-1069](orchestrator.go#L1042-L1069)

Handles structural cleanup (already implemented):
- Extracts JSON from markdown code blocks (` ```json ... ``` `)
- Finds JSON object boundaries (`{` ... `}`)
- Strips intro/outro text ("Here's your plan:" etc.)

### stripMarkdownFromJSON() - [orchestrator.go:1076-1113](orchestrator.go#L1076-L1113)

Handles formatting cleanup (already implemented):
- Strips bold markers: `**text**` → `text`
- Strips italic markers: `*text*` → `text`
- Conservative matching to avoid breaking valid JSON

**Note:** The markdown stripping algorithm is detailed in INTELLIGENT_PARAMETER_BINDING.md, including regex patterns, safety guarantees, and debug logging instructions.

### LLM Provider Behaviors

| Provider | JSON Mode | Markdown Tendency | Arithmetic Tendency |
|----------|-----------|-------------------|---------------------|
| **OpenAI GPT-4** | `response_format: json_object` | Low | Low |
| **Anthropic Claude** | No native JSON mode | Low | Medium |
| **Google Gemini** | `responseMimeType: application/json` | High | Medium |
| **Groq** | Follows OpenAI format | Medium | Low |

---

## Proposed: Retry Mechanism

For cases where prevention fails, a retry mechanism with error feedback would provide additional resilience.

### Design

Extend `generateExecutionPlan()` to retry on JSON parse failures, following the existing `regeneratePlan()` pattern used for validation errors.

### Configuration

| Config Field | Default | Environment Variable | Description |
|--------------|---------|---------------------|-------------|
| `PlanParseRetryEnabled` | `true` | `GOMIND_PLAN_RETRY_ENABLED` | Enable retry on JSON parse failures |
| `PlanParseMaxRetries` | `2` | `GOMIND_PLAN_RETRY_MAX` | Maximum retry attempts after initial failure |

### Error Feedback Prompt

On retry, the LLM receives context about the parse failure:

```
IMPORTANT: Your previous response could not be parsed as valid JSON.

Parse Error: invalid character '*' after object key:value pair

Common JSON mistakes to avoid:
- NO arithmetic expressions: "amount": 100 * price is INVALID
- NO markdown formatting: **bold** and *italic* are INVALID in JSON strings
- NO code blocks: Do not wrap JSON in triple backticks

Please regenerate a VALID JSON execution plan.
```

---

## Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| Prompt: markdown prevention | **Existing** | [default_prompt_builder.go:266-272](default_prompt_builder.go#L266-L272) |
| Prompt: arithmetic prevention | **Implemented** | [default_prompt_builder.go:273-274](default_prompt_builder.go#L273-L274) |
| Cleanup: code block extraction | **Existing** | [orchestrator.go:1042-1069](orchestrator.go#L1042-L1069) |
| Cleanup: markdown stripping | **Existing** | [orchestrator.go:1076-1113](orchestrator.go#L1076-L1113) |
| Retry mechanism | Proposed | See checklist below |

---

## Implementation Checklist (For Retry Mechanism)

- [ ] Add `PlanParseRetryEnabled` and `PlanParseMaxRetries` to `OrchestratorConfig`
- [ ] Update `DefaultConfig()` with env var loading
- [ ] Add `WithPlanParseRetry()` functional option
- [ ] Modify `generateExecutionPlan()` with retry loop
- [ ] Add `buildPlanningPromptWithParseError()` function
- [ ] Add telemetry functions
- [ ] Write unit tests
- [ ] Update ENVIRONMENT_VARIABLES_GUIDE.md

---

## Related Documentation

| Document | Relevance |
|----------|-----------|
| [INTELLIGENT_PARAMETER_BINDING.md](./INTELLIGENT_PARAMETER_BINDING.md) | **Primary reference** - Contains full details on markdown stripping (§ Markdown Stripping from LLM Responses, lines 554-777), LLM provider behaviors, and regex patterns |
| [README.md](./README.md) | Module overview and architecture |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Module design principles |
| [ENVIRONMENT_VARIABLES_GUIDE.md](../docs/ENVIRONMENT_VARIABLES_GUIDE.md) | Configuration options |

**Architecture Note:** This document and INTELLIGENT_PARAMETER_BINDING.md together describe the complete LLM response handling strategy for the orchestration module. The cleanup functions (`cleanLLMResponse()`, `stripMarkdownFromJSON()`) are shared infrastructure used by both plan generation and parameter binding.
