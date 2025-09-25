# GoMind Kubernetes Infrastructure

Welcome to your production-ready GoMind infrastructure! This guide will walk you through deploying a complete Kubernetes setup that supports all GoMind examples and applications. Think of this as the foundation that makes everything else work seamlessly. 🏗️

## 🎯 What Is This and Why Should You Care?

### The City Infrastructure Analogy

Imagine you're building a smart city with different services:
- **Electric power grid** (Redis for service discovery)
- **Communication network** (OTEL Collector for telemetry)
- **Monitoring stations** (Prometheus for metrics)
- **Security cameras** (Jaeger for tracing)
- **Control center dashboard** (Grafana for visualization)

That's exactly what this k8-deployment setup provides for your GoMind applications! It creates the essential infrastructure that every GoMind component needs to work together effectively.

### What This Infrastructure Provides

1. **🏠 Shared Namespace** - A dedicated space (`gomind-examples`) for all your components
2. **📡 Service Discovery** - Redis registry so components can find each other
3. **📊 Observability** - Complete monitoring stack with metrics, logs, and traces
4. **🔍 Debugging** - Visual dashboards to understand what's happening
5. **🚀 Production-Ready** - Persistent storage, security, and scaling configurations

## 📚 Infrastructure Components

| Component | Purpose | Access | Storage |
|-----------|---------|--------|---------|
| **Namespace** | Isolated environment for GoMind apps | N/A | N/A |
| **Redis** | Service discovery registry | `redis:6379` | Persistent volume |
| **OTEL Collector** | Telemetry aggregation and routing | `otel-collector:4318` | None |
| **Prometheus** | Metrics storage and querying | `prometheus:9090` | Persistent volume |
| **Jaeger** | Distributed tracing | `jaeger-query:16686` | None |
| **Grafana** | Visualization dashboard | `grafana:3000` | Persistent volume |

## 🚀 Quick Start

### Prerequisites

**For Local Development (Kind):**
- Docker Desktop or Docker Engine
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) installed
- kubectl configured
- Basic Kubernetes knowledge

**For Server/Cloud Kubernetes:**
- Running Kubernetes cluster (v1.20+)
- kubectl configured with cluster access
- StorageClass available for persistent volumes
- Ingress controller (recommended)

### Installation

#### Method 1: One-Command Setup (Easiest!)

```bash
# Clone the repository (if not already done)
cd examples/k8-deployment

# Deploy everything at once
kubectl apply -k .
```

This uses Kustomization to deploy all components in the correct order with proper dependencies.

#### Method 2: Step-by-Step Deployment (For Understanding)

```bash
# 1. Create the namespace first
kubectl apply -f namespace.yaml

# 2. Deploy Redis (service discovery)
kubectl apply -f redis.yaml

# 3. Deploy OTEL Collector (telemetry)
kubectl apply -f otel-collector.yaml

# 4. Deploy Prometheus (metrics)
kubectl apply -f prometheus.yaml

# 5. Deploy Jaeger (tracing)
kubectl apply -f jaeger.yaml

# 6. Deploy Grafana (dashboards)
kubectl apply -f grafana.yaml
```

#### Verify Installation

```bash
# Check all pods are running
kubectl get pods -n gomind-examples

# Expected output:
# NAME                              READY   STATUS    RESTARTS
# redis-xxx                         1/1     Running   0
# otel-collector-xxx                1/1     Running   0
# prometheus-xxx                    1/1     Running   0
# jaeger-xxx                        1/1     Running   0
# grafana-xxx                       1/1     Running   0
```

## 🏠 Local Development with Kind

Kind (Kubernetes in Docker) is perfect for local development and testing.

### Setting Up Kind Cluster

```bash
# Create a kind cluster with specific configuration
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: gomind-dev
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  # Expose common ports for local access
  - containerPort: 30080  # HTTP services
    hostPort: 30080
    protocol: TCP
  - containerPort: 30090  # Prometheus
    hostPort: 30090
    protocol: TCP
  - containerPort: 30030  # Grafana
    hostPort: 30030
    protocol: TCP
  - containerPort: 30160  # Jaeger
    hostPort: 30160
    protocol: TCP
EOF
```

