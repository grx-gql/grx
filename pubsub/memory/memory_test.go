package memory

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/grx-gql/grx/pubsub"
)

func TestMemoryPublishToSubscriber(t *testing.T) {
	bus := New()
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

func TestMemorySubscribeOnlyDeliversToTopic(t *testing.T) {
	bus := New()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chA, _ := bus.Subscribe(ctx, "message.posted.room-a")
	chB, _ := bus.Subscribe(ctx, "message.posted.room-b")

	_ = bus.Publish(context.Background(), "message.posted.room-a", []byte("alpha"))

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
	bus := New()
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
	bus := New(Config{Buffer: 1})
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
			t.Fatalf("expected first published payload, got %+v", msg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected at least one message")
	}
}

func TestMemoryCloseStopsSubscribers(t *testing.T) {
	bus := New()
	ctx := context.Background()

	const n = 5
	channels := make([]<-chan pubsub.Message, n)
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
		go func(ch <-chan pubsub.Message) {
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

	if err := bus.Publish(context.Background(), "topic", []byte("x")); !errors.Is(err, pubsub.ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
	if _, err := bus.Subscribe(context.Background(), "topic"); !errors.Is(err, pubsub.ErrClosed) {
		t.Fatalf("expected ErrClosed on Subscribe after Close, got %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got %v", err)
	}
}

func TestMemoryCloseWhileSubscribersCancel(t *testing.T) {
	bus := New()
	const n = 32
	channels := make([]<-chan pubsub.Message, n)
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
	bus := New(Config{Buffer: 64})
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const subscribers = 16
	const messages = 64
	channels := make([]<-chan pubsub.Message, subscribers)
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
			}
		}
		if count != messages {
			t.Fatalf("subscriber received %d/%d messages", count, messages)
		}
	}
}

func TestMemoryPublishConcurrentCloseDoesNotPanic(t *testing.T) {
	const (
		attempts    = 20
		publishers  = 8
		publishes   = 32
		subscribers = 1024
	)

	for attempt := 0; attempt < attempts; attempt++ {
		bus := New(Config{Buffer: publishers * publishes})
		for i := 0; i < subscribers; i++ {
			if _, err := bus.Subscribe(context.Background(), "topic"); err != nil {
				t.Fatalf("Subscribe: %v", err)
			}
		}

		start := make(chan struct{})
		panics := make(chan any, publishers)
		var wg sync.WaitGroup
		for i := 0; i < publishers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if recovered := recover(); recovered != nil {
						panics <- recovered
					}
				}()
				<-start
				for j := 0; j < publishes; j++ {
					_ = bus.Publish(context.Background(), "topic", []byte("payload"))
				}
			}()
		}

		close(start)
		if err := bus.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		wg.Wait()

		select {
		case recovered := <-panics:
			t.Fatalf("concurrent Publish and Close panicked: %v", recovered)
		default:
		}
	}
}
