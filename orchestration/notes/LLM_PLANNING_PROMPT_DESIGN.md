# LLM Planning Prompt System Design

This document describes the design decisions and architecture of the PromptBuilder system for customizing LLM orchestration prompts.

> **Implementation Guide**: For usage examples, configuration reference, and troubleshooting, see [docs/guides/LLM_PLANNING_PROMPT_GUIDE.md](../../docs/guides/LLM_PLANNING_PROMPT_GUIDE.md).

## Design Goals

1. **Zero-config by default** - Works out of the box with sensible defaults
2. **Progressive customization** - Simple extensions don't require complex code
3. **Full control when needed** - Custom implementations for advanced use cases
4. **Graceful degradation** - Never fail due to configuration issues
5. **Domain agnostic** - Framework has no domain knowledge; LLM handles semantics

## Three-Layer Architecture

The system uses a layered architecture that balances ease of use with flexibility:

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Layer 3: Custom PromptBuilder                    │
│   Full control via interface injection                               │
│   Use case: Compliance requirements, external service integration    │
├─────────────────────────────────────────────────────────────────────┤
│                     Layer 2: TemplatePromptBuilder                   │
│   Structural customization via Go text/template                      │
│   Use case: Different prompt structures, multi-language support      │
├─────────────────────────────────────────────────────────────────────┤
│                     Layer 1: DefaultPromptBuilder                    │
│   Zero-config with optional type rules and custom instructions       │
│   Use case: Most applications, domain-specific type hints            │
└─────────────────────────────────────────────────────────────────────┘
```

### Why Three Layers?

| Layer | Complexity | Flexibility | Use When |
|-------|------------|-------------|----------|
| Layer 1 | Low | Medium | 90% of cases - add type rules and instructions |
| Layer 2 | Medium | High | Need different prompt structure |
| Layer 3 | High | Full | Compliance, audit, external services |

## Key Design Decisions

### 1. Domain-Agnostic Framework

The framework contains **no domain-specific knowledge**. All semantic understanding is delegated to the LLM.

**Rationale**:
- Domains are infinite (healthcare, finance, travel, legal, retail, etc.)
- Hardcoding domain rules creates maintenance burden
- LLMs are better at semantic understanding than static rules

**Implementation**: `Domain` field provides hints to built-in domain sections, but custom domains work equally well with `CustomInstructions`.

### 2. Type Rules Over Schema Validation

We use `TypeRule` hints rather than strict JSON schema validation.

**Rationale**:
- LLMs occasionally generate incorrect types (e.g., `"35.6"` instead of `35.6`)
- Type rules with examples and anti-patterns guide LLM behavior
- More flexible than schema validation for natural language outputs

### 3. Graceful Degradation

Configuration errors never crash the system.

**Rationale**:
- Production systems must be resilient
- Missing template file shouldn't prevent service startup
- Fallback to working defaults is better than failure

**Implementation**: Factory pattern in `factory.go` catches errors and falls back to `DefaultPromptBuilder`.

### 4. Dependency Injection

PromptBuilder is injected via `OrchestratorDependencies`, not created internally.

**Rationale**:
- Testability - mock builders for unit tests
- Flexibility - swap implementations without code changes
- Follows framework's "Zero Framework Dependencies" principle

## Source Files

| File | Responsibility |
|------|----------------|
| [prompt_builder.go](prompt_builder.go) | Core interfaces: `PromptBuilder`, `PromptInput`, `PromptConfig`, `TypeRule` |
| [default_prompt_builder.go](default_prompt_builder.go) | Layer 1 implementation with default type rules and domain sections |
| [template_prompt_builder.go](template_prompt_builder.go) | Layer 2 implementation using Go `text/template` |
| [prompt_config_env.go](prompt_config_env.go) | Environment variable loading: `LoadFromEnv()`, `MustLoadFromEnv()` |
| [prompt_builder_metrics.go](prompt_builder_metrics.go) | Telemetry metric and span name constants |
| [factory.go](factory.go) | Builder selection logic in `CreateOrchestrator()` |
| [orchestrator.go](orchestrator.go) | Prompt invocation in `buildPlanningPrompt()` (~line 999) |

## Data Flow

```
User Request
     │
     ▼
