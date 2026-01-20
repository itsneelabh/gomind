// Package orchestration provides LLM debug payload storage for production debugging.
// This file defines the interface and data types for storing complete LLM prompts
// and responses, enabling operators to debug orchestration issues without truncation.
//
// Design follows FRAMEWORK_DESIGN_PRINCIPLES.md:
// - Interface-first design for swappable backends
// - Safe defaults (NoOp when unavailable)
// - Disabled by default (enable via GOMIND_LLM_DEBUG_ENABLED=true)
//
// See orchestration/notes/LLM_DEBUG_PAYLOAD_DESIGN.md for full design rationale.
package orchestration

import (
	"context"
	"time"
)

// LLMDebugStore stores LLM interaction payloads for debugging.
// Implementations must be safe for concurrent use.
//
// The interface supports multiple backends (Redis, PostgreSQL, S3, etc.)
// allowing teams to choose storage that fits their needs.
type LLMDebugStore interface {
	// RecordInteraction appends an LLM interaction to the debug record.
	// This is called asynchronously from the orchestrator to avoid latency impact.
	// Errors should be logged but not propagated to avoid blocking orchestration.
	RecordInteraction(ctx context.Context, requestID string, interaction LLMInteraction) error

	// GetRecord retrieves the complete debug record for a request.
	// Returns an error if the record is not found or has expired.
	GetRecord(ctx context.Context, requestID string) (*LLMDebugRecord, error)

	// SetMetadata adds metadata to an existing record.
	// Useful for adding investigation notes or flags.
	SetMetadata(ctx context.Context, requestID string, key, value string) error

	// ExtendTTL extends retention for investigation.
	// Allows keeping important records longer than the default TTL.
	ExtendTTL(ctx context.Context, requestID string, duration time.Duration) error

	// ListRecent returns recent records for UI listing.
	// Results are ordered by creation time, newest first.
	ListRecent(ctx context.Context, limit int) ([]LLMDebugRecordSummary, error)
}

// LLMDebugRecord stores all LLM interactions for a single orchestration request.
// This is the complete record stored in the backend.
type LLMDebugRecord struct {
	// RequestID is the orchestration request identifier
	RequestID string `json:"request_id"`

	// OriginalRequestID links related records across HITL resumes.
	// For initial requests: same as RequestID
	// For resume requests: the original conversation's RequestID
	// This enables finding all LLM calls in a HITL conversation.
	OriginalRequestID string `json:"original_request_id,omitempty"`

	// TraceID links to distributed tracing (Jaeger)
	TraceID string `json:"trace_id"`

	// CreatedAt is when the first interaction was recorded
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the record was last modified
	UpdatedAt time.Time `json:"updated_at"`

	// Interactions contains all LLM calls for this request
	Interactions []LLMInteraction `json:"interactions"`

	// Metadata contains additional key-value pairs for investigation
	Metadata map[string]string `json:"metadata,omitempty"`
}

// LLMInteraction captures a single LLM call (request + response).
// This includes the complete prompt and response without truncation.
type LLMInteraction struct {
	// Type identifies the interaction purpose
	// Values: "plan_generation", "synthesis", "micro_resolution", "correction", "error_analysis"
	Type string `json:"type"`

	// Timestamp is when the interaction started
	Timestamp time.Time `json:"timestamp"`

	// DurationMs is the LLM call duration in milliseconds
	DurationMs int64 `json:"duration_ms"`

	// Request fields
	Prompt       string  `json:"prompt"`                  // Complete prompt sent to LLM
	SystemPrompt string  `json:"system_prompt,omitempty"` // System prompt if used
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	Model        string  `json:"model,omitempty"`    // Model identifier (e.g., "gpt-4o-mini")
	Provider     string  `json:"provider,omitempty"` // Provider (e.g., "openai", "anthropic")

	// Response fields
	Response         string `json:"response"` // Complete LLM response
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`

	// Status fields
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"` // Error message if failed
	Attempt int    `json:"attempt"`         // Attempt number (for retries)
}

// LLMDebugRecordSummary is a lightweight version for listing.
// Used by the ListRecent API to avoid loading full payloads.
type LLMDebugRecordSummary struct {
	RequestID         string    `json:"request_id"`
	OriginalRequestID string    `json:"original_request_id,omitempty"`
	TraceID           string    `json:"trace_id"`
	CreatedAt         time.Time `json:"created_at"`
	InteractionCount  int       `json:"interaction_count"`
	TotalTokens       int       `json:"total_tokens"`
	HasErrors         bool      `json:"has_errors"`
}

// LLMDebugConfig holds configuration for LLM debug storage.
// This is embedded in OrchestratorConfig.
type LLMDebugConfig struct {
	// Enabled controls whether debug payload storage is active.
	// Default: false (disabled). Enable via GOMIND_LLM_DEBUG_ENABLED=true
	Enabled bool `json:"enabled"`

	// TTL is the retention period for successful records.
	// Default: 24h. Override via GOMIND_LLM_DEBUG_TTL
	TTL time.Duration `json:"ttl"`

	// ErrorTTL is the retention period for records with errors.
	// Default: 168h (7 days). Override via GOMIND_LLM_DEBUG_ERROR_TTL
	ErrorTTL time.Duration `json:"error_ttl"`

	// RedisDB is the Redis database number for storage.
	// Default: 7 (core.RedisDBLLMDebug). Override via GOMIND_LLM_DEBUG_REDIS_DB
	RedisDB int `json:"redis_db"`
}

// DefaultLLMDebugConfig returns the default configuration for LLM debug storage.
// Feature is disabled by default per design principles.
func DefaultLLMDebugConfig() LLMDebugConfig {
	return LLMDebugConfig{
		Enabled:  false,              // Disabled by default
		TTL:      24 * time.Hour,     // 24 hours for success
		ErrorTTL: 7 * 24 * time.Hour, // 7 days for errors
		RedisDB:  7,                  // core.RedisDBLLMDebug
	}
}
