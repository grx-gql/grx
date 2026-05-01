package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	pubsub "github.com/patrickkabwe/grx/pkg/pubsub"
	redispubsub "github.com/patrickkabwe/grx/pkg/pubsub/redis"
	"github.com/redis/go-redis/v9"
)

func newTestBus(t *testing.T, prefix string) (*redispubsub.PubSub, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	bus, err := redispubsub.New(redispubsub.Config{Client: rdb, Prefix: prefix})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus, mr
}

func TestNewRequiresClient(t *testing.T) {
	if _, err := redispubsub.New(redispubsub.Config{}); err == nil {
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

func TestFiltersGateDelivery(t *testing.T) {
	bus, _ := newTestBus(t, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	only := pubsub.PayloadFunc(func(b []byte) bool { return string(b) == "ok" })

	ch, err := bus.Subscribe(ctx, "topic", only)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for _, p := range []string{"skip", "ok", "skip", "ok"} {
		if err := bus.Publish(context.Background(), "topic", []byte(p)); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	got := []string{}
	deadline := time.After(2 * time.Second)
loop:
	for len(got) < 2 {
		select {
		case msg := <-ch:
			got = append(got, string(msg.Payload))
		case <-deadline:
			break loop
		}
	}

	if len(got) != 2 || got[0] != "ok" || got[1] != "ok" {
		t.Fatalf("expected only ok messages, got %v", got)
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

func TestTypedRoundTripOverRedis(t *testing.T) {
	bus, _ := newTestBus(t, "")

	type chatEvent struct {
		Room string `json:"room"`
		Body string `json:"body"`
	}

	typed := pubsub.NewTyped[chatEvent](bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := typed.Subscribe(ctx, "chat", func(e chatEvent) bool { return e.Room == "general" })
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for _, e := range []chatEvent{
		{Room: "random", Body: "noise"},
		{Room: "general", Body: "hi"},
		{Room: "general", Body: "bye"},
	} {
		if err := typed.Publish(context.Background(), "chat", e); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	got := []chatEvent{}
	deadline := time.After(2 * time.Second)
loop:
	for len(got) < 2 {
		select {
		case e := <-ch:
			got = append(got, e)
		case <-deadline:
			break loop
		}
	}

	if len(got) != 2 || got[0].Body != "hi" || got[1].Body != "bye" {
		t.Fatalf("unexpected events %+v", got)
	}
}
