package pubsub

import (
	"context"
	"encoding/json"
)

// Codec converts values of type T to and from the byte payloads carried
// by [PubSub]. It exists so [Typed] can interoperate with any backend
// that speaks bytes (Redis, NATS, Kafka, in-process Memory, etc.) while
// callers work with strongly typed Go values.
type Codec[T any] interface {
	Encode(value T) ([]byte, error)
	Decode(data []byte) (T, error)
}

// JSONCodec encodes values with [encoding/json]. It is the default
// codec for [Typed] and the right choice for almost every GraphQL
// payload because the values are already trees of strings, numbers,
// and structs.
type JSONCodec[T any] struct{}

// Encode marshals value to JSON.
func (JSONCodec[T]) Encode(value T) ([]byte, error) { return json.Marshal(value) }

// Decode unmarshals data into a fresh T.
func (JSONCodec[T]) Decode(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

// Typed wraps a [PubSub] with a [Codec] to provide a type-safe API for
// a single message type. Resolvers should normally talk to a Typed[T]
// rather than the raw PubSub: it removes the marshal/unmarshal
// boilerplate from every Publish and Subscribe call site, and the
// returned channel carries decoded T values directly.
//
// Typed is safe for concurrent use; it stores no per-call state. It is
// also cheap to construct, so the conventional pattern is to create
// one near the resolver that owns the topic and pass it around like a
// repository or service.
type Typed[T any] struct {
	bus   PubSub
	codec Codec[T]
}

// NewTyped wraps bus with the default [JSONCodec] for T.
func NewTyped[T any](bus PubSub) *Typed[T] {
	return NewTypedWith[T](bus, JSONCodec[T]{})
}

// NewTypedWith wraps bus with the supplied Codec. Use it when JSON is
// the wrong choice for T (for example, when interoperating with a
// non-Go publisher that already uses a fixed binary format).
func NewTypedWith[T any](bus PubSub, codec Codec[T]) *Typed[T] {
	if codec == nil {
		codec = Codec[T](JSONCodec[T]{})
	}
	return &Typed[T]{bus: bus, codec: codec}
}

// Bus returns the underlying [PubSub]. Useful when callers need to
// reach through Typed for backend-specific features (e.g. graceful
// shutdown via Close).
func (t *Typed[T]) Bus() PubSub { return t.bus }

// Publish encodes value and forwards it to the underlying [PubSub].
// The codec error is returned without publishing if encoding fails,
// so a misconfigured codec never produces a corrupt frame on the wire.
func (t *Typed[T]) Publish(ctx context.Context, topic string, value T) error {
	payload, err := t.codec.Encode(value)
	if err != nil {
		return err
	}
	return t.bus.Publish(ctx, topic, payload)
}

// Subscribe registers a typed consumer on topic and returns a channel
// of decoded T values. The variadic predicates are evaluated against
// the decoded value, so they can use the full Go type system rather
// than poking at raw bytes. Backend-level [Filter]s (which see the
// undecoded message) are not exposed here; use [Typed.Bus] +
// [PubSub.Subscribe] when raw filtering is required.
//
// Messages whose payload fails to decode are silently dropped: a single
// poison message must not stall the rest of the stream. Wrap the
// returned channel in your own goroutine if you need visibility into
// decode errors.
func (t *Typed[T]) Subscribe(ctx context.Context, topic string, predicates ...func(T) bool) (<-chan T, error) {
	source, err := t.bus.Subscribe(ctx, topic)
	if err != nil {
		return nil, err
	}
	out := make(chan T)
	go func() {
		defer close(out)
		for msg := range source {
			value, err := t.codec.Decode(msg.Payload)
			if err != nil {
				continue
			}
			if !matchAllTyped(predicates, value) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- value:
			}
		}
	}()
	return out, nil
}

func matchAllTyped[T any](predicates []func(T) bool, value T) bool {
	for _, p := range predicates {
		if p == nil {
			continue
		}
		if !p(value) {
			return false
		}
	}
	return true
}
