package openai

// OpenAIResponse represents the response from OpenAI API
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a response choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ErrorResponse represents an error from OpenAI API
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ModelAliases maps common model aliases to provider-specific model names (Phase 2)
// This enables portable model names across different OpenAI-compatible providers.
//
// Example usage:
//   client, _ := ai.NewClient(
//       ai.WithProviderAlias("openai.deepseek"),
//       ai.WithModel("smart"),  // Resolves to "deepseek-reasoner"
//   )
var ModelAliases = map[string]map[string]string{
	"openai": {
		"fast":    "gpt-3.5-turbo",
		"smart":   "gpt-4",
		"vision":  "gpt-4-vision-preview",
		"code":    "gpt-4",
		"default": "gpt-3.5-turbo",
	},
	"openai.deepseek": {
		"fast":    "deepseek-chat",
		"smart":   "deepseek-reasoner",
		"code":    "deepseek-coder",
		"default": "deepseek-chat",
	},
	"openai.groq": {
		"fast":    "llama-3.3-70b-versatile",
		"smart":   "mixtral-8x7b-32768",
		"code":    "llama-3.3-70b-versatile",
		"default": "llama-3.3-70b-versatile",
	},
	"openai.together": {
		"fast":    "meta-llama/Llama-3-8b-chat-hf",
		"smart":   "meta-llama/Llama-3-70b-chat-hf",
		"code":    "deepseek-ai/deepseek-coder-33b-instruct",
		"default": "meta-llama/Llama-3-70b-chat-hf",
	},
	"openai.xai": {
		"fast":    "grok-2",
		"smart":   "grok-2",
		"code":    "grok-2",
		"default": "grok-2",
	},
	"openai.qwen": {
		"fast":    "qwen-turbo",
		"smart":   "qwen-plus",
		"code":    "qwen-plus",
		"default": "qwen-turbo",
	},
}

// ResolveModel resolves a model alias to the actual model name (Phase 2)
// This function enables portable model names across providers.
//
// Parameters:
//   - providerAlias: The provider alias (e.g., "openai.deepseek")
//   - model: The model name or alias (e.g., "smart" or "gpt-4")
//
// Returns:
//   - The actual model name to use with the provider
//
// Behavior:
//   - If model matches an alias for the provider, returns the mapped model
//   - If no alias match, returns model unchanged (pass-through)
//   - If providerAlias is empty, defaults to "openai"
//
// Example:
//   ResolveModel("openai.deepseek", "smart") → "deepseek-reasoner"
//   ResolveModel("openai.groq", "fast") → "llama-3.3-70b-versatile"
//   ResolveModel("openai", "gpt-4") → "gpt-4" (pass-through, not an alias)
func ResolveModel(providerAlias string, model string) string {
	// If no alias is set, use vanilla OpenAI
	if providerAlias == "" {
		providerAlias = "openai"
	}

	// Check if model is an alias for this provider
	if aliases, exists := ModelAliases[providerAlias]; exists {
		if actualModel, exists := aliases[model]; exists {
			return actualModel
		}
	}

	// Not an alias, return as-is (pass-through for explicit model names)
	return model
}