### Deploy Infrastructure to Kind

```bash
# Set kubectl context to kind cluster
kubectl cluster-info --context kind-gomind-dev

# Deploy the infrastructure
cd examples/k8-deployment
kubectl apply -k .

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod --all -n gomind-examples --timeout=300s
```

### Local Access URLs

With Kind setup above, access services locally:

```bash
# Get service URLs (using port-forward)
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090 &
kubectl port-forward -n gomind-examples svc/grafana 3000:3000 &
kubectl port-forward -n gomind-examples svc/jaeger-query 16686:16686 &

# Access in browser:
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
# - Jaeger: http://localhost:16686
```

### Kind-Specific Configuration

For Kind clusters, the infrastructure automatically:
- Uses `emptyDir` volumes (data is ephemeral)
- Configures smaller resource limits
- Skips ingress setup (use port-forward instead)

## 🌐 Production Server Deployment

For production Kubernetes clusters (AWS EKS, Google GKE, Azure AKS, on-premises):

### Storage Requirements

Ensure your cluster has a default StorageClass:

```bash
# Check available StorageClasses
kubectl get storageclass

# If none exists, create one (example for AWS EBS)
cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp2-retain
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: kubernetes.io/aws-ebs
parameters:
  type: gp2
  fsType: ext4
reclaimPolicy: Retain
allowVolumeExpansion: true
EOF
```

### Ingress Configuration (Recommended)

For production access, set up ingress:

```bash
# Example Ingress for NGINX Ingress Controller
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-infrastructure
  namespace: gomind-examples
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: prometheus.gomind.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: prometheus
            port:
              number: 9090
  - host: grafana.gomind.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: grafana
            port:
              number: 3000
  - host: jaeger.gomind.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: jaeger-query
            port:
              number: 16686
EOF
```

### Production Scaling

Scale components based on your needs:

```bash
# Scale Redis for high availability
kubectl patch deployment redis -n gomind-examples -p '{"spec":{"replicas":3}}'

# Scale OTEL Collector for high throughput
kubectl patch deployment otel-collector -n gomind-examples -p '{"spec":{"replicas":2}}'

# Scale Prometheus for reliability
kubectl patch statefulset prometheus -n gomind-examples -p '{"spec":{"replicas":2}}'
```

## 📊 Component Configuration Deep Dive

### Redis Configuration

Redis serves as the service discovery backend for all GoMind components.

**Key Features:**
- Persistent storage with volume claims
- Memory optimization for large deployments
- Health checks and restart policies
- Configurable maxmemory policies

**Environment Variables:**
```yaml
# In redis.yaml
args:
  - --appendonly yes          # Enable persistence
  - --appendfsync everysec    # Sync every second
  - --maxmemory 1gb          # Memory limit
  - --maxmemory-policy allkeys-lru  # Eviction policy
```

**Accessing Redis:**
```bash
# Connect to Redis from within cluster
kubectl exec -it -n gomind-examples deployment/redis -- redis-cli

# Check registration keys
redis-cli --scan --pattern "gomind:*"

# Monitor service registrations
redis-cli MONITOR
```

### OTEL Collector Configuration

OTEL Collector bridges between GoMind's OpenTelemetry exports and your monitoring stack.

**Key Features:**
- Receives OTLP over HTTP and gRPC
- Exports to Prometheus (metrics) and Jaeger (traces)
- Configurable pipelines and processors
- Memory and CPU optimization

**Pipeline Flow:**
```
GoMind Apps → OTLP → OTEL Collector → Prometheus (metrics)
                                   → Jaeger (traces)
```

**Configuration Highlights:**
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318  # GoMind apps connect here

processors:
  batch:  # Improves performance
  memory_limiter:  # Prevents OOM

exporters:
  prometheus:
    endpoint: "0.0.0.0:8888"  # Prometheus scrapes here
  jaeger:
    endpoint: jaeger-collector:14250
