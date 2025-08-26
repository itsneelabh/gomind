package capabilities

import (
	"reflect"
)

// CapabilityMetadata represents rich metadata for agent capabilities
// Enhanced for autonomous AI systems with dual metadata support
type CapabilityMetadata struct {
	// Core Capability Identity
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Domain      string `json:"domain" yaml:"domain"`

	// Performance Characteristics
	Complexity      string  `json:"complexity" yaml:"complexity"`
	Latency         string  `json:"latency" yaml:"latency"`
	Cost            string  `json:"cost" yaml:"cost"`
	ConfidenceLevel float64 `json:"confidence_level" yaml:"confidence_level"`

	// Business Context (Enhanced for Autonomous AI)
	BusinessValue  []string        `json:"business_value" yaml:"business_value"`
	BusinessImpact *BusinessImpact `json:"business_impact,omitempty" yaml:"business_impact,omitempty"`
	UseCases       []string        `json:"use_cases" yaml:"use_cases"`
	QualityMetrics *QualityMetrics `json:"quality_metrics,omitempty" yaml:"quality_metrics,omitempty"`

	// LLM-Friendly Fields for Agentic Communication
	LLMPrompt   string   `json:"llm_prompt,omitempty" yaml:"llm_prompt,omitempty"`
	Specialties []string `json:"specialties,omitempty" yaml:"specialties,omitempty"`

	// Technical Requirements
	Prerequisites []string              `json:"prerequisites" yaml:"prerequisites"`
	Dependencies  []string              `json:"dependencies" yaml:"dependencies"`
	InputTypes    []string              `json:"input_types" yaml:"input_types"`
	OutputFormats []string              `json:"output_formats" yaml:"output_formats"`
	ResourceReqs  *ResourceRequirements `json:"resource_requirements,omitempty" yaml:"resource_requirements,omitempty"`

	// Capability Relationships
	ComplementaryTo []string `json:"complementary_to" yaml:"complementary_to"`
	AlternativeTo   []string `json:"alternative_to" yaml:"alternative_to"`

	// Autonomous Operation Support
	AutomationLevel string       `json:"automation_level" yaml:"automation_level"` // "manual", "semi-auto", "autonomous"
	RiskProfile     *RiskProfile `json:"risk_profile,omitempty" yaml:"risk_profile,omitempty"`

	// Metadata Source Tracking
	Source *MetadataSource `json:"source,omitempty" yaml:"source,omitempty"`

	// Framework Integration (existing)
	Method reflect.Method    `json:"-" yaml:"-"`
	Tags   map[string]string `json:"tags" yaml:"tags"`
}

// BusinessImpact represents the business impact assessment
type BusinessImpact struct {
	Criticality    string   `json:"criticality" yaml:"criticality"`       // "low", "medium", "high", "critical"
	RevenueImpact  string   `json:"revenue_impact" yaml:"revenue_impact"` // "positive", "neutral", "negative"
	CustomerFacing bool     `json:"customer_facing" yaml:"customer_facing"`
	Stakeholders   []string `json:"stakeholders" yaml:"stakeholders"`
}

// QualityMetrics represents quality and reliability metrics
type QualityMetrics struct {
	Accuracy     float64 `json:"accuracy" yaml:"accuracy"`           // 0.0 - 1.0
	Reliability  float64 `json:"reliability" yaml:"reliability"`     // 0.0 - 1.0
	ResponseTime string  `json:"response_time" yaml:"response_time"` // "fast", "medium", "slow"
	Throughput   string  `json:"throughput" yaml:"throughput"`       // "low", "medium", "high"
	ErrorRate    float64 `json:"error_rate" yaml:"error_rate"`       // 0.0 - 1.0
}

// ResourceRequirements represents computational resource needs
type ResourceRequirements struct {
	CPU    string `json:"cpu" yaml:"cpu"`       // "low", "medium", "high"
	Memory string `json:"memory" yaml:"memory"` // "low", "medium", "high"
	IO     string `json:"io" yaml:"io"`         // "low", "medium", "high"
}

// RiskProfile represents operational risk assessment
type RiskProfile struct {
	DataSensitivity string   `json:"data_sensitivity" yaml:"data_sensitivity"` // "public", "internal", "confidential", "restricted"
	OperationalRisk string   `json:"operational_risk" yaml:"operational_risk"` // "low", "medium", "high"
	ComplianceReqs  []string `json:"compliance_requirements" yaml:"compliance_requirements"`
}

// MetadataSource tracks where metadata originated
type MetadataSource struct {
	Type        string `json:"type" yaml:"type"` // "comment", "yaml", "merged"
	File        string `json:"file" yaml:"file"`
	Line        int    `json:"line" yaml:"line"`
	LastUpdated string `json:"last_updated" yaml:"last_updated"`
}