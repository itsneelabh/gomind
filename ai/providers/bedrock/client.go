//go:build bedrock
// +build bedrock

package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/itsneelabh/gomind/ai/providers"
	"github.com/itsneelabh/gomind/core"
)

// Client implements core.AIClient for AWS Bedrock
type Client struct {
	*providers.BaseClient
	bedrockClient *bedrockruntime.Client
	region        string
}

// NewClient creates a new AWS Bedrock client
func NewClient(cfg aws.Config, region string, logger core.Logger) *Client {
	// Create Bedrock Runtime client
	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	
	// Create base client with defaults
	base := providers.NewBaseClient(30*time.Second, logger)
	base.DefaultModel = ModelClaude3Sonnet // Default to Claude Sonnet
	base.DefaultMaxTokens = 1000
	
	return &Client{
		BaseClient:    base,
		bedrockClient: bedrockClient,
		region:        region,
	}
}

// GenerateResponse generates a response using AWS Bedrock's Converse API
func (c *Client) GenerateResponse(ctx context.Context, prompt string, options *core.AIOptions) (*core.AIResponse, error) {
	// Apply defaults
	options = c.ApplyDefaults(options)
	
	// Log request
	c.LogRequest("bedrock", options.Model, prompt)
	startTime := time.Now()
	
	// Build messages for Converse API
	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: prompt,
				},
			},
		},
	}
	
	// Build the Converse input
	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(options.Model),
		Messages: messages,
	}
	
	// Add system prompt if provided
	if options.SystemPrompt != "" {
		input.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{
				Value: options.SystemPrompt,
			},
		}
	}
	
	// Add inference configuration
	inferenceConfig := &types.InferenceConfiguration{}
	configSet := false
	
	if options.MaxTokens > 0 {
		inferenceConfig.MaxTokens = aws.Int32(int32(options.MaxTokens))
		configSet = true
	}
	
	if options.Temperature > 0 {
		inferenceConfig.Temperature = aws.Float32(options.Temperature)
		configSet = true
	}
	
	if configSet {
		input.InferenceConfig = inferenceConfig
	}
	
	// Make the request to AWS Bedrock
	output, err := c.bedrockClient.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse error: %w", err)
	}
	
	// Extract text content from response
	if output.Output == nil {
		return nil, fmt.Errorf("no output in Bedrock response")
	}
	
	var content string
	switch v := output.Output.(type) {
	case *types.ConverseOutputMemberMessage:
		for _, block := range v.Value.Content {
			switch b := block.(type) {
			case *types.ContentBlockMemberText:
				content += b.Value
			}
		}
	default:
		return nil, fmt.Errorf("unexpected output type from Bedrock")
	}
	
	if content == "" {
		return nil, fmt.Errorf("no text content in Bedrock response")
	}
	
	// Build the response
	result := &core.AIResponse{
		Content: content,
		Model:   options.Model,
	}
	
	// Add usage information if available
	if output.Usage != nil {
		result.Usage = core.TokenUsage{
			PromptTokens:     int(*output.Usage.InputTokens),
			CompletionTokens: int(*output.Usage.OutputTokens),
			TotalTokens:      int(*output.Usage.TotalTokens),
		}
	}
	
	// Add stop reason if available
	if output.StopReason != "" {
		// Store in metadata if needed
	}
	
	// Log response
	c.LogResponse("bedrock", result.Model, result.Usage, time.Since(startTime))
	c.LogResponseContent("bedrock", result.Model, result.Content)

	return result, nil
}

// StreamResponse generates a streaming response using AWS Bedrock's ConverseStream API
func (c *Client) StreamResponse(ctx context.Context, prompt string, options *core.AIOptions, stream chan<- string) error {
	defer close(stream)
	
	// Apply defaults
	options = c.ApplyDefaults(options)
	
	// Build messages for ConverseStream API
	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: prompt,
				},
			},
		},
	}
	
	// Build the ConverseStream input
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  aws.String(options.Model),
		Messages: messages,
	}
	
	// Add system prompt if provided
	if options.SystemPrompt != "" {
		input.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{
				Value: options.SystemPrompt,
			},
		}
	}
	
	// Add inference configuration
	inferenceConfig := &types.InferenceConfiguration{}
	if options.MaxTokens > 0 {
		inferenceConfig.MaxTokens = aws.Int32(int32(options.MaxTokens))
	}
	if options.Temperature > 0 {
		inferenceConfig.Temperature = aws.Float32(options.Temperature)
	}
	input.InferenceConfig = inferenceConfig
	
	// Start the stream
	output, err := c.bedrockClient.ConverseStream(ctx, input)
	if err != nil {
		return fmt.Errorf("bedrock stream error: %w", err)
	}
	
	// Process the stream
	eventStream := output.GetStream()
	defer eventStream.Close()
	
	for {
		event, ok := <-eventStream.Events()
		if !ok {
			break
		}
		
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if v.Value.Delta != nil {
				switch d := v.Value.Delta.(type) {
				case *types.ContentBlockDeltaMemberText:
					select {
					case stream <- d.Value:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		case *types.ConverseStreamOutputMemberMessageStop:
			// Stream ended normally
			return nil
		}
	}
	
	// Check for stream errors
	if err := eventStream.Err(); err != nil {
		return fmt.Errorf("bedrock stream error: %w", err)
	}
	
	return nil
}

// InvokeModel provides direct access to specific model APIs (for advanced use cases)
// This bypasses the Converse API and uses model-specific formats
func (c *Client) InvokeModel(ctx context.Context, modelID string, body []byte) ([]byte, error) {
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        body,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	}
	
	output, err := c.bedrockClient.InvokeModel(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock invoke model error: %w", err)
	}
	
	return output.Body, nil
}

// GetEmbeddings generates embeddings using Amazon Titan Embed model
func (c *Client) GetEmbeddings(ctx context.Context, text string) ([]float32, error) {
	// Build request for Titan Embed model
	request := map[string]interface{}{
		"inputText": text,
	}
	
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}
	
	// Invoke Titan Embed model
	responseBody, err := c.InvokeModel(ctx, ModelTitanEmbed, body)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var response struct {
		Embedding []float32 `json:"embedding"`
	}
	
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse embed response: %w", err)
	}
	
	return response.Embedding, nil
}

// CreateAWSConfig creates an AWS configuration for Bedrock
// This can use various authentication methods:
// 1. IAM role (when running on EC2/ECS/Lambda)
// 2. AWS credentials from environment variables
// 3. AWS profile from ~/.aws/credentials
// 4. Explicit credentials passed in
func CreateAWSConfig(ctx context.Context, region string, credentials ...aws.CredentialsProvider) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	
	// Add explicit credentials if provided
	if len(credentials) > 0 && credentials[0] != nil {
		opts = append(opts, config.WithCredentialsProvider(credentials[0]))
	}
	
	// Load the configuration
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	return cfg, nil
}