```

### Prometheus Configuration

Prometheus scrapes metrics from the OTEL Collector and provides querying capabilities.

**Key Features:**
- Persistent storage for historical data
- Automatic service discovery
- Configurable retention policies
- Web UI for queries and debugging

**Scrape Configuration:**
```yaml
scrape_configs:
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['otel-collector:8888']
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['gomind-examples']
```

### Jaeger Configuration

Jaeger collects and visualizes distributed traces from GoMind applications.

**Key Features:**
- All-in-one deployment for simplicity
- Web UI for trace exploration
- Automatic trace correlation
- Memory storage (configurable to Elasticsearch/Cassandra)

**Components:**
- **Collector**: Receives traces from OTEL Collector
- **Query**: Web UI and API
- **Agent**: Not needed (using OTEL Collector)

### Grafana Configuration

Grafana provides rich dashboards for metrics visualization.

**Key Features:**
- Pre-configured Prometheus datasource
- Default admin/admin credentials
- Persistent storage for dashboards
- Custom dashboard support

**Default Datasources:**
```yaml
datasources:
  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090
    isDefault: true
```

## 🔧 Customization and Advanced Configuration

### Environment-Specific Kustomization

Create environment overlays:

```bash
# Development overlay
mkdir -p overlays/dev
cat <<EOF > overlays/dev/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

patchesStrategicMerge:
- redis-dev.yaml

configMapGenerator:
- name: environment-config
  literals:
  - ENVIRONMENT=development
  - LOG_LEVEL=debug
EOF
```

### Custom Resource Limits

Adjust resources based on your cluster capacity:

```bash
# Create resource patches
cat <<EOF > resource-limits.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: gomind-examples
spec:
  template:
    spec:
      containers:
      - name: prometheus
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
EOF

kubectl patch deployment prometheus -n gomind-examples --patch-file resource-limits.yaml
```

### Adding Custom Dashboards

```bash
# Create ConfigMap with dashboard JSON
kubectl create configmap custom-dashboard \
  --from-file=dashboard.json \
  -n gomind-examples

# Mount in Grafana deployment
kubectl patch deployment grafana -n gomind-examples -p '{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "grafana",
          "volumeMounts": [{
            "name": "custom-dashboard",
            "mountPath": "/var/lib/grafana/dashboards/custom"
          }]
        }],
        "volumes": [{
          "name": "custom-dashboard",
          "configMap": {"name": "custom-dashboard"}
        }]
      }
    }
  }
}'
```

## 🔍 Troubleshooting Common Issues

### Pods Not Starting

**Check pod status:**
```bash
kubectl get pods -n gomind-examples
kubectl describe pod <pod-name> -n gomind-examples
kubectl logs <pod-name> -n gomind-examples
```

**Common Issues:**
1. **ImagePullBackOff**: Check internet connectivity and image names
2. **Pending**: Check resource availability and storage classes
3. **CrashLoopBackOff**: Check container logs and configuration

### Storage Issues

**Check PVC status:**
```bash
kubectl get pvc -n gomind-examples

# If PVC is pending, check StorageClass
kubectl get storageclass
kubectl describe storageclass <default-class>
```

**Fix common storage issues:**
```bash
# For Kind clusters, ensure PVCs use emptyDir
kubectl patch deployment redis -n gomind-examples -p '{
  "spec": {
    "template": {
      "spec": {
        "volumes": [{
          "name": "redis-data",
          "emptyDir": {}
        }]
      }
    }
  }
}'
```

### Service Discovery Issues

**Check Redis connectivity:**
```bash
# Test Redis from another pod
kubectl run redis-test --image=redis:7-alpine -it --rm --restart=Never \
  -- redis-cli -h redis.gomind-examples.svc.cluster.local ping

# Should return: PONG
```

**Monitor service registrations:**
```bash
kubectl exec -it -n gomind-examples deployment/redis -- redis-cli MONITOR
```

### OTEL Collector Issues

**Check OTEL Collector logs:**
```bash
kubectl logs -n gomind-examples deployment/otel-collector -f
```

**Test OTLP endpoint:**
```bash
# From within cluster
kubectl run test-pod --image=curlimages/curl -it --rm --restart=Never \
  -- curl -X POST http://otel-collector.gomind-examples.svc.cluster.local:4318/v1/traces
