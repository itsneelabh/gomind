# GoMind Monitoring and Observability

Welcome to your comprehensive monitoring guide! This is where you'll learn to become a detective of your distributed system - understanding what's happening, why it's happening, and how to make everything run better. Think of this as your mission control center! 🚀

## 🎯 What Is Monitoring and Why Should You Care?

### The Air Traffic Control Analogy

Imagine you're running air traffic control for a busy airport:
- **Radar screens** show where every plane is (metrics)
- **Flight recordings** capture what pilots say (logs)
- **Route tracking** follows each journey from takeoff to landing (traces)
- **Control tower dashboard** gives you the big picture (Grafana)

That's exactly what monitoring does for your GoMind applications! It gives you eyes and ears into your distributed system so you can:
- **Spot problems** before users notice them
- **Debug issues** faster when they occur
- **Optimize performance** based on real data
- **Plan capacity** for future growth

### The Three Pillars of Observability

1. **📊 Metrics** - "How much/many?" (CPU usage, request count, response time)
2. **📝 Logs** - "What happened?" (Error messages, debug info, events)
3. **🔍 Traces** - "How did this request flow?" (Request journey across services)

## 📚 Monitoring Stack Overview

Your complete monitoring setup includes:

| Tool | Type | Purpose | Access |
|------|------|---------|---------|
| **Prometheus** | Metrics Storage | Collect and store metrics | `:9090` |
| **Grafana** | Visualization | Create dashboards and alerts | `:3000` |
| **Jaeger** | Tracing | Track requests across services | `:16686` |
| **OTEL Collector** | Telemetry Gateway | Receive and route telemetry data | `:4318` |

## 🚀 Quick Start - Your First Dashboard

### Prerequisites

Make sure you have the infrastructure running from [k8-deployment](../k8-deployment/README.md):

```bash
# Check infrastructure is running
kubectl get pods -n gomind-examples

# Should see: prometheus, grafana, jaeger, otel-collector, redis
```

### Access Your Monitoring Tools

#### Option 1: Port Forward (Local Development)

```bash
# Open three terminals and run:
kubectl port-forward -n gomind-examples svc/grafana 3000:3000 &
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090 &
kubectl port-forward -n gomind-examples svc/jaeger-query 16686:16686 &

# Access in browser:
# - Grafana: http://localhost:3000 (admin/admin)
# - Prometheus: http://localhost:9090
# - Jaeger: http://localhost:16686
```

#### Option 2: Ingress (Production)

```bash
# If you have ingress configured (see k8-deployment guide)
# Access via your configured hostnames:
# - https://grafana.gomind.local
# - https://prometheus.gomind.local
# - https://jaeger.gomind.local
```

### Your First Look Around

1. **Grafana Dashboard** - Login with admin/admin, explore the interface
2. **Prometheus Targets** - Go to Status → Targets to see what's being monitored
3. **Jaeger Services** - Look at the service list (will be empty until you run GoMind apps)

## 📊 Understanding Metrics with Prometheus

### What Prometheus Monitors

Prometheus automatically collects metrics from:
- **GoMind Applications** - Via OTEL Collector
- **Kubernetes Resources** - Pods, nodes, services
- **Infrastructure Components** - Redis, OTEL Collector itself

### Key GoMind Metrics

When you run GoMind applications, you'll see these important metrics:

#### Core Framework Metrics
```promql
# HTTP request metrics (standard Prometheus metrics with GoMind labels)
http_requests_total{gomind_framework_type="tool"}     # Total requests by method, path, status
http_request_duration_seconds{gomind_framework_type="agent"} # Request latency histogram
up{job=~"gomind-.*"}                                  # Component availability

# GoMind-specific telemetry metrics (via OTEL Collector)
agent_discovery_registrations_total   # Discovery operations (success/failure)
agent_discovery_lookups_total         # Service discovery lookups
agent_health                          # Component health status (0=unhealthy, 1=healthy)

# Agent lifecycle metrics
agent_startup_duration               # Time taken to start up
agent_uptime                        # Current uptime
agent_capabilities_count            # Number of registered capabilities
```

