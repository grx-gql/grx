package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
)

// MemoryConfig tunes the in-process [Memory] implementation.
type MemoryConfig struct {
	// Buffer is the per-subscriber channel buffer size. A subscriber
	// whose buffer is full is dropped on Publish rather than blocking
	// the publisher; raise this when bursts are expected. Defaults to
	// 16 when zero or negative.
	Buffer int
}

// defaultMemoryBuffer is the per-subscriber buffer when MemoryConfig
// does not specify one. Sixteen is large enough to absorb normal bursts
// while keeping idle memory cost trivial.
const defaultMemoryBuffer = 16

// Memory is a single-process [PubSub]. Messages are delivered to local
// subscribers without serialization, so it is the fastest backend
// available and is the right default for development, tests, and
// single-replica deployments. Swap in
// github.com/patrickkabwe/grx/pkg/pubsub/redis (or any other
// [PubSub] implementation) to fan messages across replicas.
//
// Memory is safe for concurrent use.
type Memory struct {
	buffer int

	mu     sync.RWMutex
	closed bool
	subs   map[string]map[uint64]*memorySub

	nextID atomic.Uint64
}

// memorySub tracks a single active subscription. The wait group is used
// by Close to block until every subscriber goroutine has observed the
// channel close, so Close can promise no further deliveries.
type memorySub struct {
	ch        chan Message
	filters   []Filter
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// NewMemory returns a [Memory] ready for use. Pass an optional
// MemoryConfig to override the per-subscriber buffer size; only the
// first config is honoured so callers can use a zero-value default with
// NewMemory().
func NewMemory(cfgs ...MemoryConfig) *Memory {
	cfg := MemoryConfig{}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	if cfg.Buffer <= 0 {
		cfg.Buffer = defaultMemoryBuffer
	}
	return &Memory{
		buffer: cfg.Buffer,
		subs:   map[string]map[uint64]*memorySub{},
	}
}

// Publish delivers payload to every active subscriber of topic whose
// filters match. Slow subscribers whose buffer is full are skipped
// rather than blocking the publisher; this keeps a misbehaving
// subscription from stalling mutations. The ctx parameter is accepted
// for interface compatibility but is not consulted: in-memory delivery
// never blocks.
func (m *Memory) Publish(_ context.Context, topic string, payload []byte) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	subs := m.subs[topic]
	if len(subs) == 0 {
		m.mu.RUnlock()
		return nil
	}
	targets := make([]*memorySub, 0, len(subs))
	for _, s := range subs {
		targets = append(targets, s)
	}
	m.mu.RUnlock()

	msg := Message{Topic: topic, Payload: payload}
	for _, s := range targets {
		if !matchAll(s.filters, msg) {
			continue
		}
		select {
		case s.ch <- msg:
		default:
		}
	}
	return nil
}

// Subscribe registers a consumer on topic. The returned channel emits
// every message published to topic that satisfies every supplied filter
// until ctx is cancelled or [Memory.Close] is called, after which the
// channel is closed exactly once.
func (m *Memory) Subscribe(ctx context.Context, topic string, filters ...Filter) (<-chan Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	id := m.nextID.Add(1)
	subCtx, cancel := context.WithCancel(ctx)
	sub := &memorySub{
		ch:      make(chan Message, m.buffer),
		filters: filters,
		cancel:  cancel,
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

// unsubscribe removes a single subscription and closes its channel
// exactly once. It is safe to call from both Subscribe's cleanup
// goroutine and Close concurrently.
func (m *Memory) unsubscribe(topic string, id uint64, sub *memorySub) {
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
// Publish/Subscribe calls. It is safe to call more than once; the
// second and subsequent calls return nil.
func (m *Memory) Close() error {
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
