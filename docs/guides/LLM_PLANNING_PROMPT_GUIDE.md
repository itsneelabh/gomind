# PromptBuilder Guide

This guide covers the extensible PromptBuilder system for customizing LLM orchestration prompts in GoMind.

## Overview

The PromptBuilder system enables customization of prompts sent to the LLM for execution plan generation. It follows a three-layer architecture that balances ease of use with flexibility:

> 📖 **Output Structure**: For the JSON plan structure that the LLM generates, see [LLM-Generated Execution Plan Structure](README.md#llm-execution-plan).

| Layer | Builder Type | Use Case | Configuration |
|-------|-------------|----------|---------------|
| **Layer 1** | `DefaultPromptBuilder` | Zero-config, works out of the box | `PromptConfig.AdditionalTypeRules`, `Domain` |
| **Layer 2** | `TemplatePromptBuilder` | Structural customization via Go templates | `PromptConfig.Template` or `TemplateFile` |
| **Layer 3** | Custom `PromptBuilder` | Complete control via interface injection | `OrchestratorDependencies.PromptBuilder` |

## Quick Start

### Layer 1: Default Builder (Zero Configuration)

Works immediately with sensible defaults:

```go
import "github.com/itsneelabh/gomind/orchestration"

config := orchestration.DefaultConfig()
deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,
    AIClient:  aiClient,
}

orchestrator, err := orchestration.CreateOrchestrator(config, deps)
```

The default builder includes type rules for:
- `string` → JSON strings
- `number`, `float64`, `float32`, `float` → JSON numbers
- `integer`, `int`, `int64`, `int32` → JSON integers
- `boolean`, `bool` → JSON booleans
- `array`, `[]string`, `[]int`, `[]float64`, `list` → JSON arrays
- `object`, `map`, `struct`, `map[string]interface{}` → JSON objects

### Layer 1: With Additional Type Rules

Extend defaults with domain-specific types:

```go
config := orchestration.DefaultConfig()
config.PromptConfig = orchestration.PromptConfig{
    Domain: "healthcare",
    AdditionalTypeRules: []orchestration.TypeRule{
        {
            TypeNames:   []string{"patient_id", "mrn"},
            JsonType:    "JSON strings",
            Example:     `"P12345"`,
            Description: "Medical record numbers must be quoted strings",
        },
        {
            TypeNames:   []string{"dosage"},
            JsonType:    "JSON numbers",
            Example:     `250.5`,
            AntiPattern: `"250.5"`,
            Description: "Medication dosage in milligrams",
        },
    },
    CustomInstructions: []string{
        "Never include PHI in execution plan metadata",
        "Prefer HIPAA-compliant tools when available",
    },
}
```

### Layer 2: Template-Based Customization

Use Go `text/template` for structural changes:

```go
config := orchestration.DefaultConfig()
config.PromptConfig = orchestration.PromptConfig{
    Template: `You are an AI orchestrator for {{.Domain}} domain.

Available Capabilities:
{{.CapabilityInfo}}

User Request: {{.Request}}

{{.TypeRules}}

{{.CustomInstructions}}

Generate a JSON execution plan with this structure:
{{.JSONStructure}}

Response (JSON only):`,
}
```

### Layer 2: Template from ConfigMap (Kubernetes)

Mount a ConfigMap and reference the file:

```go
config.PromptConfig = orchestration.PromptConfig{
    TemplateFile: "/config/planning-prompt.tmpl",
    Domain:       "finance",
}
```

### Layer 3: Custom Builder (Full Control)

Implement the `PromptBuilder` interface:

```go
type PromptBuilder interface {
    BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error)
}
```

Inject via dependencies:

```go
type MyCustomBuilder struct{}

func (b *MyCustomBuilder) BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error) {
    // Custom logic: call external service, use ML model, etc.
    return myCustomPrompt, nil
}

deps := orchestration.OrchestratorDependencies{
    Discovery:     discovery,
    AIClient:      aiClient,
    PromptBuilder: &MyCustomBuilder{},
}
```

## Configuration Reference

### PromptConfig

```go
type PromptConfig struct {
    // Layer 1: Extend default type rules
    AdditionalTypeRules []TypeRule `json:"additional_type_rules,omitempty"`

    // Layer 2: Template customization (TemplateFile takes precedence)
    TemplateFile string `json:"template_file,omitempty"`
    Template     string `json:"template,omitempty"`

    // Common options
    CustomInstructions []string `json:"custom_instructions,omitempty"`
    Domain             string   `json:"domain,omitempty"`
    IncludeAntiPatterns *bool   `json:"include_anti_patterns,omitempty"`
}
```

### TypeRule

```go
type TypeRule struct {
    TypeNames   []string `json:"type_names"`           // e.g., ["number", "float64"]
    JsonType    string   `json:"json_type"`            // e.g., "JSON numbers"
    Example     string   `json:"example"`              // e.g., "35.6897"
    AntiPattern string   `json:"anti_pattern,omitempty"` // e.g., "\"35.6897\""
    Description string   `json:"description,omitempty"`
}
```

### PromptInput

Data passed to `BuildPlanningPrompt`:

```go
type PromptInput struct {
    CapabilityInfo string                 // Formatted capabilities from catalog
    Request        string                 // User's natural language request
    Metadata       map[string]interface{} // Optional context (user_id, priority, etc.)
}
```

### TemplateData (Layer 2)

Available fields in templates:

| Field | Type | Description |
|-------|------|-------------|
| `.CapabilityInfo` | `string` | Formatted agent/tool capabilities |
| `.Request` | `string` | User's request |
| `.TypeRules` | `string` | Pre-formatted type rules section |
| `.CustomInstructions` | `string` | Pre-formatted custom instructions |
| `.Domain` | `string` | Domain context (healthcare, finance, etc.) |
| `.Metadata` | `map[string]interface{}` | Request metadata |
| `.JSONStructure` | `string` | Example JSON structure for plans |

## Domain-Specific Behavior

The `Domain` field triggers built-in domain requirements:

### Healthcare Domain
```go
config.PromptConfig.Domain = "healthcare"
```
Adds:
- PHI protection reminders
- HIPAA compliance preferences
- Audit trail requirements

### Finance Domain
```go
config.PromptConfig.Domain = "finance"
```
Adds:
- Decimal precision requirements
- Transaction ID tracking
- SOX compliance preferences

### Legal Domain
```go
config.PromptConfig.Domain = "legal"
```
Adds:
- Chain of custody requirements
- Timestamp/source attribution
- Document modification flags

## Environment Variable Configuration

Load configuration from environment using methods on `PromptConfig`:

```go
config := &orchestration.PromptConfig{}
err := config.LoadFromEnv()

// Or with panic on error (use in main() or init()):
config := &orchestration.PromptConfig{}
config.MustLoadFromEnv()
```

Supported environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `GOMIND_PROMPT_TEMPLATE_FILE` | Path to template file | `/config/prompt.tmpl` |
| `GOMIND_PROMPT_DOMAIN` | Domain context | `healthcare` |
| `GOMIND_PROMPT_TYPE_RULES` | JSON array of type rules | `[{"type_names":["uuid"],"json_type":"JSON strings","example":"\"abc-123\""}]` |
| `GOMIND_PROMPT_CUSTOM_INSTRUCTIONS` | JSON array of instructions | `["Prefer local tools"]` |

## Telemetry Integration

PromptBuilder emits metrics and traces when telemetry is configured:

### Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `orchestrator.prompt.built` | Counter | `builder_type`, `domain`, `status` |
| `orchestrator.prompt.build_duration_ms` | Histogram | `builder_type`, `domain`, `status` |
| `orchestrator.prompt.size_bytes` | Gauge | `builder_type` |
| `orchestrator.prompt.template_fallback` | Counter | `reason` |

### Distributed Tracing

Spans are created with:
- Span name: `prompt-builder.build`
- Attributes: `builder_type`, `domain`, `type_rules_count`, `request_length`, `prompt_length`, `duration_ms`

### Setup

```go
import "github.com/itsneelabh/gomind/telemetry"

// Initialize telemetry first
telemetry.Initialize(telemetry.Config{
    Enabled:     true,
    ServiceName: "my-agent",
    Endpoint:    "otel-collector:4318",
})

// Factory automatically injects telemetry
deps := orchestration.OrchestratorDependencies{
    Discovery: discovery,
    AIClient:  aiClient,
    Telemetry: telemetry.GetTelemetryProvider(),
}

orchestrator, _ := orchestration.CreateOrchestrator(config, deps)
```

## Graceful Degradation

The system is designed to never fail due to prompt configuration issues:

1. **Invalid template syntax** → Falls back to `DefaultPromptBuilder`
2. **Missing template file** → Falls back to `DefaultPromptBuilder`
3. **Template execution error** → Falls back to `DefaultPromptBuilder`
4. **Missing telemetry** → Continues without metrics/traces
5. **Missing logger** → Continues without logging

Example fallback behavior:

```go
config.PromptConfig = orchestration.PromptConfig{
    TemplateFile: "/nonexistent/path.tmpl", // File doesn't exist
}

// Still succeeds - uses DefaultPromptBuilder
orchestrator, err := orchestration.CreateOrchestrator(config, deps)
// err == nil, orchestrator uses DefaultPromptBuilder with warning logged
```

## Security Considerations

> **SECURITY**: Templates must come from trusted sources only (admin-managed ConfigMaps
> or compiled-in config). Do NOT accept templates from user input.

### Why Templates Are Safe When Admin-Controlled

1. **No user input in templates**: Templates are loaded from files or config, not user requests
2. **Safe data types**: `TemplateData` only exposes strings and maps, no methods
3. **Graceful degradation**: Invalid templates fall back to safe defaults

### Best Practices

- Store templates in ConfigMaps with restricted RBAC
- Never interpolate user input into template strings
- Use `TemplateFile` in production for auditability
- Review template changes in code review

## Testing

### Unit Testing Custom Builders

```go
func TestMyCustomBuilder(t *testing.T) {
    builder := &MyCustomBuilder{}

    input := orchestration.PromptInput{
        CapabilityInfo: "weather-tool: get_weather(city string)",
        Request:        "What's the weather in Tokyo?",
    }

    prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if !strings.Contains(prompt, "Tokyo") {
        t.Error("prompt should contain the request")
    }
}
```

### Integration Testing with Factory

```go
func TestOrchestratorWithCustomBuilder(t *testing.T) {
    // Track if our custom builder was called
    builderCalled := false

    customBuilder := &mockPromptBuilder{
        buildFunc: func(ctx context.Context, input PromptInput) (string, error) {
            builderCalled = true
            return "Custom: " + input.Request, nil
        },
    }

    deps := orchestration.OrchestratorDependencies{
        Discovery:     NewMockDiscovery(),
        AIClient:      NewMockAIClient(),
        PromptBuilder: customBuilder,
    }

    orchestrator, err := orchestration.CreateOrchestrator(
        orchestration.DefaultConfig(),
        deps,
    )
    require.NoError(t, err)

    // Verify by making a request (tests within the orchestration package
    // can access orchestrator.promptBuilder directly)
    _, _ = orchestrator.ProcessRequest(ctx, "test request", nil)
    assert.True(t, builderCalled, "Custom builder should have been called")
}
```

## Migration Guide

### From Hardcoded Prompts

Before:
```go
prompt := fmt.Sprintf("You are an orchestrator. Request: %s", request)
```

After:
```go
config := orchestration.DefaultConfig()
config.PromptConfig.CustomInstructions = []string{
    "Your custom instruction here",
}
```

### From Custom String Building

Before:
```go
func buildPrompt(capabilities, request string) string {
    return fmt.Sprintf(`...complex prompt...`, capabilities, request)
}
```

After:
```go
config.PromptConfig.Template = `...your template with {{.CapabilityInfo}} and {{.Request}}...`
```

## Examples

### E-commerce Domain

```go
config.PromptConfig = orchestration.PromptConfig{
    Domain: "retail",
    AdditionalTypeRules: []orchestration.TypeRule{
        {
            TypeNames: []string{"sku", "product_id"},
            JsonType:  "JSON strings",
            Example:   `"SKU-12345"`,
        },
        {
            TypeNames: []string{"price", "amount"},
            JsonType:  "JSON numbers",
            Example:   `29.99`,
            AntiPattern: `"29.99"`,
        },
        {
            TypeNames: []string{"quantity"},
            JsonType:  "JSON integers",
            Example:   `5`,
        },
    },
    CustomInstructions: []string{
        "Always validate inventory before order placement",
        "Include shipping estimates in travel-related queries",
    },
}
```

### Multi-Language Support

```go
// Template with language awareness
config.PromptConfig.Template = `{{if eq .Domain "ja"}}
あなたはAIオーケストレーターです。
{{else}}
You are an AI orchestrator.
{{end}}

Available: {{.CapabilityInfo}}
Request: {{.Request}}
{{.TypeRules}}

Response (JSON only):`
```

### Kubernetes ConfigMap Template

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orchestrator-prompt
  namespace: gomind
data:
  planning-prompt.tmpl: |
    You are an AI orchestrator for the {{.Domain}} domain.

    Available Capabilities:
    {{.CapabilityInfo}}

    User Request: {{.Request}}

    Type Rules:
    {{.TypeRules}}

    {{if .CustomInstructions}}
    Additional Instructions:
    {{.CustomInstructions}}
    {{end}}

    Generate a JSON execution plan:
    {{.JSONStructure}}

    Response (JSON only):
```

Mount in deployment:
```yaml
spec:
  containers:
  - name: agent
    env:
    - name: GOMIND_PROMPT_TEMPLATE_FILE
      value: /config/planning-prompt.tmpl
    volumeMounts:
    - name: prompt-config
      mountPath: /config
  volumes:
  - name: prompt-config
    configMap:
      name: orchestrator-prompt
```

## API Reference

### Interfaces

```go
// PromptBuilder defines the interface for building LLM orchestration prompts
type PromptBuilder interface {
    BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error)
}
```

### Constructors

```go
// Create default builder with optional config
func NewDefaultPromptBuilder(config *PromptConfig) (*DefaultPromptBuilder, error)

