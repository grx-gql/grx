package graph_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/examples/sse/graph"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/pubsub/memory"
	"github.com/grx-gql/grx/schema"
)

func newExecutor(t *testing.T) core.Executor {
	t.Helper()
	bus := memory.New()
	t.Cleanup(func() { _ = bus.Close() })
	compiled, err := schema.Build(graph.Schema(bus))
	if err != nil {
		t.Fatal(err)
	}
	return exec.New(compiled, nil)
}

func TestNotificationsStreamSentNotifications(t *testing.T) {
	executor := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream, err := executor.Subscribe(ctx, core.Request{Query: `subscription { notifications(channel: "ops") { text } }`})
	if err != nil {
		t.Fatal(err)
	}
	response := executor.Execute(ctx, core.Request{Query: `mutation { sendNotification(input: {channel: "ops", text: "deploy done"}) { id } }`})
	if len(response.Errors) != 0 || response.Data == nil {
		t.Fatalf("mutation: %+v", response)
	}

	select {
	case event, ok := <-stream:
		if !ok || len(event.Errors) != 0 {
			t.Fatalf("subscription closed or failed: %+v", event)
		}
		encoded, err := json.Marshal(event.Data)
		if err != nil {
			t.Fatal(err)
		}
		if want := `{"notifications":{"text":"deploy done"}}`; string(encoded) != want {
			t.Fatalf("got %s, want %s", encoded, want)
		}
	case <-ctx.Done():
		t.Fatal("subscription did not receive the notification")
	}
}

func TestCountdownCountsToZeroThenCompletes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	sub := graph.CountdownSubscription{Interval: time.Millisecond}
	ticks, err := sub.Countdown(ctx, graph.CountdownArgs{From: 3})
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for tick := range ticks {
		got = append(got, tick.Remaining)
	}
	if want := []int{3, 2, 1, 0}; len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}
}

func TestCountdownRejectsOutOfRangeStart(t *testing.T) {
	for _, from := range []int{-1, 61} {
		if _, err := (graph.CountdownSubscription{}).Countdown(context.Background(), graph.CountdownArgs{From: from}); err == nil {
			t.Errorf("from=%d: expected an error", from)
		}
	}
}

func TestSendNotificationRequiresChannelAndText(t *testing.T) {
	executor := newExecutor(t)
	for _, query := range []string{
		`mutation { sendNotification(input: {channel: " ", text: "x"}) { id } }`,
		`mutation { sendNotification(input: {channel: "ops", text: " "}) { id } }`,
	} {
		if response := executor.Execute(context.Background(), core.Request{Query: query}); len(response.Errors) == 0 {
			t.Errorf("%s: expected an error", query)
		}
	}
}
