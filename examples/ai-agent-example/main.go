package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/itsneelabh/gomind/ai"
	"github.com/itsneelabh/gomind/core"

	// Import AI providers for auto-detection
	_ "github.com/itsneelabh/gomind/ai/providers/openai"    
	_ "github.com/itsneelabh/gomind/ai/providers/anthropic" 
	_ "github.com/itsneelabh/gomind/ai/providers/gemini"    
)

// AIFirstAgent demonstrates an AI-FIRST architecture where:
// 1. EVERY request goes through AI for initial understanding and planning
// 2. AI creates execution plans before any tool calls
// 3. AI continuously guides the process with intelligent decision-making
// 4. AI synthesizes final results with full context awareness
//
// This is different from the Enhanced Agent which uses AI for analysis AFTER gathering data.
// Here, AI is the "brain" that drives the entire process from start to finish.
type AIFirstAgent struct {
	*core.BaseAgent
	aiClient core.AIClient
}

// NewAIFirstAgent creates an agent that puts AI at the center of every decision
// Unlike traditional agents that follow predefined logic, this agent uses AI to:
// - Understand what the user REALLY wants (intent recognition)
// - Plan the optimal approach to achieve that goal
// - Make real-time decisions during execution
// - Adapt when things don't go as planned
func NewAIFirstAgent() (*AIFirstAgent, error) {
	// Step 1: Create base agent with discovery powers
	agent := core.NewBaseAgent("ai-first-assistant")

	// Step 2: AI is REQUIRED for this agent - it won't work without it
	// This demonstrates a hard dependency on AI for core functionality
	aiClient, err := ai.NewClient()
	if err != nil {
		return nil, fmt.Errorf("AI-First Agent requires an AI provider. Please set OPENAI_API_KEY, GROQ_API_KEY, or another supported provider: %w", err)
	}

	// Step 3: Store AI client - this is the "brain" of our agent
	agent.AI = aiClient

	aiFirstAgent := &AIFirstAgent{
		BaseAgent: agent,
		aiClient:  aiClient,
	}

	// Step 4: Register AI-first capabilities
	// Each capability follows the pattern: AI Plans → Execute → AI Synthesizes
	aiFirstAgent.registerAIFirstCapabilities()
	return aiFirstAgent, nil
}

// registerAIFirstCapabilities sets up capabilities that ALWAYS start with AI planning
// This shows the AI-first pattern where intelligence drives every aspect of execution
func (a *AIFirstAgent) registerAIFirstCapabilities() {
	// 🧠 CAPABILITY 1: Intelligent Query Processor
	// Pattern: AI understands query → AI plans approach → AI executes → AI responds
	a.RegisterCapability(core.Capability{
		Name:        "process_intelligent_query",
		Description: "AI-first processing of any query with intelligent planning and execution",
		InputTypes:  []string{"json", "text"},
		OutputTypes: []string{"json"},
		Handler:     a.handleIntelligentQuery,
	})

	// 🎯 CAPABILITY 2: Goal-Oriented Task Executor  
	// Pattern: AI breaks down goals → AI creates execution plan → AI monitors progress → AI adapts
	a.RegisterCapability(core.Capability{
		Name:        "execute_goal_oriented_task",
		Description: "AI plans and executes complex tasks by breaking them into achievable sub-goals",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     a.handleGoalOrientedTask,
	})

	// 🔍 CAPABILITY 3: Intelligent Service Orchestrator
	// Pattern: AI analyzes need → AI discovers optimal services → AI coordinates execution → AI optimizes
	a.RegisterCapability(core.Capability{
		Name:        "orchestrate_intelligent_services",
		Description: "AI-driven orchestration that intelligently selects and coordinates multiple services",
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     a.handleIntelligentOrchestration,
	})

	// 🚀 CAPABILITY 4: Adaptive Problem Solver
	// Pattern: AI understands problem → AI generates solution strategies → AI executes best strategy → AI learns
	a.RegisterCapability(core.Capability{
		Name:        "solve_adaptive_problem",
		Description: "AI-powered problem solving that adapts approach based on available resources and constraints",
		Endpoint:    "/solve-problem", // Custom endpoint
		InputTypes:  []string{"json"},
		OutputTypes: []string{"json"},
		Handler:     a.handleAdaptiveProblemSolver,
	})
}

