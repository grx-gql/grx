module github.com/grx-gql/grx/pubsub/redis

go 1.26.0

// The Redis pubsub backend lives in its own Go module so the root grx
// module remains free of third-party runtime dependencies. The replace
// directive points at the working tree so local development picks up
// in-tree changes to the pubsub interface without a publish
// round-trip.
replace github.com/grx-gql/grx => ../../

require (
	github.com/alicebob/miniredis/v2 v2.40.0
	github.com/grx-gql/grx v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.23.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
