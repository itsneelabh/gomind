package routing

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// WorkflowRouter uses predefined workflows for routing
type WorkflowRouter struct {
	workflows    map[string]*WorkflowDefinition
	patterns     map[string]*regexp.Regexp
	cache        RoutingCache
	stats        RouterStats
	mu           sync.RWMutex
	workflowPath string
	cacheEnabled bool
	cacheTTL     time.Duration
}

// NewWorkflowRouter creates a new workflow-based router
func NewWorkflowRouter(workflowPath string, options ...WorkflowOption) (*WorkflowRouter, error) {
	r := &WorkflowRouter{
		workflows:    make(map[string]*WorkflowDefinition),
		patterns:     make(map[string]*regexp.Regexp),
		workflowPath: workflowPath,
		cacheEnabled: true,
		cacheTTL:     10 * time.Minute,
	}
	
	// Apply options
	for _, opt := range options {
		opt(r)
	}
	
	// Initialize cache if enabled
	if r.cacheEnabled && r.cache == nil {
		r.cache = NewSimpleCache()
	}
	
	// Load workflows
	if err := r.loadWorkflows(); err != nil {
		return nil, fmt.Errorf("failed to load workflows: %w", err)
	}
	
	return r, nil
}

// WorkflowOption configures the workflow router
type WorkflowOption func(*WorkflowRouter)

// WithWorkflowCache sets a custom cache
func WithWorkflowCache(cache RoutingCache) WorkflowOption {
	return func(r *WorkflowRouter) {
		r.cache = cache
		r.cacheEnabled = cache != nil
	}
}

// WithWorkflowCacheTTL sets the cache TTL
func WithWorkflowCacheTTL(ttl time.Duration) WorkflowOption {
	return func(r *WorkflowRouter) {
		r.cacheTTL = ttl
	}
}

// loadWorkflows loads all workflow definitions from the specified path
func (r *WorkflowRouter) loadWorkflows() error {
	// Check if path exists
	files, err := ioutil.ReadDir(r.workflowPath)
	if err != nil {
		// If directory doesn't exist, that's okay - no workflows defined yet
		return nil
	}
	
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}
		
		path := filepath.Join(r.workflowPath, file.Name())
		if err := r.loadWorkflowFile(path); err != nil {
			// Log error but continue loading other workflows
			fmt.Printf("Warning: failed to load workflow %s: %v\n", path, err)
		}
	}
	
	return nil
}

// loadWorkflowFile loads a single workflow definition
func (r *WorkflowRouter) loadWorkflowFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	
	var workflow WorkflowDefinition
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}
	
	// Store workflow
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.workflows[workflow.Name] = &workflow
	
	// Compile patterns
	for _, pattern := range workflow.Triggers.Patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Printf("Warning: invalid pattern '%s' in workflow %s: %v\n", pattern, workflow.Name, err)
			continue
		}
		r.patterns[workflow.Name+":"+pattern] = compiled
	}
	
	return nil
}

// Route analyzes the prompt and creates a routing plan based on workflows
func (r *WorkflowRouter) Route(ctx context.Context, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	startTime := time.Now()
	
	// Update stats
	r.mu.Lock()
	r.stats.TotalRequests++
	r.mu.Unlock()
	
	// Check cache first
	if r.cacheEnabled && r.cache != nil {
		if cachedPlan, found := r.cache.Get(prompt); found {
			r.mu.Lock()
			r.stats.CacheHits++
			r.mu.Unlock()
			return cachedPlan, nil
		}
		r.mu.Lock()
		r.stats.CacheMisses++
		r.mu.Unlock()
	}
	
	// Find matching workflow
	workflow := r.findMatchingWorkflow(prompt, metadata)
	if workflow == nil {
		r.mu.Lock()
		r.stats.FailedRoutes++
		r.mu.Unlock()
		return nil, &RoutingError{
			Code:    ErrWorkflowNotFound,
			Message: "No matching workflow found for this request",
		}
	}
	
	// Generate routing plan from workflow
	plan, err := r.generatePlanFromWorkflow(workflow, prompt, metadata)
	if err != nil {
		r.mu.Lock()
		r.stats.FailedRoutes++
		r.mu.Unlock()
		return nil, err
	}
	
	// Cache the plan if enabled
	if r.cacheEnabled && r.cache != nil && plan != nil {
		r.cache.Set(prompt, plan, r.cacheTTL)
	}
	
	// Update stats
	r.mu.Lock()
	r.stats.SuccessfulRoutes++
	r.stats.LastRoutingTime = time.Now()
	latency := time.Since(startTime)
	if r.stats.AverageLatency == 0 {
		r.stats.AverageLatency = latency
	} else {
		r.stats.AverageLatency = (r.stats.AverageLatency + latency) / 2
	}
	r.mu.Unlock()
	
	return plan, nil
}

