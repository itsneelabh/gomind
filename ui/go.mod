module github.com/itsneelabh/gomind/ui

go 1.25

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/itsneelabh/gomind/core v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.3.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	golang.org/x/text v0.28.0 // indirect
)

replace github.com/itsneelabh/gomind/core => ../core
