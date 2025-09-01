package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHandleFunc tests the new HandleFunc method
func TestHandleFunc(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*BaseAgent) error
		pattern     string
		handler     http.HandlerFunc
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful registration",
			pattern: "/api/test",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: false,
		},
		{
			name:    "duplicate pattern registration",
			setupFunc: func(agent *BaseAgent) error {
				return agent.HandleFunc("/api/duplicate", func(w http.ResponseWriter, r *http.Request) {})
			},
			pattern: "/api/duplicate",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr:     true,
			errContains: "handler already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := NewBaseAgent("test-agent")
			
			// Run setup if provided
			if tt.setupFunc != nil {
				if err := tt.setupFunc(agent); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}
			
			// Test HandleFunc
			err := agent.HandleFunc(tt.pattern, tt.handler)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("HandleFunc() expected error but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("HandleFunc() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("HandleFunc() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestHandleFuncAfterStart tests that HandleFunc fails after server starts
func TestHandleFuncAfterStart(t *testing.T) {
	agent := NewBaseAgent("test-agent")
	
	// Start server in background
	go func() {
		_ = agent.Start(0) // Use port 0 for random port
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Try to register handler after start
	err := agent.HandleFunc("/api/late", func(w http.ResponseWriter, r *http.Request) {})
	
	if err == nil {
		t.Errorf("HandleFunc() should fail after server starts")
	} else if !strings.Contains(err.Error(), "server already started") {
		t.Errorf("HandleFunc() error = %v, want error containing 'server already started'", err)
	}
	
	// Clean up
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = agent.Stop(ctx)
}

// TestHandleFuncIntegration tests that registered handlers actually work
func TestHandleFuncIntegration(t *testing.T) {
	agent := NewBaseAgent("test-agent")
	
	// Register a custom handler
	called := false
	err := agent.HandleFunc("/api/custom", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("custom response"))
	})
	
	if err != nil {
		t.Fatalf("HandleFunc() failed: %v", err)
	}
	
	// Test the handler directly through the mux
	req := httptest.NewRequest("GET", "/api/custom", nil)
	rec := httptest.NewRecorder()
	
	agent.mux.ServeHTTP(rec, req)
	
	if !called {
		t.Errorf("Custom handler was not called")
	}
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	
	if rec.Body.String() != "custom response" {
		t.Errorf("Expected body 'custom response', got '%s'", rec.Body.String())
	}
}

// TestHandleFuncThreadSafety tests concurrent handler registration
func TestHandleFuncThreadSafety(t *testing.T) {
	agent := NewBaseAgent("test-agent")
	
	// Try to register handlers concurrently
	errors := make(chan error, 10)
	
	for i := 0; i < 10; i++ {
		go func(n int) {
			pattern := fmt.Sprintf("/api/concurrent%d", n)
			err := agent.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {})
			errors <- err
		}(i)
	}
	
	// Collect results
	for i := 0; i < 10; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent registration failed: %v", err)
		}
	}
	
	// Verify all patterns were registered
	if len(agent.registeredPatterns) < 10 {
		t.Errorf("Expected at least 10 registered patterns, got %d", len(agent.registeredPatterns))
	}
}

