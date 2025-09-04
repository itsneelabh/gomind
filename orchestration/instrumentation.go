package orchestration

import "github.com/itsneelabh/gomind/telemetry"

func init() {
	// Declare workflow executor metrics
	telemetry.DeclareMetrics("workflow", telemetry.ModuleConfig{
		Metrics: []telemetry.MetricDefinition{
			{
				Name:   "workflow.started",
				Type:   "counter",
				Help:   "Workflows started",
				Labels: []string{"workflow_name"},
			},
			{
				Name:   "workflow.completed",
				Type:   "counter",
				Help:   "Workflows completed",
				Labels: []string{"workflow_name", "status"},
			},
			{
				Name:    "workflow.duration_ms",
				Type:    "histogram",
				Help:    "Workflow execution time in milliseconds",
				Labels:  []string{"workflow_name", "status"},
				Unit:    "ms",
				Buckets: []float64{10, 100, 1000, 10000, 60000},
			},
			{
				Name:    "workflow.step.duration_ms",
				Type:    "histogram",
				Help:    "Individual step duration in milliseconds",
				Labels:  []string{"workflow_name", "step_name", "status"},
				Unit:    "ms",
				Buckets: []float64{1, 10, 100, 1000, 10000},
			},
			{
				Name:   "workflow.step.failures",
				Type:   "counter",
				Help:   "Step failures",
				Labels: []string{"workflow_name", "step_name", "error_type"},
			},
			{
				Name:   "workflow.active",
				Type:   "gauge",
				Help:   "Currently active workflows",
				Labels: []string{"workflow_name"},
			},
			{
				Name:   "workflow.queue.size",
				Type:   "gauge",
				Help:   "Number of workflows in queue",
				Labels: []string{"workflow_name"},
			},
		},
	})
	
	// Declare pipeline executor metrics
	telemetry.DeclareMetrics("pipeline", telemetry.ModuleConfig{
		Metrics: []telemetry.MetricDefinition{
			{
				Name:   "pipeline.executions",
				Type:   "counter",
				Help:   "Pipeline executions",
				Labels: []string{"pipeline_name"},
			},
			{
				Name:    "pipeline.stage.duration_ms",
				Type:    "histogram",
				Help:    "Pipeline stage duration",
				Labels:  []string{"pipeline_name", "stage_name", "status"},
				Unit:    "ms",
				Buckets: []float64{10, 100, 1000, 10000},
			},
			{
				Name:   "pipeline.stage.failures",
				Type:   "counter",
				Help:   "Pipeline stage failures",
				Labels: []string{"pipeline_name", "stage_name", "error_type"},
			},
			{
				Name:   "pipeline.throughput",
				Type:   "gauge",
				Help:   "Pipeline throughput (items/sec)",
				Labels: []string{"pipeline_name"},
			},
		},
	})
	
	// Declare task executor metrics
	telemetry.DeclareMetrics("executor", telemetry.ModuleConfig{
		Metrics: []telemetry.MetricDefinition{
			{
				Name:   "executor.tasks.submitted",
				Type:   "counter",
				Help:   "Tasks submitted to executor",
				Labels: []string{"executor_name", "priority"},
			},
			{
				Name:   "executor.tasks.completed",
				Type:   "counter",
				Help:   "Tasks completed by executor",
				Labels: []string{"executor_name", "status"},
			},
			{
				Name:   "executor.queue.depth",
				Type:   "gauge",
				Help:   "Current queue depth",
				Labels: []string{"executor_name"},
			},
			{
				Name:   "executor.workers.active",
				Type:   "gauge",
				Help:   "Active worker count",
				Labels: []string{"executor_name"},
			},
			{
				Name:    "executor.task.wait_ms",
				Type:    "histogram",
				Help:    "Time spent waiting in queue",
				Labels:  []string{"executor_name", "priority"},
				Unit:    "ms",
				Buckets: []float64{1, 10, 100, 1000, 10000},
			},
			{
				Name:    "executor.task.duration_ms",
				Type:    "histogram",
				Help:    "Task execution duration",
				Labels:  []string{"executor_name", "task_type"},
				Unit:    "ms",
				Buckets: []float64{1, 10, 100, 1000, 10000},
			},
		},
	})
}