#### AI Module Metrics (when using AI)
```promql
# AI request metrics (via OTEL Collector from GoMind telemetry)
agent_ai_request_duration           # AI request latency
agent_ai_prompt_tokens_total        # Input tokens used
agent_ai_completion_tokens_total    # Output tokens generated
agent_ai_request_cost               # Cost per request (if available)
agent_ai_rate_limit_hits_total      # Rate limiting events
```

### Essential Prometheus Queries

#### System Health Queries
```bash
# Open Prometheus UI (localhost:9090) and try these queries:

# 1. See all GoMind components that are up
up{job=~"gomind-.*"}

# 2. HTTP request rate (requests per second) for GoMind components
rate(http_requests_total{gomind_framework_type=~"tool|agent"}[5m])

# 3. Average response time over last 5 minutes
histogram_quantile(0.95,
  rate(http_request_duration_seconds_bucket{gomind_framework_type=~"tool|agent"}[5m])
)

# 4. Error rate percentage for GoMind services
(
  rate(http_requests_total{gomind_framework_type=~"tool|agent",status=~"5.."}[5m]) /
  rate(http_requests_total{gomind_framework_type=~"tool|agent"}[5m])
) * 100

# 5. Top 5 busiest GoMind components
topk(5,
  sum(rate(http_requests_total{gomind_framework_type=~"tool|agent"}[5m])) by (app)
)
```

#### Resource Usage Queries
```bash
# CPU usage by pod
rate(container_cpu_usage_seconds_total{namespace="gomind-examples"}[5m])

# Memory usage by pod
container_memory_working_set_bytes{namespace="gomind-examples"}

# Network traffic
rate(container_network_receive_bytes_total{namespace="gomind-examples"}[5m])
```

### Setting Up Prometheus Alerts

Create custom alerts for your GoMind applications:

```bash
# Create alerts ConfigMap
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: gomind-alerts
  namespace: gomind-examples
data:
  gomind.rules: |
    groups:
    - name: gomind-applications
      rules:
      - alert: GoMindComponentDown
        expr: up{job=~"gomind.*"} == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "GoMind component {{ \$labels.instance }} is down"
          description: "{{ \$labels.job }} on {{ \$labels.instance }} has been down for more than 30 seconds"

      - alert: GoMindHighErrorRate
        expr: (
          rate(http_requests_total{gomind_framework_type=~"tool|agent",status=~"5.."}[5m]) /
          rate(http_requests_total{gomind_framework_type=~"tool|agent"}[5m])
        ) * 100 > 5
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "High error rate for {{ \$labels.job }}"
          description: "{{ \$labels.job }} has error rate of {{ \$value }}% for the last minute"

      - alert: GoMindHighLatency
        expr: histogram_quantile(0.95,
          rate(http_request_duration_seconds_bucket{gomind_framework_type=~"tool|agent"}[5m])
        ) > 1.0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High latency for {{ \$labels.job }}"
          description: "95th percentile latency is {{ \$value }}s for {{ \$labels.job }}"
EOF

# Restart Prometheus to load the new rules
kubectl rollout restart deployment/prometheus -n gomind-examples
```

## 📈 Creating Dashboards with Grafana

### Grafana First Steps

1. **Login**: admin/admin (change password when prompted)
2. **Verify Datasource**: Go to Configuration → Data Sources → Prometheus should be configured
3. **Import Dashboards**: Use the + icon → Import

### Essential GoMind Dashboard

Create a comprehensive dashboard for your GoMind applications:

