package conversation

import (
	"context"
	"fmt"
	"reflect"
)

// ConversationalAgentAdapter wraps a framework ConversationalAgent to work with internal conversation types
type ConversationalAgentAdapter struct {
	agent interface{}
}

// NewConversationalAgentAdapter creates a new adapter
func NewConversationalAgentAdapter(agent interface{}) *ConversationalAgentAdapter {
	return &ConversationalAgentAdapter{agent: agent}
}

// HandleConversation adapts the conversation handling to internal types
func (a *ConversationalAgentAdapter) HandleConversation(ctx context.Context, message Message) (Response, error) {
	// Use reflection to call HandleConversation on the underlying agent
	agentValue := reflect.ValueOf(a.agent)
	method := agentValue.MethodByName("HandleConversation")
	
	if !method.IsValid() {
		return Response{}, fmt.Errorf("agent does not have HandleConversation method")
	}

	// Create arguments for the method call
	// We need to create the proper Message type that the agent expects
	messageValue := reflect.ValueOf(message)
	ctxValue := reflect.ValueOf(ctx)

	// Call the method
	results := method.Call([]reflect.Value{ctxValue, messageValue})
	
	if len(results) != 2 {
		return Response{}, fmt.Errorf("unexpected number of return values from HandleConversation")
	}

	// Check for error
	if !results[1].IsNil() {
		if err, ok := results[1].Interface().(error); ok {
			return Response{}, err
		}
	}

	// Try to convert the result to our Response type
	result := results[0].Interface()

	// Convert result back to internal Response type
	// This assumes the result is a map or can be converted
	if respMap, ok := result.(map[string]interface{}); ok {
		response := Response{
			Text:     getStringValue(respMap, "text"),
			Type:     ResponseType(getStringValue(respMap, "type")),
			Metadata: getMapValue(respMap, "metadata"),
		}
		
		if replies, ok := respMap["quick_replies"].([]string); ok {
			response.QuickReplies = replies
		}
		
		if actions, ok := respMap["actions"].([]Action); ok {
			response.Actions = actions
		}
		
		return response, nil
	}

	// If we can't convert, return a basic text response
	return Response{
		Text: "Response processed",
		Type: ResponseTypeText,
	}, nil
}

// Helper functions for safe type conversion
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMapValue(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}