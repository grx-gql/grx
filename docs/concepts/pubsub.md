---
title: Pub/Sub
description: How mutation resolvers feed events to subscription resolvers, with backend-agnostic pub/sub.
outline: [2, 3]
---

GraphQL subscriptions need a way for mutation resolvers (or any other
event source) to hand values to subscription resolvers, ideally without
either side caring whether the other is in the same process. grx ships
`pkg/pubsub`, a small interface plus an in-process default and an
optional Redis backend.

`pkg/pubsub` is **opt-in**. Servers without subscriptions — or
subscriptions whose resolver returns its own `<-chan` — never need to
configure a pub/sub backend. Splitting the subscription URL with
`grx.WithSubscriptionPath` only changes where WebSocket/SSE attach on the
wire; it does **not** remove the need for `pkg/pubsub` when mutations
publish events that subscription resolvers consume.

## The interface

```go
// pkg/pubsub/pubsub.go
type PubSub interface {
    Publish(ctx context.Context, topic string, payload []byte) error
    Subscribe(ctx context.Context, topic string, filters ...Filter) (<-chan Message, error)
    Close() error
}

type Message struct {
    Topic   string
    Payload []byte
}
```

Three properties are non-negotiable across implementations:

- **Best-effort delivery.** A slow subscriber must not block the
  publisher or other subscribers. Implementations drop the message for
  the slow consumer rather than holding the publisher up.
- **Subscribe-then-publish semantics.** Messages are not persisted; a
  subscriber receives messages published *after* it subscribes.
- **Concurrent-use safe.** Implementations are designed to be shared
  across goroutines.

## `Typed[T]` — the everyday API

Resolvers should rarely talk to `PubSub` directly. The `Typed[T]`
wrapper carries a `Codec[T]` (default: JSON) and exposes type-safe
`Publish` / `Subscribe`:

```go
events := pubsub.NewTyped[*Message](bus)

// Publish: encode + forward in one call.
_ = events.Publish(ctx, "message.posted", msg)

// Subscribe: predicates take the decoded value, not raw bytes.
ch, _ := events.Subscribe(ctx, "message.posted", func(m *Message) bool {
    return m.RoomID == args.RoomID
})
```

Decoded `T` values come straight off the channel. Messages whose
payload fails to decode are silently dropped so a single poison
message can't stall the rest of the stream.

If JSON is the wrong wire format for `T` (interoperating with a
non-Go publisher, for example), pass a custom `Codec[T]` to
`pubsub.NewTypedWith`.

## Filters

`PubSub.Subscribe` takes any number of `Filter` values that run on the
**publish** path so uninteresting messages never reach the consumer
goroutine. Built-in helpers:

| Helper                    | What it does                                                  |
| ------------------------- | ------------------------------------------------------------- |
| `pubsub.TopicEquals(t)`   | Match only messages whose `Topic == t`. Useful for wildcard subscribers that want to gate by exact topic. |
| `pubsub.TopicHasPrefix(p)`| Match messages whose `Topic` starts with `p`. Pairs with hierarchical topic naming.                       |
| `pubsub.PayloadFunc(fn)`  | Run `fn([]byte) bool` against the raw payload bytes.                                                      |
| `pubsub.All(filters…)`    | Match when every filter matches.                                                                          |
| `pubsub.Any(filters…)`    | Match when at least one filter matches.                                                                   |
| `pubsub.FilterFunc(fn)`   | Adapt `func(Message) bool` into `Filter`.                                                                 |

Typed subscribers (`Typed[T].Subscribe`) take simpler `func(T) bool`
predicates evaluated against the decoded value. Use raw `Filter`s when
you need to inspect bytes before decoding.

## Backends

### `pubsub.Memory` — in-process default

```go
bus := pubsub.NewMemory() // or pubsub.NewMemoryWith(pubsub.MemoryConfig{Buffer: 64})
defer bus.Close()
```

Zero dependencies, no serialisation cost. Use it for development,
tests, and single-replica deployments. Each subscriber gets its own
buffered channel; the buffer size is configurable via `MemoryConfig`
(default 16).

### `pkg/pubsub/redis` — across replicas

```go
import (
    redispubsub "github.com/patrickkabwe/grx/pkg/pubsub/redis"
    "github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
bus, err := redispubsub.New(redispubsub.Config{
    Client: rdb,
    Prefix: "grx:",
})
```

Lives in a **separate Go submodule** so the root `grx` module stays
dependency-free. Pull it in only when you actually need cross-replica
delivery.

### Bring your own broker

NATS, Kafka, Google Pub/Sub, AWS SNS, in-memory ring buffers — anything
that satisfies `pubsub.PubSub` is a valid backend. The executor and
subscription transports never look at the concrete type.

## Wiring it into a schema

The conventional pattern is to construct one bus at startup, wrap it
with `Typed[T]` per event type, and pass each typed bus to the
resolvers that publish or subscribe to it. From `examples/subscriptions`:

```go
func NewSchema(bus pubsub.PubSub) schema.Config {
    users    := pubsub.NewTyped[*User](bus)
    messages := pubsub.NewTyped[*Message](bus)

    return schema.Config{
        Query: Query{},
        Mutation: Mutation{
            UserMutation:    &UserMutation{Bus: users},
            MessageMutation: &MessageMutation{Bus: messages},
        },
        Subscription: Subscription{
            UserSubscription:    UserSubscription{Bus: users},
            MessageSubscription: MessageSubscription{Bus: messages},
        },
    }
}

func main() {
    bus := pubsub.NewMemory()
    defer bus.Close()

    srv, _ := grx.NewServer(
        grx.WithSchema(graph.New(graph.WithPubSub(bus))),
        grx.WithTransports(
            websocket.New(websocket.Config{CheckOrigin: allowOrigin}),
            sse.New(),
        ),
    )
    _ = http.ListenAndServe(":4000", srv)
}
```

The same `bus` value is shared across resolvers; the `Typed[T]`
wrappers are cheap to construct and stateless.

## When `pubsub` is the wrong tool

- **Per-request fan-in** (one upstream `<-chan` per subscriber, no
  cross-resolver coordination needed) — your subscription resolver can
  return a channel directly without involving `pubsub` at all.
- **Persistent, replayable streams** (Kafka-style consumer groups,
  outbox pattern, exactly-once delivery) — `pubsub.PubSub` is
  intentionally minimal and matches Redis-style semantics. Wrap a
  durable broker behind a custom `PubSub` only when its semantics fit;
  otherwise keep your domain stream layer separate and feed into
  `pubsub` only at the leaf where the GraphQL subscription needs it.
