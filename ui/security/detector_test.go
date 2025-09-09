//go:build security
// +build security

package security

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestInfrastructureDetector(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    map[string]string
		headers     map[string]string
		wantDetect  bool
		description string
	}{
		{
			name:        "API Gateway via environment",
			setupEnv:    map[string]string{"API_GATEWAY_ENABLED": "true"},
			headers:     map[string]string{},
			wantDetect:  true,
			description: "Should detect API Gateway from environment variable",
		},
		{
			name:        "AWS API Gateway via headers",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"X-Amzn-Trace-Id": "Root=1-abc"},
			wantDetect:  true,
			description: "Should detect AWS API Gateway from headers",
		},
		{
			name:        "Kong proxy via headers",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"X-Kong-Proxy": "true"},
			wantDetect:  true,
			description: "Should detect Kong proxy from headers",
		},
		{
			name:        "Azure API Management",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"X-Azure-Ref": "ref123"},
			wantDetect:  true,
			description: "Should detect Azure API Management",
		},
		{
			name:        "Istio service mesh",
			setupEnv:    map[string]string{"ISTIO_PROXY": "true"},
			headers:     map[string]string{},
			wantDetect:  true,
			description: "Should detect Istio from environment",
		},
		{
			name:        "Istio via tracing headers",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"X-B3-TraceId": "trace123"},
			wantDetect:  true,
			description: "Should detect Istio from B3 tracing headers",
		},
		{
			name:        "Linkerd service mesh",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"L5d-Ctx-Trace": "trace456"},
			wantDetect:  true,
			description: "Should detect Linkerd from headers",
		},
		{
			name:        "No infrastructure",
			setupEnv:    map[string]string{},
			headers:     map[string]string{},
			wantDetect:  false,
			description: "Should not detect any infrastructure",
		},
		{
			name:        "False environment values",
			setupEnv:    map[string]string{"API_GATEWAY_ENABLED": "false", "ISTIO_PROXY": "0"},
			headers:     map[string]string{},
			wantDetect:  false,
			description: "Should not detect when env vars are false",
		},
		{
			name:        "Rate limiting headers present",
			setupEnv:    map[string]string{},
			headers:     map[string]string{"X-RateLimit-Limit": "100"},
			wantDetect:  true,
			description: "Should detect rate limiting from headers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.setupEnv {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create request with headers
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Test detection
			detector := NewInfrastructureDetector()
			got := detector.IsSecurityProvidedByInfra(req)

			if got != tt.wantDetect {
				t.Errorf("%s: got %v, want %v", tt.description, got, tt.wantDetect)
			}
		})
	}
}

func TestIsRateLimitingProvidedByInfra(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   map[string]string
		headers    map[string]string
		wantDetect bool
	}{
		{
			name:       "Standard rate limit headers",
			headers:    map[string]string{"X-RateLimit-Limit": "100", "X-RateLimit-Remaining": "99"},
			wantDetect: true,
		},
		{
			name:       "Alternative rate limit headers",
			headers:    map[string]string{"RateLimit-Limit": "100"},
			wantDetect: true,
		},
		{
			name:       "Rate limiting environment variable",
			setupEnv:   map[string]string{"RATE_LIMITING_ENABLED": "true"},
			headers:    map[string]string{},
			wantDetect: true,
		},
		{
			name:       "API Gateway rate limit env",
			setupEnv:   map[string]string{"API_GATEWAY_RATE_LIMIT": "100"},
			headers:    map[string]string{},
			wantDetect: true,
		},
		{
			name:       "No rate limiting",
			setupEnv:   map[string]string{},
			headers:    map[string]string{},
			wantDetect: false,
		},
		{
			name:       "Disabled rate limiting",
			setupEnv:   map[string]string{"RATE_LIMITING_ENABLED": "false"},
			headers:    map[string]string{},
			wantDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.setupEnv {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			detector := NewInfrastructureDetector()
			got := detector.IsRateLimitingProvidedByInfra(req)

			if got != tt.wantDetect {
				t.Errorf("IsRateLimitingProvidedByInfra() = %v, want %v", got, tt.wantDetect)
			}
		})
	}
}

func TestIsCORSProvidedByInfra(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   map[string]string
		headers    http.Header
		wantDetect bool
	}{
		{
			name:       "CORS headers present",
			headers:    http.Header{"Access-Control-Allow-Origin": []string{"*"}},
			wantDetect: true,
		},
		{
			name: "Multiple CORS headers",
			headers: http.Header{
				"Access-Control-Allow-Origin":  []string{"*"},
				"Access-Control-Allow-Methods": []string{"GET, POST"},
			},
			wantDetect: true,
		},
		{
			name:       "CORS via environment",
			setupEnv:   map[string]string{"CORS_ENABLED": "true"},
			headers:    http.Header{},
			wantDetect: true,
		},
		{
			name:       "API Gateway CORS",
			setupEnv:   map[string]string{"API_GATEWAY_CORS": "1"},
			headers:    http.Header{},
			wantDetect: true,
		},
		{
			name:       "No CORS",
			setupEnv:   map[string]string{},
			headers:    http.Header{},
			wantDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.setupEnv {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			detector := NewInfrastructureDetector()
			got := detector.IsCORSProvidedByInfra(tt.headers)

			if got != tt.wantDetect {
				t.Errorf("IsCORSProvidedByInfra() = %v, want %v", got, tt.wantDetect)
			}
		})
	}
}

func TestDetectorWithNilRequest(t *testing.T) {
	detector := NewInfrastructureDetector()

	// Should handle nil request gracefully
	if detector.IsSecurityProvidedByInfra(nil) {
		t.Error("Expected false for nil request")
	}

	if detector.IsRateLimitingProvidedByInfra(nil) {
		t.Error("Expected false for nil request")
	}
}
