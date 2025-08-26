package telemetry

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ContextKey type for context keys
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation ID
	CorrelationIDKey ContextKey = "correlation_id"
	// RequestIDKey is the context key for request ID
	RequestIDKey ContextKey = "request_id"
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"
	// SessionIDKey is the context key for session ID
	SessionIDKey ContextKey = "session_id"
)

const (
	// HeaderCorrelationID is the HTTP header for correlation ID
	HeaderCorrelationID = "X-Correlation-ID"
	// HeaderRequestID is the HTTP header for request ID
	HeaderRequestID = "X-Request-ID"
	// HeaderUserID is the HTTP header for user ID
	HeaderUserID = "X-User-ID"
	// HeaderSessionID is the HTTP header for session ID
	HeaderSessionID = "X-Session-ID"
)

// CorrelationMiddleware adds correlation IDs to HTTP requests
func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Extract or generate correlation ID
		correlationID := r.Header.Get(HeaderCorrelationID)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
		
		// Extract or generate request ID
		requestID := r.Header.Get(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		
		// Extract user ID if present
		if userID := r.Header.Get(HeaderUserID); userID != "" {
			ctx = context.WithValue(ctx, UserIDKey, userID)
		}
		
		// Extract session ID if present
		if sessionID := r.Header.Get(HeaderSessionID); sessionID != "" {
			ctx = context.WithValue(ctx, SessionIDKey, sessionID)
		}
		
		// Add to span attributes if tracing is active
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(
				attribute.String("correlation.id", correlationID),
				attribute.String("request.id", requestID),
			)
			if userID := ctx.Value(UserIDKey); userID != nil {
				span.SetAttributes(attribute.String("user.id", userID.(string)))
			}
			if sessionID := ctx.Value(SessionIDKey); sessionID != nil {
				span.SetAttributes(attribute.String("session.id", sessionID.(string)))
			}
		}
		
		// Add correlation ID to response headers
		w.Header().Set(HeaderCorrelationID, correlationID)
		w.Header().Set(HeaderRequestID, requestID)
		
		// Call next handler with enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelationID retrieves correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id := ctx.Value(CorrelationIDKey); id != nil {
		return id.(string)
	}
	return ""
}

// GetRequestID retrieves request ID from context
func GetRequestID(ctx context.Context) string {
	if id := ctx.Value(RequestIDKey); id != nil {
		return id.(string)
	}
	return ""
}

// GetUserID retrieves user ID from context
func GetUserID(ctx context.Context) string {
	if id := ctx.Value(UserIDKey); id != nil {
		return id.(string)
	}
	return ""
}

// GetSessionID retrieves session ID from context
func GetSessionID(ctx context.Context) string {
	if id := ctx.Value(SessionIDKey); id != nil {
		return id.(string)
	}
	return ""
}

// InjectCorrelationHeaders injects correlation IDs into HTTP headers
func InjectCorrelationHeaders(ctx context.Context, headers http.Header) {
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		headers.Set(HeaderCorrelationID, correlationID)
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		headers.Set(HeaderRequestID, requestID)
	}
	if userID := GetUserID(ctx); userID != "" {
		headers.Set(HeaderUserID, userID)
	}
	if sessionID := GetSessionID(ctx); sessionID != "" {
		headers.Set(HeaderSessionID, sessionID)
	}
}

// ExtractCorrelationHeaders extracts correlation IDs from HTTP headers to context
func ExtractCorrelationHeaders(ctx context.Context, headers http.Header) context.Context {
	if correlationID := headers.Get(HeaderCorrelationID); correlationID != "" {
		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
	}
	if requestID := headers.Get(HeaderRequestID); requestID != "" {
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
	}
	if userID := headers.Get(HeaderUserID); userID != "" {
		ctx = context.WithValue(ctx, UserIDKey, userID)
	}
	if sessionID := headers.Get(HeaderSessionID); sessionID != "" {
		ctx = context.WithValue(ctx, SessionIDKey, sessionID)
	}
	return ctx
}

// EnrichLogFields adds correlation IDs to log fields
func EnrichLogFields(ctx context.Context, fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	
	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		fields["correlation_id"] = correlationID
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		fields["request_id"] = requestID
	}
	if userID := GetUserID(ctx); userID != "" {
		fields["user_id"] = userID
	}
	if sessionID := GetSessionID(ctx); sessionID != "" {
		fields["session_id"] = sessionID
	}
	
	// Add trace context if available
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		spanCtx := span.SpanContext()
		fields["trace_id"] = spanCtx.TraceID().String()
		fields["span_id"] = spanCtx.SpanID().String()
	}
	
	return fields
}