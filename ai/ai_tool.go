package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/itsneelabh/gomind/core"
)

// AITool extends BaseTool with AI capabilities but NO discovery
// This represents a passive tool that uses AI for its core functionality
type AITool struct {
	*core.BaseTool // Tool, no discovery
	aiClient       core.AIClient
}

// NewAITool creates a new tool with AI capabilities but no discovery
func NewAITool(name string, apiKey string) (*AITool, error) {
	tool := core.NewTool(name)
	
	// Create AI client
	aiClient, err := NewClient(
		WithProvider("openai"),
		WithAPIKey(apiKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}
	
	tool.AI = aiClient
	
	return &AITool{
		BaseTool: tool,
		aiClient: aiClient,
	}, nil
}

// ProcessWithAI processes input using AI (but cannot discover other components)
func (t *AITool) ProcessWithAI(ctx context.Context, input string) (string, error) {
	response, err := t.aiClient.GenerateResponse(ctx, input, &core.AIOptions{
		Model:       "gpt-3.5-turbo",
		Temperature: 0.7,
		MaxTokens:   500,
	})
	if err != nil {
		return "", fmt.Errorf("AI processing failed: %w", err)
	}
	
	return response.Content, nil
}

// RegisterAICapability registers an AI-powered capability for the tool
func (t *AITool) RegisterAICapability(name, description, prompt string) {
	capability := core.Capability{
		Name:        name,
		Description: description,
		Endpoint:    fmt.Sprintf("/ai/%s", name),
		Handler: func(w http.ResponseWriter, r *http.Request) {
			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request", http.StatusBadRequest)
				return
			}
			
			// Process with AI using the configured prompt
			fullPrompt := fmt.Sprintf("%s\n\nInput: %s", prompt, string(body))
			response, err := t.ProcessWithAI(r.Context(), fullPrompt)
			if err != nil {
				http.Error(w, "AI processing failed", http.StatusInternalServerError)
				return
			}
			
			// Return response
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(response))
		},
	}
	
	t.RegisterCapability(capability)
}

// Example AI tools

// NewTranslationTool creates an AI-powered translation tool
func NewTranslationTool(apiKey string) (*AITool, error) {
	tool, err := NewAITool("translation-tool", apiKey)
	if err != nil {
		return nil, err
	}
	
	tool.RegisterAICapability(
		"translate",
		"Translates text between languages",
		"You are a professional translator. Translate the following text, preserving meaning and context.",
	)
	
	return tool, nil
}

// NewSummarizationTool creates an AI-powered summarization tool
func NewSummarizationTool(apiKey string) (*AITool, error) {
	tool, err := NewAITool("summarization-tool", apiKey)
	if err != nil {
		return nil, err
	}
	
	tool.RegisterAICapability(
		"summarize",
		"Summarizes long text into key points",
		"You are an expert at summarization. Provide a concise summary of the following text, highlighting key points.",
	)
	
	return tool, nil
}

// NewSentimentAnalysisTool creates an AI-powered sentiment analysis tool
func NewSentimentAnalysisTool(apiKey string) (*AITool, error) {
	tool, err := NewAITool("sentiment-tool", apiKey)
	if err != nil {
		return nil, err
	}
	
	tool.RegisterAICapability(
		"analyze_sentiment",
		"Analyzes sentiment of text (positive, negative, neutral)",
		"You are a sentiment analysis expert. Analyze the sentiment of the following text and respond with: POSITIVE, NEGATIVE, or NEUTRAL, followed by a confidence score (0-100) and brief explanation.",
	)
	
	return tool, nil
}

// NewCodeReviewTool creates an AI-powered code review tool
func NewCodeReviewTool(apiKey string) (*AITool, error) {
	tool, err := NewAITool("code-review-tool", apiKey)
	if err != nil {
		return nil, err
	}
	
	tool.RegisterAICapability(
		"review_code",
		"Reviews code for quality, bugs, and improvements",
		"You are an expert code reviewer. Review the following code for:\n1. Potential bugs\n2. Performance issues\n3. Security vulnerabilities\n4. Code style and best practices\n5. Suggested improvements\n\nProvide specific, actionable feedback.",
	)
	
	return tool, nil
}