```bash
# Save this as gomind-dashboard.json
cat <<'EOF' > gomind-dashboard.json
{
  "dashboard": {
    "id": null,
    "title": "GoMind Applications",
    "tags": ["gomind"],
    "timezone": "",
    "panels": [
      {
        "id": 1,
        "title": "Service Health",
        "type": "stat",
        "targets": [
          {
            "expr": "up{job=~\"gomind.*\"}",
            "legendFormat": "{{ instance }}"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "color": {"mode": "thresholds"},
            "thresholds": {
              "steps": [
                {"color": "red", "value": 0},
                {"color": "green", "value": 1}
              ]
            }
          }
        },
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0}
      },
      {
        "id": 2,
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(http_requests_total{gomind_framework_type=~\"tool|agent\"}[5m])) by (app)",
            "legendFormat": "{{ app }}"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0}
      },
      {
        "id": 3,
        "title": "Response Time (95th percentile)",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{gomind_framework_type=~\"tool|agent\"}[5m]))",
            "legendFormat": "{{ app }} - 95th percentile"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8}
      },
      {
        "id": 4,
        "title": "Error Rate %",
        "type": "graph",
        "targets": [
          {
            "expr": "(rate(http_requests_total{gomind_framework_type=~\"tool|agent\",status=~\"5..\"}[5m]) / rate(http_requests_total{gomind_framework_type=~\"tool|agent\"}[5m])) * 100",
            "legendFormat": "{{ app }} errors"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8}
      }
    ],
    "time": {"from": "now-1h", "to": "now"},
    "refresh": "5s"
  }
}
EOF

# Import into Grafana
# Go to Grafana UI → + → Import → Upload JSON file → gomind-dashboard.json
```

### Pre-Built Dashboard Templates

For quick setup, use these community dashboards:

#### Kubernetes Monitoring Dashboard
```bash
# Dashboard ID: 315 (Kubernetes cluster monitoring)
# Go to Grafana → + → Import → Use ID: 315
```

#### Custom GoMind AI Dashboard

If you're using the AI module, create this specialized dashboard:

```json
{
  "panels": [
    {
      "title": "AI Request Rate",
      "targets": [
        {"expr": "rate(agent_ai_request_duration_count{gomind_framework_type=\"agent\"}[5m])"}
      ]
    },
    {
      "title": "AI Response Time (95th percentile)",
      "targets": [
        {"expr": "histogram_quantile(0.95, rate(agent_ai_request_duration_bucket{gomind_framework_type=\"agent\"}[5m]))"}
      ]
    },
    {
      "title": "Token Usage Rate",
      "targets": [
        {"expr": "rate(agent_ai_prompt_tokens_total{gomind_framework_type=\"agent\"}[5m]) + rate(agent_ai_completion_tokens_total{gomind_framework_type=\"agent\"}[5m])"}
      ]
    },
    {
      "title": "AI Request Success Rate",
      "targets": [
        {"expr": "(rate(agent_ai_request_duration_count{gomind_framework_type=\"agent\"}[5m]) - rate(agent_ai_rate_limit_hits_total{gomind_framework_type=\"agent\"}[5m])) / rate(agent_ai_request_duration_count{gomind_framework_type=\"agent\"}[5m]) * 100"}
      ]
    }
  ]
}
```

## 🔍 Distributed Tracing with Jaeger

### Understanding Traces

A trace shows the complete journey of a request through your system:

```
User Request → Agent → Tool1 → Tool2 → Database → Response
     |          |       |       |        |
   Span1    Span2   Span3   Span4    Span5
```

Each "span" represents work done by one component. Together, they form a complete "trace."

### What You'll See in Jaeger

When your GoMind applications run with tracing enabled:

#### Service Map
- Visual representation of service interactions
- Shows call volume and error rates between services
- Helps identify bottlenecks and dependencies

#### Trace Timeline
- Detailed view of individual requests
- Shows time spent in each service
- Identifies slow components
- Reveals parallel vs sequential processing

### Using Jaeger Effectively

#### 1. Finding Slow Requests

```bash
# In Jaeger UI (localhost:16686):
# 1. Select Service: your-gomind-agent
# 2. Set Min Duration: 1s (to find slow requests)
# 3. Click "Find Traces"
# 4. Click on a slow trace to see detailed timeline
```

