---
title: pkg/pubsub
description: API reference for the pkg/pubsub package, generated from Go doc comments.
outline: [2, 4]
lastUpdated: false
---

# pkg/pubsub

```go
```

Package pubsub defines the publish/subscribe primitive used by GraphQL subscription resolvers to fan messages out to connected clients.

The package centres on a small [PubSub](<#PubSub>) interface so the rest of the runtime can stay backend\-agnostic. Two things ship in the standard library:

- [Memory](<#Memory>) — an in\-process implementation suitable for development, tests, and single\-replica deployments.
- [Typed](<#Typed>) — a generic wrapper that adds type\-safe Publish and Subscribe operations on top of any [PubSub](<#PubSub>) using a pluggable [Codec](<#Codec>). Resolvers should normally talk to a Typed\[T\] rather than to the raw byte\-oriented PubSub.

Distributed backends are intentionally kept out of this package so the root module stays dependency\-free; see the github.com/patrickkabwe/grx/pkg/pubsub/redis sub\-module for a production\-ready Redis implementation. Any third\-party broker can be plugged in by satisfying the [PubSub](<#PubSub>) interface — the executor and subscription transports never look at the concrete type.

### Filters

\[Subscribe\] accepts zero or more [Filter](<#Filter>) values. Filters run on the publishing side of the channel so uninteresting messages never wake the consumer goroutine. Use [TopicEquals](<#TopicEquals>) / [TopicHasPrefix](<#TopicHasPrefix>) for topic\-based gating, [PayloadFunc](<#PayloadFunc>) for byte inspection, or compose with [All](<#All>) / [Any](<#Any>). Typed subscribers can use simpler \[func\(T\) bool\] predicates, see [Typed.Subscribe](<#Typed.Subscribe>).

## Index

- [Variables](<#variables>)
- [type Codec](<#Codec>)
- [type Filter](<#Filter>)
  - [func All\(filters ...Filter\) Filter](<#All>)
  - [func Any\(filters ...Filter\) Filter](<#Any>)
  - [func PayloadFunc\(fn func\(\[\]byte\) bool\) Filter](<#PayloadFunc>)
  - [func TopicEquals\(topic string\) Filter](<#TopicEquals>)
  - [func TopicHasPrefix\(prefix string\) Filter](<#TopicHasPrefix>)
- [type FilterFunc](<#FilterFunc>)
  - [func \(f FilterFunc\) Matches\(msg Message\) bool](<#FilterFunc.Matches>)
- [type JSONCodec](<#JSONCodec>)
  - [func \(JSONCodec\[T\]\) Decode\(data \[\]byte\) \(T, error\)](<#JSONCodec[T].Decode>)
  - [func \(JSONCodec\[T\]\) Encode\(value T\) \(\[\]byte, error\)](<#JSONCodec[T].Encode>)
- [type Memory](<#Memory>)
  - [func NewMemory\(cfgs ...MemoryConfig\) \*Memory](<#NewMemory>)
  - [func \(m \*Memory\) Close\(\) error](<#Memory.Close>)
  - [func \(m \*Memory\) Publish\(\_ context.Context, topic string, payload \[\]byte\) error](<#Memory.Publish>)
  - [func \(m \*Memory\) Subscribe\(ctx context.Context, topic string, filters ...Filter\) \(\<\-chan Message, error\)](<#Memory.Subscribe>)
- [type MemoryConfig](<#MemoryConfig>)
- [type Message](<#Message>)
- [type PubSub](<#PubSub>)
- [type Typed](<#Typed>)
  - [func NewTyped\[T any\]\(bus PubSub\) \*Typed\[T\]](<#NewTyped>)
  - [func NewTypedWith\[T any\]\(bus PubSub, codec Codec\[T\]\) \*Typed\[T\]](<#NewTypedWith>)
  - [func \(t \*Typed\[T\]\) Bus\(\) PubSub](<#Typed[T].Bus>)
  - [func \(t \*Typed\[T\]\) Publish\(ctx context.Context, topic string, value T\) error](<#Typed[T].Publish>)
  - [func \(t \*Typed\[T\]\) Subscribe\(ctx context.Context, topic string, predicates ...func\(T\) bool\) \(\<\-chan T, error\)](<#Typed[T].Subscribe>)


## Variables

<a name="ErrClosed"></a>ErrClosed is returned by [PubSub](<#PubSub>) implementations once \[PubSub.Close\] has been called. Subsequent \[PubSub.Publish\] and \[PubSub.Subscribe\] calls must surface this error so callers can fail fast.

```go
var ErrClosed = errors.New("pubsub: closed")
```

<a name="Codec"></a>
## type Codec

Codec converts values of type T to and from the byte payloads carried by [PubSub](<#PubSub>). It exists so [Typed](<#Typed>) can interoperate with any backend that speaks bytes \(Redis, NATS, Kafka, in\-process Memory, etc.\) while callers work with strongly typed Go values.

```go
type Codec[T any] interface {
    Encode(value T) ([]byte, error)
    Decode(data []byte) (T, error)
}
```

<a name="Filter"></a>
## type Filter

Filter decides whether a Message should be delivered to a subscriber. Filters run on the publish path, so they should be cheap and free of side effects.

```go
type Filter interface {
    Matches(msg Message) bool
}
```

<a name="All"></a>
### func All

```go
func All(filters ...Filter) Filter
```

All returns a [Filter](<#Filter>) that matches when every provided filter matches. With zero filters, the returned filter always matches.

<a name="Any"></a>
### func Any

```go
func Any(filters ...Filter) Filter
```

Any returns a [Filter](<#Filter>) that matches when at least one provided filter matches. With zero filters, the returned filter never matches.

<a name="PayloadFunc"></a>
### func PayloadFunc

```go
func PayloadFunc(fn func([]byte) bool) Filter
```

PayloadFunc returns a [Filter](<#Filter>) that runs fn against the raw payload bytes. It is a convenience for the common case of inspecting only the payload.

<a name="TopicEquals"></a>
### func TopicEquals

```go
func TopicEquals(topic string) Filter
```

TopicEquals returns a [Filter](<#Filter>) that matches messages whose Topic is exactly equal to topic. Useful when subscribing to a parent topic and wanting to gate by a specific child.

<a name="TopicHasPrefix"></a>
### func TopicHasPrefix

```go
func TopicHasPrefix(prefix string) Filter
```

TopicHasPrefix returns a [Filter](<#Filter>) that matches messages whose Topic starts with prefix. Combine with [Memory](<#Memory>) / \[redis.PubSub\] wildcard subscriptions to implement hierarchical channels.

<a name="FilterFunc"></a>
## type FilterFunc

FilterFunc adapts a plain predicate to the [Filter](<#Filter>) interface.

```go
type FilterFunc func(Message) bool
```

<a name="FilterFunc.Matches"></a>
### func \(FilterFunc\) Matches

```go
func (f FilterFunc) Matches(msg Message) bool
```

Matches implements [Filter](<#Filter>).

<a name="JSONCodec"></a>
## type JSONCodec

JSONCodec encodes values with [encoding/json](<https://pkg.go.dev/encoding/json/>). It is the default codec for [Typed](<#Typed>) and the right choice for almost every GraphQL payload because the values are already trees of strings, numbers, and structs.

```go
type JSONCodec[T any] struct{}
```

<a name="JSONCodec[T].Decode"></a>
### func \(JSONCodec\[T\]\) Decode

```go
func (JSONCodec[T]) Decode(data []byte) (T, error)
```

Decode unmarshals data into a fresh T.

<a name="JSONCodec[T].Encode"></a>
### func \(JSONCodec\[T\]\) Encode

```go
func (JSONCodec[T]) Encode(value T) ([]byte, error)
```

Encode marshals value to JSON.

<a name="Memory"></a>
## type Memory

Memory is a single\-process [PubSub](<#PubSub>). Messages are delivered to local subscribers without serialization, so it is the fastest backend available and is the right default for development, tests, and single\-replica deployments. Swap in github.com/patrickkabwe/grx/pkg/pubsub/redis \(or any other [PubSub](<#PubSub>) implementation\) to fan messages across replicas.

Memory is safe for concurrent use.

```go
type Memory struct {
    // contains filtered or unexported fields
}
```

<a name="NewMemory"></a>
### func NewMemory

```go
func NewMemory(cfgs ...MemoryConfig) *Memory
```

NewMemory returns a [Memory](<#Memory>) ready for use. Pass an optional MemoryConfig to override the per\-subscriber buffer size; only the first config is honoured so callers can use a zero\-value default with NewMemory\(\).

<a name="Memory.Close"></a>
### func \(\*Memory\) Close

```go
func (m *Memory) Close() error
```

Close cancels every active subscription and refuses further Publish/Subscribe calls. It is safe to call more than once; the second and subsequent calls return nil.

<a name="Memory.Publish"></a>
### func \(\*Memory\) Publish

```go
func (m *Memory) Publish(_ context.Context, topic string, payload []byte) error
```

Publish delivers payload to every active subscriber of topic whose filters match. Slow subscribers whose buffer is full are skipped rather than blocking the publisher; this keeps a misbehaving subscription from stalling mutations. The ctx parameter is accepted for interface compatibility but is not consulted: in\-memory delivery never blocks.

<a name="Memory.Subscribe"></a>
### func \(\*Memory\) Subscribe

```go
func (m *Memory) Subscribe(ctx context.Context, topic string, filters ...Filter) (<-chan Message, error)
```

Subscribe registers a consumer on topic. The returned channel emits every message published to topic that satisfies every supplied filter until ctx is cancelled or [Memory.Close](<#Memory.Close>) is called, after which the channel is closed exactly once.

<a name="MemoryConfig"></a>
## type MemoryConfig

MemoryConfig tunes the in\-process [Memory](<#Memory>) implementation.

```go
type MemoryConfig struct {
    // Buffer is the per-subscriber channel buffer size. A subscriber
    // whose buffer is full is dropped on Publish rather than blocking
    // the publisher; raise this when bursts are expected. Defaults to
    // 16 when zero or negative.
    Buffer int
}
```

<a name="Message"></a>
## type Message

Message is the envelope delivered to subscribers. Topic is the channel the message was published on \(useful when a Filter or a wildcard subscription sees several topics\) and Payload is the raw wire bytes — implementations must not mutate it after delivery.

```go
type Message struct {
    Topic   string
    Payload []byte
}
```

<a name="PubSub"></a>
## type PubSub

PubSub is a transport\-agnostic publish/subscribe primitive. Implementations must be safe for concurrent use by multiple goroutines.

A PubSub is the contract between mutation resolvers \(which Publish domain events\) and subscription resolvers \(which Subscribe to those events and forward them to connected GraphQL clients\). The semantics match Redis PUBLISH/SUBSCRIBE: messages are not persisted, slow consumers may drop messages rather than block publishers, and subscribers receive messages published after they subscribe.

```go
type PubSub interface {
    // Publish broadcasts payload to every active subscriber of topic.
    // Implementations must apply best-effort delivery: a slow or
    // stalled subscriber must not block the publisher or other
    // subscribers. ctx may be honoured for cancellation by network-
    // backed implementations.
    Publish(ctx context.Context, topic string, payload []byte) error

    // Subscribe registers a consumer on topic and returns a stream of
    // matching messages. The returned channel is closed exactly once
    // when ctx is cancelled or the PubSub is closed, so callers may
    // range over it safely. Filters are evaluated server-side before
    // the message is enqueued so unmatched messages never reach the
    // channel.
    Subscribe(ctx context.Context, topic string, filters ...Filter) (<-chan Message, error)

    // Close releases resources held by the implementation, cancels in-
    // flight subscriptions, and prevents further Publish/Subscribe
    // calls from succeeding. Close must be safe to call more than
    // once; the second and subsequent calls return nil.
    Close() error
}
```

<a name="Typed"></a>
## type Typed

Typed wraps a [PubSub](<#PubSub>) with a [Codec](<#Codec>) to provide a type\-safe API for a single message type. Resolvers should normally talk to a Typed\[T\] rather than the raw PubSub: it removes the marshal/unmarshal boilerplate from every Publish and Subscribe call site, and the returned channel carries decoded T values directly.

Typed is safe for concurrent use; it stores no per\-call state. It is also cheap to construct, so the conventional pattern is to create one near the resolver that owns the topic and pass it around like a repository or service.

```go
type Typed[T any] struct {
    // contains filtered or unexported fields
}
```

<a name="NewTyped"></a>
### func NewTyped

```go
func NewTyped[T any](bus PubSub) *Typed[T]
```

NewTyped wraps bus with the default [JSONCodec](<#JSONCodec>) for T.

<a name="NewTypedWith"></a>
### func NewTypedWith

```go
func NewTypedWith[T any](bus PubSub, codec Codec[T]) *Typed[T]
```

NewTypedWith wraps bus with the supplied Codec. Use it when JSON is the wrong choice for T \(for example, when interoperating with a non\-Go publisher that already uses a fixed binary format\).

<a name="Typed[T].Bus"></a>
### func \(\*Typed\[T\]\) Bus

```go
func (t *Typed[T]) Bus() PubSub
```

Bus returns the underlying [PubSub](<#PubSub>). Useful when callers need to reach through Typed for backend\-specific features \(e.g. graceful shutdown via Close\).

<a name="Typed[T].Publish"></a>
### func \(\*Typed\[T\]\) Publish

```go
func (t *Typed[T]) Publish(ctx context.Context, topic string, value T) error
```

Publish encodes value and forwards it to the underlying [PubSub](<#PubSub>). The codec error is returned without publishing if encoding fails, so a misconfigured codec never produces a corrupt frame on the wire.

<a name="Typed[T].Subscribe"></a>
### func \(\*Typed\[T\]\) Subscribe

```go
func (t *Typed[T]) Subscribe(ctx context.Context, topic string, predicates ...func(T) bool) (<-chan T, error)
```

Subscribe registers a typed consumer on topic and returns a channel of decoded T values. The variadic predicates are evaluated against the decoded value, so they can use the full Go type system rather than poking at raw bytes. Backend\-level \[Filter\]s \(which see the undecoded message\) are not exposed here; use [Typed.Bus](<#Typed.Bus>) \+ \[PubSub.Subscribe\] when raw filtering is required.

Messages whose payload fails to decode are silently dropped: a single poison message must not stall the rest of the stream. Wrap the returned channel in your own goroutine if you need visibility into decode errors.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
