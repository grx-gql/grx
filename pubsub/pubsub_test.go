package pubsub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testBus struct {
	mu     sync.Mutex
	closed bool
	subs   map[string]map[uint64]chan Message
	nextID atomic.Uint64
}

func newTestBus() *testBus {
	return &testBus{subs: map[string]map[uint64]chan Message{}}
}

func (b *testBus) Publish(_ context.Context, topic string, payload []byte) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	group := b.subs[topic]
	targets := make([]chan Message, 0, len(group))
	for _, ch := range group {
		targets = append(targets, ch)
	}
	b.mu.Unlock()

	msg := Message{Topic: topic, Payload: payload}
	for _, ch := range targets {
		select {
		case ch <- msg:
		default:
		}
	}
	return nil
}

func (b *testBus) Subscribe(ctx context.Context, topic string) (<-chan Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrClosed
	}
	id := b.nextID.Add(1)
	ch := make(chan Message, 8)
	if b.subs[topic] == nil {
		b.subs[topic] = map[uint64]chan Message{}
	}
	b.subs[topic][id] = ch
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if subs, ok := b.subs[topic]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.subs, topic)
			}
		}
		b.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

func (b *testBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	pending := make([]chan Message, 0)
	for _, subs := range b.subs {
		for _, ch := range subs {
			pending = append(pending, ch)
		}
	}
	b.subs = map[string]map[uint64]chan Message{}
	b.mu.Unlock()

	for _, ch := range pending {
		close(ch)
	}
	return nil
}

func TestPublishSubscribeRoundTrip(t *testing.T) {
	bus := newTestBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "chat")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	want := []byte("hello")
	if err := bus.Publish(context.Background(), "chat", want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Topic != "chat" || string(msg.Payload) != string(want) {
			t.Fatalf("unexpected message %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber did not receive event")
	}
}

func TestSubscribeIsolatedByTopic(t *testing.T) {
	bus := newTestBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chA, err := bus.Subscribe(ctx, "room.a")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	chB, err := bus.Subscribe(ctx, "room.b")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), "room.a", []byte("alpha")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-chA:
		if string(msg.Payload) != "alpha" {
			t.Fatalf("room.a got %q", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("room.a did not receive message")
	}

	select {
	case msg := <-chB:
		t.Fatalf("room.b should not receive message; got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPublishWithNoSubscribers(t *testing.T) {
	bus := newTestBus()
	if err := bus.Publish(context.Background(), "empty", []byte("x")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

func TestCloseRejectsFurtherCalls(t *testing.T) {
	bus := newTestBus()
	if err := bus.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := bus.Publish(context.Background(), "topic", []byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed on Publish, got %v", err)
	}
	if _, err := bus.Subscribe(context.Background(), "topic"); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed on Subscribe, got %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got %v", err)
	}
}
