package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/grx-gql/grx/pubsub"
	redisps "github.com/grx-gql/grx/pubsub/redis"
	goredis "github.com/redis/go-redis/v9"
)

func newTestBus(t *testing.T, prefix string) (*redisps.PubSub, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	bus, err := redisps.New(redisps.Config{Client: rdb, Prefix: prefix})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus, mr
}

func TestNewRequiresClient(t *testing.T) {
	if _, err := redisps.New(redisps.Config{}); err == nil {
		t.Fatalf("expected error when Client is nil")
	}
}

func TestPublishSubscribeRoundTrip(t *testing.T) {
	bus, _ := newTestBus(t, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Subscribe(ctx, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), "topic", []byte("hello")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Topic != "topic" || string(msg.Payload) != "hello" {
			t.Fatalf("unexpected message %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("subscriber did not receive message")
	}
}

func TestPrefixIsAppliedOnTheWire(t *testing.T) {
	bus, mr := newTestBus(t, "grx:")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := bus.Subscribe(ctx, "events"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mr.PubSubNumSub("grx:events")["grx:events"] > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected subscriber on prefixed channel grx:events")
}

func TestSubscribeIsolatedByTopic(t *testing.T) {
	bus, _ := newTestBus(t, "")

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
	case <-time.After(2 * time.Second):
		t.Fatalf("room.a did not receive message")
	}

	select {
	case msg := <-chB:
		t.Fatalf("room.b should not receive message; got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSubscribeNilContextDefaultsToBackground(t *testing.T) {
	bus, _ := newTestBus(t, "")

	ch, err := bus.Subscribe(nil, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bus.Publish(context.Background(), "topic", []byte("ok")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-ch:
		if string(msg.Payload) != "ok" {
			t.Fatalf("got %q", msg.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("subscriber did not receive message")
	}
}

func TestSubscribeChannelClosedOnContextCancel(t *testing.T) {
	bus, _ := newTestBus(t, "")

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := bus.Subscribe(ctx, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("channel was not closed after cancel")
	}
}

func TestCloseRejectsFurtherCalls(t *testing.T) {
	bus, _ := newTestBus(t, "")

	if err := bus.Close(); err != nil {
		t.Fatalf("Close: %v", err)
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
