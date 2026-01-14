package orchestration

import (
	"context"
	"os"
	"strings"
	"testing"
)

// =============================================================================
// TypeRule Validation Tests
// =============================================================================

func TestValidateTypeRule_Valid(t *testing.T) {
	rule := TypeRule{
		TypeNames: []string{"number", "float64"},
		JsonType:  "JSON numbers",
		Example:   "42.5",
	}

	err := ValidateTypeRule(rule)
	if err != nil {
		t.Errorf("expected no error for valid rule, got: %v", err)
	}
}

func TestValidateTypeRule_EmptyTypeNames(t *testing.T) {
	rule := TypeRule{
		TypeNames: []string{},
		JsonType:  "JSON numbers",
		Example:   "42.5",
	}

	err := ValidateTypeRule(rule)
	if err == nil {
		t.Error("expected error for empty TypeNames")
	}
	if verr, ok := err.(*ValidationError); ok {
		if verr.Field != "TypeNames" {
			t.Errorf("expected field 'TypeNames', got: %s", verr.Field)
		}
	}
}

func TestValidateTypeRule_EmptyJsonType(t *testing.T) {
	rule := TypeRule{
		TypeNames: []string{"number"},
		JsonType:  "",
		Example:   "42.5",
	}

	err := ValidateTypeRule(rule)
	if err == nil {
		t.Error("expected error for empty JsonType")
	}
	if verr, ok := err.(*ValidationError); ok {
		if verr.Field != "JsonType" {
			t.Errorf("expected field 'JsonType', got: %s", verr.Field)
		}
	}
}

func TestValidateTypeRule_EmptyExample(t *testing.T) {
	rule := TypeRule{
		TypeNames: []string{"number"},
		JsonType:  "JSON numbers",
		Example:   "",
	}

	err := ValidateTypeRule(rule)
	if err == nil {
		t.Error("expected error for empty Example")
	}
	if verr, ok := err.(*ValidationError); ok {
		if verr.Field != "Example" {
			t.Errorf("expected field 'Example', got: %s", verr.Field)
		}
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "TestField", Message: "test message"}
	expected := "validation error for TestField: test message"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

// =============================================================================
// DefaultPromptBuilder Tests
// =============================================================================

func TestNewDefaultPromptBuilder_NilConfig(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if builder == nil {
		t.Fatal("expected builder, got nil")
	}

	// Should have default type rules
	rules := builder.GetTypeRules()
	if len(rules) < 6 {
		t.Errorf("expected at least 6 default rules, got: %d", len(rules))
	}
}

func TestNewDefaultPromptBuilder_WithAdditionalRules(t *testing.T) {
	config := &PromptConfig{
		AdditionalTypeRules: []TypeRule{
			{
				TypeNames: []string{"currency"},
				JsonType:  "JSON strings",
				Example:   `"USD"`,
			},
		},
	}

	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rules := builder.GetTypeRules()
	// Should have default rules + 1 additional
	if len(rules) < 7 {
		t.Errorf("expected at least 7 rules (6 default + 1 custom), got: %d", len(rules))
	}
}

func TestNewDefaultPromptBuilder_InvalidAdditionalRule(t *testing.T) {
	config := &PromptConfig{
		AdditionalTypeRules: []TypeRule{
			{
				TypeNames: []string{}, // Invalid: empty
				JsonType:  "JSON strings",
				Example:   `"test"`,
			},
		},
	}

	_, err := NewDefaultPromptBuilder(config)
	if err == nil {
		t.Error("expected error for invalid additional rule")
	}
}

func TestDefaultPromptBuilder_BuildPlanningPrompt(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: weather-tool, currency-tool",
		Request:        "What is the weather in Tokyo?",
		Metadata:       nil,
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify prompt contains key elements
	checks := []string{
		"Available tools: weather-tool, currency-tool",
		"What is the weather in Tokyo?",
		"JSON numbers",
		"JSON strings",
		"plan_id",
		"step_id",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt should contain %q", check)
		}
	}
}

