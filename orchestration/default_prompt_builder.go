package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

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

// SetLogger sets the logger for debug output
func (d *DefaultPromptBuilder) SetLogger(logger core.Logger) {
	d.logger = logger
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
		ctx, span = d.telemetry.StartSpan(ctx, SpanPromptBuilderBuild)
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
      "instruction": "specific instruction for this agent",
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
    }
  ]
}

CRITICAL - Parameter Type Rules:
%s
%s
%s
Important:
1. Only use agents and capabilities that exist in the catalog
2. Ensure parameter names AND TYPES match exactly what the capability expects
3. Order steps based on dependencies
4. Include all necessary steps to fulfill the request
5. Be specific in instructions
6. For coordinates (lat/lon), use numeric values like 35.6897 not "35.6897"

Response (JSON only, no explanation):`,
		input.CapabilityInfo,
		input.Request,
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
		d.logger.Debug("Built planning prompt", map[string]interface{}{
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
		instructions = append(instructions, fmt.Sprintf("%d. %s", i+7, inst)) // Start after default instructions
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
