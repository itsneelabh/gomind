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

---

## Previous Releases

See git history for changes prior to this changelog.