#### 2. Debugging Errors

```bash
# To find errors:
# 1. Select Service and set Tags: error=true
# 2. Look for red spans in trace timeline
# 3. Check span details for error messages and stack traces
```

#### 3. Understanding Dependencies

```bash
# Use the System Architecture tab to see:
# - How services call each other
# - Request volume between services
# - Error rates on specific connections
```

### Correlating Traces with Metrics

Link Jaeger traces with Prometheus metrics:

1. **Find slow request in Jaeger** (trace ID: abc123)
2. **Check metrics in Prometheus**: `gomind_http_request_duration_seconds{trace_id="abc123"}`
3. **See dashboard in Grafana**: Filter by time range when slow trace occurred

## 📝 Log Aggregation and Analysis

### GoMind Logging Structure

GoMind applications use structured logging with these standard fields:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "component": "ai-agent",
  "trace_id": "abc123def456",
  "span_id": "789ghi",
  "message": "Processing user request",
  "fields": {
    "user_id": "user123",
    "request_id": "req456",
    "duration_ms": 150
  }
}
```

### Viewing Logs

#### Quick Log Access
```bash
# View logs from specific GoMind application
kubectl logs -n gomind-examples -l app=your-app-name -f

# View logs from all GoMind apps
kubectl logs -n gomind-examples -l gomind.framework/type -f

# Search for errors
kubectl logs -n gomind-examples -l gomind.framework/type -f | grep -i error

# Follow specific trace
kubectl logs -n gomind-examples -l gomind.framework/type -f | grep "trace_id=abc123"
```

#### Advanced Log Analysis with Grafana Loki (Optional)

For production log aggregation, consider adding Loki:

```bash
# Add Loki to your monitoring stack
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm install loki grafana/loki-stack \
  --namespace gomind-examples \
  --set grafana.enabled=false \
  --set prometheus.enabled=false
```

## 🚨 Alerting and Notification Setup

### Alert Manager Configuration

Set up Alert Manager to send notifications:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: gomind-examples
data:
  alertmanager.yml: |
    global:
      smtp_smarthost: 'smtp.gmail.com:587'
      smtp_from: 'alerts@yourcompany.com'

    route:
      group_by: ['alertname']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 1h
      receiver: 'web.hook'

    receivers:
    - name: 'web.hook'
      email_configs:
      - to: 'devops@yourcompany.com'
        subject: 'GoMind Alert: {{ .GroupLabels.alertname }}'
        body: |
          {{ range .Alerts }}
          Alert: {{ .Annotations.summary }}
          Description: {{ .Annotations.description }}
          {{ end }}

    - name: 'slack'
      slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK_URL'
        channel: '#alerts'
        title: 'GoMind Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
EOF
```

### Alert Severity Levels

Configure different notification channels based on severity:

```yaml
# Critical alerts → Page on-call engineer
- match:
    severity: critical
  receiver: pagerduty

# Warning alerts → Slack channel
- match:
    severity: warning
  receiver: slack

# Info alerts → Email
- match:
    severity: info
  receiver: email
```

## 🔧 Advanced Monitoring Techniques

### Custom Metrics in GoMind Applications

Add custom business metrics to your GoMind components:

```go
// In your GoMind tool or agent
func (t *MyTool) RegisterBusinessMetrics() {
    // Counter for business events
    ordersProcessed := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "business_orders_processed_total",
            Help: "Total orders processed",
        },
        []string{"status", "region"},
    )

    // Gauge for current state
    activeConnections := prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "business_active_connections",
            Help: "Current active database connections",
        },
    )

    // Histogram for business latency
    orderProcessingTime := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "business_order_processing_duration_seconds",
            Help: "Time spent processing orders",
            Buckets: prometheus.DefBuckets,
        },
        []string{"order_type"},
    )
}
```

