package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// ErrorInjectionConfig controls how errors are injected for resilience testing
type ErrorInjectionConfig struct {
	Mode            string  `json:"mode"`              // "normal", "rate_limit", "server_error"
	RateLimitAfter  int     `json:"rate_limit_after"`  // Return 429 after N requests
	ServerErrorRate float64 `json:"server_error_rate"` // Probability of 500 (0.0-1.0)
	RetryAfterSecs  int     `json:"retry_after_secs"`  // Retry-After header value for 429
	requestCount    int64
	mu              sync.RWMutex
}

// InjectErrorRequest is the payload for configuring error injection
type InjectErrorRequest struct {
	Mode            string  `json:"mode"`
	RateLimitAfter  int     `json:"rate_limit_after,omitempty"`
	ServerErrorRate float64 `json:"server_error_rate,omitempty"`
	RetryAfterSecs  int     `json:"retry_after_secs,omitempty"`
}

// Config is the global config instance
var Config = &ErrorInjectionConfig{
	Mode:           "normal",
	RateLimitAfter: 5,
	RetryAfterSecs: 5,
}

// GetConfig returns current config (thread-safe)
func GetConfig() ErrorInjectionConfig {
	Config.mu.RLock()
	defer Config.mu.RUnlock()
	return ErrorInjectionConfig{
		Mode:            Config.Mode,
		RateLimitAfter:  Config.RateLimitAfter,
		ServerErrorRate: Config.ServerErrorRate,
		RetryAfterSecs:  Config.RetryAfterSecs,
		requestCount:    atomic.LoadInt64(&Config.requestCount),
	}
}

// SetConfig updates config (thread-safe)
func SetConfig(req InjectErrorRequest) {
	Config.mu.Lock()
	defer Config.mu.Unlock()

	Config.Mode = req.Mode
	if req.RateLimitAfter > 0 {
		Config.RateLimitAfter = req.RateLimitAfter
	}
	if req.ServerErrorRate >= 0 && req.ServerErrorRate <= 1 {
		Config.ServerErrorRate = req.ServerErrorRate
	}
	if req.RetryAfterSecs > 0 {
		Config.RetryAfterSecs = req.RetryAfterSecs
	}
	atomic.StoreInt64(&Config.requestCount, 0)
	log.Printf("[ERROR-INJECTION] Config updated: mode=%s, rate_limit_after=%d, server_error_rate=%.2f",
		Config.Mode, Config.RateLimitAfter, Config.ServerErrorRate)
}

// ResetConfig resets to normal mode
func ResetConfig() {
	Config.mu.Lock()
	defer Config.mu.Unlock()
	Config.Mode = "normal"
	Config.ServerErrorRate = 0
	atomic.StoreInt64(&Config.requestCount, 0)
	log.Println("[ERROR-INJECTION] Reset to normal mode")
}

// ErrorInjectionMiddleware wraps handlers with error injection logic
func ErrorInjectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip error injection for admin endpoints and health checks
		if strings.HasPrefix(r.URL.Path, "/admin") || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		cfg := GetConfig()

		switch cfg.Mode {
		case "rate_limit":
			count := atomic.AddInt64(&Config.requestCount, 1)
			if int(count) > cfg.RateLimitAfter {
				log.Printf("[ERROR-INJECTION] Rate limit triggered: %d requests (limit: %d)", count, cfg.RateLimitAfter)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", cfg.RetryAfterSecs))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":          "Rate limit exceeded",
					"code":           "RATE_LIMIT_EXCEEDED",
					"requests_made":  count,
					"requests_limit": cfg.RateLimitAfter,
					"retry_after":    fmt.Sprintf("%ds", cfg.RetryAfterSecs),
				})
				return
			}

		case "server_error":
			if rand.Float64() < cfg.ServerErrorRate {
				log.Printf("[ERROR-INJECTION] Server error triggered (rate: %.0f%%)", cfg.ServerErrorRate*100)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "Internal server error (simulated for resilience testing)",
					"code":       "INTERNAL_SERVER_ERROR",
					"error_rate": fmt.Sprintf("%.0f%%", cfg.ServerErrorRate*100),
				})
				return
			}
		}

		// Pass through to actual handler
		next.ServeHTTP(w, r)
	})
}

// AdminInjectErrorHandler handles POST /admin/inject-error
func AdminInjectErrorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InjectErrorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	validModes := map[string]bool{"normal": true, "rate_limit": true, "server_error": true}
	if !validModes[req.Mode] {
		http.Error(w, "Invalid mode. Use: normal, rate_limit, or server_error", http.StatusBadRequest)
		return
	}

	SetConfig(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Error injection mode set to '%s'", req.Mode),
		"config":  GetConfig(),
	})
}

// AdminStatusHandler handles GET /admin/status
func AdminStatusHandler(w http.ResponseWriter, r *http.Request) {
	cfg := GetConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":              cfg.Mode,
		"rate_limit_after":  cfg.RateLimitAfter,
		"server_error_rate": cfg.ServerErrorRate,
		"retry_after_secs":  cfg.RetryAfterSecs,
		"request_count":     cfg.requestCount,
	})
}

// AdminResetHandler handles POST /admin/reset
func AdminResetHandler(w http.ResponseWriter, r *http.Request) {
	ResetConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Error injection reset to normal mode",
		"config":  GetConfig(),
	})
}
