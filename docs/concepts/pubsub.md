---
title: Events between resolvers
description: Pub/sub so mutations (or anything else) can fan out data to subscription resolvers  -  in-memory vs Redis backends and when to use which.
outline: [2, 3]
---

# Events between resolvers

GraphQL subscriptions need a way for mutation resolvers (or any other
event source) to hand values to subscription resolvers, ideally without
either side caring whether the other is in the same process. grx ships
[`memory-pubsub`](https://pkg.go.dev/github.com/grx-gql/grx/memory-pubsub)
(`package pubsub`, in-process) and the optional
[`redis-pubsub`](https://pkg.go.dev/github.com/grx-gql/grx/redis-pubsub)
submodule for cross-replica fan-out.

`memory-pubsub` is **opt-in**. Servers without subscriptions  -  or
subscriptions whose resolver returns its own `<-chan`  -  never need to
configure a pub/sub backend. Splitting the subscription URL with
`grx.WithSubscriptionPath` only changes where WebSocket/SSE attach on the
wire; it does **not** remove the need for `memory-pubsub` when mutations
publish events that subscription resolvers consume.

## Choosing a backend {#choosing-a-backend}

Both implementations satisfy the same [`PubSub`](#the-interface) contract; your
resolvers and `Typed[T]` wiring stay identical  -  only **how you construct the
bus** changes.

| | **In-memory** ([`memory-pubsub`](https://pkg.go.dev/github.com/grx-gql/grx/memory-pubsub)) | **Redis** ([`redis-pubsub`](https://pkg.go.dev/github.com/grx-gql/grx/redis-pubsub)) |
| --- | --- | --- |
| **Use when** | One running server process (or single replica) handles both the publishing mutation and the subscription clients you care about | Multiple instances sit behind a load balancer and **any** replica may **publish** while **another** holds the WebSocket/SSE |
| **Dependencies** | None beyond the main `grx` module | A reachable Redis; add the nested `redis-pubsub` module to your `go.mod` |
| **Delivery** | Goroutines in the same process | Redis `PUBLISH` / `SUBSCRIBE`  -  every GraphQL replica subscribed to a channel sees the payload |
| **Semantics** | Same as the interface: **best-effort**, **subscribe-then-publish**, **not durable** | Same  -  Redis pub/sub **drops** messages if no subscriber is connected; it is **not** a persistent event log |

**Rule of thumb:** start with **`pubsub.NewMemory()`** for development and any
deployment where a single process owns the whole graph. Move to
**`redis-pubsub`** the first time you scale out horizontally and notice
subscriptions stop seeing mutations that ran on a different pod or VM.

Same limitations apply to both: slow subscribers are shed so publishers never
block; there is **no replay** of historical events. If you need durable,
replayable streams (Kafka, JetStream, an outbox table), keep that system as
your source of truth and only bridge into `PubSub` at the edge where GraphQL
needs a live fan-out.

API references: [`memory-pubsub` package](/reference/memory-pubsub/) ·
[`redis-pubsub` package](/reference/redis-pubsub/).

## The interface

```go
// github.com/grx-gql/grx/memory-pubsub (package pubsub)
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

## `Typed[T]`  -  the everyday API

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

### `pubsub.Memory`  -  in-process default

[`NewMemory`](https://pkg.go.dev/github.com/grx-gql/grx/memory-pubsub#NewMemory)
(and [`NewMemoryWith`](https://pkg.go.dev/github.com/grx-gql/grx/memory-pubsub#NewMemoryWith)
for tuned buffer sizes) allocates the cheapest path: payloads stay inside one
binary and never hit Redis.

```go
import pubsub "github.com/grx-gql/grx/memory-pubsub"

bus := pubsub.NewMemory() // or pubsub.NewMemoryWith(pubsub.MemoryConfig{Buffer: 64})
defer bus.Close()
```

**Choose this** for unit tests, local dev, CI, and production that runs **exactly one**
graphql instance (no horizontal scale). Zero extra infrastructure; subscribers
receive direct channel hand-offs backed by buffered Go channels (`MemoryConfig.Buffer`,
default 16).

### `redis-pubsub`  -  across replicas

Uses your Redis client's native pub/sub primitives so publishes from **any**
replica wake subscribers on **every** replica that subscribed (subject to Redis's
usual network partitioning behaviour).

```go
import (
	redispubsub "github.com/grx-gql/grx/redis-pubsub"

	"github.com/redis/go-redis/v9"
)

rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
bus, err := redispubsub.New(redispubsub.Config{
	Client: rdb,
	Prefix: "grx:", // prefixes channel names  -  isolate environments / tenants / stacks
})
```

**Choose this** when mutations and subscriptions might land on **different**
processes (Kubernetes rollout, autoscaled VMs, blue/green). The submodule lives
outside the core `go.mod` so production Redis clients do not become dependencies
for apps that never scale out ([`redis-pubsub` module docs](/reference/redis-pubsub/)).

**Operational checklist:** Redis must be reachable from every replica, **firewall /
ACL** openings match your cluster layout, **`Prefix`** separates staging vs prod (and
often per-tenant shards), and you accept Redis pub/sub's **volatile** semantics  -  if
Redis restarts or a replica is briefly partitioned, messages in flight may be missed
with no automatic replay ([Deployment notes](/guides/deployment) touch on infra wiring).

### Bring your own broker

NATS, Kafka, Google Pub/Sub, AWS SNS, in-memory ring buffers  -  anything
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
  cross-resolver coordination needed)  -  your subscription resolver can
  return a channel directly without involving `pubsub` at all.
- **Persistent, replayable streams** (Kafka-style consumer groups,
  outbox pattern, exactly-once delivery)  -  `pubsub.PubSub` is
  intentionally minimal and matches Redis-style semantics. Wrap a
  durable broker behind a custom `PubSub` only when its semantics fit;
  otherwise keep your domain stream layer separate and feed into
  `pubsub` only at the leaf where the GraphQL subscription needs it.