### Service Level Objectives (SLOs)

Define and monitor SLOs for your GoMind applications:

```promql
# Availability SLO: 99.9% uptime
(
  sum(rate(http_requests_total{gomind_framework_type=~"tool|agent",status!~"5.."}[30d])) /
  sum(rate(http_requests_total{gomind_framework_type=~"tool|agent"}[30d]))
) * 100

# Latency SLO: 95% of requests under 500ms
histogram_quantile(0.95,
  rate(http_request_duration_seconds_bucket{gomind_framework_type=~"tool|agent"}[5m])
) < 0.5

# Error Budget: How much error budget remains
error_budget_remaining = (1 - slo_target) - current_error_rate
```

### Capacity Planning

Use metrics for capacity planning:

```bash
# Create capacity planning dashboard with:

# 1. Resource utilization trends
rate(container_cpu_usage_seconds_total[5m])

# 2. Request growth rate
increase(gomind_http_requests_total[24h])

# 3. Peak usage patterns
max_over_time(
  rate(gomind_http_requests_total[5m])[1d:1m]
)

# 4. Storage growth
increase(redis_used_memory_bytes[7d])
```

## 🔄 Integration with CI/CD

### Monitoring During Deployments

Monitor deployment health:

```bash
# Create deployment monitoring dashboard
# Track these metrics during rollouts:

# 1. Error rate before/after deployment
rate(gomind_http_requests_total{status=~"5.."}[5m])

# 2. Response time degradation
increase(gomind_http_request_duration_seconds{quantile="0.95"}[10m])

# 3. New version health
up{version=~"new-deployment-version"}
```

### Automated Rollback Triggers

Set up automatic rollback based on metrics:

```bash
# If error rate exceeds threshold for 2 minutes, rollback
(
  rate(gomind_http_requests_total{status=~"5.."}[5m]) /
  rate(gomind_http_requests_total[5m])
) * 100 > 10
```

## 🛠️ Troubleshooting Monitoring Setup

### Common Issues and Solutions

#### Metrics Not Appearing

```bash
# Check OTEL Collector is receiving data
kubectl logs -n gomind-examples deployment/otel-collector

# Check Prometheus is scraping
curl http://localhost:9090/api/v1/targets

# Verify service discovery
kubectl get endpoints -n gomind-examples
```

#### Grafana Dashboard Empty

```bash
# Check Prometheus data source
# Grafana → Configuration → Data Sources → Prometheus
# Test connection should succeed

# Verify Prometheus has data
curl http://localhost:9090/api/v1/query?query=up

# Check time range in dashboard (might be too narrow)
```

#### Jaeger Not Showing Traces

```bash
# Check OTEL Collector is forwarding traces
kubectl logs -n gomind-examples deployment/otel-collector | grep jaeger

# Verify Jaeger is receiving data
kubectl logs -n gomind-examples deployment/jaeger

# Check your applications are sending traces
# Look for OTEL_EXPORTER_OTLP_ENDPOINT in app config
```

#### High Cardinality Issues

```bash
# If Prometheus is using too much memory:

# 1. Check metric cardinality
prometheus_tsdb_symbol_table_size_bytes

# 2. Find high-cardinality metrics
topk(10, count by (__name__)({__name__=~".+"}))

# 3. Reduce label values or add recording rules
```

### Performance Optimization

```bash
# Optimize Prometheus performance
# 1. Adjust scrape intervals
scrape_interval: 30s  # Default: 15s

# 2. Set retention period
storage.tsdb.retention.time: 15d  # Default: 15d

# 3. Configure recording rules for expensive queries
groups:
- name: gomind-recording-rules
  rules:
  - record: gomind:request_rate_5m
    expr: rate(gomind_http_requests_total[5m])
```

## 📊 Monitoring Different GoMind Patterns

### Tool Monitoring

For GoMind Tools (passive components):

**Key Metrics to Watch:**
- Request rate and latency
- Error rates
- Resource usage (CPU/memory)
- Task processing time

