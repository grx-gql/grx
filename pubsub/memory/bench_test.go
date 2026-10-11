package memory

import (
	"context"
	"testing"
)

func BenchmarkPublishNoSubscribers(b *testing.B) {
	bus := New()
	b.Cleanup(func() { _ = bus.Close() })

	ctx := context.Background()
	payload := []byte("hello")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(ctx, "chat", payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishWithSubscriber(b *testing.B) {
	bus := New()
	b.Cleanup(func() { _ = bus.Close() })

	ctx := context.Background()
	ch, err := bus.Subscribe(ctx, "chat")
	if err != nil {
		b.Fatal(err)
	}
	go func() {
		for range ch {
		}
	}()

	payload := []byte("hello")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(ctx, "chat", payload); err != nil {
			b.Fatal(err)
		}
	}
}