func TestDefaultPromptBuilder_DomainHealthcare(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{
		Domain: "healthcare",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: patient-lookup",
		Request:        "Look up patient records",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify healthcare-specific content
	if !strings.Contains(prompt, "HEALTHCARE DOMAIN REQUIREMENTS") {
		t.Error("prompt should contain healthcare domain requirements")
	}
	if !strings.Contains(prompt, "HIPAA") {
		t.Error("prompt should mention HIPAA")
	}
}

func TestDefaultPromptBuilder_DomainFinance(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{
		Domain: "finance",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: trading-tool",
		Request:        "Execute trade",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "FINANCE DOMAIN REQUIREMENTS") {
		t.Error("prompt should contain finance domain requirements")
	}
}

func TestDefaultPromptBuilder_DomainLegal(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{
		Domain: "legal",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: document-tool",
		Request:        "Review contract",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "LEGAL DOMAIN REQUIREMENTS") {
		t.Error("prompt should contain legal domain requirements")
	}
}

func TestDefaultPromptBuilder_CustomInstructions(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{
		CustomInstructions: []string{
			"Always use local tools first",
			"Minimize API calls",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: test-tool",
		Request:        "Test request",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "Always use local tools first") {
		t.Error("prompt should contain first custom instruction")
	}
	if !strings.Contains(prompt, "Minimize API calls") {
		t.Error("prompt should contain second custom instruction")
	}
}

func TestDefaultPromptBuilder_DisableAntiPatterns(t *testing.T) {
	includeAnti := false
	builder, err := NewDefaultPromptBuilder(&PromptConfig{
		IncludeAntiPatterns: &includeAnti,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: test-tool",
		Request:        "Test request",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Anti-patterns should NOT be included
	if strings.Contains(prompt, `NOT strings (e.g., "35.6897")`) {
		t.Error("prompt should NOT contain anti-patterns when disabled")
	}
}

func TestDefaultPromptBuilder_SetLogger(t *testing.T) {
	builder, _ := NewDefaultPromptBuilder(nil)
	mockLogger := &MockLogger{}

	builder.SetLogger(mockLogger)

	// Build prompt to trigger logging
	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}
	_, _ = builder.BuildPlanningPrompt(context.Background(), input)

	// Logger should have been called
	if len(mockLogger.debugCalls) == 0 {
		t.Error("expected logger.Debug to be called")
	}
}

// =============================================================================
// Step Reference Instructions Tests (STEP_REFERENCE_TEMPLATE_BUG fix)
// =============================================================================

func TestDefaultPromptBuilder_IncludesStepReferenceInstructions(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Available tools: item-lookup-tool, inventory-tool",
		Request:        "Get item details and check inventory",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify critical step reference instructions are present
	criticalElements := []string{
		"CROSS-STEP DATA REFERENCES",
		"{{<step_id>.response.<field_path>}}",
		"DOUBLE curly braces",
		"depends_on",
	}

	for _, element := range criticalElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("prompt should contain %q for step reference guidance", element)
		}
	}
}

func TestDefaultPromptBuilder_IncludesStep2TemplateExample(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify step-2 example with template references is present
	step2Elements := []string{
		"step-2",
		`"depends_on": ["step-1"]`,
		"{{step-1.response.data.id}}",
		"{{step-1.response.data.name}}",
	}

	for _, element := range step2Elements {
		if !strings.Contains(prompt, element) {
			t.Errorf("prompt should contain step-2 example with %q", element)
		}
	}
}

func TestDefaultPromptBuilder_IncludesAntiPatternWarnings(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify anti-pattern warnings are present
	antiPatterns := []string{
		"WRONG (will fail)",
		"{step-1.response.data.id}", // Single braces shown as wrong
		"Single braces - NOT supported",
	}

	for _, antiPattern := range antiPatterns {
		if !strings.Contains(prompt, antiPattern) {
			t.Errorf("prompt should contain anti-pattern warning %q", antiPattern)
		}
	}
}

func TestDefaultPromptBuilder_IncludesResolutionExamples(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify resolution examples are present (shows what templates resolve to)
	resolutionElements := []string{
		"RESOLUTION EXAMPLES",
		"{{step-1.response.status}}",
		"{{step-1.response.data.id}}",
		"Resolves To",
	}

	for _, element := range resolutionElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("prompt should contain resolution example %q", element)
		}
	}
}

