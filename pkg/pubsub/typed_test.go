package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"
)

type chatEvent struct {
	Room string `json:"room"`
	Body string `json:"body"`
}

func TestTypedRoundTrip(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	typed := NewTyped[chatEvent](bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := typed.Subscribe(ctx, "chat")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	want := chatEvent{Room: "general", Body: "hello"}
	if err := typed.Publish(context.Background(), "chat", want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber did not receive event")
	}
}

func TestTypedPredicatesFilterOnDecodedValue(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	typed := NewTyped[chatEvent](bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := typed.Subscribe(ctx, "chat", func(e chatEvent) bool {
		return e.Room == "general"
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for _, e := range []chatEvent{
		{Room: "random", Body: "noise"},
		{Room: "general", Body: "hi"},
		{Room: "random", Body: "more noise"},
		{Room: "general", Body: "bye"},
	} {
		if err := typed.Publish(context.Background(), "chat", e); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	got := []chatEvent{}
	deadline := time.After(time.Second)
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
		t.Fatalf("unexpected events delivered: %+v", got)
	}
}

func TestTypedPoisonMessageDropped(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	typed := NewTyped[chatEvent](bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := typed.Subscribe(ctx, "chat")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), "chat", []byte("not json")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := typed.Publish(context.Background(), "chat", chatEvent{Room: "g", Body: "ok"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-ch:
		if got.Body != "ok" {
			t.Fatalf("expected only the well-formed event, got %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber stalled after poison message")
	}
}

type failingCodec struct{}

func (failingCodec) Encode(int) ([]byte, error) { return nil, errors.New("encode boom") }
func (failingCodec) Decode([]byte) (int, error) { return 0, errors.New("decode boom") }

func TestTypedPublishSurfacesEncodeError(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	typed := NewTypedWith[int](bus, failingCodec{})
	if err := typed.Publish(context.Background(), "t", 1); err == nil {
		t.Fatalf("expected encode error")
	}
}

func TestTypedBusReturnsUnderlyingBus(t *testing.T) {
	bus := NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	typed := NewTyped[chatEvent](bus)
	if typed.Bus() != bus {
		t.Fatalf("expected Bus() to return underlying PubSub")
	}
}
