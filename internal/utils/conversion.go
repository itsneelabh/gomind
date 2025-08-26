package utils

import (
	"github.com/itsneelabh/gomind/pkg/discovery"
	"github.com/itsneelabh/gomind/pkg/telemetry"
)

// CapabilityMetadata represents framework capability metadata
type CapabilityMetadata struct {
	Name            string
	Description     string
	Domain          string
	Complexity      string
	ConfidenceLevel float64
	BusinessValue   []string
	LLMPrompt       string
	Specialties     []string
	// Add other fields as needed for conversions
}

// ConvertToDiscoveryCapabilities converts framework CapabilityMetadata to discovery CapabilityMetadata
func ConvertToDiscoveryCapabilities(capabilities []CapabilityMetadata) []discovery.CapabilityMetadata {
	var discoveryCapabilities []discovery.CapabilityMetadata
	for _, cap := range capabilities {
		discoveryCap := discovery.CapabilityMetadata{
			Name:        cap.Name,
			Description: cap.Description,
			Domain:      cap.Domain,
			LLMPrompt:   cap.LLMPrompt,
			Specialties: cap.Specialties,
			// Map other fields as needed
		}
		discoveryCapabilities = append(discoveryCapabilities, discoveryCap)
	}
	return discoveryCapabilities
}

// ConvertToTelemetryCapability converts framework CapabilityMetadata to telemetry CapabilityMetadata
func ConvertToTelemetryCapability(capability CapabilityMetadata) telemetry.CapabilityMetadata {
	return telemetry.CapabilityMetadata{
		Name:            capability.Name,
		Domain:          capability.Domain,
		Complexity:      capability.Complexity,
		ConfidenceLevel: capability.ConfidenceLevel,
		BusinessValue:   capability.BusinessValue,
		LLMPrompt:       capability.LLMPrompt,
		Specialties:     capability.Specialties,
	}
}
