# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Orchestration Module
- **Layer 4: Semantic Retry (Contextual Re-Resolution)** - LLM-based parameter computation when standard error analysis cannot fix the issue. Uses full execution trajectory including user query, source data from dependent steps, and error context to compute corrected parameters.
  - New `ContextualReResolver` component in `orchestration/contextual_re_resolver.go`
  - Environment variables: `GOMIND_SEMANTIC_RETRY_ENABLED` (default: true), `GOMIND_SEMANTIC_RETRY_MAX_ATTEMPTS` (default: 2)
  - Full observability with Jaeger span events and Prometheus metrics
  - 26 unit tests covering all scenarios

- **LLM Debug Payload Store** - Complete LLM request/response capture for production debugging, addressing Jaeger's payload truncation limitation.
  - Three storage implementations: `RedisLLMDebugStore` (production), `MemoryLLMDebugStore` (testing), `NoOpLLMDebugStore` (disabled/fallback)
  - 6 recording sites: `plan_generation`, `correction`, `synthesis`, `synthesis_streaming`, `micro_resolution`, `semantic_retry`
  - Three-layer resilience: built-in retry → optional circuit breaker → NoOp fallback
  - Provider tracking in `core.AIResponse` (openai, anthropic, gemini, bedrock)
  - Environment variables: `GOMIND_LLM_DEBUG_ENABLED` (default: false), `GOMIND_LLM_DEBUG_TTL`, `GOMIND_LLM_DEBUG_ERROR_TTL`, `GOMIND_LLM_DEBUG_REDIS_DB`
  - Factory options: `WithLLMDebug()`, `WithLLMDebugStore()`, `WithLLMDebugTTL()`, `WithLLMDebugErrorTTL()`
  - Dedicated Redis DB 7 (`core.RedisDBLLMDebug`) for isolation
  - 22 unit tests covering stores, propagation, shutdown, factory options
  - Design doc: `orchestration/notes/LLM_DEBUG_PAYLOAD_DESIGN.md`

### Changed
- Updated parameter resolution system from three-layer to **four-layer** architecture
- Error handling table now shows Layer 3 → Layer 4 flow for correctable errors

### Documentation
- Updated `docs/ENVIRONMENT_VARIABLES_GUIDE.md` with semantic retry configuration
- Updated `README.md` with semantic retry as a key selling point
- Updated `orchestration/README.md` with comprehensive Layer 4 documentation
- Updated `docs/INTELLIGENT_ERROR_HANDLING.md` with semantic retry section
- Updated `orchestration/ARCHITECTURE.md` with semantic retry design
- Updated `docs/framework_capabilities_guide.md` with intelligent error recovery section
- Moved `SEMANTIC_RETRY_DESIGN.md` to `orchestration/notes/` for reference
- Added Key Changes Summary to `orchestration/notes/LLM_DEBUG_PAYLOAD_DESIGN.md`
- Updated `orchestration/README.md` with LLM Debug Payload Store section
- Updated `orchestration/ARCHITECTURE.md` with LLM Debug Store architecture
- Updated `docs/API_REFERENCE.md` with LLM Debug Store interfaces
- Updated `docs/ENVIRONMENT_VARIABLES_GUIDE.md` with LLM Debug environment variables

---

## Previous Releases

See git history for changes prior to this changelog.