// 🧠 CAPABILITY 1: Intelligent Query Processor
// This demonstrates the complete AI-first flow for any type of query
func (a *AIFirstAgent) handleIntelligentQuery(rw http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	
	a.Logger.Info("Starting AI-first query processing", map[string]interface{}{
		"method": req.Method,
		"path":   req.URL.Path,
	})

	// Step 1: Extract the raw query from the request
	var request struct {
		Query       string            `json:"query"`           // The user's natural language query
		Context     string            `json:"context"`         // Additional context about the query
		Preferences map[string]string `json:"preferences"`     // User preferences for processing
		Constraints []string          `json:"constraints"`     // Any constraints to consider
	}

	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()

	// Step 2: AI PLANNING PHASE - This is what makes it "AI-first"
	// Instead of hardcoded logic, AI decides what to do based on the query
	a.Logger.Info("AI analyzing query intent and planning approach", map[string]interface{}{
		"query": request.Query,
	})

	queryAnalysis, err := a.analyzeQueryWithAI(ctx, request.Query, request.Context)
	if err != nil {
		a.Logger.Error("AI query analysis failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "AI analysis failed", http.StatusInternalServerError)
		return
	}

	// Step 3: AI EXECUTION PLANNING
	// AI creates a specific execution plan based on its understanding
	executionPlan, err := a.createAIExecutionPlan(ctx, queryAnalysis, request.Preferences, request.Constraints)
	if err != nil {
		a.Logger.Error("AI execution planning failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Execution planning failed", http.StatusInternalServerError)
		return
	}

	a.Logger.Info("AI created execution plan", map[string]interface{}{
		"plan_steps": len(executionPlan.Steps),
		"intent":     queryAnalysis.Intent,
		"confidence": queryAnalysis.Confidence,
	})

	// Step 4: AI-GUIDED EXECUTION
	// AI monitors execution and adapts as needed
	executionResults, err := a.executeAIPlan(ctx, executionPlan)
	if err != nil {
		a.Logger.Error("AI-guided execution failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Execution failed", http.StatusInternalServerError)
		return
	}

	// Step 5: AI SYNTHESIS AND RESPONSE GENERATION
	// AI creates the final response with full context awareness
	finalResponse, err := a.synthesizeAIResponse(ctx, request.Query, queryAnalysis, executionPlan, executionResults)
	if err != nil {
		a.Logger.Error("AI response synthesis failed", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(rw, "Response synthesis failed", http.StatusInternalServerError)
		return
	}

	// Step 6: Build comprehensive response showing the AI-first process
	response := map[string]interface{}{
		"query":           request.Query,
		"ai_analysis":     queryAnalysis,
		"execution_plan":  executionPlan,
		"execution_results": executionResults,
		"ai_response":     finalResponse,
		"processing_time": time.Since(startTime).String(),
		"ai_driven":       true,
		"metadata": map[string]interface{}{
			"ai_confidence":    queryAnalysis.Confidence,
			"plan_complexity":  len(executionPlan.Steps),
			"services_used":    len(executionResults),
		},
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	a.Logger.Info("AI-first query processing completed", map[string]interface{}{
		"query":         request.Query,
		"intent":        queryAnalysis.Intent,
		"processing_time": time.Since(startTime).String(),
		"ai_confidence": queryAnalysis.Confidence,
	})
}

// 🎯 CAPABILITY 2: Goal-Oriented Task Executor
// This shows how AI can break down complex goals into achievable tasks
func (a *AIFirstAgent) handleGoalOrientedTask(rw http.ResponseWriter, req *http.Request) {
	var taskRequest struct {
		Goal         string              `json:"goal"`           // High-level goal to achieve
		Success      []string            `json:"success_criteria"` // How to measure success
		Resources    map[string]string   `json:"available_resources"` // What resources are available
		Timeline     string              `json:"timeline"`       // When this needs to be completed
		Priority     string              `json:"priority"`       // high, medium, low
	}

	if err := json.NewDecoder(req.Body).Decode(&taskRequest); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()
	taskID := fmt.Sprintf("goal_task_%d", time.Now().Unix())

	a.Logger.Info("Starting goal-oriented task execution", map[string]interface{}{
		"task_id": taskID,
		"goal":    taskRequest.Goal,
		"priority": taskRequest.Priority,
	})

	// Step 1: AI GOAL DECOMPOSITION
	// AI breaks down the high-level goal into specific, actionable sub-goals
	goalDecomposition, err := a.decomposeGoalWithAI(ctx, taskRequest.Goal, taskRequest.Success, taskRequest.Resources)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Goal decomposition failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 2: AI RESOURCE MATCHING
	// AI matches available services to the sub-goals
	resourcesMap := make(map[string]interface{})
	for k, v := range taskRequest.Resources {
		resourcesMap[k] = v
	}
	resourceMapping, err := a.mapResourcesToGoalsWithAI(ctx, goalDecomposition, resourcesMap)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Resource mapping failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 3: AI EXECUTION STRATEGY
	// AI determines the optimal order and approach for executing sub-goals
	executionStrategy, err := a.planExecutionStrategyWithAI(ctx, goalDecomposition, resourceMapping, taskRequest.Timeline)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Execution strategy planning failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 4: AI-MONITORED EXECUTION
	// AI executes the plan while monitoring progress and adapting as needed
	executionResults, err := a.executeGoalPlanWithAI(ctx, executionStrategy)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Goal execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 5: AI SUCCESS EVALUATION
	// AI evaluates whether the goal was achieved and how well
	successEvaluation, err := a.evaluateGoalSuccessWithAI(ctx, taskRequest.Goal, taskRequest.Success, executionResults)
	if err != nil {
		// Continue with basic evaluation if AI evaluation fails
		successEvaluation = map[string]interface{}{
			"achieved": len(executionResults) > 0,
			"confidence": 0.5,
			"evaluation": "Basic evaluation - AI evaluation failed",
		}
	}

	response := map[string]interface{}{
		"task_id":             taskID,
		"goal":               taskRequest.Goal,
		"goal_decomposition":  goalDecomposition,
		"resource_mapping":    resourceMapping,
		"execution_strategy":  executionStrategy,
		"execution_results":   executionResults,
		"success_evaluation":  successEvaluation,
		"ai_driven":          true,
		"status":             "completed",
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	a.Logger.Info("Goal-oriented task completed", map[string]interface{}{
		"task_id": taskID,
		"goal":    taskRequest.Goal,
		"achieved": successEvaluation,
	})
}

// 🔍 CAPABILITY 3: Intelligent Service Orchestrator  
// This demonstrates AI-driven service selection and coordination
func (a *AIFirstAgent) handleIntelligentOrchestration(rw http.ResponseWriter, req *http.Request) {
	var orchRequest struct {
		Need        string            `json:"need"`         // What the user needs accomplished
		Quality     string            `json:"quality"`      // Quality requirements (fast, accurate, comprehensive)
		Budget      string            `json:"budget"`       // Resource constraints (low, medium, high)
		Context     string            `json:"context"`      // Context about the need
		Preferences map[string]string `json:"preferences"`  // User preferences
	}

	if err := json.NewDecoder(req.Body).Decode(&orchRequest); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	ctx := req.Context()
	orchestrationID := fmt.Sprintf("orch_%d", time.Now().Unix())

	a.Logger.Info("Starting intelligent service orchestration", map[string]interface{}{
		"orchestration_id": orchestrationID,
		"need":            orchRequest.Need,
		"quality":         orchRequest.Quality,
	})

	// Step 1: AI SERVICE DISCOVERY AND ANALYSIS
	// AI discovers available services and analyzes their capabilities in context
	availableServices, err := a.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		http.Error(rw, fmt.Sprintf("Service discovery failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	serviceAnalysis, err := a.analyzeServicesWithAI(ctx, availableServices, orchRequest.Need, orchRequest.Quality)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Service analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 2: AI ORCHESTRATION STRATEGY
	// AI creates an orchestration strategy based on the need and available services
	orchestrationStrategy, err := a.createOrchestrationStrategyWithAI(ctx, orchRequest, serviceAnalysis)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Orchestration strategy creation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 3: AI-OPTIMIZED EXECUTION
	// AI executes the orchestration while optimizing for the specified quality and budget constraints
	orchestrationResults, err := a.executeOrchestrationWithAI(ctx, orchestrationStrategy, orchRequest.Quality)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Orchestration execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 4: AI QUALITY ASSESSMENT
	// AI assesses the quality of the orchestration results against the requirements
	qualityAssessment, err := a.assessOrchestrationQualityWithAI(ctx, orchRequest, orchestrationResults)
	if err != nil {
		// Provide basic assessment if AI assessment fails
		qualityAssessment = map[string]interface{}{
			"quality_score": 0.7,
			"assessment": "Basic quality assessment - AI assessment unavailable",
		}
	}

	response := map[string]interface{}{
		"orchestration_id":     orchestrationID,
		"need":                orchRequest.Need,
		"service_analysis":     serviceAnalysis,
		"orchestration_strategy": orchestrationStrategy,
		"execution_results":    orchestrationResults,
		"quality_assessment":   qualityAssessment,
		"ai_optimized":        true,
		"status":              "completed",
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	a.Logger.Info("Intelligent service orchestration completed", map[string]interface{}{
		"orchestration_id": orchestrationID,
		"need":            orchRequest.Need,
		"quality_score":   qualityAssessment,
	})
}

// 🚀 CAPABILITY 4: Adaptive Problem Solver
// This shows AI continuously adapting its approach based on results
func (a *AIFirstAgent) handleAdaptiveProblemSolver(rw http.ResponseWriter, req *http.Request) {
	var problemRequest struct {
		Problem     string   `json:"problem"`     // Description of the problem
		Constraints []string `json:"constraints"` // Constraints to work within
		Attempts    int      `json:"attempts"`    // How many solution attempts to try
		Learn       bool     `json:"learn"`       // Whether to learn from failures
	}

	if err := json.NewDecoder(req.Body).Decode(&problemRequest); err != nil {
		http.Error(rw, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Default values
	if problemRequest.Attempts == 0 {
		problemRequest.Attempts = 3
	}

	ctx := req.Context()
	problemID := fmt.Sprintf("problem_%d", time.Now().Unix())

	a.Logger.Info("Starting adaptive problem solving", map[string]interface{}{
		"problem_id": problemID,
		"problem":    problemRequest.Problem,
		"max_attempts": problemRequest.Attempts,
		"learning_enabled": problemRequest.Learn,
	})

	// Step 1: AI PROBLEM ANALYSIS
	// AI analyzes the problem to understand its nature and complexity
	problemAnalysis, err := a.analyzeProblemWithAI(ctx, problemRequest.Problem, problemRequest.Constraints)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Problem analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Step 2: ADAPTIVE SOLUTION ATTEMPTS
	// AI tries multiple approaches, learning from each attempt
	var allAttempts []map[string]interface{}
	var finalSolution map[string]interface{}
	
	for attempt := 1; attempt <= problemRequest.Attempts; attempt++ {
		a.Logger.Info("AI attempting solution", map[string]interface{}{
			"problem_id": problemID,
			"attempt":    attempt,
		})

		// AI generates a solution strategy based on problem analysis and previous attempts
		solutionStrategy, err := a.generateSolutionStrategyWithAI(ctx, problemAnalysis, allAttempts, attempt)
		if err != nil {
			continue // Skip this attempt if strategy generation fails
		}

		// AI executes the solution strategy
		attemptResult, err := a.executeSolutionStrategyWithAI(ctx, solutionStrategy)
		if err != nil {
			attemptResult = map[string]interface{}{
				"success": false,
				"error":   err.Error(),
				"strategy": solutionStrategy,
			}
		}

		// Add attempt metadata
		attemptResult["attempt"] = attempt
		attemptResult["timestamp"] = time.Now().Format(time.RFC3339)
		allAttempts = append(allAttempts, attemptResult)

		// Check if this attempt was successful
		if success, ok := attemptResult["success"].(bool); ok && success {
			finalSolution = attemptResult
			break // Solution found!
		}

		// AI learns from this attempt if learning is enabled
		if problemRequest.Learn && attempt < problemRequest.Attempts {
			learningInsights, _ := a.learnFromAttemptWithAI(ctx, problemRequest.Problem, attemptResult, attempt)
			attemptResult["learning_insights"] = learningInsights
		}
	}

	// Step 3: AI SOLUTION EVALUATION
	// AI evaluates the final solution (or explains why no solution was found)
	var solutionEvaluation map[string]interface{}
	if finalSolution != nil {
		evaluation, err := a.evaluateSolutionWithAI(ctx, problemRequest.Problem, finalSolution, allAttempts)
		if err == nil {
			solutionEvaluation = evaluation
		}
	} else {
		// AI analyzes why no solution was found
		failureAnalysis, _ := a.analyzeFailureWithAI(ctx, problemRequest.Problem, allAttempts)
		solutionEvaluation = map[string]interface{}{
			"solution_found": false,
			"failure_analysis": failureAnalysis,
			"recommendations": "Consider adjusting constraints or problem definition",
		}
	}

	response := map[string]interface{}{
		"problem_id":         problemID,
		"problem":           problemRequest.Problem,
		"problem_analysis":   problemAnalysis,
		"all_attempts":       allAttempts,
		"final_solution":     finalSolution,
		"solution_evaluation": solutionEvaluation,
		"adaptive_learning":  problemRequest.Learn,
		"total_attempts":     len(allAttempts),
		"solved":            finalSolution != nil,
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)

	a.Logger.Info("Adaptive problem solving completed", map[string]interface{}{
		"problem_id":     problemID,
		"total_attempts": len(allAttempts),
		"solved":        finalSolution != nil,
	})
}

// AI-First Supporting Methods - These implement the core AI-first patterns

// QueryAnalysis represents AI's understanding of a query
type QueryAnalysis struct {
	Intent      string            `json:"intent"`      // What the user wants to achieve
	Entities    []string          `json:"entities"`    // Important entities in the query
	Confidence  float64           `json:"confidence"`  // AI's confidence in its understanding
	Category    string            `json:"category"`    // Category of query (data, analysis, action, etc.)
	Complexity  string            `json:"complexity"`  // simple, moderate, complex
	Approach    string            `json:"approach"`    // How AI thinks this should be approached
}

// ExecutionPlan represents AI's plan for achieving the intent
type ExecutionPlan struct {
	Intent       string                 `json:"intent"`       // The intent being addressed
	Steps        []ExecutionStep        `json:"steps"`        // Ordered steps to execute
	Rationale    string                `json:"rationale"`    // Why AI chose this approach
	Complexity   string                `json:"complexity"`   // Estimated complexity
	ExpectedTime string                `json:"expected_time"` // Estimated execution time
}

// ExecutionStep represents a single step in the AI's plan
type ExecutionStep struct {
	ID          string            `json:"id"`          // Unique step identifier
	Action      string            `json:"action"`      // What to do
	Service     string            `json:"service"`     // Which service to use (if any)
	Parameters  map[string]interface{} `json:"parameters"`  // Parameters for the action
	Rationale   string            `json:"rationale"`   // Why this step is needed
	DependsOn   []string          `json:"depends_on"`  // Dependencies on other steps
}

// analyzeQueryWithAI - AI understands what the user really wants
func (a *AIFirstAgent) analyzeQueryWithAI(ctx context.Context, query, context string) (*QueryAnalysis, error) {
	prompt := fmt.Sprintf(`Analyze this user query and provide deep understanding:

Query: "%s"
Context: %s

Please analyze and respond with JSON containing:
{
  "intent": "what the user wants to achieve",
  "entities": ["important", "entities", "in", "query"],
  "confidence": 0.95,
  "category": "data|analysis|action|information|problem_solving",
  "complexity": "simple|moderate|complex",
  "approach": "how this should be approached"
}

Focus on understanding the underlying need, not just the literal request.`, query, context)

	response, err := a.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.2, // Lower temperature for analytical accuracy
		MaxTokens:   500,
	})
	if err != nil {
		return nil, err
	}

	var analysis QueryAnalysis
	if err := json.Unmarshal([]byte(response.Content), &analysis); err != nil {
		// Fallback if JSON parsing fails
		return &QueryAnalysis{
			Intent:      query,
			Confidence:  0.5,
			Category:    "information",
			Complexity:  "moderate",
			Approach:    response.Content,
		}, nil
	}

	return &analysis, nil
}

// createAIExecutionPlan - AI creates a specific plan to achieve the intent
func (a *AIFirstAgent) createAIExecutionPlan(ctx context.Context, analysis *QueryAnalysis, preferences map[string]string, constraints []string) (*ExecutionPlan, error) {
	// Discover available services first
	availableServices, err := a.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		availableServices = []*core.ServiceInfo{} // Continue with empty list
	}

	// Build service descriptions for AI planning
	serviceDescriptions := make([]string, len(availableServices))
	for i, service := range availableServices {
		capNames := make([]string, len(service.Capabilities))
		for j, cap := range service.Capabilities {
			capNames[j] = cap.Name
		}
		serviceDescriptions[i] = fmt.Sprintf("- %s (%s): %s [%s]", 
			service.Name, service.Type, service.Description, strings.Join(capNames, ", "))
	}

	prompt := fmt.Sprintf(`Create an execution plan to achieve this intent:

Intent: %s
Category: %s
Complexity: %s
User Preferences: %v
Constraints: %v

Available Services:
%s

Create a detailed execution plan in JSON format:
{
  "intent": "%s",
  "steps": [
    {
      "id": "step1",
      "action": "specific action to take",
      "service": "service-name (if using a service)",
      "parameters": {"key": "value"},
      "rationale": "why this step is needed",
      "depends_on": ["previous_step_id"]
    }
  ],
  "rationale": "overall strategy explanation",
  "complexity": "simple|moderate|complex",
  "expected_time": "estimated time"
}

Focus on creating an efficient, logical sequence that maximizes the use of available services.`, 
		analysis.Intent, analysis.Category, analysis.Complexity, 
		preferences, constraints, strings.Join(serviceDescriptions, "\n"), analysis.Intent)

	response, err := a.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.3, // Balanced creativity for planning
		MaxTokens:   1000,
	})
	if err != nil {
		return nil, err
	}

	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(response.Content), &plan); err != nil {
		// Create a basic plan if JSON parsing fails
		return &ExecutionPlan{
			Intent:       analysis.Intent,
			Steps:        []ExecutionStep{{ID: "step1", Action: "basic_execution", Rationale: "fallback plan"}},
			Rationale:    response.Content,
			Complexity:   analysis.Complexity,
			ExpectedTime: "unknown",
		}, nil
	}

	return &plan, nil
}

// executeAIPlan - AI executes the plan while monitoring and adapting
func (a *AIFirstAgent) executeAIPlan(ctx context.Context, plan *ExecutionPlan) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	
	a.Logger.Info("Executing AI-generated plan", map[string]interface{}{
		"intent":    plan.Intent,
		"steps":     len(plan.Steps),
	})

	// Execute each step in the plan
	for _, step := range plan.Steps {
		stepResult, err := a.executeAIStep(ctx, step, results)
		if err != nil {
			// AI adapts to failures - ask AI what to do when a step fails
			adaptation, _ := a.adaptToFailureWithAI(ctx, step, err, results)
			stepResult = map[string]interface{}{
				"success":    false,
				"error":      err.Error(),
				"adaptation": adaptation,
			}
		}
		
		results[step.ID] = stepResult
		
		a.Logger.Info("AI step executed", map[string]interface{}{
			"step_id": step.ID,
			"action":  step.Action,
			"success": stepResult,
		})
	}

	return results, nil
}

// executeAIStep - Execute a single step in the AI plan
func (a *AIFirstAgent) executeAIStep(ctx context.Context, step ExecutionStep, previousResults map[string]interface{}) (map[string]interface{}, error) {
	switch step.Action {
	case "discover_services":
		return a.executeServiceDiscovery(ctx, step)
	case "call_service":
		return a.executeServiceCall(ctx, step, previousResults)
	case "analyze_data":
		return a.executeDataAnalysis(ctx, step, previousResults)
	case "synthesize_results":
		return a.executeSynthesis(ctx, step, previousResults)
	default:
		// For unknown actions, ask AI how to execute
		return a.executeGenericAIStep(ctx, step, previousResults)
	}
}

// executeServiceDiscovery - AI-guided service discovery
func (a *AIFirstAgent) executeServiceDiscovery(ctx context.Context, step ExecutionStep) (map[string]interface{}, error) {
	filter := core.DiscoveryFilter{}
	
	// Extract filter parameters from step parameters
	if stepType, ok := step.Parameters["type"].(string); ok {
		if stepType == "tool" {
			filter.Type = core.ComponentTypeTool
		} else if stepType == "agent" {
			filter.Type = core.ComponentTypeAgent
		}
	}

	services, err := a.Discovery.Discover(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"services_found": len(services),
		"services":       services,
		"filter_used":    filter,
		"success":       true,
	}, nil
}

// executeServiceCall - AI-guided service call
func (a *AIFirstAgent) executeServiceCall(ctx context.Context, step ExecutionStep, previousResults map[string]interface{}) (map[string]interface{}, error) {
	serviceName, ok := step.Parameters["service"].(string)
	if !ok {
		return nil, fmt.Errorf("service name not specified in step parameters")
	}

	// Find the service
	services, err := a.Discovery.Discover(ctx, core.DiscoveryFilter{})
	if err != nil {
		return nil, err
	}

	var targetService *core.ServiceInfo
	for _, service := range services {
		if strings.Contains(strings.ToLower(service.Name), strings.ToLower(serviceName)) {
			targetService = service
			break
		}
	}

	if targetService == nil {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	// Make the service call
	result := a.callServiceIntelligently(ctx, targetService, step.Parameters)
	return map[string]interface{}{
		"service_called": targetService.Name,
		"result":        result,
		"success":       result != nil,
	}, nil
}

// executeDataAnalysis - AI performs data analysis
func (a *AIFirstAgent) executeDataAnalysis(ctx context.Context, step ExecutionStep, previousResults map[string]interface{}) (map[string]interface{}, error) {
	// Extract data from previous results
	var dataToAnalyze interface{}
	if dataKey, ok := step.Parameters["data_source"].(string); ok {
		if data, exists := previousResults[dataKey]; exists {
			dataToAnalyze = data
		}
	}

	if dataToAnalyze == nil {
		return nil, fmt.Errorf("no data found for analysis")
	}

	// Use AI to analyze the data
	analysisPrompt := fmt.Sprintf(`Analyze this data and provide insights:

Data: %v

Analysis requested: %s

Provide structured analysis with key findings, patterns, and recommendations.`, 
		dataToAnalyze, step.Parameters["analysis_type"])

	response, err := a.aiClient.GenerateResponse(ctx, analysisPrompt, &core.AIOptions{
		Temperature: 0.3,
		MaxTokens:   800,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"analysis":        response.Content,
		"data_analyzed":   true,
		"analysis_length": len(response.Content),
		"success":        true,
	}, nil
}

// executeSynthesis - AI synthesizes multiple results
func (a *AIFirstAgent) executeSynthesis(ctx context.Context, step ExecutionStep, previousResults map[string]interface{}) (map[string]interface{}, error) {
	synthesisPrompt := fmt.Sprintf(`Synthesize these results into a coherent response:

Previous Results: %v

Synthesis Goal: %s

Create a comprehensive synthesis that addresses the original intent.`, 
		previousResults, step.Parameters["goal"])

	response, err := a.aiClient.GenerateResponse(ctx, synthesisPrompt, &core.AIOptions{
		Temperature: 0.4,
		MaxTokens:   1000,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"synthesis":      response.Content,
		"results_count":  len(previousResults),
		"success":       true,
	}, nil
}

// executeGenericAIStep - Let AI figure out how to execute unknown steps
func (a *AIFirstAgent) executeGenericAIStep(ctx context.Context, step ExecutionStep, previousResults map[string]interface{}) (map[string]interface{}, error) {
	executionPrompt := fmt.Sprintf(`Execute this step in a plan:

Step Action: %s
Step Parameters: %v
Step Rationale: %s
Previous Results: %v

Based on the action and parameters, determine what should be done and simulate the execution.
Provide a realistic result that would help achieve the overall goal.`, 
		step.Action, step.Parameters, step.Rationale, previousResults)

	response, err := a.aiClient.GenerateResponse(ctx, executionPrompt, &core.AIOptions{
		Temperature: 0.5,
		MaxTokens:   600,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"ai_execution":  response.Content,
		"step_action":   step.Action,
		"simulated":    true,
		"success":      true,
	}, nil
}

// Supporting methods for other capabilities (simplified for brevity)

func (a *AIFirstAgent) decomposeGoalWithAI(ctx context.Context, goal string, successCriteria []string, resources map[string]string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`Break down this goal into specific, actionable sub-goals:

Goal: %s
Success Criteria: %v
Available Resources: %v

Create a JSON response with sub-goals, priorities, and dependencies.`, goal, successCriteria, resources)

	response, err := a.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{Temperature: 0.3, MaxTokens: 800})
	if err != nil {
		return nil, err
	}

	var decomposition map[string]interface{}
	json.Unmarshal([]byte(response.Content), &decomposition)
	return decomposition, nil
}

func (a *AIFirstAgent) synthesizeAIResponse(ctx context.Context, originalQuery string, analysis *QueryAnalysis, plan *ExecutionPlan, results map[string]interface{}) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`Create a final response to the user's query:

Original Query: %s
AI's Understanding: %v
Execution Plan: %v  
Execution Results: %v

Create a comprehensive, user-friendly response that directly addresses their query while showing the value of the AI-first approach.`, 
		originalQuery, analysis, plan, results)

	response, err := a.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{
		Temperature: 0.6,
		MaxTokens:   1200,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"response":     response.Content,
		"ai_processed": true,
		"comprehensive": true,
	}, nil
}

// Additional AI-first helper methods (abbreviated for space)

func (a *AIFirstAgent) adaptToFailureWithAI(ctx context.Context, failedStep ExecutionStep, err error, currentResults map[string]interface{}) (string, error) {
	prompt := fmt.Sprintf(`A step in my execution plan failed. How should I adapt?

Failed Step: %v
Error: %s
Current Results: %v

Suggest an adaptation strategy.`, failedStep, err.Error(), currentResults)

	response, _ := a.aiClient.GenerateResponse(ctx, prompt, &core.AIOptions{Temperature: 0.4, MaxTokens: 300})
	return response.Content, nil
}

func (a *AIFirstAgent) callServiceIntelligently(ctx context.Context, service *core.ServiceInfo, parameters map[string]interface{}) map[string]interface{} {
	// Simplified intelligent service call - in production, this would be more sophisticated
	if len(service.Capabilities) == 0 {
		return map[string]interface{}{"error": "no capabilities available"}
	}

	capability := service.Capabilities[0]
	endpoint := capability.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("/api/capabilities/%s", capability.Name)
	}

	url := fmt.Sprintf("http://%s:%d%s", service.Address, service.Port, endpoint)
	
	// Create request payload
	requestData, _ := json.Marshal(parameters)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result interface{}
	json.Unmarshal(body, &result)

	return map[string]interface{}{
		"service": service.Name,
		"result":  result,
		"success": resp.StatusCode < 400,
	}
}

// Placeholder methods for other AI-first capabilities (would be fully implemented in production)
func (a *AIFirstAgent) mapResourcesToGoalsWithAI(ctx context.Context, goals, resources map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"mapping": "ai_generated"}, nil
}

