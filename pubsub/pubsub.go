// Package pubsub defines the publish/subscribe primitive used by GraphQL
// subscription resolvers to fan messages out to connected clients.
//
// The package centres on a small [PubSub] interface so the rest of the
// runtime can stay backend-agnostic. Implementations live in sibling
// packages (for example [github.com/grx-gql/grx/pubsub/memory] for
// in-process delivery and [github.com/grx-gql/grx/pubsub/redis] for
// cross-replica fan-out). Any third-party broker can be plugged in by
// satisfying [PubSub] — the executor and subscription transports never
// look at the concrete type.
//
// Messages are byte-oriented on the wire. Application code typically
// JSON-encodes domain values at publish time and decodes in subscription
// resolvers; the bus itself does not marshal or unmarshal.
//
// Filtering is topic-based: [Subscribe] registers on an exact topic and
// receives only messages published to that topic. Use hierarchical topic
// names (for example message.posted.room-1) to scope streams.
package pubsub

import (
	"context"
	"errors"
)

// ErrClosed is returned by [PubSub] implementations once [PubSub.Close]
// has been called. Subsequent [PubSub.Publish] and [PubSub.Subscribe]
// calls must surface this error so callers can fail fast.
var ErrClosed = errors.New("pubsub: closed")

// Message is the envelope delivered to subscribers. Topic is the channel
// the message was published on and Payload is the raw wire bytes.
//
// Payload ownership: after [PubSub.Publish] returns, the publisher must
// not mutate payload or reuse its backing array (for example by returning
// it to a [sync.Pool]). Subscribers must treat Payload as read-only;
// multiple subscribers may share the same backing array. Copy the bytes
// when you need to retain or modify them after receive.
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
	//
	// The caller must not mutate payload after Publish returns; see
	// [Message] for the ownership contract shared by all backends.
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe registers a consumer on topic and returns a stream of
	// messages published to that topic. The returned channel is closed
	// exactly once when ctx is cancelled or the PubSub is closed, so
	// callers may range over it safely.
	Subscribe(ctx context.Context, topic string) (<-chan Message, error)

	// Close releases resources held by the implementation, cancels in-
	// flight subscriptions, and prevents further Publish/Subscribe
	// calls from succeeding. Close must be safe to call more than
	// once; the second and subsequent calls return nil.
	Close() error
}