func TestDefaultPromptBuilder_ImportantSectionIncludesStepReferences(t *testing.T) {
	builder, err := NewDefaultPromptBuilder(&PromptConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Important section mentions step references
	importantRules := []string{
		"Use {{stepId.response.field}} syntax for cross-step data references",
		"Always declare dependencies in the depends_on array before referencing",
	}

	for _, rule := range importantRules {
		if !strings.Contains(prompt, rule) {
			t.Errorf("prompt Important section should contain %q", rule)
		}
	}
}

// =============================================================================
// TemplatePromptBuilder Tests
// =============================================================================

func TestNewTemplatePromptBuilder_NilConfig(t *testing.T) {
	_, err := NewTemplatePromptBuilder(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewTemplatePromptBuilder_NoTemplate(t *testing.T) {
	_, err := NewTemplatePromptBuilder(&PromptConfig{})
	if err == nil {
		t.Error("expected error when neither TemplateFile nor Template is set")
	}
}

func TestNewTemplatePromptBuilder_InlineTemplate(t *testing.T) {
	config := &PromptConfig{
		Template: `Capabilities: {{.CapabilityInfo}}
Request: {{.Request}}
Type Rules: {{.TypeRules}}`,
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if builder == nil {
		t.Fatal("expected builder, got nil")
	}
}

func TestNewTemplatePromptBuilder_InvalidTemplate(t *testing.T) {
	config := &PromptConfig{
		Template: `{{.InvalidSyntax`, // Missing closing braces
	}

	_, err := NewTemplatePromptBuilder(config)
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestTemplatePromptBuilder_BuildPlanningPrompt(t *testing.T) {
	config := &PromptConfig{
		Template: `=== CUSTOM TEMPLATE ===
Capabilities: {{.CapabilityInfo}}
Request: {{.Request}}
Domain: {{.Domain}}
=== END ===`,
		Domain: "test-domain",
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "tool-a, tool-b",
		Request:        "Do something",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"=== CUSTOM TEMPLATE ===",
		"Capabilities: tool-a, tool-b",
		"Request: Do something",
		"Domain: test-domain",
		"=== END ===",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt should contain %q, got: %s", check, prompt)
		}
	}
}

func TestTemplatePromptBuilder_TemplateWithTypeRules(t *testing.T) {
	config := &PromptConfig{
		Template: `Type Rules:
{{.TypeRules}}`,
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "test",
		Request:        "test",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include default type rules
	if !strings.Contains(prompt, "JSON numbers") {
		t.Error("prompt should contain type rules from fallback")
	}
}

func TestTemplatePromptBuilder_FileNotFound(t *testing.T) {
	config := &PromptConfig{
		TemplateFile: "/nonexistent/path/template.tmpl",
	}

	_, err := NewTemplatePromptBuilder(config)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTemplatePromptBuilder_TemplateFile(t *testing.T) {
	// Create temporary template file
	tmpFile, err := os.CreateTemp("", "test-template-*.tmpl")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	templateContent := `FILE TEMPLATE
Request: {{.Request}}
Capabilities: {{.CapabilityInfo}}`
	if _, err := tmpFile.WriteString(templateContent); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}
	tmpFile.Close()

	config := &PromptConfig{
		TemplateFile: tmpFile.Name(),
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "file-test-tool",
		Request:        "file test request",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "FILE TEMPLATE") {
		t.Error("prompt should contain content from template file")
	}
	if !strings.Contains(prompt, "file-test-tool") {
		t.Error("prompt should contain capability info")
	}
}

func TestTemplatePromptBuilder_GetFallback(t *testing.T) {
	config := &PromptConfig{
		Template: "test",
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fallback := builder.GetFallback()
	if fallback == nil {
		t.Error("expected fallback builder, got nil")
	}
}

func TestTemplatePromptBuilder_SetLoggerPropagates(t *testing.T) {
	config := &PromptConfig{
		Template: "test",
	}

	builder, err := NewTemplatePromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockLogger := &MockLogger{}
	builder.SetLogger(mockLogger)

	// Fallback should also have the logger
	if builder.GetFallback() == nil {
		t.Error("fallback should exist")
	}
}

// =============================================================================
// PromptConfig Environment Loading Tests
// =============================================================================

func TestPromptConfig_LoadFromEnv_TemplateFile(t *testing.T) {
	os.Setenv("GOMIND_PROMPT_TEMPLATE_FILE", "/config/custom-template.tmpl")
	defer os.Unsetenv("GOMIND_PROMPT_TEMPLATE_FILE")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.TemplateFile != "/config/custom-template.tmpl" {
		t.Errorf("expected template file path, got: %s", config.TemplateFile)
	}
}

func TestPromptConfig_LoadFromEnv_Domain(t *testing.T) {
	os.Setenv("GOMIND_PROMPT_DOMAIN", "healthcare")
	defer os.Unsetenv("GOMIND_PROMPT_DOMAIN")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Domain != "healthcare" {
		t.Errorf("expected domain 'healthcare', got: %s", config.Domain)
	}
}

func TestPromptConfig_LoadFromEnv_TypeRules(t *testing.T) {
	rulesJSON := `[{"type_names":["custom_type"],"json_type":"JSON custom","example":"test"}]`
	os.Setenv("GOMIND_PROMPT_TYPE_RULES", rulesJSON)
	defer os.Unsetenv("GOMIND_PROMPT_TYPE_RULES")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config.AdditionalTypeRules) != 1 {
		t.Fatalf("expected 1 type rule, got: %d", len(config.AdditionalTypeRules))
	}
	if config.AdditionalTypeRules[0].TypeNames[0] != "custom_type" {
		t.Errorf("unexpected type name: %v", config.AdditionalTypeRules[0].TypeNames)
	}
}

func TestPromptConfig_LoadFromEnv_InvalidTypeRulesJSON(t *testing.T) {
	os.Setenv("GOMIND_PROMPT_TYPE_RULES", "invalid json")
	defer os.Unsetenv("GOMIND_PROMPT_TYPE_RULES")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPromptConfig_LoadFromEnv_InvalidTypeRule(t *testing.T) {
	// Valid JSON but invalid rule (empty type_names)
	rulesJSON := `[{"type_names":[],"json_type":"JSON custom","example":"test"}]`
	os.Setenv("GOMIND_PROMPT_TYPE_RULES", rulesJSON)
	defer os.Unsetenv("GOMIND_PROMPT_TYPE_RULES")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid type rule")
	}
}

func TestPromptConfig_LoadFromEnv_CustomInstructions(t *testing.T) {
	instructionsJSON := `["instruction 1", "instruction 2"]`
	os.Setenv("GOMIND_PROMPT_CUSTOM_INSTRUCTIONS", instructionsJSON)
	defer os.Unsetenv("GOMIND_PROMPT_CUSTOM_INSTRUCTIONS")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config.CustomInstructions) != 2 {
		t.Fatalf("expected 2 instructions, got: %d", len(config.CustomInstructions))
	}
}

func TestPromptConfig_LoadFromEnv_InvalidCustomInstructionsJSON(t *testing.T) {
	os.Setenv("GOMIND_PROMPT_CUSTOM_INSTRUCTIONS", "not json array")
	defer os.Unsetenv("GOMIND_PROMPT_CUSTOM_INSTRUCTIONS")

	config := &PromptConfig{}
	err := config.LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPromptConfig_MustLoadFromEnv_Panic(t *testing.T) {
	os.Setenv("GOMIND_PROMPT_TYPE_RULES", "invalid json")
	defer os.Unsetenv("GOMIND_PROMPT_TYPE_RULES")

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid config")
		}
	}()

	config := &PromptConfig{}
	config.MustLoadFromEnv() // Should panic
}

// =============================================================================
// Mock Logger for Testing
// =============================================================================

type MockLogger struct {
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
	debugCalls []string
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.warnCalls = append(m.warnCalls, msg)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.errorCalls = append(m.errorCalls, msg)
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.debugCalls = append(m.debugCalls, msg)
}

func (m *MockLogger) InfoWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *MockLogger) WarnWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.warnCalls = append(m.warnCalls, msg)
}

func (m *MockLogger) ErrorWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.errorCalls = append(m.errorCalls, msg)
}