func (a *AIFirstAgent) planExecutionStrategyWithAI(ctx context.Context, goals, resources map[string]interface{}, timeline string) (map[string]interface{}, error) {
	return map[string]interface{}{"strategy": "ai_optimized"}, nil
}

func (a *AIFirstAgent) executeGoalPlanWithAI(ctx context.Context, strategy map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"execution": "completed"}, nil
}

func (a *AIFirstAgent) evaluateGoalSuccessWithAI(ctx context.Context, goal string, criteria []string, results map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"achieved": true, "confidence": 0.8}, nil
}

func (a *AIFirstAgent) analyzeServicesWithAI(ctx context.Context, services []*core.ServiceInfo, need, quality string) (map[string]interface{}, error) {
	return map[string]interface{}{"analysis": "ai_powered"}, nil
}

func (a *AIFirstAgent) createOrchestrationStrategyWithAI(ctx context.Context, request struct {
	Need        string            `json:"need"`
	Quality     string            `json:"quality"`
	Budget      string            `json:"budget"`
	Context     string            `json:"context"`
	Preferences map[string]string `json:"preferences"`
}, analysis map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"strategy": "intelligent"}, nil
}

func (a *AIFirstAgent) executeOrchestrationWithAI(ctx context.Context, strategy map[string]interface{}, quality string) (map[string]interface{}, error) {
	return map[string]interface{}{"results": "optimized"}, nil
}

