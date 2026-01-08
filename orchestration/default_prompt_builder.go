package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// stepReferenceInstructions provides clear guidance for LLM on cross-step data references.
// This addresses the template syntax mismatch bug where LLM generates single braces
// but executor expects double braces.
const stepReferenceInstructions = `
CROSS-STEP DATA REFERENCES:
When a step needs data from a previous step's output, use template syntax.

SYNTAX RULES:
| Component       | Format                              |
|-----------------|-------------------------------------|
| Full template   | {{<step_id>.response.<field_path>}} |
| Step ID         | Must exist in depends_on array      |
| Field path      | Dot notation for nested fields      |
| Braces          | DOUBLE curly braces {{ }}           |

RESOLUTION EXAMPLES:
If step-1 returns: {"status": "ok", "data": {"id": 123, "items": ["a", "b"], "currency": {"code": "EUR", "name": "Euro"}}}

| Template Reference                        | Resolves To    |
|-------------------------------------------|----------------|
| {{step-1.response.status}}                | "ok"           |
| {{step-1.response.data.id}}               | 123            |
| {{step-1.response.data.items}}            | ["a", "b"]     |
| {{step-1.response.data.currency.code}}    | "EUR"          |
| {{step-1.response.data.currency.name}}    | "Euro"         |

NESTED OBJECT ACCESS (IMPORTANT):
When referencing data from steps that return complex objects, always access the SPECIFIC FIELD you need:

| If step returns an object like              | Use this template                          | NOT this                          |
|---------------------------------------------|--------------------------------------------|-----------------------------------|
| {"currency": {"code": "EUR", "name": "Euro"}} | {{step.response.data.currency.code}}     | {{step.response.data.currency}}   |
| {"location": {"lat": 35.6, "lon": 139.7}}   | {{step.response.data.location.lat}}       | {{step.response.data.location}}   |
| {"country": {"name": "France", "code": "FR"}} | {{step.response.data.country.code}}     | {{step.response.data.country}}    |

WHY: Downstream tools expect primitive values (strings, numbers), not JSON objects.

CORRECT USAGE:
{
  "step_id": "step-2",
  "depends_on": ["step-1"],
  "metadata": {
    "parameters": { "id": "{{step-1.response.data.id}}" }
  }
}

WRONG (will fail):
{ "id": "{step-1.response.data.id}" }   // Single braces - NOT supported
{ "id": "{{step-1.data.id}}" }          // Missing 'response' in path
{ "id": "{{step-1.response.data.id}}" } // Without depends_on - NOT resolved

CRITICAL: Always use DOUBLE curly braces {{...}}, never single braces {...}`

// DefaultPromptBuilder provides comprehensive type rules out of the box.
// It handles all common JSON types and can be extended with additional rules.
//
// This is the default builder used when no custom PromptBuilder is configured.
// It follows the "works with zero configuration" principle.
type DefaultPromptBuilder struct {
	config    *PromptConfig
	typeRules []TypeRule
	logger    core.Logger
	telemetry core.Telemetry
}

// NewDefaultPromptBuilder creates a builder with sensible defaults.
//
// Default type rules cover:
//   - string: JSON strings
//   - number/float64/float32/float: JSON numbers
//   - integer/int/int64/int32: JSON integers
//   - boolean/bool: JSON booleans
//   - array/[]string/[]int/list: JSON arrays
//   - object/map/struct: JSON objects
//
// Additional rules from config.AdditionalTypeRules are appended.
func NewDefaultPromptBuilder(config *PromptConfig) (*DefaultPromptBuilder, error) {
	if config == nil {
		config = &PromptConfig{}
	}

	// Core type rules (always included)
	// These cover the most common parameter types
	defaultRules := []TypeRule{
		{
			TypeNames:   []string{"string"},
			JsonType:    "JSON strings",
			Example:     `"text value"`,
			Description: "Text values should be quoted strings",
		},
		{
			TypeNames:   []string{"number", "float64", "float32", "float"},
			JsonType:    "JSON numbers",
			Example:     `35.6897`,
			AntiPattern: `"35.6897"`,
			Description: "Numeric values with decimals (coordinates, amounts, rates)",
		},
		{
			TypeNames:   []string{"integer", "int", "int64", "int32"},
			JsonType:    "JSON integers",
			Example:     `10`,
			AntiPattern: `"10"`,
			Description: "Whole numbers without decimals (counts, IDs)",
		},
		{
			TypeNames:   []string{"boolean", "bool"},
			JsonType:    "JSON booleans",
			Example:     `true`,
			AntiPattern: `"true"`,
			Description: "Boolean true/false values (flags, toggles)",
		},
		{
			TypeNames:   []string{"array", "[]string", "[]int", "[]float64", "list"},
			JsonType:    "JSON arrays",
			Example:     `["item1", "item2"]`,
			AntiPattern: `"[\"item1\"]"`,
			Description: "Arrays/lists of values (tags, currencies, IDs)",
		},
		{
			TypeNames:   []string{"object", "map", "struct", "map[string]interface{}"},
			JsonType:    "JSON objects",
			Example:     `{"key": "value", "count": 5}`,
			AntiPattern: `"{\"key\": \"value\"}"`,
			Description: "Nested objects with key-value pairs (options, filters)",
		},
	}

	// Validate additional rules from config
	for i, rule := range config.AdditionalTypeRules {
		if err := ValidateTypeRule(rule); err != nil {
			return nil, fmt.Errorf("invalid additional type rule at index %d: %w", i, err)
		}
	}

	// Merge additional rules from config
	allRules := append(defaultRules, config.AdditionalTypeRules...)

	return &DefaultPromptBuilder{
		config:    config,
		typeRules: allRules,
	}, nil
}

