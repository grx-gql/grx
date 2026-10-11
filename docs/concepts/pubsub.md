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
	redisps "github.com/grx-gql/grx/redis"
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

```go
type Events struct {
	Bus pubsub.PubSub
}

func (m MessageMutation) PostMessage(ctx context.Context, message *Message) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return m.Bus.Publish(ctx, "message.posted."+message.RoomID, payload)
}

func (s MessageSubscription) MessagePosted(ctx context.Context, roomID string) (<-chan *Message, error) {
	msgs, err := s.Bus.Subscribe(ctx, "message.posted."+roomID)
	if err != nil {
		return nil, err
	}

	out := make(chan *Message)
	go func() {
		defer close(out)
		for msg := range msgs {
			var message Message
			if err := json.Unmarshal(msg.Payload, &message); err != nil {
				continue
			}
			select {
			case out <- &message:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
```
