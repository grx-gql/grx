module github.com/grx-gql/grx/redis-pubsub

go 1.25

// The Redis pubsub backend lives in its own Go module so the root grx
// module remains free of third-party runtime dependencies. The replace
// directive points at the working tree so local development picks up
// in-tree changes to the memory-pubsub interfaces without a publish
// round-trip.
replace github.com/grx-gql/grx => ../

require (
	github.com/alicebob/miniredis/v2 v2.33.0
	github.com/grx-gql/grx v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.7.0
)

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)
