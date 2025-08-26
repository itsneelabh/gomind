## Kubernetes Service–Fronted Discovery (Single Registry Entry per Agent)

Date: 2025-08-13

This document specifies how discovery works when each agent is deployed as a Kubernetes Deployment and fronted by a Service that load balances across replicas.

### Why this matters
- Callers should route to the Service, not individual pod IPs.
- Autoscaling should not multiply registry entries; there should be one logical registry entry per agent capability, regardless of replica count.

### Defaults at a glance (sensible out-of-the-box)
- Registration scope: service (lock-free, idempotent)
- Address: http://AGENT_SERVICE_NAME:8080 (defaults: SERVICE_PORT=8080, AGENT_SERVICE_NAME=AGENT_NAME)
- Redis keys: one agents:<agent_name>, capability sets contain agent names
- Cache: enabled; refresh every 15s; independent of heartbeat
- Startup policy: non-strict readiness by default; liveness independent of Redis
- Fallbacks: persisted snapshot optional; otherwise wait for Redis

### Registration scopes
1) Service-scoped (recommended in K8s)
   - Single registry entry per logical agent name.
   - Address is the Service DNS name and port.
   - Multiple pods sit behind the Service; replicas do not create extra entries.

2) Pod-scoped (optional)
   - One entry per pod/instance (address = pod IP, port = containerPort).
   - Capability indices point to instance IDs; callers can choose an instance directly.

### Configuration (minimal)
Required:
- AGENT_NAME
- REDIS_URL (e.g., redis://redis-service:6379)

Optional (defaults in parentheses):
- AGENT_SERVICE_NAME (defaults to AGENT_NAME)
- SERVICE_PORT (8080)
- DISCOVERY_REGISTRATION_SCOPE (service)
- DISCOVERY_STRICT_STARTUP (false)
- DISCOVERY_CACHE_PERSIST_ENABLED (false)

### Redis key model
- Service scope (recommended, lock-free):
  - agents:<agent_name> → JSON AgentRegistration {Name, Address: "http://<AGENT_SERVICE_NAME>:<SERVICE_PORT>", Port, Capabilities, Status, KubernetesMetadata}
  - capabilities:<capability_name> → Set of agent names that provide it
  - Note: Any pod may write/refresh these keys; content is identical → last-write-wins is benign.

- Pod scope:
  - agents:<agent_id> → JSON AgentRegistration {ID, Name, Address: "<POD_IP>", Port, ...}
  - capabilities:<capability_name> → Set of agent IDs

### Lock-free, idempotent service-scoped writes (recommended)
- Every pod writes the same service-scoped record and SADDs the agent name to capability sets.
- Heartbeat: any pod may refresh TTLs on agents:<agent_name> and capabilities:<capability> keys.
- Benefits: no Redis locks or leader election; resilient (as long as one pod is alive, TTLs stay fresh); simple to operate.

Alternative (optional): leader election
- If stricter write minimization is desired, a Redis lock (agents:<agent_name>:lock) can be used so only one pod refreshes. The rest of the design remains unchanged.

### Discovery and calling flow (service scope)
1) Caller FindCapability("cap") → returns agent names that provide it.
2) Caller FindAgent("my-agent") → returns AgentRegistration with Address=http://my-agent:8080.
3) HTTP traffic goes via the Service and load-balances across ready pods.

### Autoscaling behavior
- Scaling replicas up/down does not change registry cardinality in service scope.
- New pods write idempotently; the registry remains stable.
- Readiness probes ensure only ready pods receive traffic via the Service.

### Step-by-step (service scope)
1. Pod starts; framework discovers capabilities.
2. Pod writes agents:<agent_name> with Address=Service DNS and sets TTL (idempotent).
3. Pod SADDs agent_name into capabilities:<capability_name> and sets/refreshes TTLs.
4. Heartbeat loop: any pod refreshes both agent record and capability indices.
5. Callers query capabilities, resolve the agent name, then call the Service address.

### Step-by-step (pod scope)
1. Each pod registers agents:<agent_id> with Address=POD_IP:PORT and TTL.
2. Each pod SADDs its agent_id to capabilities:<capability_name> with TTL.
3. Heartbeat refresh per pod updates its agent record TTL.
4. Callers query capabilities, select an instance, and call directly.

