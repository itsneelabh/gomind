# GoMind Framework Environment Variables Guide

This document provides a comprehensive reference for all environment variables supported by the GoMind framework. Variables are organized by module/functionality with their default values, descriptions, and verification status.

## Important Notes

**Variable Status Legend:**
- **Implemented** - Variable is actively read via `os.Getenv()` and works
- **Struct Tag Only** - Defined in struct tags but NOT currently loaded (requires code changes to work)
- **Example Only** - Used in example applications, not core framework

## Table of Contents

1. [Kubernetes Deployment Requirements](#kubernetes-deployment-requirements) *(Start here for K8s)*
2. [Core Configuration](#core-configuration)
3. [HTTP Server Configuration](#http-server-configuration)
4. [CORS Configuration](#cors-configuration)
5. [Discovery Configuration](#discovery-configuration)
6. [AI Configuration](#ai-configuration)
7. [AI Provider-Specific Variables](#ai-provider-specific-variables)
8. [Telemetry Configuration](#telemetry-configuration)
9. [Memory Configuration](#memory-configuration)
10. [Logging Configuration](#logging-configuration)
11. [Development Configuration](#development-configuration)
12. [Kubernetes Configuration](#kubernetes-configuration)
13. [Orchestration Configuration](#orchestration-configuration)
14. [LLM Debug Configuration](#llm-debug-configuration)
15. [Async Task Configuration](#async-task-configuration)
16. [Prompt Configuration](#prompt-configuration)
17. [Example/Tool Specific Variables](#exampletool-specific-variables)
18. [Quick Reference Table](#quick-reference-table)

---

## Kubernetes Deployment Requirements

This section provides a quick reference for deploying GoMind agents and tools in Kubernetes. Variables are categorized by requirement level to help you configure deployments correctly.

> **Working Examples**: See [agent-example/k8-deployment.yaml](../examples/agent-example/k8-deployment.yaml) for a complete agent deployment and [tool-example/k8-deployment.yaml](../examples/tool-example/k8-deployment.yaml) for a complete tool deployment.

### Required Variables (All Deployments)

These variables are **mandatory** for proper operation in Kubernetes:

| Variable | Why Required | How to Set |
|----------|--------------|------------|
| `GOMIND_AGENT_NAME` | Validation fails without it ([config.go:894](../core/config.go#L894)) | Static value |
| `REDIS_URL` | Required when discovery enabled (auto-enabled in K8s) | ConfigMap or static value |
| `GOMIND_K8S_SERVICE_NAME` | **Critical** for service-fronted discovery URL registration | Must match your K8s Service name |
| `GOMIND_K8S_SERVICE_PORT` | Service port for discovery URL (default: 80) | Must match your K8s Service port |

### Required Variables (via fieldRef)

These are populated automatically by Kubernetes using `fieldRef`:

| Variable | fieldRef Source | Purpose |
|----------|-----------------|---------|
| `GOMIND_K8S_NAMESPACE` | `metadata.namespace` | Pod namespace for service URL construction |
| `GOMIND_K8S_POD_IP` | `status.podIP` | Pod IP address for health checks |
| `GOMIND_K8S_NODE_NAME` | `spec.nodeName` | Node placement info for debugging |

### Required for AI Agents Only

Tools don't need AI configuration, but agents using AI features require:

| Variable | Purpose |
|----------|---------|
| `OPENAI_API_KEY` | Or any other AI provider key (see [AI Provider-Specific Variables](#ai-provider-specific-variables)) |

### Strongly Recommended

| Variable | Default | Why Recommended |
|----------|---------|-----------------|
| `GOMIND_DISCOVERY_RETRY` | `false` | Set to `true` to handle Redis startup race conditions |
| `GOMIND_DISCOVERY_RETRY_INTERVAL` | `30s` | Background retry interval when Redis unavailable |
| `GOMIND_DEV_MODE` | `false` | Explicitly disable for production |

### Auto-Detected (No Need to Set)

| Variable | Auto-Detection |
|----------|----------------|
| `KUBERNETES_SERVICE_HOST` | Set by K8s automatically, triggers K8s mode |
| `HOSTNAME` | Set by K8s to pod name |
| `GOMIND_ADDRESS` | Auto-set to `0.0.0.0` in K8s mode |
| `GOMIND_LOG_FORMAT` | Auto-set to `json` in K8s mode |

### Minimal Tool Deployment

```yaml
env:
  # REQUIRED - Core Identity
  - name: GOMIND_AGENT_NAME
    value: "weather-tool"

  # REQUIRED - Service Discovery
  - name: REDIS_URL
    value: "redis://redis.gomind-examples:6379"

  # REQUIRED - Service-Fronted Discovery (must match Service definition)
  - name: GOMIND_K8S_SERVICE_NAME
    value: "weather-tool-service"
  - name: GOMIND_K8S_SERVICE_PORT
    value: "80"

  # REQUIRED - K8s Metadata (via fieldRef)
  - name: GOMIND_K8S_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: GOMIND_K8S_POD_IP
    valueFrom:
      fieldRef:
        fieldPath: status.podIP
  - name: GOMIND_K8S_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName

  # RECOMMENDED - Resilience
  - name: GOMIND_DISCOVERY_RETRY
    value: "true"
  - name: GOMIND_DISCOVERY_RETRY_INTERVAL
    value: "30s"
```

### Minimal Agent Deployment (with AI)

```yaml
env:
  # Same as Tool (above), plus:

  # REQUIRED - AI Provider Key
  - name: OPENAI_API_KEY
    valueFrom:
      secretKeyRef:
        name: ai-provider-keys
        key: OPENAI_API_KEY

  # OPTIONAL - Telemetry
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.gomind-examples:4318"
```

### Common Mistakes to Avoid

1. **Missing `GOMIND_K8S_SERVICE_NAME`**: Without this, discovery registers the wrong URL and other services can't find your tool/agent.

2. **Mismatched Service Name/Port**: The `GOMIND_K8S_SERVICE_NAME` must exactly match your Kubernetes Service's `metadata.name`, and `GOMIND_K8S_SERVICE_PORT` must match the Service's `port` (not `targetPort`).

3. **Setting redundant variables**: You don't need both `REDIS_URL` and `GOMIND_REDIS_URL` - the framework checks both (with `GOMIND_REDIS_URL` taking precedence).

4. **Setting `GOMIND_ADDRESS`**: This is auto-detected as `0.0.0.0` in K8s - no need to set it.

---

## Core Configuration

These variables configure the fundamental settings of a GoMind agent or tool.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_AGENT_NAME` | `gomind-agent` | **Implemented** | Name of the agent | [core/config.go:422](../core/config.go#L422) |
| `GOMIND_AGENT_ID` | (auto-generated) | **Implemented** | Unique identifier for the agent instance | [core/config.go:433](../core/config.go#L433) |
| `GOMIND_PORT` | `8080` | **Implemented** | HTTP server port | [core/config.go:444](../core/config.go#L444) |
| `GOMIND_ADDRESS` | `localhost` (local) / `0.0.0.0` (K8s) | **Implemented** | Bind address for the HTTP server | [core/config.go:463](../core/config.go#L463) |
| `GOMIND_NAMESPACE` | `default` | **Implemented** | Logical namespace for multi-tenancy | [core/config.go:474](../core/config.go#L474) |

### Example

```bash
export GOMIND_AGENT_NAME="weather-tool"
export GOMIND_PORT=8085
export GOMIND_NAMESPACE="production"
```

---

## HTTP Server Configuration

Configure HTTP server timeouts and health check settings.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_HTTP_READ_TIMEOUT` | `30s` | **Implemented** | Maximum duration for reading the entire request | [core/config.go:487](../core/config.go#L487) |
| `GOMIND_HTTP_WRITE_TIMEOUT` | `30s` | **Implemented** | Maximum duration for writing the response | [core/config.go:492](../core/config.go#L492) |
| `GOMIND_HTTP_READ_HEADER_TIMEOUT` | `10s` | Struct Tag Only | Maximum duration for reading request headers | [core/config.go:78](../core/config.go#L78) |
| `GOMIND_HTTP_IDLE_TIMEOUT` | `120s` | Struct Tag Only | Maximum duration to wait for the next request | [core/config.go:80](../core/config.go#L80) |
| `GOMIND_HTTP_MAX_HEADER_BYTES` | `1048576` (1MB) | Struct Tag Only | Maximum size of request headers | [core/config.go:81](../core/config.go#L81) |
| `GOMIND_HTTP_SHUTDOWN_TIMEOUT` | `10s` | Struct Tag Only | Graceful shutdown timeout | [core/config.go:82](../core/config.go#L82) |
| `GOMIND_HTTP_HEALTH_CHECK` | `true` | Struct Tag Only | Enable health check endpoint | [core/config.go:83](../core/config.go#L83) |
| `GOMIND_HTTP_HEALTH_PATH` | `/health` | Struct Tag Only | Path for health check endpoint | [core/config.go:84](../core/config.go#L84) |

### Example

```bash
# For long-running AI workflows (these are implemented)
export GOMIND_HTTP_READ_TIMEOUT="5m"
export GOMIND_HTTP_WRITE_TIMEOUT="5m"
```

---

## CORS Configuration

Configure Cross-Origin Resource Sharing settings.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_CORS_ENABLED` | `false` | **Implemented** | Enable CORS support | [core/config.go:499](../core/config.go#L499) |
| `GOMIND_CORS_ORIGINS` | (none) | **Implemented** | Comma-separated list of allowed origins | [core/config.go:502](../core/config.go#L502) |
| `GOMIND_CORS_METHODS` | `GET,POST,PUT,DELETE,OPTIONS` | **Implemented** | Allowed HTTP methods | [core/config.go:505](../core/config.go#L505) |
| `GOMIND_CORS_HEADERS` | `Content-Type,Authorization` | **Implemented** | Allowed request headers | [core/config.go:508](../core/config.go#L508) |
| `GOMIND_CORS_CREDENTIALS` | `false` | **Implemented** | Allow credentials (cookies, auth headers) | [core/config.go:511](../core/config.go#L511) |
| `GOMIND_CORS_EXPOSED_HEADERS` | (none) | Struct Tag Only | Headers exposed to the browser | [core/config.go:110](../core/config.go#L110) |
| `GOMIND_CORS_MAX_AGE` | `86400` (24h) | Struct Tag Only | Preflight cache duration in seconds | [core/config.go:112](../core/config.go#L112) |

### Example

```bash
export GOMIND_CORS_ENABLED=true
export GOMIND_CORS_ORIGINS="https://app.example.com,https://*.example.com"
export GOMIND_CORS_CREDENTIALS=true
```

---

## Discovery Configuration

Configure service discovery for agent/tool registration and lookup.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_DISCOVERY_ENABLED` | `false` (local) / `true` (K8s) | **Implemented** | Enable service discovery | [core/config.go:516](../core/config.go#L516) |
| `GOMIND_DISCOVERY_PROVIDER` | `redis` | **Implemented** | Discovery backend provider | [core/config.go:519](../core/config.go#L519) |
| `GOMIND_REDIS_URL` | `redis://localhost:6379` | **Implemented** | Redis connection URL (takes precedence) | [core/config.go:522](../core/config.go#L522) |
| `REDIS_URL` | (fallback) | **Implemented** | Standard Redis URL (fallback if GOMIND_REDIS_URL not set) | [core/config.go:533](../core/config.go#L533) |
| `GOMIND_DISCOVERY_CACHE` | `true` | **Implemented** | Enable local caching of discovery results | [core/config.go:545](../core/config.go#L545) |
| `GOMIND_DISCOVERY_RETRY` | `false` | **Implemented** | Enable background retry on initial connection failure | [core/config.go:548](../core/config.go#L548) |
| `GOMIND_DISCOVERY_RETRY_INTERVAL` | `30s` | **Implemented** | Starting retry interval (increases exponentially) | [core/config.go:559](../core/config.go#L559) |
| `GOMIND_DISCOVERY_CACHE_TTL` | `5m` | Struct Tag Only | Cache time-to-live | [core/config.go:123](../core/config.go#L123) |
| `GOMIND_DISCOVERY_HEARTBEAT` | `10s` | Struct Tag Only | Heartbeat interval for registration refresh | [core/config.go:124](../core/config.go#L124) |
| `GOMIND_DISCOVERY_TTL` | `30s` | Struct Tag Only | Registration TTL | [core/config.go:125](../core/config.go#L125) |

### Variable Precedence

For Redis URL, the precedence order is:
1. Explicit configuration via `WithRedisURL()`
2. `GOMIND_REDIS_URL` environment variable
3. `REDIS_URL` environment variable
4. Default based on environment detection

### Example

```bash
export REDIS_URL="redis://redis:6379"
export GOMIND_DISCOVERY_CACHE=true
export GOMIND_DISCOVERY_RETRY=true
export GOMIND_DISCOVERY_RETRY_INTERVAL="30s"
```

---

## AI Configuration

Configure AI client settings for LLM integration.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_AI_ENABLED` | `false` | **Implemented** | Enable AI features | [core/config.go:579](../core/config.go#L579) |
| `GOMIND_AI_API_KEY` | (none) | **Implemented** | API key for the provider (auto-enables AI) | [core/config.go:582](../core/config.go#L582) |
| `OPENAI_API_KEY` | (fallback) | **Implemented** | Fallback API key (auto-enables AI) | [core/config.go:592](../core/config.go#L592) |
| `GOMIND_AI_MODEL` | `gpt-4` | **Implemented** | Model name to use | [core/config.go:603](../core/config.go#L603) |
| `GOMIND_AI_BASE_URL` | Provider-specific | **Implemented** | Custom base URL for API calls | [core/config.go:606](../core/config.go#L606) |
| `GOMIND_AI_PROVIDER` | `openai` | Struct Tag Only | AI provider | [core/config.go:138](../core/config.go#L138) |
| `GOMIND_AI_TEMPERATURE` | `0.7` | Struct Tag Only | Sampling temperature (0.0-2.0) | [core/config.go:142](../core/config.go#L142) |
| `GOMIND_AI_MAX_TOKENS` | `2000` | Struct Tag Only | Maximum tokens in response | [core/config.go:143](../core/config.go#L143) |
| `GOMIND_AI_TIMEOUT` | `30s` | Struct Tag Only | Request timeout | [core/config.go:144](../core/config.go#L144) |
| `GOMIND_AI_RETRY_ATTEMPTS` | `3` | Struct Tag Only | Number of retry attempts | [core/config.go:145](../core/config.go#L145) |
| `GOMIND_AI_RETRY_DELAY` | `1s` | Struct Tag Only | Delay between retries | [core/config.go:146](../core/config.go#L146) |

### Example

```bash
export GOMIND_AI_ENABLED=true
export GOMIND_AI_MODEL="gpt-4-turbo"
export GOMIND_AI_BASE_URL="https://api.openai.com/v1"
```

---

## AI Provider-Specific Variables

The framework supports multiple AI providers with automatic detection based on available API keys.

### OpenAI (Priority: 100)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `OPENAI_API_KEY` | (none) | **Implemented** | OpenAI API key | [ai/providers/openai/factory.go:164](../ai/providers/openai/factory.go#L164) |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | **Implemented** | OpenAI API base URL | [ai/providers/openai/factory.go:168](../ai/providers/openai/factory.go#L168) |

### Anthropic Claude (Priority: 80)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `ANTHROPIC_API_KEY` | (none) | **Implemented** | Anthropic API key | [ai/providers/anthropic/factory.go:37](../ai/providers/anthropic/factory.go#L37) |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | **Implemented** | Anthropic API base URL | [ai/providers/anthropic/factory.go:43](../ai/providers/anthropic/factory.go#L43) |

### Groq (Priority: 95)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GROQ_API_KEY` | (none) | **Implemented** | Groq API key | [ai/providers/openai/factory.go:112](../ai/providers/openai/factory.go#L112) |
| `GROQ_BASE_URL` | `https://api.groq.com/openai/v1` | **Implemented** | Groq API base URL | [ai/providers/openai/factory.go:115](../ai/providers/openai/factory.go#L115) |

### DeepSeek (Priority: 90)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `DEEPSEEK_API_KEY` | (none) | **Implemented** | DeepSeek API key | [ai/providers/openai/factory.go:103](../ai/providers/openai/factory.go#L103) |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | **Implemented** | DeepSeek API base URL | [ai/providers/openai/factory.go:106](../ai/providers/openai/factory.go#L106) |

### xAI Grok (Priority: 85)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `XAI_API_KEY` | (none) | **Implemented** | xAI API key | [ai/providers/openai/factory.go:121](../ai/providers/openai/factory.go#L121) |
| `XAI_BASE_URL` | `https://api.x.ai/v1` | **Implemented** | xAI API base URL | [ai/providers/openai/factory.go:124](../ai/providers/openai/factory.go#L124) |

### Alibaba Qwen (Priority: 80)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `QWEN_API_KEY` | (none) | **Implemented** | Qwen API key | [ai/providers/openai/factory.go:130](../ai/providers/openai/factory.go#L130) |
| `QWEN_BASE_URL` | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | **Implemented** | Qwen API base URL | [ai/providers/openai/factory.go:133](../ai/providers/openai/factory.go#L133) |

### Together AI (Priority: 75)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `TOGETHER_API_KEY` | (none) | **Implemented** | Together AI API key | [ai/providers/openai/factory.go:139](../ai/providers/openai/factory.go#L139) |
| `TOGETHER_BASE_URL` | `https://api.together.xyz/v1` | **Implemented** | Together AI API base URL | [ai/providers/openai/factory.go:142](../ai/providers/openai/factory.go#L142) |

### Google Gemini (Priority: 70)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GEMINI_API_KEY` | (none) | **Implemented** | Google Gemini API key | [ai/providers/gemini/factory.go:37](../ai/providers/gemini/factory.go#L37) |
| `GOOGLE_API_KEY` | (fallback) | **Implemented** | Alternative Google API key | [ai/providers/gemini/factory.go:40](../ai/providers/gemini/factory.go#L40) |
| `GEMINI_BASE_URL` | `https://generativelanguage.googleapis.com` | **Implemented** | Gemini API base URL | [ai/providers/gemini/factory.go:47](../ai/providers/gemini/factory.go#L47) |

### Ollama (Priority: 50 - Local)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `OLLAMA_BASE_URL` | `http://localhost:11434/v1` | **Implemented** | Ollama local server URL | [ai/providers/openai/factory.go:151](../ai/providers/openai/factory.go#L151) |

### AWS Bedrock (Priority: 60)

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `AWS_ACCESS_KEY_ID` | (none) | **Implemented** | AWS access key | [ai/providers/bedrock/factory.go:120](../ai/providers/bedrock/factory.go#L120) |
| `AWS_SECRET_ACCESS_KEY` | (none) | **Implemented** | AWS secret key | [ai/providers/bedrock/factory.go:120](../ai/providers/bedrock/factory.go#L120) |
| `AWS_SESSION_TOKEN` | (none) | **Implemented** | AWS session token (temporary credentials) | [ai/providers/bedrock/factory.go:64](../ai/providers/bedrock/factory.go#L64) |
| `AWS_REGION` | `us-east-1` | **Implemented** | AWS region | [ai/providers/bedrock/factory.go:45](../ai/providers/bedrock/factory.go#L45) |
| `AWS_DEFAULT_REGION` | (fallback) | **Implemented** | Alternative region variable | [ai/providers/bedrock/factory.go:47](../ai/providers/bedrock/factory.go#L47) |
| `AWS_PROFILE` | (none) | **Implemented** | AWS CLI profile name | [ai/providers/bedrock/factory.go:125](../ai/providers/bedrock/factory.go#L125) |
| `AWS_EXECUTION_ENV` | (auto) | **Implemented** | Set by AWS Lambda | [ai/providers/bedrock/factory.go:131](../ai/providers/bedrock/factory.go#L131) |
| `AWS_LAMBDA_FUNCTION_NAME` | (auto) | **Implemented** | Set in Lambda environment | [ai/providers/bedrock/factory.go:131](../ai/providers/bedrock/factory.go#L131) |
| `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` | (auto) | **Implemented** | Set in ECS environment | [ai/providers/bedrock/factory.go:136](../ai/providers/bedrock/factory.go#L136) |

### Auto-Detection Priority

When no explicit provider is specified, the framework auto-detects available providers:

1. **OpenAI** (100) - `OPENAI_API_KEY`
2. **Groq** (95) - `GROQ_API_KEY`
3. **DeepSeek** (90) - `DEEPSEEK_API_KEY`
4. **xAI** (85) - `XAI_API_KEY`
5. **Anthropic** (80) - `ANTHROPIC_API_KEY`
6. **Qwen** (80) - `QWEN_API_KEY`
7. **Together AI** (75) - `TOGETHER_API_KEY`
8. **Gemini** (70) - `GEMINI_API_KEY` or `GOOGLE_API_KEY`
9. **Bedrock** (60) - AWS credentials
10. **Ollama** (50) - Local service check

### Overriding Auto-Detection Priority

The priority values above are **hardcoded defaults** used only during auto-detection (when calling `ai.NewClient()` without specifying a provider). Developers can override this behavior in several ways:

**1. Explicit Provider Selection** - Bypasses auto-detection entirely:
```go
// Use Anthropic regardless of OpenAI having higher priority
client, _ := ai.NewClient(ai.WithProvider("anthropic"))
```

**2. Provider Aliases** - Select specific OpenAI-compatible services:
```go
// Use DeepSeek even if OPENAI_API_KEY is also set
client, _ := ai.NewClient(ai.WithProviderAlias("openai.deepseek"))
```

**3. Chain Client** - Define your own failover order:
```go
// Your order: Groq → DeepSeek → OpenAI (ignores default priorities)
client, _ := ai.NewChainClient(
    ai.WithProviderChain("openai.groq", "openai.deepseek", "openai"),
)
```

**4. Custom Provider with Higher Priority** - Become the default:
```go
func (p *CustomProvider) DetectEnvironment() (priority int, available bool) {
    if os.Getenv("CUSTOM_LLM_KEY") != "" {
        return 200, true  // Higher than OpenAI's 100
    }
    return 0, false
}
```

> **Note**: There is no environment variable to change the priority order. To use a specific provider, use explicit selection or provider aliases in your code. See [AI Module README](../ai/README.md) for detailed examples.

---

## Telemetry Configuration

Configure OpenTelemetry-based observability (metrics and tracing).

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_TELEMETRY_ENABLED` | `false` | **Implemented** | Enable telemetry collection | [core/config.go:611](../core/config.go#L611) |
| `GOMIND_TELEMETRY_ENDPOINT` | (none) | **Implemented** | OTLP receiver endpoint (auto-enables telemetry) | [core/config.go:614](../core/config.go#L614) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (fallback) | **Implemented** | Standard OTEL endpoint variable (auto-enables telemetry) | [core/config.go:624](../core/config.go#L624) |
| `GOMIND_TELEMETRY_SERVICE_NAME` | Agent name | **Implemented** | Service name for traces/metrics | [core/config.go:635](../core/config.go#L635) |
| `OTEL_SERVICE_NAME` | (fallback) | **Implemented** | Standard OTEL service name | [core/config.go:637](../core/config.go#L637) |
| `GOMIND_TELEMETRY_PROVIDER` | `otel` | Struct Tag Only | Telemetry provider | [core/config.go:154](../core/config.go#L154) |
| `GOMIND_TELEMETRY_METRICS` | `true` | Struct Tag Only | Enable metrics collection | [core/config.go:157](../core/config.go#L157) |
| `GOMIND_TELEMETRY_TRACING` | `true` | Struct Tag Only | Enable distributed tracing | [core/config.go:158](../core/config.go#L158) |
| `GOMIND_TELEMETRY_SAMPLING_RATE` | `1.0` | Struct Tag Only | Trace sampling rate (0.0-1.0) | [core/config.go:159](../core/config.go#L159) |
| `GOMIND_TELEMETRY_INSECURE` | `true` | Struct Tag Only | Use insecure connection (no TLS) | [core/config.go:160](../core/config.go#L160) |

### Telemetry Logger Variables

These are used by the telemetry module's internal logger:

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_LOG_LEVEL` | `INFO` | **Implemented** | Log level for telemetry logger | [telemetry/logger.go:65](../telemetry/logger.go#L65) |
| `GOMIND_DEBUG` | `false` | **Implemented** | Enable debug logging | [telemetry/logger.go:71](../telemetry/logger.go#L71) |
| `TELEMETRY_DEBUG` | `false` | **Implemented** | Enable telemetry-specific debug | [telemetry/logger.go:72](../telemetry/logger.go#L72) |
| `GOMIND_LOG_FORMAT` | `text` (local) / `json` (K8s) | **Implemented** | Log format override | [telemetry/logger.go:81](../telemetry/logger.go#L81) |
| `KUBERNETES_SERVICE_HOST` | (auto) | **Implemented** | Auto-detect K8s for JSON logging | [telemetry/logger.go:77](../telemetry/logger.go#L77) |

### Example

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
export OTEL_SERVICE_NAME="weather-tool"
```

---

## Memory Configuration

Configure state storage for agents.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_MEMORY_PROVIDER` | `inmemory` | **Implemented** | Storage provider (inmemory, redis) | [core/config.go:644](../core/config.go#L644) |
| `GOMIND_MEMORY_REDIS_URL` | (from discovery) | **Implemented** | Redis URL for memory storage | [core/config.go:647](../core/config.go#L647) |
| `GOMIND_MEMORY_MAX_SIZE` | `1000` | Struct Tag Only | Maximum items in memory | [core/config.go:169](../core/config.go#L169) |
| `GOMIND_MEMORY_DEFAULT_TTL` | `1h` | Struct Tag Only | Default TTL for stored items | [core/config.go:170](../core/config.go#L170) |
| `GOMIND_MEMORY_CLEANUP_INTERVAL` | `10m` | Struct Tag Only | Interval for cleanup | [core/config.go:171](../core/config.go#L171) |

---

## Logging Configuration

Configure logging output format and level.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_LOG_LEVEL` | `info` | **Implemented** | Minimum log level (debug, info, warn, error) | [core/config.go:652](../core/config.go#L652) |
| `GOMIND_LOG_FORMAT` | `json` (K8s) / `text` (local) | **Implemented** | Output format | [core/config.go:655](../core/config.go#L655) |
| `GOMIND_LOG_OUTPUT` | `stdout` | Struct Tag Only | Output destination | [core/config.go:216](../core/config.go#L216) |
| `GOMIND_LOG_TIME_FORMAT` | RFC3339Nano | Struct Tag Only | Timestamp format | [core/config.go:217](../core/config.go#L217) |

---

## Development Configuration

Configure development-mode settings.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_DEV_MODE` | `false` | **Implemented** | Enable development mode (sets debug logging, text format) | [core/config.go:660](../core/config.go#L660) |
| `GOMIND_MOCK_AI` | `false` | **Implemented** | Use mock AI responses | [core/config.go:668](../core/config.go#L668) |
| `GOMIND_MOCK_DISCOVERY` | `false` | **Implemented** | Use in-memory mock discovery | [core/config.go:671](../core/config.go#L671) |
| `GOMIND_DEBUG` | `false` | **Implemented** | Enable debug logging | [core/config.go:674](../core/config.go#L674) |
| `GOMIND_PRETTY_LOGS` | `false` | Struct Tag Only | Enable human-readable logs | [core/config.go:230](../core/config.go#L230) |

### Effects of `GOMIND_DEV_MODE=true`

When enabled, automatically sets:
- `GOMIND_LOG_LEVEL` to `debug`
- `GOMIND_LOG_FORMAT` to `text`
- Pretty logs enabled

---

## Kubernetes Configuration

Kubernetes-specific settings. Most are auto-detected when running in K8s.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `KUBERNETES_SERVICE_HOST` | (auto) | **Implemented** | Auto-set by K8s, triggers K8s mode | [core/config.go:682](../core/config.go#L682) |
| `HOSTNAME` | (auto) | **Implemented** | Pod name, auto-set by K8s | [core/config.go:684](../core/config.go#L684) |
| `GOMIND_K8S_NAMESPACE` | (auto-detected) | **Implemented** | Pod namespace | [core/config.go:687](../core/config.go#L687) |
| `GOMIND_K8S_SERVICE_NAME` | Agent name | **Implemented** | Kubernetes service name | [core/config.go:696](../core/config.go#L696) |
| `GOMIND_K8S_SERVICE_PORT` | `80` | **Implemented** | Kubernetes service port | [core/config.go:699](../core/config.go#L699) |
| `GOMIND_K8S_POD_IP` | (auto) | **Implemented** | Pod IP address | [core/config.go:704](../core/config.go#L704) |
| `GOMIND_K8S_NODE_NAME` | (auto) | **Implemented** | Node name where pod is running | [core/config.go:707](../core/config.go#L707) |
| `GOMIND_K8S_SA_PATH` | `/var/run/secrets/kubernetes.io/serviceaccount` | Struct Tag Only | Service account mount path | [core/config.go:246](../core/config.go#L246) |
| `GOMIND_K8S_SERVICE_DISCOVERY` | `true` | Struct Tag Only | Enable K8s service discovery | [core/config.go:247](../core/config.go#L247) |
| `GOMIND_K8S_LEADER_ELECTION` | `false` | Struct Tag Only | Enable leader election | [core/config.go:248](../core/config.go#L248) |

### Kubernetes Deployment Example

```yaml
env:
  - name: GOMIND_AGENT_NAME
    value: "weather-tool"
  - name: GOMIND_K8S_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: GOMIND_K8S_POD_IP
    valueFrom:
      fieldRef:
        fieldPath: status.podIP
  - name: GOMIND_K8S_SERVICE_NAME
    value: "weather-tool"
  - name: GOMIND_K8S_SERVICE_PORT
    value: "8080"
```

---

## Orchestration Configuration

Configure the AI orchestrator for multi-agent coordination.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_ORCHESTRATION_TIMEOUT` | `60s` | **Implemented** | HTTP client timeout for tool/agent calls | [orchestration/executor.go:80](../orchestration/executor.go#L80) |
| `GOMIND_TIERED_RESOLUTION_ENABLED` | `true` | **Implemented** | Enable tiered capability resolution for LLM token optimization. Uses 2-phase approach to reduce tokens by 50-75%. | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_TIERED_MIN_TOOLS` | `20` | **Implemented** | Minimum tool count to trigger tiered resolution. Below this threshold, all tools are sent directly. Research-backed default. | [orchestration/tiered_capability_provider.go](../orchestration/tiered_capability_provider.go) |
| `GOMIND_CAPABILITY_SERVICE_URL` | (none) | **Implemented** | External capability service URL | [orchestration/capability_provider.go:83](../orchestration/capability_provider.go#L83) |
| `CAPABILITY_SERVICE_URL` | (fallback) | **Implemented** | Alternative capability service URL | [orchestration/capability_provider.go:81](../orchestration/capability_provider.go#L81) |
| `GOMIND_CAPABILITY_TOP_K` | `20` | **Implemented** | Number of capabilities to return | [orchestration/capability_provider.go:91](../orchestration/capability_provider.go#L91) |
| `GOMIND_CAPABILITY_THRESHOLD` | `0.7` | **Implemented** | Minimum similarity threshold | [orchestration/capability_provider.go:103](../orchestration/capability_provider.go#L103) |
| `GOMIND_PLAN_RETRY_ENABLED` | `true` | **Implemented** | Retry plan generation on JSON parse failures | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_PLAN_RETRY_MAX` | `2` | **Implemented** | Maximum retry attempts for plan parsing (0 = disabled) | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_VALIDATE_PAYLOADS` | `false` | **Implemented** | Enable schema validation for AI-generated payloads | [examples/agent-example/orchestration.go:670](../examples/agent-example/orchestration.go#L670) |
| `GOMIND_AGENT_MAX_RETRIES` | `2` | Example Only | Max retries for agent execution | [examples/agent-example/orchestration.go:157](../examples/agent-example/orchestration.go#L157) |
| `GOMIND_AGENT_USE_AI_CORRECTION` | `true` | Example Only | Enable AI-based parameter correction | [examples/agent-example/orchestration.go:162](../examples/agent-example/orchestration.go#L162) |
| `GOMIND_ORCHESTRATOR_MODE` | (none) | Example Only | Orchestrator mode selection | [examples/agent-with-orchestration/main.go:290](../examples/agent-with-orchestration/main.go#L290) |

### Tiered Capability Resolution

Tiered resolution is a research-backed optimization that reduces LLM token usage by 50-75% for deployments with 20+ tools. It works by first sending lightweight tool summaries to select relevant tools, then fetching full schemas only for selected tools.

```bash
# Tiered resolution is enabled by default
export GOMIND_TIERED_RESOLUTION_ENABLED=true

# Adjust threshold for when tiering kicks in (default: 20)
export GOMIND_TIERED_MIN_TOOLS=25

# Disable for small deployments (< 20 tools)
export GOMIND_TIERED_RESOLUTION_ENABLED=false
```

**When to use:**
- **< 20 tools**: Disable tiered resolution (overhead not worth it)
- **20-100 tools**: Use tiered resolution (default, 50-75% token savings)
- **100s+ tools**: Consider ServiceCapabilityProvider for semantic search

See [Tiered Capability Resolution Design](../orchestration/notes/TIERED_CAPABILITY_RESOLUTION.md) for detailed research and implementation.

### Plan Parse Retry

When LLMs generate execution plans, JSON parsing may fail due to:
- Arithmetic expressions in values (e.g., `"amount": 100 * price`)
- Malformed JSON syntax (trailing commas, missing quotes)
- Invalid JSON structures

The retry mechanism provides error feedback to the LLM, allowing it to correct its output:

```bash
# Disable retry (fail fast on parse errors)
export GOMIND_PLAN_RETRY_ENABLED=false

# Increase retry attempts (default: 2)
export GOMIND_PLAN_RETRY_MAX=3

# Disable by setting max retries to 0
export GOMIND_PLAN_RETRY_MAX=0
```

See [PLAN_GENERATION_RETRY.md](../orchestration/notes/PLAN_GENERATION_RETRY.md) for implementation details.

### Semantic Retry Configuration (Layer 4)

Semantic Retry is an advanced error recovery feature that uses LLM analysis to compute corrected parameters when standard error analysis cannot fix the issue. When a tool call fails (e.g., `amount: 0` instead of `amount: 46828.5`), the contextual re-resolver uses full execution context—including the user's original query and source data from dependent steps—to compute the correct value.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_SEMANTIC_RETRY_ENABLED` | `true` | **Implemented** | Enable Layer 4 semantic retry with LLM-based parameter re-computation | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS` | `2` | **Implemented** | Maximum semantic retry attempts per step (0 = disabled) | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS` | `true` | **Implemented** | Enable semantic retry for steps without dependencies (first steps, parallel steps) | [orchestration/interfaces.go](../orchestration/interfaces.go) |

**How Semantic Retry Works:**

1. **Error Detected**: Tool returns 4xx error (400, 404, 409, 422)
2. **Layer 3 Analysis**: ErrorAnalyzer determines it cannot fix the issue
3. **Layer 4 Activation**: ContextualReResolver receives full execution context:
   - User's original query (intent)
   - Source data from dependent steps (what to compute from)
   - Failed parameters and error message
4. **LLM Computation**: The LLM analyzes the context and computes corrected parameters
5. **Retry**: The step is retried with computed parameters

**Example scenario:**
```
User: "Sell 100 Tesla shares and convert proceeds to EUR"
Step 1 (stock-tool): Returns {price: 468.285}
Step 2 (currency-tool): Fails with "amount must be > 0" (amount: 0)

→ Layer 4 computes: amount = 100 × 468.285 = 46828.5
→ Retries currency conversion with corrected amount
```

**Disabling Semantic Retry:**

```bash
# Disable semantic retry entirely
export GOMIND_SEMANTIC_RETRY_ENABLED=false

# Or limit retry attempts
export GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS=1

# Disable only for independent steps (revert to old behavior)
export GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS=false
```

### Example

```bash
# For long-running AI workflows
export GOMIND_ORCHESTRATION_TIMEOUT=5m

# For capability service integration
export GOMIND_CAPABILITY_SERVICE_URL="http://capability-service:8080"
export GOMIND_CAPABILITY_TOP_K=30
export GOMIND_CAPABILITY_THRESHOLD=0.75

# For plan parse retry configuration
export GOMIND_PLAN_RETRY_ENABLED=true
export GOMIND_PLAN_RETRY_MAX=2

# For semantic retry configuration (Layer 4)
export GOMIND_SEMANTIC_RETRY_ENABLED=true
export GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS=2
export GOMIND_SEMANTIC_RETRY_INDEPENDENT_STEPS=true
```

---

## LLM Debug Configuration

Configure LLM debug payload storage for debugging orchestration issues. This feature captures full LLM request/response payloads to help diagnose planning failures, parse errors, and unexpected AI behavior.

> **Important**: This feature is **disabled by default** to minimize storage overhead. Enable only when debugging is needed.

### LLM Debug Variables

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_LLM_DEBUG_ENABLED` | `false` | **Implemented** | Enable LLM debug payload storage | [orchestration/interfaces.go](../orchestration/interfaces.go) |
| `GOMIND_LLM_DEBUG_TTL` | `24h` | **Implemented** | Retention period for successful debug records | [orchestration/redis_llm_debug_store.go](../orchestration/redis_llm_debug_store.go) |
| `GOMIND_LLM_DEBUG_ERROR_TTL` | `168h` (7 days) | **Implemented** | Retention period for error debug records (longer for troubleshooting) | [orchestration/redis_llm_debug_store.go](../orchestration/redis_llm_debug_store.go) |
| `GOMIND_LLM_DEBUG_REDIS_DB` | `7` | **Implemented** | Redis database number for debug storage (uses `core.RedisDBLLMDebug`) | [orchestration/redis_llm_debug_store.go](../orchestration/redis_llm_debug_store.go) |

### How It Works

When enabled, the orchestrator automatically captures:
- **Request payloads**: Full prompts sent to the LLM
- **Response payloads**: Complete LLM responses (parsed and raw)
- **Timing metadata**: Duration, timestamps, retry attempts
- **Error context**: Parse failures, validation errors with original content

### Example: Enable Debug Storage

```bash
# Enable LLM debug storage
export GOMIND_LLM_DEBUG_ENABLED=true

# Increase retention for debugging (optional)
export GOMIND_LLM_DEBUG_TTL=48h
export GOMIND_LLM_DEBUG_ERROR_TTL=168h
```

### Example: Kubernetes Deployment

```yaml
env:
  - name: GOMIND_LLM_DEBUG_ENABLED
    value: "true"
  - name: GOMIND_LLM_DEBUG_TTL
    value: "48h"
  - name: GOMIND_LLM_DEBUG_ERROR_TTL
    value: "168h"
```

### Viewing Debug Records

Use the Registry Viewer App to view captured debug records:
1. Navigate to the "LLM Debug" tab in the sidebar
2. Browse records by agent, timestamp, or status (success/error)
3. Inspect full request/response payloads for troubleshooting

---

## Async Task Configuration

Configure asynchronous task processing for long-running operations. The async task system enables the HTTP 202 + Polling pattern for operations that may take minutes to complete.

### Deployment Mode

| Variable | Default | Status | Description | Used In |
|----------|---------|--------|-------------|---------|
| `GOMIND_MODE` | (empty) | **Example Only** | Deployment mode: `api`, `worker`, or empty for embedded mode | [agent-with-async](../examples/agent-with-async/) |
| `WORKER_COUNT` | `5` | **Example Only** | Number of concurrent task workers | [agent-with-async](../examples/agent-with-async/) |

### Deployment Modes Explained

The `GOMIND_MODE` variable controls how the async agent is deployed:

| Mode | Description | Use Case |
|------|-------------|----------|
| (empty) | **Embedded mode** - API and workers in same process | Development, single-pod deployments |
| `api` | **API mode** - Only serves HTTP endpoints, enqueues tasks | Production API tier (scalable) |
| `worker` | **Worker mode** - Only processes tasks from queue | Production worker tier (scalable) |

### Example: Embedded Mode (Development)

```bash
# Single process handles both API and task processing
export GOMIND_AGENT_NAME="async-travel-agent"
export WORKER_COUNT=3
# GOMIND_MODE not set = embedded mode
```

### Example: Production Split Deployment

**API Pod:**
```yaml
env:
  - name: GOMIND_MODE
    value: "api"
  - name: GOMIND_AGENT_NAME
    value: "async-travel-agent"
  - name: REDIS_URL
    value: "redis://redis:6379"
```

**Worker Pod:**
```yaml
env:
  - name: GOMIND_MODE
    value: "worker"
  - name: GOMIND_AGENT_NAME
    value: "async-travel-agent"
  - name: WORKER_COUNT
    value: "5"
  - name: REDIS_URL
    value: "redis://redis:6379"
```

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                     Production Deployment                         │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────┐         ┌─────────────────┐                 │
│  │   API Pod       │         │   Worker Pod    │                 │
│  │ (GOMIND_MODE=   │         │ (GOMIND_MODE=   │                 │
│  │     api)        │         │    worker)      │                 │
│  │                 │         │                 │                 │
│  │ POST /api/v1/   │         │  ┌───────────┐  │                 │
│  │     tasks ──────┼────┬────┼─►│ Worker 1  │  │                 │
│  │ GET  /api/v1/   │    │    │  ├───────────┤  │                 │
│  │     tasks/:id   │    │    │  │ Worker 2  │  │                 │
│  └─────────────────┘    │    │  ├───────────┤  │                 │
│                         │    │  │ Worker N  │  │                 │
│                         │    │  └───────────┘  │                 │
│                         │    └─────────────────┘                 │
│                         │                                        │
│                    ┌────▼────┐                                   │
│                    │  Redis  │                                   │
│                    │ (Queue) │                                   │
│                    └─────────┘                                   │
└──────────────────────────────────────────────────────────────────┘
```

### Related Framework Types

The async task system uses these core framework types (no additional env vars required):

- `core.Task` - Task data structure with status, progress, result
- `core.TaskQueue` - Redis-backed task queue interface
- `core.TaskStore` - Redis-backed task state storage
- `core.TaskWorkerPool` - Worker pool implementation
- `orchestration.TaskAPIHandler` - HTTP API handler for task endpoints

See [Async Orchestration Guide](ASYNC_ORCHESTRATION_GUIDE.md) for detailed usage.

---

## Prompt Configuration

Configure LLM prompt customization for orchestration.

| Variable | Default | Status | Description | Source |
|----------|---------|--------|-------------|--------|
| `GOMIND_PROMPT_TEMPLATE_FILE` | (none) | **Implemented** | Path to custom prompt template file | [orchestration/prompt_config_env.go:22](../orchestration/prompt_config_env.go#L22) |
| `GOMIND_PROMPT_DOMAIN` | (none) | **Implemented** | Domain context (healthcare, finance, legal, retail) | [orchestration/prompt_config_env.go:27](../orchestration/prompt_config_env.go#L27) |
| `GOMIND_PROMPT_TYPE_RULES` | (none) | **Implemented** | JSON array of additional type rules | [orchestration/prompt_config_env.go:32](../orchestration/prompt_config_env.go#L32) |
| `GOMIND_PROMPT_CUSTOM_INSTRUCTIONS` | (none) | **Implemented** | JSON array of custom instructions | [orchestration/prompt_config_env.go:47](../orchestration/prompt_config_env.go#L47) |

### Example

```bash
export GOMIND_PROMPT_DOMAIN="healthcare"
export GOMIND_PROMPT_TYPE_RULES='[{"type_names":["uuid"],"json_type":"JSON strings","example":"\"abc-123\""}]'
export GOMIND_PROMPT_CUSTOM_INSTRUCTIONS='["Prefer local tools", "Minimize API calls"]'
```

---

## Example/Tool Specific Variables

These variables are used in example applications but are not part of the core framework.

### Common Example Variables

| Variable | Default | Description | Used In |
|----------|---------|-------------|---------|
| `PORT` | `8080` | HTTP server port | All examples |
| `NAMESPACE` | `default` | Kubernetes namespace | All examples |
| `APP_ENV` | `development` | Application environment | Most examples |
| `DEV_MODE` | `false` | Development mode flag | Most examples |

### Tool-Specific API Keys

| Variable | Description | Used In |
|----------|-------------|---------|
| `GNEWS_API_KEY` | GNews API key | [news-tool](../examples/news-tool/) |
| `FINNHUB_API_KEY` | Finnhub stock API key | [stock-market-tool](../examples/stock-market-tool/) |
| `WEATHER_API_KEY` | Weather API key | [tool-example](../examples/tool-example/) |
| `GROCERY_API_URL` | Grocery API base URL | [grocery-tool](../examples/grocery-tool/) |

### Multi-Provider Example Variables

| Variable | Default | Description | Used In |
|----------|---------|-------------|---------|
| `TOOL_PORT` | `8085` | Port for tool server | [ai-multi-provider](../examples/ai-multi-provider/) |
| `AGENT_PORT` | `8086` | Port for agent server | [ai-multi-provider](../examples/ai-multi-provider/) |

### Test/Mock Variables

| Variable | Default | Description | Used In |
|----------|---------|-------------|---------|
| `MOCK_DELAY_MS` | `0` | Simulated response delay in ms | [tests/fixtures/mock-ai](../tests/fixtures/mock-ai/) |
| `MOCK_ERROR_RATE` | `0` | Error rate for testing (0.0-1.0) | [tests/fixtures/mock-ai](../tests/fixtures/mock-ai/) |

---

## Quick Reference Table

### Essential Variables for Production (All Implemented)

```bash
# Core
export GOMIND_AGENT_NAME="my-agent"
export GOMIND_PORT=8080
export GOMIND_NAMESPACE="production"

# Discovery
export REDIS_URL="redis://redis:6379"

# AI (one of these)
export OPENAI_API_KEY="sk-..."
# or: GROQ_API_KEY, ANTHROPIC_API_KEY, etc.

# Telemetry
export OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
export OTEL_SERVICE_NAME="my-agent"

# Orchestration (for long-running AI workflows)
export GOMIND_ORCHESTRATION_TIMEOUT=5m
export GOMIND_HTTP_WRITE_TIMEOUT=5m
export GOMIND_HTTP_READ_TIMEOUT=5m
```

### Essential Variables for Development (All Implemented)

```bash
export GOMIND_DEV_MODE=true
export GOMIND_MOCK_DISCOVERY=true
export GOMIND_DEBUG=true
```

### LLM Debug Variables (for Troubleshooting)

```bash
# Enable debug payload storage (disabled by default)
export GOMIND_LLM_DEBUG_ENABLED=true
export GOMIND_LLM_DEBUG_TTL=24h           # Success record retention
export GOMIND_LLM_DEBUG_ERROR_TTL=168h    # Error record retention (7 days)
export GOMIND_LLM_DEBUG_REDIS_DB=7        # Redis database number
```

### Kubernetes ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gomind-config
data:
  GOMIND_AGENT_NAME: "research-agent"
  GOMIND_NAMESPACE: "production"
  GOMIND_LOG_LEVEL: "info"
  GOMIND_LOG_FORMAT: "json"
  GOMIND_DISCOVERY_CACHE: "true"
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.observability:4318"
```

### Kubernetes Secret Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gomind-secrets
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-..."
  REDIS_URL: "redis://:password@redis:6379"
```

---

## Configuration Priority

The framework applies configuration in this order (highest to lowest priority):

1. **Functional options** (code) - `WithName("my-agent")`
2. **Environment variables** - `GOMIND_AGENT_NAME`
3. **Default values** - Framework defaults

For environment variables with multiple names (e.g., `REDIS_URL` vs `GOMIND_REDIS_URL`):
- `GOMIND_REDIS_URL` takes precedence over `REDIS_URL`
- This allows framework-specific overrides while maintaining compatibility

---

## Variables Marked as "Struct Tag Only"

These variables are defined in Go struct tags with `env:"..."` annotations but are **not currently loaded** in the `LoadFromEnv()` function. They may work if you're using a reflection-based configuration loader, but the default `LoadFromEnv()` does not process them.

To use these variables, you would need to either:
1. Use programmatic configuration via functional options
2. Implement reflection-based struct tag parsing
3. Submit a PR to add explicit loading for these variables

---

## See Also

- [Core Module README](../core/README.md)
- [Orchestration README](../orchestration/README.md)
- [AI Module README](../ai/README.md)
- [Telemetry README](../telemetry/README.md)
- [Kubernetes Deployment Guide](k8s-service-fronted-discovery.md)