ProcessRequest(ctx, request, metadata)
     │
     ▼
buildPlanningPrompt(ctx, request)           [orchestrator.go:999]
     │
     ├─► capabilityProvider.GetCapabilities()
     │
     ▼
promptBuilder.BuildPlanningPrompt(PromptInput{
    CapabilityInfo: "tool-a: action_1...",
    Request:        "User's query",
    Metadata:       {"user_id": "123"},
})
     │
     ▼
┌────────────────────────────────────────────┐
│  DefaultPromptBuilder / TemplatePromptBuilder │
│                                            │
│  Assembles:                                │
│  • System instructions                     │
│  • Capability info                         │
│  • Type rules (default + additional)       │
│  • Domain section (if configured)          │
│  • Custom instructions                     │
│  • JSON structure examples                 │
└────────────────────────────────────────────┘
     │
     ▼
Complete prompt string → LLM
     │
     ▼
JSON execution plan
```

## Factory Selection Logic

The factory in `CreateOrchestrator()` selects the builder:

```go
// Priority order (factory.go:112-157)
if deps.PromptBuilder != nil {
    // Layer 3: Use injected custom builder
    orchestrator.SetPromptBuilder(deps.PromptBuilder)
} else if config.PromptConfig.TemplateFile != "" || config.PromptConfig.Template != "" {
    // Layer 2: Create TemplatePromptBuilder
    builder, err := NewTemplatePromptBuilder(&config.PromptConfig)
    if err != nil {
        // Fallback to Layer 1 on error
        defaultBuilder, _ := NewDefaultPromptBuilder(&config.PromptConfig)
        orchestrator.SetPromptBuilder(defaultBuilder)
    }
} else {
    // Layer 1: DefaultPromptBuilder
    defaultBuilder, _ := NewDefaultPromptBuilder(&config.PromptConfig)
    orchestrator.SetPromptBuilder(defaultBuilder)
}
```

## Integration Points

### With Orchestrator

The orchestrator calls the prompt builder before each LLM planning request:

```go
// orchestrator.go
func (o *AIOrchestrator) buildPlanningPrompt(ctx context.Context, request string) (string, error) {
    capabilityInfo, _ := o.capabilityProvider.GetCapabilities(ctx, request, nil)

    if o.promptBuilder != nil {
        return o.promptBuilder.BuildPlanningPrompt(ctx, PromptInput{
            CapabilityInfo: capabilityInfo,
            Request:        request,
        })
    }
    // Fallback to hardcoded prompt (backward compatibility)
    return fmt.Sprintf(`...`, capabilityInfo, request), nil
}
```

### With Telemetry

Metrics and spans are emitted when telemetry is configured:

- **Metrics**: `orchestrator.prompt.built`, `orchestrator.prompt.build_duration_ms`
- **Spans**: `prompt-builder.build` with attributes for debugging

### With Kubernetes

Templates can be loaded from ConfigMap-mounted files:

```yaml
env:
- name: GOMIND_PROMPT_TEMPLATE_FILE
  value: /config/planning-prompt.tmpl
volumeMounts:
- name: prompt-config
  mountPath: /config
```

## Security Considerations

1. **Templates from trusted sources only** - Never accept user input as templates
2. **Safe template data** - `TemplateData` exposes only strings/maps, no methods
3. **No code execution** - Go templates don't allow arbitrary code

## Extension Points

### Adding a New Domain

Add to `buildDomainSection()` in `default_prompt_builder.go`:

```go
case "your-domain":
    return "YOUR DOMAIN REQUIREMENTS: ..."
```

### Custom Type Coercion

Implement a custom `PromptBuilder` that adds type coercion logic before calling the LLM.

### External Capability Filtering

Implement `CapabilityProvider` interface to filter capabilities based on external service (RAG, etc.).

## Related Documents

- [Implementation Guide](../../docs/guides/LLM_PLANNING_PROMPT_GUIDE.md) - Usage, configuration, examples
- [Orchestration README](README.md) - Module overview and execution plan structure
- [INTELLIGENT_PARAMETER_BINDING.md](INTELLIGENT_PARAMETER_BINDING.md) - 4-layer parameter resolution system
