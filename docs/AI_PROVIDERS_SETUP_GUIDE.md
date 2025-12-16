# AI Providers Setup Guide

Welcome to the GoMind AI providers guide! This document explains how to configure AI providers for your agents and tools, from simple single-provider setups to production-ready multi-provider failover systems. Think of this as your complete reference for doing AI integration the right way.

## Table of Contents

- [Why This Guide Exists](#why-this-guide-exists)
- [The Two Types of AI Clients](#the-two-types-of-ai-clients)
  - [Single Client: The Simple Path](#single-client-the-simple-path)
  - [Chain Client: Production-Grade Resilience](#chain-client-production-grade-resilience)
  - [When to Use Which](#when-to-use-which)
- [Provider Aliases: The Clean Way to Configure](#provider-aliases-the-clean-way-to-configure)
  - [What Problem Do They Solve?](#what-problem-do-they-solve)
  - [Complete Provider Alias Reference](#complete-provider-alias-reference)
  - [How Auto-Configuration Works](#how-auto-configuration-works)
- [Model Aliases: Portable Model Names](#model-aliases-portable-model-names)
  - [The Model Name Problem](#the-model-name-problem)
  - [Standard Model Aliases](#standard-model-aliases)
  - [Environment Variable Overrides](#environment-variable-overrides)
  - [How Model Resolution Works](#how-model-resolution-works)
- [Understanding Failover Behavior](#understanding-failover-behavior)
  - [How Chain Client Decides to Failover](#how-chain-client-decides-to-failover)
  - [Why Authentication Errors Allow Failover](#why-authentication-errors-allow-failover)
  - [The Options Isolation Problem (And How We Solved It)](#the-options-isolation-problem-and-how-we-solved-it)
- [Operational Scenarios](#operational-scenarios)
  - [Scenario 1: Local Development with Ollama](#scenario-1-local-development-with-ollama)
  - [Scenario 2: Development with Cloud Providers](#scenario-2-development-with-cloud-providers)
  - [Scenario 3: Staging Environment](#scenario-3-staging-environment)
  - [Scenario 4: Production with High Availability](#scenario-4-production-with-high-availability)
  - [Scenario 5: Cost-Optimized Production](#scenario-5-cost-optimized-production)
  - [Scenario 6: Privacy-First Deployment](#scenario-6-privacy-first-deployment)
  - [Scenario 7: Multi-Region Deployment](#scenario-7-multi-region-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
  - [Managing API Keys with Secrets](#managing-api-keys-with-secrets)
  - [Managing Model Aliases with ConfigMaps](#managing-model-aliases-with-configmaps)
  - [Same Image, Different Behavior](#same-image-different-behavior)
  - [Rolling Updates Without Downtime](#rolling-updates-without-downtime)
- [Troubleshooting Common Issues](#troubleshooting-common-issues)
  - [Issue 1: "No Providers Available" Error](#issue-1-no-providers-available-error)
  - [Issue 2: Wrong Model Being Used](#issue-2-wrong-model-being-used)
  - [Issue 3: Failover Not Working](#issue-3-failover-not-working)
  - [Issue 4: Model Not Found After Failover](#issue-4-model-not-found-after-failover)
  - [Issue 5: Unexpected Provider Being Used](#issue-5-unexpected-provider-being-used)
- [Debugging and Observability](#debugging-and-observability)
  - [Enabling Debug Logging](#enabling-debug-logging)
  - [Understanding AI Module Logs](#understanding-ai-module-logs)
  - [Tracing AI Requests in Jaeger](#tracing-ai-requests-in-jaeger)
- [Quick Reference](#quick-reference)
  - [Environment Variable Cheat Sheet](#environment-variable-cheat-sheet)
  - [Decision Tree: Which Client Type?](#decision-tree-which-client-type)
  - [Error Classification Reference](#error-classification-reference)

---

## Why This Guide Exists

In a production system, AI integration is rarely as simple as "call OpenAI and hope for the best." You need to handle:

- **Provider outages**: What happens when OpenAI is down?
- **Cost management**: How do you use cheaper models in development?
- **API key rotation**: How do you change keys without redeploying?
- **Regional routing**: How do you route traffic to regional endpoints (e.g., EU data residency)?

Without a clear strategy, you end up with:
- Hardcoded API keys (security nightmare)
- No failover (single point of failure)
- Different code paths for different environments (maintenance nightmare)

This guide ensures every GoMind deployment handles AI providers in a consistent, production-ready way.

---

## The Two Types of AI Clients

GoMind provides two ways to connect to AI providers. Understanding when to use each is the first decision you'll make.

### Single Client: The Simple Path

A Single Client connects directly to one provider. It's the simplest approach and works great when you don't need failover.

```go
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"
)

// The simplest possible setup - auto-detects from environment
client, err := ai.NewClient()

// Or explicitly choose a provider
client, err := ai.NewClient(
    ai.WithProviderAlias("openai.groq"),
    ai.WithModel("smart"),
)
```

**Behind the scenes**, when you call `ai.NewClient()` without arguments:
1. The module checks registered providers in priority order
2. Each provider's `DetectEnvironment()` method checks for API keys
3. The first available provider wins
4. You get a configured client without writing any configuration

**When Single Client makes sense:**
- Development and testing
- Simple applications where downtime is acceptable
- When you're locked into one provider (e.g., enterprise agreement)
- Background jobs where latency isn't critical

### Chain Client: Production-Grade Resilience

A Chain Client tries multiple providers in order until one succeeds. It's the production-ready approach for systems that can't afford downtime.

```go
import (
    "github.com/itsneelabh/gomind/ai"
    _ "github.com/itsneelabh/gomind/ai/providers/openai"
    _ "github.com/itsneelabh/gomind/ai/providers/anthropic"
)

// Create a chain with automatic failover
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic", "openai.groq"),
)

// Use it exactly like a single client
response, err := client.GenerateResponse(ctx, "Analyze this data...", nil)
```

**Behind the scenes**, when you make a request:
1. Chain Client tries Provider 1 (OpenAI)
2. If it fails with a retryable error, it tries Provider 2 (Anthropic)
3. If that fails too, it tries Provider 3 (Groq)
4. Returns the first successful response
5. If all fail, returns an error with details from the last attempt

**The key insight**: Each provider in the chain resolves model aliases independently. When you pass `Model: "smart"`, OpenAI resolves it to `o3`, Anthropic resolves it to `claude-sonnet-4-5`, and Groq resolves it to `llama-3.3-70b-versatile`. You don't need different code for different providers.

### When to Use Which

| Situation | Recommended | Why |
|-----------|-------------|-----|
| Local development | Single Client | Simpler, faster iteration |
| Staging/testing | Chain Client | Test failover behavior before prod |
| Production API | Chain Client | High availability is essential |
| Background processing | Either | Depends on retry strategy |
| Cost-sensitive batch jobs | Chain Client | Try cheap providers first |
| Compliance-restricted | Single Client | May not be allowed to send data to multiple providers |

**The golden rule**: If you'd lose money or users when AI is down, use Chain Client.

---

## Provider Aliases: The Clean Way to Configure

### What Problem Do They Solve?

Before provider aliases, configuring an OpenAI-compatible service looked like this:

```go
// The old, messy way
client, _ := ai.NewClient(
    ai.WithProvider("openai"),
    ai.WithBaseURL("https://api.groq.com/openai/v1"),  // Have to remember this
    ai.WithAPIKey(os.Getenv("GROQ_API_KEY")),          // Different env var
    ai.WithModel("llama-3.3-70b-versatile"),           // Provider-specific model
)
```

Every new provider meant remembering URLs, env vars, and model names. And if Groq changed their URL? You'd have to update every project.

**With provider aliases**, it's one line:

```go
// The clean way
client, _ := ai.NewClient(ai.WithProviderAlias("openai.groq"))
```

The framework knows that `openai.groq` means:
- Use `GROQ_API_KEY` for authentication
- Connect to `https://api.groq.com/openai/v1`
- Use Groq's model naming conventions

### Complete Provider Alias Reference

| Alias | Service | API Key Env Var | Base URL Env Var | Default URL |
|-------|---------|-----------------|------------------|-------------|
| `openai` | OpenAI | `OPENAI_API_KEY` | `OPENAI_BASE_URL` | `https://api.openai.com/v1` |
| `openai.deepseek` | DeepSeek | `DEEPSEEK_API_KEY` | `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` |
| `openai.groq` | Groq | `GROQ_API_KEY` | `GROQ_BASE_URL` | `https://api.groq.com/openai/v1` |
| `openai.xai` | xAI Grok | `XAI_API_KEY` | `XAI_BASE_URL` | `https://api.x.ai/v1` |
| `openai.qwen` | Alibaba Qwen | `QWEN_API_KEY` | `QWEN_BASE_URL` | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` |
| `openai.together` | Together AI | `TOGETHER_API_KEY` | `TOGETHER_BASE_URL` | `https://api.together.xyz/v1` |
| `openai.ollama` | Ollama (local) | _(none)_ | `OLLAMA_BASE_URL` | `http://localhost:11434/v1` |
| `anthropic` | Anthropic Claude | `ANTHROPIC_API_KEY` | _(N/A - native API)_ | _(native implementation)_ |
| `gemini` | Google Gemini | `GEMINI_API_KEY` | _(N/A - native API)_ | _(native implementation)_ |

### How Auto-Configuration Works

When you use a provider alias, the framework resolves configuration in this order:

1. **Explicit code configuration** (highest priority)
   ```go
   ai.WithAPIKey("sk-explicit-key")  // This wins
   ```

2. **Provider-specific environment variables**
   ```bash
   GROQ_API_KEY=gsk-from-env  # Used if no explicit key
   ```

3. **Base URL overrides** (for proxies, regional endpoints)
   ```bash
   GROQ_BASE_URL=https://eu.api.groq.com/openai/v1  # Optional override
   ```

4. **Default values** (built into the framework)
   - Default URL for the provider
   - Default model aliases

**Practical example**: Your code uses `openai.groq`. In production, you set:
```bash
GROQ_API_KEY=gsk-prod-key
GROQ_BASE_URL=https://ai-proxy.company.internal/groq  # Route through internal proxy
```

No code changes needed. The proxy gets all Groq traffic for logging/monitoring.

---

## Model Aliases: Portable Model Names

### The Model Name Problem

Every AI provider has different model names:
- OpenAI: `gpt-4.1-mini`, `o3`, `gpt-4.1`
- Anthropic: `claude-sonnet-4-5-20250929`, `claude-haiku-4-5-20251001`
- Groq: `llama-3.3-70b-versatile`, `llama-3.1-8b-instant`

If you hardcode model names, switching providers means changing code everywhere. And when providers release new models? More code changes.

**Model aliases solve this** by providing portable names:

```go
// This code works with ANY provider
client, _ := ai.NewClient(
    ai.WithProviderAlias("openai"),  // or "anthropic", or "openai.groq"
    ai.WithModel("smart"),           // Resolves to the right model for each provider
)
```

### Standard Model Aliases

| Alias | Purpose | OpenAI | Anthropic | Gemini | Groq | DeepSeek |
|-------|---------|--------|-----------|--------|------|----------|
| `default` | General use, balanced cost/quality | `gpt-4.1-mini` | `claude-sonnet-4-5` | `gemini-2.5-flash` | `llama-3.3-70b-versatile` | `deepseek-chat` |
| `fast` | Speed and cost optimized | `gpt-4.1-mini` | `claude-haiku-4-5` | `gemini-2.5-flash-lite` | `llama-3.1-8b-instant` | `deepseek-chat` |
| `smart` | Best reasoning quality | `o3` | `claude-sonnet-4-5` | `gemini-2.5-pro` | `llama-3.3-70b-versatile` | `deepseek-reasoner` |
| `premium` | Maximum intelligence | _(N/A)_ | `claude-opus-4-5` | `gemini-3-pro-preview` | _(N/A)_ | _(N/A)_ |
| `code` | Code generation | `o3` | `claude-sonnet-4-5` | `gemini-2.5-pro` | `llama-3.3-70b-versatile` | `deepseek-chat` |
| `vision` | Image understanding | `gpt-4.1` | `claude-sonnet-4-5` | `gemini-2.5-flash` | _(N/A)_ | _(N/A)_ |

> **Note**: The `premium` alias is only available for Anthropic and Gemini. For other providers, use `smart` for best reasoning quality.

### Environment Variable Overrides

Here's where it gets powerful. You can override any alias at runtime without changing code:

```bash
# Pattern: GOMIND_{PROVIDER}_MODEL_{ALIAS}=actual-model-name

# Override OpenAI's "smart" alias
export GOMIND_OPENAI_MODEL_SMART=gpt-4.1

# Override Anthropic's "fast" alias
export GOMIND_ANTHROPIC_MODEL_FAST=claude-haiku-4-5-20251001

# For OpenAI-compatible providers, strip the "openai." prefix
export GOMIND_GROQ_MODEL_DEFAULT=llama-3.1-8b-instant
export GOMIND_DEEPSEEK_MODEL_SMART=deepseek-reasoner
```

**Why this matters for ops**:
- **Cost control**: Set `GOMIND_OPENAI_MODEL_SMART=gpt-4.1-mini` in dev to save money
- **A/B testing**: Route traffic to different models without code changes
- **Rollback**: If a new model has issues, switch back via env var

### How Model Resolution Works

When you call `client.GenerateResponse(ctx, prompt, &core.AIOptions{Model: "smart"})`, here's the resolution order:

1. **Environment variable** (highest priority)
   ```bash
   GOMIND_OPENAI_MODEL_SMART=gpt-4.1  # If set, use this
   ```

2. **Hardcoded alias mapping**
   ```go
   modelAliases["openai"]["smart"] = "o3"  // Built-in default
   ```

3. **Pass-through** (lowest priority)
   ```go
   // If "smart" isn't recognized, use it literally
   // This lets you use explicit model names when needed
   ```

**Example flow**:
```
Request: Model="smart", Provider="openai"
  ↓
Check: GOMIND_OPENAI_MODEL_SMART env var?
  → Not set
  ↓
Check: modelAliases["openai"]["smart"]?
  → Returns "o3"
  ↓
Result: Use model "o3"
```

---

## Understanding Failover Behavior

### How Chain Client Decides to Failover

Not all errors should trigger failover. If your request is malformed, trying another provider won't help—you'll just get the same error three times.

Chain Client classifies errors into two categories:

**Errors that ALLOW failover** (tries next provider):

| Error Type | Examples | Why Failover Makes Sense |
|------------|----------|--------------------------|
| Authentication (401) | "invalid api key", "unauthorized" | Different providers have different keys |
| Server errors (5xx) | "internal server error" | Provider-specific outage |
| Rate limits (429) | "too many requests" | Limits are per-provider |
| Network errors | "connection timeout" | Might be routing/DNS issue |
| Not found (404) | "model not found" | Model might exist on another provider |

**Errors that STOP failover** (fails immediately):

| Error Type | Examples | Why Failover Won't Help |
|------------|----------|-------------------------|
| Bad request (400) | "invalid parameter" | Same input fails everywhere |
| Content policy | "content blocked" | Same content fails everywhere |
| Malformed input | "JSON parse error" | Structural issue in your code |

### Why Authentication Errors Allow Failover

This is a design decision that confuses some people. Traditionally, a 401 error means "your credentials are wrong, stop trying." But in a multi-provider chain, each provider has its own API key.

Consider this scenario:
```
Chain: ["openai", "anthropic", "openai.groq"]

Environment:
  OPENAI_API_KEY=sk-expired-key     # Oops, forgot to rotate
  ANTHROPIC_API_KEY=sk-ant-valid    # Works fine
  GROQ_API_KEY=gsk-valid            # Works fine
```

With traditional error handling:
```
Request → OpenAI → 401 "invalid key" → ERROR (stop)
User gets an error even though two providers would work
```

With Chain Client:
```
Request → OpenAI → 401 "invalid key" → Try next
        → Anthropic → Success!
User gets their response, ops gets alerted about OpenAI key
```

**The tradeoff**: You might make extra API calls before finding a working provider. But in production, uptime usually matters more than a few extra milliseconds.

### The Options Isolation Problem (And How We Solved It)

This is a subtle bug that caused real production issues before we fixed it. Here's what happened:

**The problem**: When Provider 1 fails, Provider 2 receives Provider 1's resolved model name.

```
Step 1: Request with Model="smart"
        Chain Client tries OpenAI
        OpenAI resolves "smart" → "o3"
        OpenAI fails with 401

Step 2: Chain Client tries Anthropic
        Options still has Model="o3" (from OpenAI!)
        Anthropic doesn't know "o3"
        Anthropic uses default model instead of resolving "smart"
```

**The fix**: Chain Client now clones options for each provider and resets the model to the original value:

```go
// Inside Chain Client (simplified)
originalModel := options.Model  // Save "smart"

for _, provider := range providers {
    providerOpts := cloneOptions(options)
    providerOpts.Model = originalModel  // Reset to "smart"

    response, err := provider.GenerateResponse(ctx, prompt, providerOpts)
    // Now each provider resolves "smart" independently
}
```

**What this means for you**: Model aliases work correctly during failover. You don't need to do anything special.

---

## Operational Scenarios

This section covers real-world deployment scenarios. Find the one that matches your situation.

### Scenario 1: Local Development with Ollama

**Goal**: Develop without cloud API costs, test offline

**Setup**:
```bash
# Install and start Ollama
ollama serve

# Pull a model
ollama pull llama3.2

# Your .env file
OLLAMA_BASE_URL=http://localhost:11434/v1
GOMIND_OLLAMA_MODEL_DEFAULT=llama3.2
GOMIND_OLLAMA_MODEL_SMART=llama3.2  # Use same model for all aliases locally
```

**Code**:
```go
// Single client is fine for local dev
client, err := ai.NewClient(ai.WithProviderAlias("openai.ollama"))
```

**Pro tip**: Create a `make dev` target that starts Ollama:
```makefile
dev:
    ollama serve &
    OLLAMA_BASE_URL=http://localhost:11434/v1 go run .
```

### Scenario 2: Development with Cloud Providers

**Goal**: Use cloud AI for development, but minimize costs

**Setup**:
```bash
# .env.development
GROQ_API_KEY=gsk-dev-key  # Groq has generous free tier

# Use smaller/cheaper models in dev
GOMIND_GROQ_MODEL_DEFAULT=llama-3.1-8b-instant
GOMIND_GROQ_MODEL_SMART=llama-3.1-8b-instant
```

**Code**:
```go
// Use Groq's free tier for development
client, err := ai.NewClient(ai.WithProviderAlias("openai.groq"))
```

**Why Groq for dev?**
- Free tier with 14,000 tokens/minute
- Ultra-fast inference (great for iteration)
- OpenAI-compatible API (easy to switch later)

### Scenario 3: Staging Environment

**Goal**: Mirror production setup, test failover, validate before deployment

**Setup**:
```bash
# .env.staging
# Use same providers as production
OPENAI_API_KEY=sk-staging-key
ANTHROPIC_API_KEY=sk-ant-staging-key
GROQ_API_KEY=gsk-staging-key

# But use mid-tier models to save costs
GOMIND_OPENAI_MODEL_SMART=gpt-4.1-mini
GOMIND_ANTHROPIC_MODEL_SMART=claude-haiku-4-5-20251001
```

**Code**:
```go
// Same chain as production
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic", "openai.groq"),
)
```

**Testing failover in staging**:
```bash
# Temporarily break OpenAI to test failover
export OPENAI_API_KEY=sk-invalid-key

# Make a request - should succeed via Anthropic
curl -X POST http://staging:8080/api/analyze

# Check logs for:
# {"message": "Provider failed, trying next", "provider": "openai", "error": "401"}
# {"message": "Request succeeded", "provider": "anthropic"}
```

### Scenario 4: Production with High Availability

**Goal**: Maximum uptime, automatic failover, best models

**Setup**:
```bash
# .env.production
OPENAI_API_KEY=sk-prod-key
ANTHROPIC_API_KEY=sk-ant-prod-key
GROQ_API_KEY=gsk-prod-key

# Use best models in production
GOMIND_OPENAI_MODEL_SMART=o3
GOMIND_ANTHROPIC_MODEL_SMART=claude-sonnet-4-5-20250929
GOMIND_GROQ_MODEL_SMART=llama-3.3-70b-versatile
```

**Code**:
```go
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic", "openai.groq"),
)
```

**Monitoring production failover**:
```bash
# Alert on consistent failover (indicates provider issues)
# Prometheus query:
rate(ai_chain_failover_total{from_provider="openai"}[5m]) > 0.1
```

### Scenario 5: Cost-Optimized Production

**Goal**: Minimize AI costs while maintaining quality

**Setup**:
```bash
# .env.production-cost-optimized
# Order providers by cost (cheapest first)
GROQ_API_KEY=gsk-key      # Free tier / very cheap
DEEPSEEK_API_KEY=sk-key   # Very affordable
OPENAI_API_KEY=sk-key     # Premium fallback

# Use smaller models where acceptable
GOMIND_GROQ_MODEL_DEFAULT=llama-3.1-8b-instant
GOMIND_DEEPSEEK_MODEL_DEFAULT=deepseek-chat
```

**Code**:
```go
// Cost-optimized chain: try cheapest first
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai.groq", "openai.deepseek", "openai"),
)
```

**Cost monitoring**:
```go
// Log token usage per request for cost analysis
response, err := client.GenerateResponse(ctx, prompt, nil)
logger.Info("Request completed", map[string]interface{}{
    "model":  response.Model,           // Which model was used
    "tokens": response.Usage.TotalTokens,
})
// Note: Provider tracking is available via telemetry spans (ai.chain.provider attribute)
```

### Scenario 6: Privacy-First Deployment

**Goal**: Keep sensitive data local, use cloud only as fallback

**Setup**:
```bash
# .env.production-privacy
# Local model is primary
OLLAMA_BASE_URL=http://gpu-server.internal:11434/v1

# Cloud fallback for when local is overloaded
OPENAI_API_KEY=sk-key

# Use capable local model
GOMIND_OLLAMA_MODEL_DEFAULT=llama3.2:70b
GOMIND_OLLAMA_MODEL_SMART=llama3.2:70b
```

**Code**:
```go
// Privacy-first: local → cloud
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai.ollama", "openai"),
)
```

**Privacy monitoring** (for compliance):
```go
// Track provider usage via telemetry for audit purposes
// The ai.chain.provider span attribute shows which provider handled each request
// You can query this in Jaeger/Prometheus:
//   rate(ai_chain_attempt{provider="openai",status="success"}[5m]) > 0
// This tells you when requests are being routed to cloud providers
```

### Scenario 7: Multi-Region Deployment

**Goal**: Route to nearest provider, handle regional outages

**Setup**:
```bash
# .env.production-us
OPENAI_API_KEY=sk-us-key
OPENAI_BASE_URL=https://api.openai.com/v1  # US endpoint

# .env.production-eu
OPENAI_API_KEY=sk-eu-key
OPENAI_BASE_URL=https://eu.api.openai.com/v1  # EU endpoint (if available)
DEEPSEEK_BASE_URL=https://eu.api.deepseek.com  # EU regional
```

**Code** (same for all regions):
```go
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "openai.deepseek", "openai.groq"),
)
```

**The key insight**: Same code, different environment variables per region. Kubernetes handles the routing.

---

## Kubernetes Deployment

### Managing API Keys with Secrets

Never put API keys in ConfigMaps or environment variables directly. Use Kubernetes Secrets:

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-api-keys
  namespace: production
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-prod-..."
  ANTHROPIC_API_KEY: "sk-ant-prod-..."
  GROQ_API_KEY: "gsk-prod-..."
```

**Apply it**:
```bash
kubectl apply -f secrets.yaml
```

**Reference in deployment**:
```yaml
spec:
  containers:
  - name: app
    envFrom:
    - secretRef:
        name: ai-api-keys
```

### Managing Model Aliases with ConfigMaps

Model aliases aren't secrets—they can go in ConfigMaps:

```yaml
# configmap-dev.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-model-config
  namespace: development
data:
  GOMIND_OPENAI_MODEL_SMART: "gpt-4.1-mini"
  GOMIND_ANTHROPIC_MODEL_SMART: "claude-haiku-4-5-20251001"
  GOMIND_GROQ_MODEL_DEFAULT: "llama-3.1-8b-instant"

---
# configmap-prod.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-model-config
  namespace: production
data:
  GOMIND_OPENAI_MODEL_SMART: "o3"
  GOMIND_ANTHROPIC_MODEL_SMART: "claude-sonnet-4-5-20250929"
  GOMIND_GROQ_MODEL_DEFAULT: "llama-3.3-70b-versatile"
```

### Same Image, Different Behavior

The power of this setup: **one container image works in all environments**.

```yaml
# deployment.yaml (same for dev, staging, prod)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-ai-service
spec:
  template:
    spec:
      containers:
      - name: app
        image: my-registry/my-ai-service:v1.2.3  # Same image everywhere
        envFrom:
        - secretRef:
            name: ai-api-keys      # Different per namespace
        - configMapRef:
            name: ai-model-config  # Different per namespace
```

**Promotion workflow**:
```bash
# Dev → Staging (same image, different config)
kubectl apply -f configmap-staging.yaml -n staging

# Staging → Prod (same image, different config)
kubectl apply -f configmap-prod.yaml -n production
```

### Rolling Updates Without Downtime

When you need to change models or API keys:

```bash
# Update the ConfigMap
kubectl edit configmap ai-model-config -n production
# Change GOMIND_OPENAI_MODEL_SMART from "o3" to "gpt-4.1"

# Trigger rolling restart
kubectl rollout restart deployment/my-ai-service -n production

# Watch the rollout
kubectl rollout status deployment/my-ai-service -n production
```

Pods restart one at a time, picking up the new environment variables. Zero downtime.

---

## Troubleshooting Common Issues

### Issue 1: "No Providers Available" Error

**Symptom**: `NewChainClient` returns "no providers could be initialized"

**Cause**: None of the providers in your chain have valid API keys configured.

**Diagnosis**:
```bash
# Check which env vars are set
echo "OPENAI_API_KEY: ${OPENAI_API_KEY:-(not set)}"
echo "ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-(not set)}"
echo "GROQ_API_KEY: ${GROQ_API_KEY:-(not set)}"
```

**Fix**: Set at least one API key:
```bash
export OPENAI_API_KEY=sk-your-key
```

**In Kubernetes**:
```bash
# Check if secret is mounted
kubectl exec -it pod/my-ai-service-xxx -- env | grep API_KEY
```

### Issue 2: Wrong Model Being Used

**Symptom**: Logs show `gpt-4.1-mini` but you expected `o3`

**Cause**: Environment variable override is taking precedence

**Diagnosis**:
```bash
# Check for overrides
env | grep GOMIND_

# Look for:
GOMIND_OPENAI_MODEL_SMART=gpt-4.1-mini  # This overrides the default!
```

**Fix**: Clear the override or set it to the model you want:
```bash
unset GOMIND_OPENAI_MODEL_SMART
# or
export GOMIND_OPENAI_MODEL_SMART=o3
```

### Issue 3: Failover Not Working

**Symptom**: Request fails immediately instead of trying next provider

**Cause**: Error is classified as "client error" (non-retryable)

**Diagnosis**: Check logs for error classification:
```json
{"message": "Provider failed", "error": "bad request: invalid prompt", "is_client_error": true}
```

**Understanding**: `is_client_error: true` means the error is in your request, not the provider. Failover won't help because the same request will fail everywhere.

**Fix**: Fix the underlying request issue (malformed JSON, invalid parameters, etc.)

### Issue 4: Model Not Found After Failover

**Symptom**: First provider fails, second provider says "model not found"

**Cause**: You might be running an old version with the options mutation bug

**Diagnosis**: Check the logs:
```json
{"provider": "anthropic", "model": "o3"}  // Anthropic shouldn't see "o3"!
```

**Fix**: Update to the latest GoMind version. The options cloning fix was added in December 2025.

### Issue 5: Unexpected Provider Being Used

**Symptom**: You expected OpenAI but Groq handled the request

**Cause**: Auto-detection picked a different provider based on available env vars

**Diagnosis**:
```bash
# If OPENAI_API_KEY isn't set but GROQ_API_KEY is...
# Auto-detection will use Groq

echo "OPENAI_API_KEY: ${OPENAI_API_KEY:-(not set)}"
echo "GROQ_API_KEY: ${GROQ_API_KEY:-(not set)}"
```

**Fix**: Either set the expected API key, or use explicit provider alias:
```go
// Force OpenAI specifically
client, err := ai.NewClient(ai.WithProviderAlias("openai"))
```

---

## Debugging and Observability

### Enabling Debug Logging

For detailed provider resolution logs, set the debug environment variable:

```bash
# Enable debug logging for all GoMind components
export GOMIND_DEBUG=true

# Or set log level directly
export GOMIND_LOG_LEVEL=debug
```

Alternatively, use a custom logger with your chain client:

```go
import "github.com/itsneelabh/gomind/core"

// Create a production logger with debug config
logger := core.NewProductionLogger(
    core.LoggingConfig{Level: "debug", Format: "json"},
    core.DevelopmentConfig{DebugLogging: true},
    "my-service",
)

// Pass to chain client
client, err := ai.NewChainClient(
    ai.WithProviderChain("openai", "anthropic"),
    ai.WithChainLogger(logger),
)
```

### Understanding AI Module Logs

AI module logs use the component prefix `framework/ai`. Here's what to look for:

**Successful request**:
```json
{
  "component": "framework/ai",
  "level": "INFO",
  "message": "AI request completed",
  "operation": "ai_request_success",
  "provider": "openai",
  "model": "o3",
  "prompt_tokens": 150,
  "completion_tokens": 200,
  "duration_ms": 1250
}
```

**Failover event**:
```json
{
  "component": "framework/ai",
  "level": "WARN",
  "message": "Provider failed, trying next",
  "provider": "openai",
  "error": "401 unauthorized",
  "next_provider": "anthropic"
}
```

**All providers exhausted**:
```json
{
  "component": "framework/ai",
  "level": "ERROR",
  "message": "All providers failed",
  "providers_tried": ["openai", "anthropic", "openai.groq"],
  "last_error": "rate limit exceeded"
}
```

### Tracing AI Requests in Jaeger

If you have telemetry enabled, AI requests create spans:

1. Open Jaeger: `http://localhost:16686`
2. Select your service
3. Find traces with `ai.generate_response` spans
4. Expand to see:
   - `ai.provider`: Which provider handled the request
   - `ai.model`: Resolved model name
   - `ai.prompt_tokens`, `ai.completion_tokens`: Token usage
   - `ai.attempt` spans: Each provider attempt during failover

---

## Quick Reference

### Environment Variable Cheat Sheet

**API Keys**:
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
DEEPSEEK_API_KEY=sk-...
GROQ_API_KEY=gsk-...
XAI_API_KEY=xai-...
QWEN_API_KEY=...
TOGETHER_API_KEY=...
```

**Base URL Overrides**:
```bash
OPENAI_BASE_URL=https://...
DEEPSEEK_BASE_URL=https://...
GROQ_BASE_URL=https://...
OLLAMA_BASE_URL=http://...
```

**Model Alias Overrides**:
```bash
# Pattern: GOMIND_{PROVIDER}_MODEL_{ALIAS}
GOMIND_OPENAI_MODEL_SMART=gpt-4.1
GOMIND_ANTHROPIC_MODEL_FAST=claude-haiku-4-5-20251001
GOMIND_GROQ_MODEL_DEFAULT=llama-3.3-70b-versatile
# For openai.deepseek, strip prefix → GOMIND_DEEPSEEK_MODEL_*
```

### Decision Tree: Which Client Type?

```
Do you need 99.9%+ uptime for AI features?
├── YES → Use Chain Client
│         └── How many providers in your chain?
│             ├── 2 providers → Good for most cases
│             └── 3+ providers → Maximum resilience
│
└── NO → Use Single Client
         └── Is cost a concern?
             ├── YES → Use openai.groq (free tier)
             └── NO → Use your preferred provider
```

### Error Classification Reference

| Error Pattern | Failover? | Rationale |
|---------------|-----------|-----------|
| `401`, `authentication`, `unauthorized` | Yes | Different keys per provider |
| `api key`, `invalid key` | Yes | Different keys per provider |
| `5xx`, `server error`, `bad gateway` | Yes | Provider outage |
| `timeout`, `connection failed` | Yes | Network issue |
| `429`, `rate limit` | Yes | Per-provider limits |
| `404`, `not found` | Yes | Model might exist elsewhere |
| `400`, `bad request` | No | Same input fails everywhere |
| `invalid parameter`, `malformed` | No | Fix your request |
| `content policy`, `blocked` | No | Same content fails everywhere |

---

## See Also

- **[ai/README.md](../ai/README.md)** - AI module overview and quick start
- **[ai/ARCHITECTURE.md](../ai/ARCHITECTURE.md)** - Technical architecture details
- **[ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md](../ai/MODEL_ALIAS_CROSS_PROVIDER_PROPOSAL.md)** - Implementation details for model aliases and bug fixes
- **[LOGGING_IMPLEMENTATION_GUIDE.md](./LOGGING_IMPLEMENTATION_GUIDE.md)** - Logging patterns including AI module logging
- **[DISTRIBUTED_TRACING_GUIDE.md](./DISTRIBUTED_TRACING_GUIDE.md)** - Tracing AI requests in Jaeger