func (a *AIFirstAgent) assessOrchestrationQualityWithAI(ctx context.Context, request struct {
	Need        string            `json:"need"`
	Quality     string            `json:"quality"`
	Budget      string            `json:"budget"`
	Context     string            `json:"context"`
	Preferences map[string]string `json:"preferences"`
}, results map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"quality_score": 0.9}, nil
}

func (a *AIFirstAgent) analyzeProblemWithAI(ctx context.Context, problem string, constraints []string) (map[string]interface{}, error) {
	return map[string]interface{}{"analysis": "comprehensive"}, nil
}

func (a *AIFirstAgent) generateSolutionStrategyWithAI(ctx context.Context, analysis map[string]interface{}, previousAttempts []map[string]interface{}, attempt int) (map[string]interface{}, error) {
	return map[string]interface{}{"strategy": fmt.Sprintf("attempt_%d", attempt)}, nil
}

func (a *AIFirstAgent) executeSolutionStrategyWithAI(ctx context.Context, strategy map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"success": true, "result": "solution_found"}, nil
}

func (a *AIFirstAgent) learnFromAttemptWithAI(ctx context.Context, problem string, attempt map[string]interface{}, attemptNum int) (string, error) {
	return fmt.Sprintf("Learning insight from attempt %d", attemptNum), nil
}