// findMatchingWorkflow finds a workflow that matches the prompt
func (r *WorkflowRouter) findMatchingWorkflow(prompt string, metadata map[string]interface{}) *WorkflowDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	promptLower := strings.ToLower(prompt)
	
	// Check each workflow
	for name, workflow := range r.workflows {
		// Check patterns
		for _, pattern := range workflow.Triggers.Patterns {
			key := name + ":" + pattern
			if regex, exists := r.patterns[key]; exists {
				if regex.MatchString(prompt) {
					return workflow
				}
			}
		}
		
		// Check keywords (all must match)
		if len(workflow.Triggers.Keywords) > 0 {
			allMatch := true
			for _, keyword := range workflow.Triggers.Keywords {
				if !strings.Contains(promptLower, strings.ToLower(keyword)) {
					allMatch = false
					break
				}
			}
			if allMatch {
				return workflow
			}
		}
		
		// Check intents (any can match)
		for _, intent := range workflow.Triggers.Intents {
			if strings.Contains(promptLower, strings.ToLower(intent)) {
				return workflow
			}
		}
	}
	
	return nil
}

// generatePlanFromWorkflow creates a routing plan from a workflow definition
func (r *WorkflowRouter) generatePlanFromWorkflow(workflow *WorkflowDefinition, prompt string, metadata map[string]interface{}) (*RoutingPlan, error) {
	plan := &RoutingPlan{
		ID:             uuid.New().String(),
		Mode:           ModeWorkflow,
		OriginalPrompt: prompt,
		Confidence:     0.95, // High confidence for predefined workflows
		CreatedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"workflow_name":        workflow.Name,
			"workflow_description": workflow.Description,
		},
	}
	
	// Build step dependency map
	stepNameToOrder := make(map[string]int)
	
	// Process each workflow step
	for i, wfStep := range workflow.Steps {
		order := i + 1
		stepNameToOrder[wfStep.Name] = order
		
		// Process instruction template
		instruction, err := r.processTemplate(wfStep.Instruction, metadata, workflow.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to process instruction template: %w", err)
		}
		
		// Parse timeout if specified
		var timeout time.Duration
		if wfStep.Timeout != "" {
			timeout, _ = time.ParseDuration(wfStep.Timeout)
		}
		if timeout == 0 {
			timeout = 30 * time.Second // Default timeout
		}
		
		step := RoutingStep{
			Order:       order,
			StepID:      fmt.Sprintf("%s-%d-%s", workflow.Name, order, uuid.New().String()[:8]),
			AgentName:   wfStep.Agent,
			Namespace:   wfStep.Namespace,
			Instruction: instruction,
			Parallel:    wfStep.Parallel,
			Required:    wfStep.Required,
			Timeout:     timeout,
		}
		
		// Add retry policy for required steps
		if step.Required {
			step.RetryPolicy = &RetryPolicy{
				MaxAttempts: 3,
				Delay:       2 * time.Second,
				BackoffType: "exponential",
			}
		}
		
		plan.Steps = append(plan.Steps, step)
	}
	
	// Resolve dependencies (convert step names to orders)
	for i, wfStep := range workflow.Steps {
		if len(wfStep.DependsOn) > 0 {
			var deps []int
			for _, depName := range wfStep.DependsOn {
				if depOrder, exists := stepNameToOrder[depName]; exists {
					deps = append(deps, depOrder)
				}
			}
			plan.Steps[i].DependsOn = deps
		}
	}
	
	// Estimate duration
	plan.EstimatedDuration = r.estimateDuration(plan.Steps)
	
	return plan, nil
}