// Create template-based builder (requires Template or TemplateFile)
func NewTemplatePromptBuilder(config *PromptConfig) (*TemplatePromptBuilder, error)
```

### Validation

```go
// Validate a TypeRule before use
func ValidateTypeRule(rule TypeRule) error
```

### Configuration Loading

```go
// Load from environment variables (method on PromptConfig)
func (c *PromptConfig) LoadFromEnv() error

// Load from environment, panic on error (method on PromptConfig)
func (c *PromptConfig) MustLoadFromEnv()
```

## Troubleshooting

### Template Not Loading

Check logs for:
```
WARN Failed to create TemplatePromptBuilder, using default
```

Verify:
1. File path is correct and accessible
2. Template syntax is valid Go `text/template`
3. ConfigMap is mounted correctly (Kubernetes)

### Type Rules Not Applied

Verify:
1. `TypeNames` matches capability parameter types exactly
2. Rules are in `AdditionalTypeRules` array
3. Check logs for `type_rules_count` attribute

### Missing Metrics

Verify:
1. Telemetry is initialized before orchestrator creation
2. `deps.Telemetry` is set (or use factory auto-injection)
3. OTEL collector is receiving data

### Prompt Too Long

If prompts exceed token limits:
1. Reduce `CustomInstructions`
2. Set `IncludeAntiPatterns: false`
3. Use template to remove unnecessary sections

## Source Files

This guide references the following source files:

| File | Description |
|------|-------------|
| [prompt_builder.go](prompt_builder.go) | `PromptBuilder` interface, `PromptInput`, `TypeRule`, `PromptConfig` structs |
| [default_prompt_builder.go](default_prompt_builder.go) | `DefaultPromptBuilder` implementation with type rules |
| [template_prompt_builder.go](template_prompt_builder.go) | `TemplatePromptBuilder` implementation |
| [prompt_config_env.go](prompt_config_env.go) | Environment variable loading (`LoadFromEnv`, `MustLoadFromEnv`) |
| [prompt_builder_metrics.go](prompt_builder_metrics.go) | Metric and span name constants |
| [factory.go](factory.go) | `CreateOrchestrator` factory, `OrchestratorDependencies` struct |
| [interfaces.go](interfaces.go) | `OrchestratorConfig` struct, `DefaultConfig()` |