func (m *MockLogger) DebugWithContext(ctx context.Context, msg string, fields map[string]interface{}) {
	m.debugCalls = append(m.debugCalls, msg)
}

// =============================================================================
// SystemInstructions / Persona Tests
// =============================================================================

func TestBuildPersonaSection_DefaultWhenEmpty(t *testing.T) {
	config := &PromptConfig{SystemInstructions: ""}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := builder.buildPersonaSection()

	expected := "You are an AI orchestrator managing a multi-agent system."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	// Should NOT contain the subordinate phrasing when using default
	if strings.Contains(result, "As an AI orchestrator") {
		t.Error("default persona should not contain 'As an AI orchestrator' phrasing")
	}
}

func TestBuildPersonaSection_CustomPersonaWithOrchestratorRole(t *testing.T) {
	config := &PromptConfig{
		SystemInstructions: "You are a travel planning specialist.",
	}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := builder.buildPersonaSection()

	// Should contain the custom persona
	if !strings.Contains(result, "You are a travel planning specialist.") {
		t.Error("result should contain custom persona")
	}

	// Should also contain the orchestrator function as subordinate role
	if !strings.Contains(result, "As an AI orchestrator, you manage a multi-agent system") {
		t.Error("result should contain orchestrator function")
	}
}

func TestBuildPersonaSection_MultilineInstructions(t *testing.T) {
	config := &PromptConfig{
		SystemInstructions: `You are a travel planning assistant.
Always check weather before recommending outdoor activities.
Prefer real-time data sources over cached data.`,
	}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := builder.buildPersonaSection()

	// Should contain all lines of the custom instructions
	if !strings.Contains(result, "travel planning assistant") {
		t.Error("result should contain first line of instructions")
	}
	if !strings.Contains(result, "weather") {
		t.Error("result should contain weather instruction")
	}
	if !strings.Contains(result, "real-time data") {
		t.Error("result should contain real-time data instruction")
	}
}