```promql
# Tool-specific queries
rate(http_requests_total{gomind_framework_type="tool"}[5m])
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{gomind_framework_type="tool"}[5m]))
agent_capabilities_count{gomind_framework_type="tool"}
```

### Agent Monitoring

For GoMind Agents (orchestrating components):

**Key Metrics to Watch:**
- Discovery operations
- Orchestration complexity
- AI request patterns (if using AI)
- Component coordination success

```promql
# Agent-specific queries
rate(agent_discovery_registrations_total{gomind_framework_type="agent"}[5m])
rate(agent_discovery_lookups_total{gomind_framework_type="agent"}[5m])
rate(http_requests_total{gomind_framework_type="agent"}[5m])
rate(agent_ai_request_duration_count{gomind_framework_type="agent"}[5m])
```

### AI Module Monitoring

For AI-enhanced components:

**Key Metrics to Watch:**
- AI provider response times
- Token usage and costs
- Provider failover events
- Model performance differences

```promql
# AI-specific queries
histogram_quantile(0.95, rate(agent_ai_request_duration_bucket{gomind_framework_type="agent"}[5m]))
rate(agent_ai_prompt_tokens_total{gomind_framework_type="agent"}[5m])
rate(agent_ai_completion_tokens_total{gomind_framework_type="agent"}[5m])
rate(agent_ai_rate_limit_hits_total{gomind_framework_type="agent"}[5m])
agent_ai_request_cost{gomind_framework_type="agent"}
```

## 🎯 Monitoring Best Practices

### The Four Golden Signals

Focus on these four key metrics for any service:

1. **Latency** - How long do requests take?
2. **Traffic** - How many requests are being served?
3. **Errors** - What's the failure rate?
4. **Saturation** - How full is the service?

### Effective Alerting Rules

**DO:**
- Alert on user-visible symptoms, not internal metrics
- Set meaningful thresholds based on historical data
- Include actionable information in alert descriptions
- Use different severity levels appropriately

**DON'T:**
- Alert on every small blip or anomaly
- Set thresholds too sensitive (alert fatigue)
- Create alerts without clear resolution steps
- Alert on things that can't be acted upon immediately

### Dashboard Design Principles

**Effective Dashboards:**
- Start with high-level health, drill down to details
- Use consistent colors and scales
- Include both current values and historical trends
- Group related metrics together
- Add annotations for deployments and changes

## 🎉 Summary

Congratulations! You now have a comprehensive understanding of monitoring your GoMind applications. You've learned:

1. **📊 Complete Observability** - Metrics, logs, and traces working together
2. **🔍 Debugging Techniques** - How to find and fix issues quickly
3. **📈 Dashboard Creation** - Building useful visualizations
4. **🚨 Alerting Setup** - Getting notified when things go wrong
5. **🔧 Advanced Monitoring** - Custom metrics, SLOs, and capacity planning

### Your Monitoring Journey

1. **Start Simple** - Use the basic dashboard templates provided
2. **Monitor Key Metrics** - Focus on the four golden signals
3. **Add Alerting** - Set up notifications for critical issues
4. **Iterate and Improve** - Refine dashboards based on real usage
5. **Plan for Scale** - Add capacity planning and SLO tracking

### What's Next?

1. Deploy GoMind applications and watch the metrics flow in
2. Create custom dashboards for your specific use cases
3. Set up alerting for your critical services
4. Explore advanced features like distributed tracing correlation
5. Build monitoring into your CI/CD pipeline

**🎊 You're now equipped to run observable, reliable GoMind applications!** Your monitoring setup will help you catch issues early, debug problems quickly, and optimize performance continuously. Happy monitoring! 🚀

---

**Pro Tip:** Monitoring is not "set it and forget it" - it's an ongoing practice. Regularly review your dashboards, adjust alerts based on operational learnings, and always ask "what would I need to know to debug this faster next time?"