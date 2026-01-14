package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/core"
)

// handleCreateSession creates a new chat session.
func (t *TravelChatAgent) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Only POST requests are supported", nil)
		return
	}

	// Parse optional metadata from request body
	var metadata map[string]interface{}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
			// Ignore decode errors, proceed without metadata
			metadata = nil
		}
	}

	// Create session
	session := t.sessionStore.Create(metadata)

	t.Logger.InfoWithContext(ctx, "Session created", map[string]interface{}{
		"operation":  "create_session",
		"session_id": session.ID,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"session_id": session.ID,
		"created_at": session.CreatedAt,
		"expires_at": session.CreatedAt.Add(30 * time.Minute),
	})
}

// handleGetSession retrieves session information.
func (t *TravelChatAgent) handleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Only GET requests are supported", nil)
		return
	}

	// Extract session ID from URL path
	sessionID := extractPathParam(r.URL.Path, "/chat/session/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		writeError(w, http.StatusBadRequest, "Invalid session ID", nil)
		return
	}

	session := t.sessionStore.Get(sessionID)
	if session == nil {
		writeError(w, http.StatusNotFound, "Session not found or expired", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":    session.ID,
		"created_at":    session.CreatedAt,
		"updated_at":    session.UpdatedAt,
		"message_count": len(session.Messages),
		"metadata":      session.Metadata,
	})
}

// handleGetHistory retrieves conversation history for a session.
func (t *TravelChatAgent) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Only GET requests are supported", nil)
		return
	}

	// Extract session ID from URL path (e.g., /chat/session/{id}/history)
	path := strings.TrimPrefix(r.URL.Path, "/chat/session/")
	path = strings.TrimSuffix(path, "/history")
	sessionID := path

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	session := t.sessionStore.Get(sessionID)
	if session == nil {
		writeError(w, http.StatusNotFound, "Session not found or expired", nil)
		return
	}

	messages := t.sessionStore.GetHistory(sessionID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"messages":   messages,
		"count":      len(messages),
	})
}

// handleHealth returns health status with orchestrator metrics.
func (t *TravelChatAgent) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "travel-chat-agent",
	}

	// Check Redis/Discovery connection
	if t.Discovery != nil {
		_, err := t.Discovery.Discover(ctx, core.DiscoveryFilter{})
		if err != nil {
			health["status"] = "degraded"
			health["redis"] = "unavailable"
			t.Logger.WarnWithContext(ctx, "Health check: Redis unavailable", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			health["redis"] = "healthy"
		}
	} else {
		health["redis"] = "not configured"
	}

	// Check orchestrator status
	orch := t.GetOrchestrator()
	if orch != nil {
		metrics := orch.GetMetrics()
		health["orchestrator"] = map[string]interface{}{
			"status":              "active",
			"total_requests":      metrics.TotalRequests,
			"successful_requests": metrics.SuccessfulRequests,
			"failed_requests":     metrics.FailedRequests,
			"average_latency_ms":  metrics.AverageLatency.Milliseconds(),
		}
	} else {
		health["orchestrator"] = "initializing"
	}

	// Check AI provider
	if t.AI != nil {
		health["ai_provider"] = "connected"
	} else {
		health["ai_provider"] = "not configured"
	}

	// Add session stats
	health["active_sessions"] = t.sessionStore.GetActiveSessionCount()

	// Set appropriate status code
	statusCode := http.StatusOK
	if health["status"] == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)
}

// handleDiscover shows available tools and their capabilities.
func (t *TravelChatAgent) handleDiscover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	t.Logger.InfoWithContext(ctx, "Discovering components", map[string]interface{}{
		"path": r.URL.Path,
	})

	if t.Discovery == nil {
		writeError(w, http.StatusServiceUnavailable, "Service discovery not configured", nil)
		return
	}

	allComponents, err := t.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		t.Logger.ErrorWithContext(ctx, "Discovery failed", map[string]interface{}{
			"error": err.Error(),
		})
		writeError(w, http.StatusServiceUnavailable, "Discovery failed", err)
		return
	}

	tools := make([]*core.ServiceInfo, 0)
	agents := make([]*core.ServiceInfo, 0)

	for _, component := range allComponents {
		switch component.Type {
		case core.ComponentTypeTool:
			tools = append(tools, component)
		case core.ComponentTypeAgent:
			if component.ID != t.GetID() {
				agents = append(agents, component)
			}
		}
	}

	response := map[string]interface{}{
		"discovery_summary": map[string]interface{}{
			"total_components": len(allComponents),
			"tools":            len(tools),
			"agents":           len(agents),
			"discovery_time":   time.Now().Format(time.RFC3339),
		},
		"tools":  tools,
		"agents": agents,
	}

	writeJSON(w, http.StatusOK, response)
}

// Helper functions

// setCORSHeaders sets CORS headers for cross-origin requests.
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

// writeJSON writes a JSON response with CORS headers.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response with CORS headers.
func writeError(w http.ResponseWriter, statusCode int, message string, err error) {
	response := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"success": false,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// extractPathParam extracts a path parameter from a URL path.
func extractPathParam(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	param := strings.TrimPrefix(path, prefix)
	// Remove any trailing path segments
	if idx := strings.Index(param, "/"); idx != -1 {
		param = param[:idx]
	}
	return param
}
