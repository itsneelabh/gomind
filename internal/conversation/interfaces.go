package conversation

import (
	"context"
)

// ConversationalAgent interface for agents that handle conversations
// This is a subset of the main framework's ConversationalAgent interface
type ConversationalAgent interface {
	HandleConversation(ctx context.Context, message Message) (Response, error)
}

// Message represents an incoming conversational message
type Message struct {
	Text      string                 `json:"text"`
	SessionID string                 `json:"session_id"`
	UserID    string                 `json:"user_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	Media     []MediaAttachment      `json:"media,omitempty"`
}

// Response represents a conversational response
type Response struct {
	Text         string                 `json:"text"`
	Type         ResponseType           `json:"type"`
	QuickReplies []string               `json:"quick_replies,omitempty"`
	Actions      []Action               `json:"actions,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ResponseType defines the type of conversational response
type ResponseType string

const (
	ResponseTypeText     ResponseType = "text"
	ResponseTypeTyping   ResponseType = "typing"
	ResponseTypeProgress ResponseType = "progress"
	ResponseTypeComplete ResponseType = "complete"
	ResponseTypeError    ResponseType = "error"
)

// Action represents an action that can be taken in conversation
type Action struct {
	Type    string                 `json:"type"`
	Label   string                 `json:"label"`
	Payload map[string]interface{} `json:"payload"`
}

// MediaAttachment represents media content in messages
type MediaAttachment struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}