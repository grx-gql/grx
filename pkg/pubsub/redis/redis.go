// Package redis is a Redis-backed implementation of
// github.com/patrickkabwe/grx/pkg/pubsub.PubSub.
//
// It lives in a separate Go module so the root grx module remains
// dependency-free. Import it explicitly when you need cross-replica
// fan-out for GraphQL subscriptions:
//
//	import (
//	    redispubsub "github.com/patrickkabwe/grx/pkg/pubsub/redis"
//	    "github.com/redis/go-redis/v9"
//	)
//
//	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	bus, err := redispubsub.New(redispubsub.Config{Client: rdb, Prefix: "grx:"})
//
// The implementation uses standard Redis PUBLISH/SUBSCRIBE so it is
// fire-and-forget: published messages that have no live subscribers are
// dropped on the floor, matching Redis pub/sub semantics. Use Redis
// Streams (or another broker entirely) when delivery durability is
// required.
package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	pubsub "github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/redis/go-redis/v9"
)

// Config configures the Redis-backed [PubSub].
type Config struct {
	// Client is the underlying go-redis client. Required. The caller
	// owns its lifecycle; PubSub.Close does not close the client so
	// the same connection pool can be reused for other Redis
	// operations (cache, queues, etc.).
	Client redis.UniversalClient

	// Prefix is prepended to every topic before it is published or
	// subscribed. Use it to namespace channels alongside other
	// applications sharing the same Redis cluster (for example
	// "myapp:graphql:"). Optional.
	Prefix string

	// Buffer is the per-subscriber channel buffer. Slow consumers
	// whose buffer is full are dropped on Publish rather than blocking
	// the publisher. Defaults to 16 when zero or negative.
	Buffer int
}

// defaultBuffer is the fallback per-subscriber buffer size.
const defaultBuffer = 16

// PubSub is a Redis-backed implementation of [pubsub.PubSub]. It uses
// a long-lived go-redis pubsub connection per subscription which lets
// every Subscribe call receive its own filtered stream while sharing
// the application's Redis client pool.
//
// PubSub is safe for concurrent use.
type PubSub struct {
	client redis.UniversalClient
	prefix string
	buffer int

	mu     sync.Mutex
	closed bool
	subs   map[uint64]*subscription
	nextID atomic.Uint64
}

// subscription tracks one active Subscribe call so Close can tear it
// down deterministically.
type subscription struct {
	ps     *redis.PubSub
	cancel context.CancelFunc
	done   chan struct{}
}

// New constructs a [PubSub] from cfg. It returns an error when cfg is
// missing the required Client.
func New(cfg Config) (*PubSub, error) {
	if cfg.Client == nil {
		return nil, errors.New("redis pubsub: Config.Client is required")
	}
	if cfg.Buffer <= 0 {
		cfg.Buffer = defaultBuffer
	}
	return &PubSub{
		client: cfg.Client,
		prefix: cfg.Prefix,
		buffer: cfg.Buffer,
		subs:   map[uint64]*subscription{},
	}, nil
}

// Publish sends payload to the Redis channel that corresponds to topic.
// Returns [pubsub.ErrClosed] after Close. Network errors from go-redis
// are returned verbatim.
func (p *PubSub) Publish(ctx context.Context, topic string, payload []byte) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return pubsub.ErrClosed
	}
	p.mu.Unlock()

	if err := p.client.Publish(ctx, p.channel(topic), payload).Err(); err != nil {
		return fmt.Errorf("redis pubsub: publish %q: %w", topic, err)
	}
	return nil
}

// Subscribe creates a Redis subscription on topic and forwards every
// matching message to the returned channel. The channel is closed
// exactly once when ctx is cancelled or [PubSub.Close] is called, so
// callers can range over it safely.
func (p *PubSub) Subscribe(ctx context.Context, topic string, filters ...pubsub.Filter) (<-chan pubsub.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, pubsub.ErrClosed
	}
	p.mu.Unlock()

	channel := p.channel(topic)
	rps := p.client.Subscribe(ctx, channel)

	if _, err := rps.Receive(ctx); err != nil {
		_ = rps.Close()
		return nil, fmt.Errorf("redis pubsub: subscribe %q: %w", topic, err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	id := p.nextID.Add(1)
	sub := &subscription{ps: rps, cancel: cancel, done: make(chan struct{})}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		cancel()
		_ = rps.Close()
		return nil, pubsub.ErrClosed
	}
	p.subs[id] = sub
	p.mu.Unlock()

	out := make(chan pubsub.Message, p.buffer)
	go p.forward(subCtx, id, sub, topic, out, filters)
	return out, nil
}

// forward is the per-subscription goroutine. It reads from go-redis,
// applies filters, and pushes successful matches into out. It is the
// only writer to out and closes it before returning.
func (p *PubSub) forward(ctx context.Context, id uint64, sub *subscription, topic string, out chan pubsub.Message, filters []pubsub.Filter) {
	defer p.cleanup(id, sub, out)

	source := sub.ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-source:
			if !ok {
				return
			}
			msg := pubsub.Message{Topic: topic, Payload: []byte(raw.Payload)}
			if !matchAll(filters, msg) {
				continue
			}
			select {
			case out <- msg:
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// cleanup unregisters the subscription, closes the go-redis pubsub
// connection, and closes the subscriber channel. Safe to call multiple
// times.
func (p *PubSub) cleanup(id uint64, sub *subscription, out chan pubsub.Message) {
	select {
	case <-sub.done:
		return
	default:
	}
	close(sub.done)

	p.mu.Lock()
	delete(p.subs, id)
	p.mu.Unlock()

	_ = sub.ps.Close()
	close(out)
}

// Close cancels every active subscription and rejects further
// Publish/Subscribe calls with [pubsub.ErrClosed]. The underlying
// redis.Client is left untouched because it is owned by the caller.
// Close is safe to call more than once; the second and subsequent
// calls return nil.
func (p *PubSub) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	pending := make([]*subscription, 0, len(p.subs))
	for _, s := range p.subs {
		pending = append(pending, s)
	}
	p.subs = map[uint64]*subscription{}
	p.mu.Unlock()

	for _, s := range pending {
		s.cancel()
	}
	return nil
}

// channel returns the on-the-wire channel name for topic.
func (p *PubSub) channel(topic string) string {
	if p.prefix == "" {
		return topic
	}
	return p.prefix + topic
}

func matchAll(filters []pubsub.Filter, msg pubsub.Message) bool {
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

// Compile-time assurance that *PubSub satisfies the interface.
var _ pubsub.PubSub = (*PubSub)(nil)