### Migration strategy
- Default to service scope in K8s (DISCOVERY_REGISTRATION_SCOPE=service).
- Keep pod scope for local/dev and advanced routing.
- The framework can detect scope and return identifiers accordingly (names vs IDs).

### Minimal K8s manifest hints
- Downward API env:
  - name: KUBERNETES_NAMESPACE; valueFrom: fieldRef: fieldPath: metadata.namespace
  - name: POD_IP; valueFrom: fieldRef: fieldPath: status.podIP
- Service name and port:
  - name: AGENT_SERVICE_NAME; value: "my-agent"
  - name: SERVICE_PORT; value: "8080"
- Scope:
  - name: DISCOVERY_REGISTRATION_SCOPE; value: "service"

### Notes
- This design avoids registry churn on HPA events and provides stable discovery endpoints.
- A K8s-native alternative is to watch EndpointSlices and write a single registration based on Service readiness; the lock-free Redis approach works without extra K8s API permissions.

---

## Quick starts

### Directed agent (default)
Set:
- AGENT_NAME, REDIS_URL
Optional:
- AGENT_SERVICE_NAME, SERVICE_PORT

Behavior: starts immediately, registers when Redis is up, serves via Service; keeps working using cache if Redis goes down.

### Autonomous agent (recommended flags)
Set:
- AGENT_NAME, REDIS_URL
Optional:
- DISCOVERY_STRICT_STARTUP=true (stays NotReady until a catalog exists)
- DISCOVERY_CACHE_PERSIST_ENABLED=true (boot from last-good snapshot if Redis is down)

Behavior: gates readiness on having a catalog; once available, routes autonomously via Service.

---

## LLM-assisted autonomous discovery and local registry cache

This pattern lets agents consult an LLM with a catalog of available agents and capabilities to decide which agent to call, while keeping a local cache to reduce Redis load and add resilience.

### Goals
- Provide an up-to-date local snapshot of the registry for prompt construction.
- Minimize Redis round-trips and tolerate transient Redis outages.
- Keep prompts compact and informative within token budgets.

### Local cache design
- In-memory snapshot per agent process with periodic refresh.
- Backed by simple structs and a JSON export for LLM prompts.
- Config:
  - DISCOVERY_CACHE_ENABLED=true|false (default: true)
  - DISCOVERY_CACHE_REFRESH_INTERVAL=15s (default)
  - DISCOVERY_CACHE_TTL=2m (stale cutoff when Redis unreachable)
  - DISCOVERY_CACHE_MAX_AGENTS=1000, DISCOVERY_CACHE_MAX_CAPS=5000 (guard rails)

#### Snapshot shape (JSON example)
{
  "version": 1,
  "generated_at": "2025-08-13T12:34:56Z",
  "environment": "prod",
  "agents": [
    {
      "name": "analysis-agent",
      "address": "http://analysis-agent:8080",
      "port": 8080,
      "capabilities": [
        {"name": "financial-analysis", "description": "Analyze portfolios", "domain": "finance"},
        {"name": "risk-assessment", "description": "Assess risk", "domain": "finance"}
      ],
      "status": "healthy",
      "k8s": {"namespace": "ai-agents"}
    }
  ]
}

### Refresh strategy
- Periodic full refresh (default):
  - Scan capabilities:* to get all capability names; for each, read set members (service-scope: agent names) and hydrate agents:<name>.
  - Merge into the local snapshot; evict entries older than DISCOVERY_CACHE_TTL when Redis is unreachable.
- Optional incremental:
  - Track last refresh time; only fetch recently touched keys with SCAN + pattern filters, or
  - Use Redis keyspace notifications (Ex/S$) if enabled to subscribe to SADD/DEL/SET events (ops dependent).

### LLM-assisted routing flow
1) On demand, build a compact catalog from the local snapshot (top-N capabilities or filtered by domain/business value).
2) Send prompt to the LLM describing the task and including the catalog (or a summarized form).
3) Parse the LLM response to select an agent name and capability.
4) Resolve the agent via local snapshot (fallback to Redis if missing) and call its Service endpoint.

