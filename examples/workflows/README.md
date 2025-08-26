# Workflow Examples

This directory contains example workflow definitions for the GoMind Agent Framework's routing system. These workflows demonstrate various patterns and use cases for orchestrating multi-agent interactions.

## Available Workflows

### 1. Simple Calculation (`simple-calculation.yaml`)
- **Purpose**: Basic mathematical calculation workflow
- **Pattern**: Sequential processing
- **Steps**: Parse → Calculate → Format
- **Use Case**: Simple demonstrations and testing

### 2. Data Pipeline (`data-pipeline.yaml`)
- **Purpose**: End-to-end data processing pipeline
- **Pattern**: Sequential with quality checks
- **Steps**: Validate → Extract → Transform → Load → Report
- **Use Case**: ETL processes, data analytics pipelines

### 3. Parallel Search (`parallel-search.yaml`)
- **Purpose**: Search across multiple data sources simultaneously
- **Pattern**: Parallel execution with aggregation
- **Steps**: Parallel searches → Aggregate → Format
- **Use Case**: Comprehensive search operations, data discovery

### 4. Deployment Rollout (`deployment-rollout.yaml`)
- **Purpose**: Orchestrate Kubernetes deployments
- **Pattern**: Sequential with validation and rollback
- **Steps**: Validate → Backup → Deploy → Test → Update
- **Use Case**: Production deployments, CI/CD pipelines

### 5. Customer Support (`customer-support.yaml`)
- **Purpose**: Handle customer support requests
- **Pattern**: Intelligent routing with escalation
- **Steps**: Analyze → Categorize → Resolve/Route → Monitor
- **Use Case**: Customer service automation, ticket management

## Workflow Structure

### Basic Structure
```yaml
name: workflow-name
description: What this workflow does

triggers:
  patterns:    # Regex patterns to match
  keywords:    # Keywords that trigger this workflow
  intents:     # Semantic intents

variables:     # Workflow-level variables
  key: value

steps:         # Ordered list of steps
  - name: step-name
    agent: agent-name
    namespace: agent-namespace
    instruction: What the agent should do
    depends_on: [previous-steps]
    parallel: true/false
    required: true/false
    timeout: "30s"
```

### Key Concepts

#### Triggers
- **Patterns**: Regular expressions for matching user prompts
- **Keywords**: Simple keyword matching (all must be present)
- **Intents**: Semantic intent matching (any can match)

#### Dependencies
- Steps can depend on other steps using `depends_on`
- Dependencies ensure proper execution order
- Parallel steps run simultaneously when dependencies are met

#### Variables
- Support template substitution using `{{.variable_name}}`
- Can reference workflow variables or runtime metadata
- Useful for parameterizing workflows

#### Error Handling
- `required: true` - Step must succeed for workflow to continue
- `required: false` - Step failure won't block workflow
- `on_error` - Global error handling strategy

## Using Workflows

### Loading Workflows
```go
// Create workflow router with directory path
router, err := routing.NewWorkflowRouter("./examples/workflows")
if err != nil {
    log.Fatal(err)
}

// Route a user request
plan, err := router.Route(ctx, "analyze my sales data", metadata)
```

### Adding Workflows Programmatically
```go
workflow := &routing.WorkflowDefinition{
    Name: "custom-workflow",
    Description: "My custom workflow",
    Triggers: routing.WorkflowTriggers{
        Keywords: []string{"custom", "task"},
    },
    Steps: []routing.WorkflowStep{
        {
            Name:        "step1",
            Agent:       "my-agent",
            Instruction: "Do something",
            Required:    true,
        },
    },
}

err := router.AddWorkflow(workflow)
```

### Hybrid Routing
Combine workflow-based routing with LLM-based autonomous routing:
```go
hybridRouter, err := routing.NewHybridRouter(
    "./examples/workflows",
    aiClient,
    routing.WithPreferWorkflow(true),
    routing.WithFallback(true),
)
```

## Best Practices

1. **Keep workflows focused**: Each workflow should handle one specific use case
2. **Use clear naming**: Step names should describe what they do
3. **Handle errors gracefully**: Use `required: false` for optional steps
4. **Set appropriate timeouts**: Prevent workflows from hanging
5. **Use parallel execution**: When steps don't depend on each other
6. **Document variables**: Explain what variables are expected
7. **Test thoroughly**: Validate workflows with different inputs

## Testing Workflows

```go
// Test workflow matching
func TestWorkflowMatching(t *testing.T) {
    router, _ := routing.NewWorkflowRouter("./workflows")
    
    testCases := []struct{
        prompt string
        expectedWorkflow string
    }{
        {"calculate 5 + 3", "simple-calculation"},
        {"deploy my app to production", "deployment-rollout"},
        {"search everywhere for user data", "parallel-search"},
    }
    
    for _, tc := range testCases {
        plan, err := router.Route(ctx, tc.prompt, nil)
        assert.NoError(t, err)
        assert.Equal(t, tc.expectedWorkflow, 
                     plan.Metadata["workflow_name"])
    }
}
```

## Extending Workflows

Workflows can be extended by:
1. Adding new steps
2. Modifying triggers
3. Changing dependencies
4. Adding variables
5. Implementing custom error handlers

## Monitoring and Debugging

Use router statistics to monitor workflow performance:
```go
stats := router.GetStats()
fmt.Printf("Total requests: %d\n", stats.TotalRequests)
fmt.Printf("Success rate: %.2f%%\n", 
    float64(stats.SuccessfulRoutes)/float64(stats.TotalRequests)*100)
```