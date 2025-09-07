package gemini

// GeminiRequest represents the native Gemini GenerateContent API request
type GeminiRequest struct {
	Contents         []Content          `json:"contents"`
	GenerationConfig *GenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings   []SafetySetting    `json:"safetySettings,omitempty"`
	SystemInstruction *SystemInstruction `json:"systemInstruction,omitempty"`
}

// Content represents a content block in the request
type Content struct {
	Role  string `json:"role"` // "user" or "model"
	Parts []Part `json:"parts"`
}

// Part represents a part of content
type Part struct {
	Text string `json:"text"`
}

// SystemInstruction represents system instructions
type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

// GenerationConfig represents generation configuration
type GenerationConfig struct {
	Temperature     float32  `json:"temperature,omitempty"`
	TopP            float32  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// SafetySetting represents safety configuration
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiResponse represents the response from Gemini API
type GeminiResponse struct {
	Candidates     []Candidate    `json:"candidates"`
	UsageMetadata  UsageMetadata  `json:"usageMetadata"`
	ModelVersion   string         `json:"modelVersion"`
}

// Candidate represents a response candidate
type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  string         `json:"finishReason"`
	Index         int            `json:"index"`
	SafetyRatings []SafetyRating `json:"safetyRatings"`
}

// SafetyRating represents safety rating information
type SafetyRating struct {
	Category    string  `json:"category"`
	Probability string  `json:"probability"`
	Blocked     bool    `json:"blocked"`
}

// UsageMetadata represents token usage information
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ErrorResponse represents an error from Gemini API
type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}