```

### Prometheus Not Scraping

**Check Prometheus targets:**
```bash
# Access Prometheus UI and check Status -> Targets
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090
# Open http://localhost:9090/targets
```

**Common fixes:**
```bash
# Restart Prometheus to reload config
kubectl rollout restart deployment/prometheus -n gomind-examples
```

## 📈 Monitoring Your Infrastructure

### Health Check Commands

```bash
# Quick health check script
cat <<'EOF' > health-check.sh
#!/bin/bash
echo "🔍 GoMind Infrastructure Health Check"
echo "======================================"

NAMESPACE="gomind-examples"

echo "📊 Pod Status:"
kubectl get pods -n $NAMESPACE

echo -e "\n📡 Service Status:"
kubectl get svc -n $NAMESPACE

echo -e "\n💾 Storage Status:"
kubectl get pvc -n $NAMESPACE

echo -e "\n🔧 Recent Events:"
kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp' | tail -5

echo -e "\n✅ Health Check Complete!"
EOF

chmod +x health-check.sh
./health-check.sh
```

### Performance Monitoring

```bash
# Monitor resource usage
kubectl top pods -n gomind-examples

# Monitor specific component
kubectl top pod -n gomind-examples -l app=redis

# Get detailed resource usage
kubectl describe node | grep -A5 "Allocated resources"
```

## 🚀 Production Best Practices

### 1. Security Hardening

```bash
# Create network policies
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gomind-network-policy
  namespace: gomind-examples
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: gomind-examples
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: gomind-examples
EOF
```

### 2. Backup Strategy

```bash
# Backup Redis data
kubectl exec -n gomind-examples deployment/redis -- redis-cli BGSAVE

# Backup Prometheus data
kubectl exec -n gomind-examples deployment/prometheus -- \
  tar -czf /backup/prometheus-$(date +%Y%m%d).tar.gz /prometheus/data

# Backup Grafana dashboards
kubectl get configmap -n gomind-examples -o yaml > grafana-backup.yaml
```

### 3. Monitoring Alerts

```bash
# Add Prometheus alerting rules
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-alerts
  namespace: gomind-examples
data:
  alert.rules: |
    groups:
    - name: gomind
      rules:
      - alert: GoMindComponentDown
        expr: up{job="gomind-components"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "GoMind component is down"
EOF
```

### 4. Resource Management

```bash
# Set resource quotas
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ResourceQuota
metadata:
  name: gomind-quota
  namespace: gomind-examples
spec:
  hard:
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
    persistentvolumeclaims: "10"
EOF
```

## 🔄 Updates and Maintenance

### Updating Infrastructure

```bash
# Update to latest configurations
git pull origin main
kubectl apply -k examples/k8-deployment

# Rolling restart all deployments
kubectl rollout restart deployment -n gomind-examples
```

### Version Management

```bash
# Tag current configuration
kubectl annotate namespace gomind-examples \
  deployment.version="v$(date +%Y%m%d-%H%M%S)"

# Rollback if needed
kubectl rollout undo deployment/redis -n gomind-examples
```

## 🎉 Summary

This infrastructure provides the foundation for running GoMind applications at scale. You now have:

1. **🏗️ Complete Infrastructure** - Redis, OTEL Collector, Prometheus, Jaeger, and Grafana
2. **🔧 Flexible Deployment** - Works on local Kind clusters and production Kubernetes
3. **📊 Full Observability** - Metrics, logs, traces, and dashboards
4. **🛡️ Production Ready** - Persistent storage, security, and monitoring
5. **🔄 Easy Maintenance** - Health checks, updates, and troubleshooting guides

### What's Next?

1. Deploy GoMind applications using this infrastructure
2. Explore the [Monitoring Guide](../monitoring/README.md) for advanced observability
3. Check out example applications in other folders
4. Set up custom dashboards and alerts

**🎊 Congratulations!** Your GoMind infrastructure is ready for action. All your applications can now discover each other, export telemetry, and provide rich observability. Happy building! 🚀