// SetLogger sets the logger for debug output (follows framework design principles)
// The component is always set to "framework/orchestration" to ensure proper log attribution
// regardless of which agent or tool is using the orchestration module.
func (d *DefaultPromptBuilder) SetLogger(logger core.Logger) {
	if logger == nil {
		d.logger = nil
	} else {
		if cal, ok := logger.(core.ComponentAwareLogger); ok {
			d.logger = cal.WithComponent("framework/orchestration")
		} else {
			d.logger = logger
		}
	}
}

// SetTelemetry allows dependency injection of telemetry
func (d *DefaultPromptBuilder) SetTelemetry(t core.Telemetry) {
	d.telemetry = t
}

// BuildPlanningPrompt implements PromptBuilder interface
func (d *DefaultPromptBuilder) BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error) {
	start := time.Now()
	status := "success"

	// Distributed tracing: Create span if telemetry is available
	// Following FRAMEWORK_DESIGN_PRINCIPLES.md: always check for nil
	var span core.Span
	if d.telemetry != nil {
		// Note: We discard the returned context since this function doesn't make
		// downstream calls that need trace propagation. The span is purely for timing.
		_, span = d.telemetry.StartSpan(ctx, SpanPromptBuilderBuild)
		defer span.End()

		// Add span attributes for debugging
		span.SetAttribute("builder_type", "default")
		span.SetAttribute("domain", d.config.Domain)
		span.SetAttribute("type_rules_count", len(d.typeRules))
		span.SetAttribute("request_length", len(input.Request))
	}

	// Build type rules section
	typeRulesSection := d.buildTypeRulesSection()

	// Build custom instructions section
	customInstructionsSection := d.buildCustomInstructionsSection()

	// Build domain-specific section
	domainSection := d.buildDomainSection()

	// Construct the complete prompt
	prompt := fmt.Sprintf(`You are an AI orchestrator managing a multi-agent system.

%s

User Request: %s

Create an execution plan in JSON format with the following structure:
{
  "plan_id": "unique-id",
  "original_request": "the user request",
  "mode": "autonomous",
  "steps": [
    {
      "step_id": "step-1",
      "agent_name": "agent-name-from-catalog",
      "namespace": "default",
      "instruction": "specific instruction for this step",
      "depends_on": [],
      "metadata": {
        "capability": "capability-name",
        "parameters": {
          "string_param": "text value",
          "number_param": 42.5,
          "integer_param": 10,
          "boolean_param": true,
          "array_param": ["item1", "item2"]
        }
      }
    },
    {
      "step_id": "step-2",
      "agent_name": "another-agent",
      "namespace": "default",
      "instruction": "use data from step-1",
      "depends_on": ["step-1"],
      "metadata": {
        "capability": "another-capability",
        "parameters": {
          "input_id": "{{step-1.response.data.id}}",
          "input_name": "{{step-1.response.data.name}}"
        }
      }
    }
  ]
}

%s

CRITICAL - Parameter Type Rules:
%s
%s
%s
Important:
1. Only use agents and capabilities that exist in the catalog
2. Ensure parameter names AND TYPES match exactly what the capability expects
3. Order steps logically - steps can only depend on earlier steps
4. Use {{stepId.response.field}} syntax for cross-step data references
5. Always declare dependencies in the depends_on array before referencing
6. Include all necessary steps to fulfill the request
7. Be specific in instructions - what should each step accomplish?
8. For coordinates (lat/lon), use numeric values like 35.6897 not "35.6897"

CRITICAL FORMAT RULES (applies to all LLM providers):
- You are a JSON API. Your ONLY output is a raw JSON object.
- Output ONLY valid JSON - no markdown, no code blocks, no backticks
- Do NOT use any text formatting: no ** (bold), no * (italic), no __ (underline)
- Do NOT wrap JSON in code fences (no triple backticks)
- Do NOT include any explanatory text before or after the JSON
- String values must be plain text without any markdown formatting
- Start your response with { and end with } - nothing else

Response (raw JSON only, no formatting):`,
		input.CapabilityInfo,
		input.Request,
		stepReferenceInstructions,
		typeRulesSection,
		domainSection,
		customInstructionsSection,
	)

	// Calculate duration
	durationMs := float64(time.Since(start).Milliseconds())

	// Emit metrics only if telemetry is available (fail-safe pattern)
	if d.telemetry != nil {
		d.telemetry.RecordMetric("orchestrator.prompt.build_duration_ms", durationMs,
			map[string]string{
				"builder_type": "default",
				"domain":       d.config.Domain,
				"status":       status,
			})

		d.telemetry.RecordMetric("orchestrator.prompt.built", 1,
			map[string]string{
				"builder_type": "default",
				"domain":       d.config.Domain,
				"status":       status,
			})

		d.telemetry.RecordMetric("orchestrator.prompt.size_bytes", float64(len(prompt)),
			map[string]string{
				"builder_type": "default",
			})

		// Add span event for completion
		if span != nil {
			span.SetAttribute("prompt_length", len(prompt))
			span.SetAttribute("duration_ms", durationMs)
		}
	}

	// Structured logging (logger may also be nil - fail-safe)
	if d.logger != nil {
		d.logger.DebugWithContext(ctx, "Built planning prompt", map[string]interface{}{
			"builder_type":        "default",
			"type_rules_count":    len(d.typeRules),
			"custom_instructions": len(d.config.CustomInstructions),
			"domain":              d.config.Domain,
			"prompt_length":       len(prompt),
			"duration_ms":         durationMs,
		})
	}

	return prompt, nil
}

