package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/grx-gql/grx/examples/subscriptions/graph"
	"github.com/grx-gql/grx/pubsub/memory"
)

func TestNewWithoutBusOmitsEventResolvers(t *testing.T) {
	config := graph.New()
	if config.Query == nil {
		t.Fatal("expected query resolver")
	}
	if config.Mutation == nil {
		t.Fatal("expected non-event mutation resolver")
	}
	if config.Subscription != nil {
		t.Fatal("expected subscriptions to be omitted without a bus")
	}
}

func TestUserCreatedPublishSubscribe(t *testing.T) {
	bus := memory.New()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := graph.UserSubscription{Bus: bus}
	users, err := sub.UserCreated(ctx)
	if err != nil {
		t.Fatalf("UserCreated: %v", err)
	}

	mut := &graph.UserMutation{Bus: bus}
	payload, err := mut.CreateUser(ctx, graph.UserCreateArgs{
		Input: graph.UserCreateInput{Name: "Ada"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	select {
	case user := <-users:
		if user == nil || user.ID != payload.User.ID || user.Name != "Ada" {
			t.Fatalf("unexpected user %+v", user)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscription did not receive user")
	}
}

func TestMessagePostedUsesRoomTopics(t *testing.T) {
	bus := memory.New()
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subA := graph.MessageSubscription{Bus: bus}
	streamA, err := subA.MessagePosted(ctx, graph.MessagePostedArgs{RoomID: "room-a"})
	if err != nil {
		t.Fatalf("MessagePosted room-a: %v", err)
	}

	subB := graph.MessageSubscription{Bus: bus}
	streamB, err := subB.MessagePosted(ctx, graph.MessagePostedArgs{RoomID: "room-b"})
	if err != nil {
		t.Fatalf("MessagePosted room-b: %v", err)
	}

	mut := &graph.MessageMutation{Bus: bus}
	if _, err := mut.PostMessage(ctx, graph.PostMessageArgs{
		Input: graph.PostMessageInput{RoomID: "room-a", Author: "alice", Body: "hi a"},
	}); err != nil {
		t.Fatalf("PostMessage room-a: %v", err)
	}
	if _, err := mut.PostMessage(ctx, graph.PostMessageArgs{
		Input: graph.PostMessageInput{RoomID: "room-b", Author: "bob", Body: "hi b"},
	}); err != nil {
		t.Fatalf("PostMessage room-b: %v", err)
	}

	select {
	case msg := <-streamA:
		if msg.RoomID != "room-a" || msg.Body != "hi a" {
			t.Fatalf("room-a stream got %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("room-a did not receive message")
	}

	select {
	case msg := <-streamB:
		if msg.RoomID != "room-b" || msg.Body != "hi b" {
			t.Fatalf("room-b stream got %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("room-b did not receive message")
	}

	select {
	case msg := <-streamA:
		t.Fatalf("room-a should only receive one message; got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMessagePostedRequiresRoomID(t *testing.T) {
	bus := memory.New()
	t.Cleanup(func() { _ = bus.Close() })

	sub := graph.MessageSubscription{Bus: bus}
	if _, err := sub.MessagePosted(context.Background(), graph.MessagePostedArgs{}); err == nil {
		t.Fatal("expected error for empty roomId")
	}
}
