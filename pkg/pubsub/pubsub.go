// Package pubsub defines the publish/subscribe primitive used by GraphQL
// subscription resolvers to fan messages out to connected clients.
//
// The package centres on a small [PubSub] interface so the rest of the
// runtime can stay backend-agnostic. Two things ship in the standard
// library:
//
//   - [Memory] — an in-process implementation suitable for development,
//     tests, and single-replica deployments.
//   - [Typed] — a generic wrapper that adds type-safe Publish and
//     Subscribe operations on top of any [PubSub] using a pluggable
//     [Codec]. Resolvers should normally talk to a Typed[T] rather than
//     to the raw byte-oriented PubSub.
//
// Distributed backends are intentionally kept out of this package so the
// root module stays dependency-free; see the
// github.com/patrickkabwe/grx/pkg/pubsub/redis sub-module for a
// production-ready Redis implementation. Any third-party broker can be
// plugged in by satisfying the [PubSub] interface — the executor and
// subscription transports never look at the concrete type.
//
// # Filters
//
// [Subscribe] accepts zero or more [Filter] values. Filters run on the
// publishing side of the channel so uninteresting messages never wake
// the consumer goroutine. Use [TopicEquals] / [TopicHasPrefix] for
// topic-based gating, [PayloadFunc] for byte inspection, or compose with
// [All] / [Any]. Typed subscribers can use simpler [func(T) bool]
// predicates, see [Typed.Subscribe].
package pubsub

import (
	"context"
	"errors"
	"strings"
)

// ErrClosed is returned by [PubSub] implementations once [PubSub.Close]
// has been called. Subsequent [PubSub.Publish] and [PubSub.Subscribe]
// calls must surface this error so callers can fail fast.
var ErrClosed = errors.New("pubsub: closed")

// Message is the envelope delivered to subscribers. Topic is the channel
// the message was published on (useful when a Filter or a wildcard
// subscription sees several topics) and Payload is the raw wire bytes —
// implementations must not mutate it after delivery.
type Message struct {
	Topic   string
	Payload []byte
}

// PubSub is a transport-agnostic publish/subscribe primitive.
// Implementations must be safe for concurrent use by multiple goroutines.
//
// A PubSub is the contract between mutation resolvers (which Publish
// domain events) and subscription resolvers (which Subscribe to those
// events and forward them to connected GraphQL clients). The semantics
// match Redis PUBLISH/SUBSCRIBE: messages are not persisted, slow
// consumers may drop messages rather than block publishers, and
// subscribers receive messages published after they subscribe.
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

// Filter decides whether a Message should be delivered to a subscriber.
// Filters run on the publish path, so they should be cheap and free of
// side effects.
type Filter interface {
	Matches(msg Message) bool
}

// FilterFunc adapts a plain predicate to the [Filter] interface.
type FilterFunc func(Message) bool

// Matches implements [Filter].
func (f FilterFunc) Matches(msg Message) bool { return f(msg) }

// PayloadFunc returns a [Filter] that runs fn against the raw payload
// bytes. It is a convenience for the common case of inspecting only the
// payload.
func PayloadFunc(fn func([]byte) bool) Filter {
	return FilterFunc(func(msg Message) bool { return fn(msg.Payload) })
}

// TopicEquals returns a [Filter] that matches messages whose Topic is
// exactly equal to topic. Useful when subscribing to a parent topic and
// wanting to gate by a specific child.
func TopicEquals(topic string) Filter {
	return FilterFunc(func(msg Message) bool { return msg.Topic == topic })
}

// TopicHasPrefix returns a [Filter] that matches messages whose Topic
// starts with prefix. Combine with [Memory] / [redis.PubSub] wildcard
// subscriptions to implement hierarchical channels.
func TopicHasPrefix(prefix string) Filter {
	return FilterFunc(func(msg Message) bool { return strings.HasPrefix(msg.Topic, prefix) })
}

// All returns a [Filter] that matches when every provided filter
// matches. With zero filters, the returned filter always matches.
func All(filters ...Filter) Filter {
	if len(filters) == 0 {
		return FilterFunc(func(Message) bool { return true })
	}
	return FilterFunc(func(msg Message) bool {
		for _, f := range filters {
			if f == nil {
				continue
			}
			if !f.Matches(msg) {
				return false
			}
		}
		return true
	})
}

// Any returns a [Filter] that matches when at least one provided filter
// matches. With zero filters, the returned filter never matches.
func Any(filters ...Filter) Filter {
	if len(filters) == 0 {
		return FilterFunc(func(Message) bool { return false })
	}
	return FilterFunc(func(msg Message) bool {
		for _, f := range filters {
			if f == nil {
				continue
			}
			if f.Matches(msg) {
				return true
			}
		}
		return false
	})
}

// matchAll evaluates filters against msg with the same nil-safe
// semantics as [All] but without allocating a wrapper closure on the
// hot publish path. Implementations may use it directly.
func matchAll(filters []Filter, msg Message) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		if !f.Matches(msg) {
			return false
		}
	}
	return true
}