func (a *AIFirstAgent) evaluateSolutionWithAI(ctx context.Context, problem string, solution map[string]interface{}, allAttempts []map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"evaluation": "excellent", "confidence": 0.95}, nil
}

func (a *AIFirstAgent) analyzeFailureWithAI(ctx context.Context, problem string, attempts []map[string]interface{}) (string, error) {
	return "Failure analysis: insufficient resources or overly complex problem", nil
}

func main() {
	// Create AI-first agent - requires AI to function
	agent, err := NewAIFirstAgent()
	if err != nil {
		log.Fatalf("Failed to create AI-first agent: %v", err)
		log.Println("💡 To use this agent, set one of these environment variables:")
		log.Println("   export OPENAI_API_KEY=sk-your-openai-key")
		log.Println("   export GROQ_API_KEY=gsk-your-groq-key")
		log.Println("   export ANTHROPIC_API_KEY=sk-ant-your-anthropic-key")
		return
	}

	// Framework configuration for AI-first agent
	framework, err := core.NewFramework(agent,
		// Core configuration
		core.WithName("ai-first-assistant"),
		core.WithPort(8092), // Different port from other agent examples
		core.WithNamespace("examples"),

		// Discovery - essential for AI-first operations
		core.WithRedisURL(getEnvOrDefault("REDIS_URL", "redis://localhost:6379")),
		core.WithDiscovery(true, "redis"),

		// CORS for web access
		core.WithCORS([]string{"*"}, true),

		// Development mode
		core.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatalf("Failed to create framework: %v", err)
	}

	log.Println("🧠 AI-First Agent Starting...")
	log.Println("🎯 AI Provider:", getAIProviderStatus())
	log.Println("📍 Endpoints available:")
	log.Println("   - POST /api/capabilities/process_intelligent_query")
	log.Println("   - POST /api/capabilities/execute_goal_oriented_task")
	log.Println("   - POST /api/capabilities/orchestrate_intelligent_services")
	log.Println("   - POST /solve-problem")
	log.Println("   - GET  /api/capabilities (list all capabilities)")
	log.Println("   - GET  /health (health check)")
	log.Println()
	log.Println("🔗 Test commands:")
	log.Println(`   # Intelligent query processing`)
	log.Println(`   curl -X POST http://localhost:8092/api/capabilities/process_intelligent_query \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"query":"Find the weather in Tokyo and suggest what to wear","context":"traveling tomorrow"}'`)
	log.Println()
	log.Println(`   # Goal-oriented task execution`)
	log.Println(`   curl -X POST http://localhost:8092/api/capabilities/execute_goal_oriented_task \`)
	log.Println(`     -H "Content-Type: application/json" \`)
	log.Println(`     -d '{"goal":"Create a comprehensive market analysis","success_criteria":["data gathered","analysis complete"]}'`)
	log.Println()

	// Run the framework
	ctx := context.Background()
	if err := framework.Run(ctx); err != nil {
		log.Fatalf("Framework execution failed: %v", err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getAIProviderStatus() string {
	providers := []struct {
		name string
		envVar string
	}{
		{"OpenAI", "OPENAI_API_KEY"},
		{"Groq", "GROQ_API_KEY"},
		{"Anthropic", "ANTHROPIC_API_KEY"},
		{"Gemini", "GEMINI_API_KEY"},
		{"DeepSeek", "DEEPSEEK_API_KEY"},
	}

	for _, provider := range providers {
		if os.Getenv(provider.envVar) != "" {
			return provider.name
		}
	}
	
	if os.Getenv("OPENAI_BASE_URL") != "" {
		return "Custom OpenAI-Compatible"
	}
	
	return "REQUIRED - Set OPENAI_API_KEY or other provider"
}