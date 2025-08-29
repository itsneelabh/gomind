module github.com/itsneelabh/gomind

go 1.23.0

// This is now a meta-module that re-exports from submodules
// Users should import specific modules:
//   - github.com/itsneelabh/gomind/core - For lightweight tools (8MB)
//   - github.com/itsneelabh/gomind/ai - For AI capabilities
//   - github.com/itsneelabh/gomind/telemetry - For observability
//   - github.com/itsneelabh/gomind/orchestration - For multi-agent systems and workflows
//   - github.com/itsneelabh/gomind/resilience - For resilience patterns

require github.com/itsneelabh/gomind/core v0.0.0-00010101000000-000000000000

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
)

replace (
	github.com/itsneelabh/gomind/ai => ./ai
	github.com/itsneelabh/gomind/core => ./core
	github.com/itsneelabh/gomind/orchestration => ./orchestration
	github.com/itsneelabh/gomind/resilience => ./resilience
	github.com/itsneelabh/gomind/telemetry => ./telemetry
)