#### Prompt composition tips
- Include: task, constraints (latency/cost), and a shortlist of candidate agents with 1–2 line descriptions per capability.
- Keep the catalog bounded (e.g., top 50 agents or top 200 capabilities) to fit token limits.
- Optionally pre-summarize capabilities and strip rarely used fields.

### Failure modes and fallbacks
- If Redis is down: serve from local snapshot up to DISCOVERY_CACHE_TTL; warn/log when stale.
- If snapshot empty: attempt a direct Redis fetch for the requested capability; if still empty, return a structured “no route found.”
- Cache poisoning risk is low as content is sourced from your own Redis; consider signature/version fields if needed.

### Security and privacy
- Don’t include secrets or internal-only metadata in the LLM catalog.
- Consider redacting sensitive metadata and use role-based prompt templates.

### Minimal config for LLM catalogs
- DISCOVERY_CACHE_ENABLED=true (default)
- DISCOVERY_CACHE_REFRESH_INTERVAL=15s (default)
- LLM_CATALOG_MAX_AGENTS=50 (optional prompt size control)
- LLM_PROMPT_TEMPLATE=/app/config/llm_prompt.tmpl (optional)

---

## Appendix: Advanced operations

### Retry, backoff, and circuit-breaker (Redis resilience)

Design goals:
- Keep serving from the last good local snapshot while Redis is down (do not clear the cache).
- Avoid hot loops and thundering herds using jittered backoff and a simple circuit breaker.
- Run cache refresh on its own schedule, independent from heartbeat.

Mechanics:
- Refresher runs every DISCOVERY_CACHE_REFRESH_INTERVAL (e.g., 15s). On success, swaps in a new snapshot; on failure, leaves the existing snapshot intact.
- Jittered backoff on consecutive failures: 1s → 2s → 4s … capped at DISCOVERY_CACHE_BACKOFF_MAX (e.g., 60s). Reset on first success.
- Circuit breaker: after DISCOVERY_CACHE_CB_THRESHOLD consecutive failures (e.g., 5), pause refresh attempts for DISCOVERY_CACHE_CB_COOLDOWN (e.g., 2m). While open, serve from cache only.
- Read path fallback: FindCapability/FindAgent return from snapshot on Redis errors.
- Staleness: never hard-fail reads due to age; emit warnings/metrics if snapshot older than DISCOVERY_CACHE_WARN_STALE (e.g., 10m).

Suggested config knobs:
- DISCOVERY_CACHE_BACKOFF_INITIAL=1s
- DISCOVERY_CACHE_BACKOFF_MAX=60s
- DISCOVERY_CACHE_CB_THRESHOLD=5
- DISCOVERY_CACHE_CB_COOLDOWN=2m
- DISCOVERY_CACHE_WARN_STALE=10m

Optional persistence:
- Persist snapshot to disk (JSON) at shutdown and load at startup to avoid a cold empty cache:
  - DISCOVERY_CACHE_PERSIST_PATH=/data/discovery_snapshot.json
  - DISCOVERY_CACHE_PERSIST_ENABLED=true
  - If Redis is down at startup and a snapshot exists, load it to become immediately functional.

---

### Startup behavior when Redis is down (readiness vs. liveness)

Recommendation: allow pods to start and stay alive (don’t crash-loop), but control readiness based on policy.

Policies:
1) Non-strict (default):
   - Pod starts and serves HTTP.
   - Ready if app is healthy, regardless of Redis connectivity.
   - Discovery uses local snapshot; if empty on cold start, routing may be limited until first successful refresh.
   - Use for high availability and tolerant systems.

2) Strict readiness:
   - Pod remains NotReady until either:
     - Redis is reachable (first successful refresh), or
     - A persisted snapshot was loaded and is non-empty (configurable).
   - Enables strong guarantees that discovery is usable before receiving traffic.

Probe guidance:
- LivenessProbe: should not depend on Redis; check process health and internal components only. Keeps pod running during outages.
- ReadinessProbe: can reflect the chosen policy (strict or non-strict). In strict mode, gate on (redis_ok || snapshot_loaded_not_empty).