func TestBuildPlanningPrompt_WithSystemInstructions(t *testing.T) {
	config := &PromptConfig{
		SystemInstructions: "You are a financial advisor assistant.",
	}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Test capabilities",
		Request:        "Test request",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom persona should appear at the start
	if !strings.Contains(prompt, "You are a financial advisor assistant.") {
		t.Error("prompt should contain custom persona")
	}

	// Orchestrator role should be present
	if !strings.Contains(prompt, "As an AI orchestrator") {
		t.Error("prompt should contain orchestrator role")
	}

	// Should NOT have duplicate "You are an AI orchestrator" at the start
	// The custom persona replaces the default
	idx := strings.Index(prompt, "You are")
	if idx >= 0 {
		// First "You are" should be from custom instructions, not default
		nextChars := prompt[idx : idx+40]
		if strings.Contains(nextChars, "AI orchestrator managing") {
			t.Error("prompt should not start with default orchestrator persona when custom is set")
		}
	}
}

func TestBuildPlanningPrompt_WithoutSystemInstructions(t *testing.T) {
	config := &PromptConfig{}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Test capabilities",
		Request:        "Test request",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default persona should be present
	if !strings.Contains(prompt, "You are an AI orchestrator managing a multi-agent system.") {
		t.Error("prompt should contain default orchestrator persona")
	}

	// Should NOT contain "As an AI orchestrator" (that's only for custom personas)
	if strings.Contains(prompt, "As an AI orchestrator, you manage") {
		t.Error("prompt without custom persona should not have subordinate orchestrator phrasing")
	}
}

func TestSystemInstructions_CombinedWithDomain(t *testing.T) {
	config := &PromptConfig{
		SystemInstructions: "You are a healthcare data analyst.",
		Domain:             "healthcare",
	}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Test capabilities",
		Request:        "Analyze patient data",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both custom persona and domain instructions should be present
	if !strings.Contains(prompt, "healthcare data analyst") {
		t.Error("prompt should contain custom persona")
	}
	if !strings.Contains(prompt, "HEALTHCARE DOMAIN REQUIREMENTS") {
		t.Error("prompt should contain domain-specific requirements")
	}
}

func TestSystemInstructions_CombinedWithCustomInstructions(t *testing.T) {
	config := &PromptConfig{
		SystemInstructions: "You are a travel assistant.",
		CustomInstructions: []string{
			"Always prefer direct flights",
			"Include visa requirements",
		},
	}
	builder, err := NewDefaultPromptBuilder(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := PromptInput{
		CapabilityInfo: "Test capabilities",
		Request:        "Plan a trip to Tokyo",
	}

	prompt, err := builder.BuildPlanningPrompt(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All customizations should be present
	if !strings.Contains(prompt, "travel assistant") {
		t.Error("prompt should contain custom persona")
	}
	if !strings.Contains(prompt, "direct flights") {
		t.Error("prompt should contain first custom instruction")
	}
	if !strings.Contains(prompt, "visa requirements") {
		t.Error("prompt should contain second custom instruction")
	}
}
