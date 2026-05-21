package pubsub

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMemoryPublishToSubscriber(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), "topic", []byte("hi")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Topic != "topic" || string(msg.Payload) != "hi" {
			t.Fatalf("unexpected message %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber did not receive message")
	}
}

func TestMemoryFiltersGateDelivery(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	even := PayloadFunc(func(b []byte) bool {
		n, err := strconv.Atoi(string(b))
		return err == nil && n%2 == 0
	})

	ch, err := bus.Subscribe(ctx, "numbers", even)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for i := 1; i <= 4; i++ {
		if err := bus.Publish(context.Background(), "numbers", []byte(strconv.Itoa(i))); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	got := []string{}
	deadline := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case msg := <-ch:
			got = append(got, string(msg.Payload))
			if len(got) == 2 {
				break loop
			}
		case <-deadline:
			break loop
		}
	}

	if len(got) != 2 || got[0] != "2" || got[1] != "4" {
		t.Fatalf("expected only even payloads, got %v", got)
	}
}

func TestMemorySubscribeOnlyDeliversToTopic(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chA, _ := bus.Subscribe(ctx, "a")
	chB, _ := bus.Subscribe(ctx, "b")

	_ = bus.Publish(context.Background(), "a", []byte("alpha"))

	select {
	case msg := <-chA:
		if string(msg.Payload) != "alpha" {
			t.Fatalf("topic a got wrong payload %q", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("topic a did not receive message")
	}

	select {
	case msg := <-chB:
		t.Fatalf("topic b should not have received a message; got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMemorySubscribeUnsubscribesOnContextCancel(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := bus.Subscribe(ctx, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel closed after cancel, got value")
		}
	case <-time.After(time.Second):
		t.Fatalf("channel was not closed after context cancel")
	}
}

func TestMemorySlowSubscriberDropsMessages(t *testing.T) {
	bus := NewMemory(MemoryConfig{Buffer: 1})
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "t")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := bus.Publish(context.Background(), "t", []byte{byte(i)}); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	select {
	case msg := <-ch:
		if len(msg.Payload) != 1 || msg.Payload[0] != 0 {
			t.Fatalf("expected first published payload, got %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected at least one message")
	}
}

func TestMemoryCloseStopsSubscribers(t *testing.T) {
	bus := NewMemory()
	ctx := context.Background()

	const n = 5
	channels := make([]<-chan Message, n)
	for i := 0; i < n; i++ {
		ch, err := bus.Subscribe(ctx, "topic")
		if err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
		channels[i] = ch
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(ch <-chan Message) {
			defer wg.Done()
			for range ch {
			}
		}(ch)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Close did not drain subscribers")
	}

	if err := bus.Publish(context.Background(), "topic", []byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
	if _, err := bus.Subscribe(context.Background(), "topic"); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed on Subscribe after Close, got %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got %v", err)
	}
}

func TestMemoryCloseWhileSubscribersCancel(t *testing.T) {
	bus := NewMemory()
	const n = 32
	channels := make([]<-chan Message, n)
	var pendingCancel []context.CancelFunc
	defer func() {
		for _, c := range pendingCancel {
			c()
		}
	}()
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := bus.Subscribe(ctx, "topic")
		if err != nil {
			cancel()
			t.Fatalf("Subscribe: %v", err)
		}
		channels[i] = ch
		if i%2 == 0 {
			cancel()
		} else {
			pendingCancel = append(pendingCancel, cancel)
		}
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for _, ch := range channels {
		for range ch {
		}
	}
}

func TestMemoryConcurrentPublishSubscribe(t *testing.T) {
	bus := NewMemory(MemoryConfig{Buffer: 64})
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const subscribers = 16
	const messages = 64
	channels := make([]<-chan Message, subscribers)
	for i := 0; i < subscribers; i++ {
		ch, err := bus.Subscribe(ctx, "t")
		if err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
		channels[i] = ch
	}

	var pubWG sync.WaitGroup
	for i := 0; i < messages; i++ {
		pubWG.Add(1)
		go func(i int) {
			defer pubWG.Done()
			_ = bus.Publish(context.Background(), "t", []byte(strconv.Itoa(i)))
		}(i)
	}
	pubWG.Wait()

	deadline := time.Now().Add(2 * time.Second)
	for _, ch := range channels {
		count := 0
		for count < messages && time.Now().Before(deadline) {
			select {
			case _, ok := <-ch:
				if !ok {
					t.Fatalf("subscriber channel closed early")
				}
				count++
			case <-time.After(50 * time.Millisecond):
				if count == messages {

				}
			}
		}
		if count != messages {
			t.Fatalf("subscriber received %d/%d messages", count, messages)
		}
	}
}
