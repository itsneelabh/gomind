# Changelog

All notable changes to the GoMind framework will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### AI Module
- **OpenAI-Compatible Provider Support** - Clean namespace separation for alternative AI providers
  - Provider aliases for DeepSeek, Groq, Together AI, xAI, Qwen
  - Example: `ai.WithProviderAlias("openai.groq")`
  - Three-tier configuration: Explicit ’ Environment ’ Defaults
- **Chain Client** - Automatic failover across multiple AI providers
  - New `ChainClient` with graceful degradation
  - Smart error handling and retry logic
  - Example: `ai.NewChainClient(ai.WithProviderChain(providers))`
- **Model Aliases** - Portable model names across providers
  - Standard aliases: "smart", "fast", "code", "vision", "default"
  - Provider-specific model resolution
  - Pass-through support for explicit model names

#### Telemetry Module
- **Framework Logger** - Production-grade structured logging
  - Context-aware logging methods (DebugWithContext, InfoWithContext, etc.)
  - Structured field support with metadata
  - Integration with OpenTelemetry spans
- **Circuit Breaker Telemetry** - Enhanced observability for circuit breakers
  - Detailed state transition logging
  - Metrics for circuit breaker events
  - Framework integration layer

#### Resilience Module
- **Advanced Circuit Breaker** - Production-ready fault tolerance
  - Configurable thresholds and timeouts
  - Exponential backoff support
  - Comprehensive error categorization
- **Enhanced Retry Logic** - Improved retry mechanisms
  - Exponential backoff with jitter
  - Context-aware cancellation
  - Configurable retry policies
- **Panic Recovery** - Graceful error handling
  - Automatic panic recovery with logging
  - Stack trace capture
  - Circuit breaker integration

#### Core Module
- **Global Metrics Registry** - Framework-wide metrics access
  - Centralized metrics collection
  - Module registration support
  - Prometheus integration
- **Enhanced Configuration** - Improved configuration management
  - Environment variable precedence
  - Default value chains
  - Validation and sanitization

#### Documentation
- Comprehensive architecture documentation for orchestration module (1,373 lines)
- Comprehensive architecture documentation for telemetry module (1,332 lines)
- Enhanced API reference with new provider features (2,283 lines updated)
- Framework design principles documentation
- Production deployment guides for UI module

### Changed
- AI provider factories now implement three-tier configuration
- DetectEnvironment() no longer mutates global environment variables
- Circuit breaker transport returns errors instead of panicking
- Replaced math/rand with crypto/rand for secure random generation
- Updated AI provider model names to current versions
- Enhanced error logging in HTTP server

### Fixed
- Fixed test compilation errors in ai/providers/base_test.go
- Fixed missing context-aware methods in mock loggers
- Fixed environment variable mutation in provider detection
- Fixed telemetry example dependencies
- Fixed context propagation example go.mod

### Testing
- Added 4,000+ lines of new test coverage
- Production logging tests for all modules
- Advanced circuit breaker test scenarios
- Chain client integration tests
- Multi-provider configuration tests
- Panic recovery test suites

## [0.5.1] - 2024-11-22

### Fixed
- Resolved import path issues in example modules

## [0.5.0] - 2024-11-21

### Added
- Initial public release of GoMind framework
- Core module with agent and tool abstractions
- AI module with OpenAI and Anthropic support
- Resilience module with circuit breakers and retry logic
- Telemetry module with OpenTelemetry integration
- Orchestration module for multi-agent coordination
- UI module for web interfaces
- Kubernetes deployment support
- Redis-based service discovery
- Example applications demonstrating framework capabilities