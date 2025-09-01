module github.com/itsneelabh/gomind/orchestration

go 1.25

replace (
	github.com/itsneelabh/gomind/ai => ../ai
	github.com/itsneelabh/gomind/core => ../core
)

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
	github.com/itsneelabh/gomind/ai v0.0.0-00010101000000-000000000000
	github.com/itsneelabh/gomind/core v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)
