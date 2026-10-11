// Package memory is an in-process implementation of
// [github.com/grx-gql/grx/pubsub.PubSub].
package memory

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/grx-gql/grx/pubsub"
)

// Config tunes the in-process implementation.
type Config struct {
	// Buffer is the per-subscriber channel buffer size. A subscriber
	// whose buffer is full is dropped on Publish rather than blocking
	// the publisher; raise this when bursts are expected. Defaults to
	// 16 when zero or negative.
	Buffer int
}

const defaultBuffer = 16

// memoryPubSub is a single-process [pubsub.PubSub]. Messages are delivered to
// local subscribers without serialization. [memoryPubSub.Publish] forwards the
// payload slice without copying; callers and subscribers must honour the
// [pubsub.Message] ownership rules. This is the fastest backend and the
// right default for development, tests, and single-replica deployments.
// Swap in [github.com/grx-gql/grx/pubsub/redis] (or any other [pubsub.PubSub]
// implementation) to fan messages across replicas.
//
// memoryPubSub is safe for concurrent use.
type memoryPubSub struct {
	buffer int

	mu     sync.RWMutex
	closed bool
	subs   map[string]map[uint64]*memorySub

	nextID atomic.Uint64
}

type memorySub struct {
	ch        chan pubsub.Message
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// New returns a [memoryPubSub] ready for use. Pass an optional [Config] to
// override the per-subscriber buffer size; only the first config is
// honoured so callers can use New() with no arguments.
func New(cfgs ...Config) *memoryPubSub {
	cfg := Config{}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	if cfg.Buffer <= 0 {
		cfg.Buffer = defaultBuffer
	}
	return &memoryPubSub{
		buffer: cfg.Buffer,
		subs:   map[string]map[uint64]*memorySub{},
	}
}

// Publish delivers payload to every active subscriber of topic. Slow
// subscribers whose buffer is full are skipped rather than blocking
// the publisher.
func (m *memoryPubSub) Publish(_ context.Context, topic string, payload []byte) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return pubsub.ErrClosed
	}
	subs := m.subs[topic]
	if len(subs) == 0 {
		m.mu.RUnlock()
		return nil
	}

	msg := pubsub.Message{Topic: topic, Payload: payload}
	for _, s := range subs {
		select {
		case s.ch <- msg:
		default:
		}
	}
	m.mu.RUnlock()
	return nil
}

// Subscribe registers a consumer on topic until ctx is cancelled or
// [memoryPubSub.Close] is called.
func (m *memoryPubSub) Subscribe(ctx context.Context, topic string) (<-chan pubsub.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, pubsub.ErrClosed
	}
	id := m.nextID.Add(1)
	subCtx, cancel := context.WithCancel(ctx)
	sub := &memorySub{
		ch:     make(chan pubsub.Message, m.buffer),
		cancel: cancel,
	}
	if _, ok := m.subs[topic]; !ok {
		m.subs[topic] = map[uint64]*memorySub{}
	}
	m.subs[topic][id] = sub
	m.mu.Unlock()

	go func() {
		<-subCtx.Done()
		m.unsubscribe(topic, id, sub)
	}()

	return sub.ch, nil
}

func (m *memoryPubSub) unsubscribe(topic string, id uint64, sub *memorySub) {
	m.mu.Lock()
	if subs, ok := m.subs[topic]; ok {
		if existing, ok := subs[id]; ok && existing == sub {
			delete(subs, id)
			if len(subs) == 0 {
				delete(m.subs, topic)
			}
		}
	}
	m.mu.Unlock()

	sub.closeOnce.Do(func() {
		close(sub.ch)
	})
}

// Close cancels every active subscription and refuses further
// Publish/Subscribe calls.
func (m *memoryPubSub) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	pending := make([]struct {
		topic string
		id    uint64
		sub   *memorySub
	}, 0)
	for topic, subs := range m.subs {
		for id, sub := range subs {
			pending = append(pending, struct {
				topic string
				id    uint64
				sub   *memorySub
			}{topic, id, sub})
		}
	}
	m.subs = map[string]map[uint64]*memorySub{}
	m.mu.Unlock()

	for _, p := range pending {
		p.sub.cancel()
		m.unsubscribe(p.topic, p.id, p.sub)
	}
	return nil
}

var _ pubsub.PubSub = (*memoryPubSub)(nil)
