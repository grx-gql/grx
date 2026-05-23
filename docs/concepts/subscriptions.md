---
title: Subscriptions in the runtime
description: Source streams, response streams, and how the executor pumps values until the context ends.
outline: [2, 3]
---

# Subscriptions in the runtime

Subscriptions in grx are first-class. Resolvers return a Go channel; the
executor turns each value emitted into a normal field-execution pass and
the registered transport pushes the result to the client.

## Resolver shape

```go
func (T) FieldName(ctx context.Context, args TArgs) (<-chan *TResult, error)
```

The element type of the channel determines the GraphQL output type. Both
`ctx` and `args` are optional, in that order, exactly like queries and
mutations.

## Source streams vs response streams

GraphQL subscriptions distinguish two streams:

- The **source stream** is what your resolver produces — raw values you
  want to emit.
- The **response stream** is what the client sees — `{ data, errors }`
  payloads after field execution has run on each source value.

In grx the source stream is the channel you return; the response stream
is constructed by the executor. You don't need to do anything special to
turn one into the other.

## Cleanup

Always `select` on `ctx.Done()` in the goroutine that produces values:

```go
go func() {
    defer close(stream)
    for {
        select {
        case <-ctx.Done():
            return
        case ev := <-bus:
            select {
            case <-ctx.Done():
                return
            case stream <- ev:
            }
        }
    }
}()
```

The transport cancels `ctx` when:

- The client disconnects (TCP RST, WebSocket close frame, SSE EOF).
- The client sends a `complete` (`graphql-transport-ws`) or `stop`
  (`graphql-ws`) message for that subscription id.
- The server closes the connection due to write timeout, init timeout,
  protocol error, or shutdown.

## Transports

Either transport carries subscriptions; pick based on what the client can
speak.

- **WebSocket** (`pkg/websocket`) implements the `graphql-transport-ws`
  subprotocol — the modern protocol used by the `graphql-ws` library
  (v5+) and Apollo Client (v3.5+). The legacy
  `subscriptions-transport-ws` (Apollo's old `graphql-ws`) protocol is
  intentionally not supported; it was deprecated in 2021 and clients
  that have not migrated should upgrade.
- **SSE** (`pkg/sse`) is great when you only need server → client and
  want the simplest possible deployment story. There's no upgrade and no
  duplex, just `text/event-stream`.

## Single-root-field rule

The GraphQL spec requires a subscription operation to have exactly one
root field. grx enforces this at execution time; multiple root fields
produce a request error.

## Backpressure

The WebSocket transport applies `Config.WriteTimeout` to every frame, so a
slow consumer can't pin server memory. SSE inherits Go's `http.ResponseWriter`
flush semantics; a stuck client will eventually be killed by the read /
write deadlines on the underlying `net.Conn`.

If you need finer control (per-subscription buffer sizes, drop-oldest
policies), do it inside your resolver — keep that policy close to your
event source rather than baked into the transport.

## Authentication

Auth happens once, at connection open. Provide an
[`OnConnect`](/concepts/transports) hook on the WebSocket config; it
receives the init payload, returns the derived context for every
subscription on that connection, and may reject with a `4403` close.
SSE auth uses standard HTTP middleware in front of the server.

## Cross-resolver fan-out with `pubsub`

Real subscription resolvers rarely produce values themselves; they fan
out events that **mutation resolvers** publish elsewhere in the
process (or in a sibling replica, when running behind a load balancer).
grx ships [`pkg/pubsub`](/concepts/pubsub) for exactly this:

```go
bus := pubsub.NewMemory()                         // in-process default
events := pubsub.NewTyped[*Message](bus)          // type-safe wrapper

// inside the mutation resolver
_ = events.Publish(ctx, "message.posted", msg)

// inside the subscription resolver — predicates run on the publish
// path so uninteresting messages never wake the consumer goroutine.
return events.Subscribe(ctx, "message.posted", func(m *Message) bool {
    return m.RoomID == args.RoomID
})
```

Swap `pubsub.NewMemory()` for `pkg/pubsub/redis` when you need to
deliver events across replicas. See [Pub/Sub](/concepts/pubsub) for
the full surface and [Realtime subscriptions](/guides/subscriptions) for an
end-to-end chat-room example.
