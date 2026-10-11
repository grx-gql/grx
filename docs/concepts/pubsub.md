---
title: Events between resolvers
description: Pub/sub so mutations can fan out byte payloads to subscription resolvers.
outline: [2, 3]
---

# Events between resolvers

GraphQL subscription resolvers can consume events published by mutations or
other application code. grx keeps this boundary backend-neutral with the
`github.com/grx-gql/grx/pubsub` package.

## Choosing a backend {#choosing-a-backend}

| | Memory | Redis |
| --- | --- | --- |
| Import | `github.com/grx-gql/grx/pubsub/memory` | `github.com/grx-gql/grx/pubsub/redis` |
| Use when | One process owns the publishers and subscribers | Multiple replicas need fan-out |
| Delivery | Direct channel hand-off | Redis `PUBLISH` / `SUBSCRIBE` |
| Durability | Best-effort and non-durable | Best-effort and non-durable |

Both implementations satisfy the same `pubsub.PubSub` contract. Slow
subscribers may miss messages, and messages published before subscription are
not replayed. Use a durable event system separately when replay is required.

## The interface

```go
package pubsub

type Message struct {
	Topic   string
	Payload []byte
}

type PubSub interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Subscribe(ctx context.Context, topic string) (<-chan Message, error)
	Close() error
}
```

Topics are exact matches. Use hierarchical topic names such as
`message.posted.room-1` when a resolver needs scoped streams.

Payloads are opaque to the bus. The publisher owns the payload until
`Publish` returns and must not mutate or reuse its backing array afterward.
Subscribers must treat received payloads as read-only; copy them when they
need to retain or modify the bytes.

## Memory backend

```go
import "github.com/grx-gql/grx/pubsub/memory"

bus := memory.New()
defer bus.Close()
```

The Memory implementation avoids serialization and is suitable for tests,
development, and single-replica deployments.

## Redis backend

The Redis backend is a separate module so applications that do not need Redis
do not add a Redis dependency to the root module.

```go
import (
	redisps "github.com/grx-gql/grx/pubsub/redis"
	"github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
bus, err := redisps.New(redisps.Config{Client: rdb, Prefix: "grx:"})
if err != nil {
	panic(err)
}
defer bus.Close()
```

## Resolver wiring

Encode and decode domain values at the resolver boundary. This keeps every
backend interchangeable and avoids putting application-specific codecs in the
transport-agnostic bus.

Each resolver holds the same `pubsub.PubSub` directly:

```go
type MessageMutation struct {
	Bus pubsub.PubSub
}

type MessageSubscription struct {
	Bus pubsub.PubSub
}
```

The subscription example composes these dependencies with
`grx.WithSchema(graph.New(bus))`. Without a bus, `graph.New()` registers only
queries and non-event mutations, with no subscription root.

See [Realtime subscriptions](/guides/subscriptions#encode-payloads-at-the-resolver-boundary)
for the tested message resolver, JSON encoding and decoding, and shared bus wiring.
