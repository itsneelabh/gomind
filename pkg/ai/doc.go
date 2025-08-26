// Package ai provides AI client implementations for integrating Large Language Models (LLMs)
// into the GoMind Agent Framework.
//
// This package abstracts the complexity of working with various AI providers, offering a
// unified interface for text generation, streaming responses, and managing AI interactions
// within agents.
//
// # Supported Providers
//
// Currently supported AI providers:
//   - OpenAI (GPT-3.5, GPT-4, and other OpenAI models)
//   - Future: Anthropic Claude, Google Gemini, local models via Ollama
//
// # Core Interface
//
// The AIClient interface defines the contract for all AI providers:
//
//	type AIClient interface {
//	    Generate(ctx context.Context, prompt string, options *GenerationOptions) (*AIResponse, error)
//	    GenerateStream(ctx context.Context, prompt string, options *GenerationOptions) (<-chan AIStreamChunk, error)
//	    GetProviderInfo() ProviderInfo
//	}
//
// # Usage Example
//
// Creating and using an OpenAI client:
//
//	client, err := ai.NewOpenAIClient("your-api-key", logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	response, err := client.Generate(ctx, "Explain quantum computing", &ai.GenerationOptions{
//	    Model:       "gpt-4",
//	    Temperature: 0.7,
//	    MaxTokens:   1000,
//	})
//
// # Streaming Responses
//
// For real-time streaming of AI responses:
//
//	stream, err := client.GenerateStream(ctx, prompt, options)
//	if err != nil {
//	    return err
//	}
//	
//	for chunk := range stream {
//	    if chunk.Error != nil {
//	        return chunk.Error
//	    }
//	    fmt.Print(chunk.Content)
//	}
//
// # Configuration
//
// AI clients can be configured through environment variables or programmatically:
//   - OPENAI_API_KEY: API key for OpenAI
//   - AI_PROVIDER: Default AI provider (openai, claude, etc.)
//   - DEFAULT_AI_MODEL: Default model to use
//
// # Integration with Agents
//
// AI clients are automatically injected into agents that embed BaseAgent,
// providing seamless access to AI capabilities within agent methods.
package ai