Config knobs:
- DISCOVERY_STRICT_STARTUP=true|false (default: false)
- DISCOVERY_REQUIRE_SNAPSHOT_AT_STARTUP=true|false (default: false)
- DISCOVERY_MIN_SNAPSHOT_SIZE=1 (optional, to avoid being ready with an empty snapshot)

Rationale:
- K8s best practice favors readiness-gating over liveness failures for external dependencies. This avoids unnecessary restarts and allows fast recovery once Redis returns.

---

### Bootstrapping and catalog fallback sources (order of precedence)

1) Redis (primary)
- Populate the in-memory snapshot when available (service-scoped keys recommended).

2) Persisted local snapshot (best for cold starts during outages)
- Load last-good JSON snapshot from DISCOVERY_CACHE_PERSIST_PATH if Redis is unavailable.
- Mark age and serve immediately; refresh opportunistically when Redis recovers.

3) Catalog Service (optional)
- HTTP endpoint that serves the current catalog from a DB/ConfigMap.
- Useful as cross-cluster or Redis-independent source of truth.

4) Kubernetes API fallback
- List Services by label selectors (e.g., gomind.agent=true) and derive capabilities from annotations/labels.
- Requires read-only RBAC; works when Redis is down.

5) Seed list (ConfigMap/Env)
- Minimal static list of critical agents and capabilities to unblock autonomy.

6) Peer sync
- Query known peer agents for their /catalog and merge responses.

Behavior:
- If none are available and the snapshot is empty, autonomous agents should remain NotReady (strict mode) and queue work with backoff until a source yields data.

---

### Degraded autonomous operation

- Queue and retry
  - If the catalog is empty, queue autonomous tasks and retry after backoff; emit metrics and alerts if backlog grows.

- Scoped autonomy
  - Allow operation within the agent’s own capabilities and seeded peers; skip cross-domain orchestration until a catalog exists.

- Partial prompts
  - Build LLM prompts from the limited seed list (self + seeds), then re-evaluate once the full catalog is loaded.

---

### Operator visibility and SLOs

- Status fields (exposed via /health or /status)
  - registry_connected: bool
  - snapshot_loaded: bool
  - snapshot_age: duration
  - catalog_size: agents_total, capabilities_total

- Metrics
  - discovery_refresh_success_total, discovery_refresh_failure_total
  - discovery_refresh_consecutive_failures
  - discovery_snapshot_age_seconds
  - discovery_catalog_agents_total, discovery_catalog_capabilities_total
  - discovery_circuit_breaker_open (0/1)

- Logs & alerts
  - Warn when snapshot age exceeds DISCOVERY_CACHE_WARN_STALE
  - Alert when circuit breaker is open for > X minutes

---

### Suggested configuration set (cheat sheet)

- DISCOVERY_REGISTRATION_SCOPE=service
- AGENT_NAME, AGENT_SERVICE_NAME, SERVICE_PORT
- DISCOVERY_CACHE_ENABLED=true
- DISCOVERY_CACHE_REFRESH_INTERVAL=15s
- DISCOVERY_CACHE_BACKOFF_INITIAL=1s
- DISCOVERY_CACHE_BACKOFF_MAX=60s
- DISCOVERY_CACHE_CB_THRESHOLD=5
- DISCOVERY_CACHE_CB_COOLDOWN=2m
- DISCOVERY_CACHE_WARN_STALE=10m
- DISCOVERY_CACHE_PERSIST_ENABLED=true
- DISCOVERY_CACHE_PERSIST_PATH=/data/discovery_snapshot.json
- DISCOVERY_STRICT_STARTUP=true (for autonomous agents)
- DISCOVERY_REQUIRE_SNAPSHOT_AT_STARTUP=true (strict readiness until data available)
- CATALOG_URL=https://catalog-service.svc/agents (optional)
- K8S_FALLBACK_ENABLED=true (optional)
- SEED_SERVICES=analysis-agent:8080,chat-agent:8080 (optional)
- KNOWN_PEERS=coordinator-agent:8080 (optional)
