package orchestration

import (
	"context"
)

// PromptBuilder defines the interface for building LLM orchestration prompts.
// This follows the same pattern as CapabilityProvider for consistency.
//
// Implementations can:
// - Extend the default prompt with additional type rules (Layer 1)
// - Use templates for structural customization (Layer 2)
// - Provide complete custom prompt logic (Layer 3)
//
// The interface is intentionally minimal following the
// Minimal Interface Principle from CORE_DESIGN_PRINCIPLES.md.
type PromptBuilder interface {
	// BuildPlanningPrompt creates the prompt for LLM-based orchestration.
	//
	// Parameters:
	//   - ctx: Context for cancellation, tracing, and request-scoped values
	//   - input: All data needed to build the prompt
	//
	// Returns:
	//   - The complete prompt string ready for LLM consumption
	//   - Error if prompt building fails
	//
	// Implementations should:
	//   - Include capability information for agent/tool discovery
	//   - Specify JSON output format requirements
	//   - Include type rules for accurate parameter serialization
	//   - Add domain-specific instructions if applicable
	BuildPlanningPrompt(ctx context.Context, input PromptInput) (string, error)
}

// PromptInput contains all data needed to build an orchestration prompt.
// This is passed to PromptBuilder.BuildPlanningPrompt().
type PromptInput struct {
	// CapabilityInfo is the formatted string of available agents and tools.
	// This comes from AgentCatalog.FormatForLLM() or CapabilityProvider.
	// The LLM uses this to understand what capabilities are available.
	CapabilityInfo string

	// Request is the user's natural language request to be orchestrated.
	// Example: "What's the weather in Tokyo and convert 1000 USD to JPY?"
	Request string

	// Metadata contains optional context for prompt customization.
	// Examples:
	//   - "domain": "healthcare" for HIPAA-aware prompts
	//   - "user_id": "123" for audit logging
	//   - "priority": "high" for SLA-aware routing
	//   - "language": "ja" for localized prompts
	Metadata map[string]interface{}
}

// TypeRule defines how the LLM should handle a specific parameter type.
// This ensures the LLM generates correct JSON types in the execution plan.
type TypeRule struct {
	// TypeNames are the type strings that trigger this rule.
	// Multiple names allow handling type aliases.
	// Examples: ["number", "float64", "float32", "float"]
	TypeNames []string `json:"type_names"`

	// JsonType is the human-readable JSON type description.
	// This is shown to the LLM in the prompt.
	// Example: "JSON numbers"
	JsonType string `json:"json_type"`

	// Example shows a correct value for this type.
	// Example: "35.6897" for numbers
	Example string `json:"example"`

	// AntiPattern shows what NOT to do (optional).
	// This helps the LLM avoid common mistakes.
	// Example: "\"35.6897\"" (string-quoted number)
	AntiPattern string `json:"anti_pattern,omitempty"`

	// Description provides additional context (optional).
	// Example: "Numeric values with decimals, used for coordinates and amounts"
	Description string `json:"description,omitempty"`
}

// PromptConfig configures prompt building behavior.
// This is part of OrchestratorConfig.
type PromptConfig struct {
	// Layer 1: Additional type rules to extend defaults
	// These are appended to DefaultPromptBuilder's built-in rules.
	// Use this to add support for new types without replacing the entire prompt.
	AdditionalTypeRules []TypeRule `json:"additional_type_rules,omitempty"`

	// Layer 2: Template-based customization
	// TemplateFile takes precedence over Template if both are set.

	// TemplateFile is the path to a Go text/template file.
	// In Kubernetes, this is typically a ConfigMap mount path.
	// Example: "/config/planning-prompt.tmpl"
	TemplateFile string `json:"template_file,omitempty"`

	// Template is an inline Go text/template string.
	// Use this for simpler templates that don't need external files.
	Template string `json:"template,omitempty"`

	// CustomInstructions are additional instructions appended to the prompt.
	// These are added after type rules but before the response instruction.
	// Example: ["Always prefer local tools over remote ones", "Minimize API calls"]
	CustomInstructions []string `json:"custom_instructions,omitempty"`

	// Domain provides context for domain-specific prompt adjustments.
	// The DefaultPromptBuilder uses this to add domain-specific instructions.
	// Examples: "healthcare", "finance", "legal", "retail"
	Domain string `json:"domain,omitempty"`

	// IncludeAntiPatterns controls whether to show "what NOT to do" examples.
	// Default: true (recommended for better LLM guidance)
	IncludeAntiPatterns *bool `json:"include_anti_patterns,omitempty"`
}

// ValidateTypeRule validates a TypeRule for correctness.
// Returns an error if the rule is invalid.
func ValidateTypeRule(rule TypeRule) error {
	if len(rule.TypeNames) == 0 {
		return &ValidationError{Field: "TypeNames", Message: "must have at least one type name"}
	}
	if rule.JsonType == "" {
		return &ValidationError{Field: "JsonType", Message: "must not be empty"}
	}
	if rule.Example == "" {
		return &ValidationError{Field: "Example", Message: "must not be empty"}
	}
	return nil
}

// ValidationError represents a validation error for prompt builder configuration
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error for " + e.Field + ": " + e.Message
}