// buildTypeRulesSection generates the type rules text for the prompt
func (d *DefaultPromptBuilder) buildTypeRulesSection() string {
	var rules []string
	includeAnti := d.config.IncludeAntiPatterns == nil || *d.config.IncludeAntiPatterns

	for _, rule := range d.typeRules {
		typeNames := strings.Join(rule.TypeNames, `" or "`)
		line := fmt.Sprintf(`- Parameters with type "%s" MUST be %s (e.g., %s)`,
			typeNames, rule.JsonType, rule.Example)

		if includeAnti && rule.AntiPattern != "" {
			line += fmt.Sprintf(`, NOT strings (e.g., %s)`, rule.AntiPattern)
		}

		rules = append(rules, line)
	}

	return strings.Join(rules, "\n")
}

// buildCustomInstructionsSection generates custom instructions text
func (d *DefaultPromptBuilder) buildCustomInstructionsSection() string {
	if len(d.config.CustomInstructions) == 0 {
		return ""
	}

	var instructions []string
	for i, inst := range d.config.CustomInstructions {
		instructions = append(instructions, fmt.Sprintf("%d. %s", i+9, inst)) // Start after default instructions (8 default + 1)
	}

	return "\n" + strings.Join(instructions, "\n")
}

// buildDomainSection generates domain-specific instructions
func (d *DefaultPromptBuilder) buildDomainSection() string {
	switch d.config.Domain {
	case "healthcare":
		return `
HEALTHCARE DOMAIN REQUIREMENTS:
- Never include PHI (Protected Health Information) in logs
- Prefer HIPAA-compliant tools when available
- Include audit trail metadata in all steps`

	case "finance":
		return `
FINANCE DOMAIN REQUIREMENTS:
- All monetary calculations must preserve decimal precision
- Include transaction IDs for audit compliance
- Prefer SOX-compliant tools when available`

	case "legal":
		return `
LEGAL DOMAIN REQUIREMENTS:
- Maintain chain of custody for all data transformations
- Include timestamp and source attribution in all steps
- Flag any steps that modify original documents`

	default:
		return ""
	}
}

// GetTypeRules returns the current type rules (useful for debugging)
func (d *DefaultPromptBuilder) GetTypeRules() []TypeRule {
	return d.typeRules
}

// GetConfig returns the current configuration (useful for debugging)
func (d *DefaultPromptBuilder) GetConfig() *PromptConfig {
	return d.config
}