// processTemplate processes a template string with variables
func (r *WorkflowRouter) processTemplate(templateStr string, metadata map[string]interface{}, variables map[string]string) (string, error) {
	// Simple variable replacement for now
	result := templateStr
	
	// Replace workflow variables
	for key, value := range variables {
		placeholder := "{{." + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	// Replace metadata variables
	for key, value := range metadata {
		placeholder := "{{." + key + "}}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	
	// For more complex templates, use text/template
	if strings.Contains(result, "{{") {
		tmpl, err := template.New("instruction").Parse(result)
		if err != nil {
			return result, nil // Return as-is if template parsing fails
		}
		
		var buf strings.Builder
		data := make(map[string]interface{})
		for k, v := range variables {
			data[k] = v
		}
		for k, v := range metadata {
			data[k] = v
		}
		
		if err := tmpl.Execute(&buf, data); err != nil {
			return result, nil // Return as-is if execution fails
		}
		result = buf.String()
	}
	
	return result, nil
}

// estimateDuration estimates the total duration based on steps
func (r *WorkflowRouter) estimateDuration(steps []RoutingStep) time.Duration {
	// Simple estimation: max timeout of parallel steps + sum of sequential steps
	var totalDuration time.Duration
	processed := make(map[int]bool)
	
	for _, step := range steps {
		if processed[step.Order] {
			continue
		}
		
		if step.Parallel {
			// Find all parallel steps with same order
			maxTimeout := step.Timeout
			for _, otherStep := range steps {
				if otherStep.Order == step.Order && otherStep.Parallel {
					if otherStep.Timeout > maxTimeout {
						maxTimeout = otherStep.Timeout
					}
					processed[otherStep.Order] = true
				}
			}
			totalDuration += maxTimeout
		} else {
			totalDuration += step.Timeout
			processed[step.Order] = true
		}
	}
	
	return totalDuration
}

// GetMode returns the router mode
func (r *WorkflowRouter) GetMode() RouterMode {
	return ModeWorkflow
}

// SetAgentCatalog is not used by workflow router
func (r *WorkflowRouter) SetAgentCatalog(catalog string) {
	// Not needed for workflow router
}

// GetStats returns routing statistics
func (r *WorkflowRouter) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// AddWorkflow adds a new workflow definition
func (r *WorkflowRouter) AddWorkflow(workflow *WorkflowDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.workflows[workflow.Name] = workflow
	
	// Compile patterns
	for _, pattern := range workflow.Triggers.Patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
		}
		r.patterns[workflow.Name+":"+pattern] = compiled
	}
	
	return nil
}

// RemoveWorkflow removes a workflow definition
func (r *WorkflowRouter) RemoveWorkflow(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.workflows, name)
	
	// Remove associated patterns
	for key := range r.patterns {
		if strings.HasPrefix(key, name+":") {
			delete(r.patterns, key)
		}
	}
}

// ListWorkflows returns all loaded workflows
func (r *WorkflowRouter) ListWorkflows() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.workflows))
	for name := range r.workflows {
		names = append(names, name)
	}
	return names
}

// ReloadWorkflows reloads all workflows from disk
func (r *WorkflowRouter) ReloadWorkflows() error {
	r.mu.Lock()
	r.workflows = make(map[string]*WorkflowDefinition)
	r.patterns = make(map[string]*regexp.Regexp)
	r.mu.Unlock()
	
	return r.loadWorkflows()
}