package graph_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/examples/subscriptions/graph"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/pubsub/memory"
	"github.com/grx-gql/grx/schema"
)

func TestSchemaWithNilBusOmitsEventResolvers(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		config schema.Config
	}{
		{name: "nil", config: graph.Schema(nil)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			compiled, err := schema.Build(scenario.config)
			if err != nil {
				t.Fatal(err)
			}
			if compiled.Query == nil || compiled.Mutation == nil {
				t.Fatal("expected queries and non-event mutations")
			}
			if compiled.Subscription != nil {
				t.Error("subscriptions must be absent without a bus")
			}
			for _, name := range []string{"createUser", "postMessage"} {
				if _, exists := compiled.Mutation.Fields[name]; exists {
					t.Errorf("event mutation %s must be absent without a bus", name)
				}
			}
			executor := exec.New(compiled, nil)
			for _, query := range []string{
				`{ user(id: "user_1") { id name } }`,
				`mutation { createPost(input: {title: "Hello", body: "World", authorId: "user_1"}) { post { id title } } }`,
			} {
				response := executor.Execute(context.Background(), core.Request{Query: query})
				if len(response.Errors) != 0 || response.Data == nil {
					t.Fatalf("query %s: %+v", query, response)
				}
			}
			for _, query := range []string{
				`mutation { createUser(input: {name: "Ada"}) { user { id } } }`,
				`mutation { postMessage(input: {roomId: "general", author: "Ada", body: "Hello"}) { message { id } } }`,
			} {
				response := executor.Execute(context.Background(), core.Request{Query: query})
				if len(response.Errors) == 0 {
					t.Fatalf("event mutation should fail validation: %s", query)
				}
				for _, issue := range response.Errors {
					if issue.Extensions["code"] != "GRAPHQL_VALIDATION_FAILED" {
						t.Errorf("expected validation error, got %+v", issue)
					}
				}
			}
		})
	}
}

func TestSchemaWithBusExecutesEventMutationsAndSubscriptions(t *testing.T) {
	bus := memory.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Error(err)
		}
	})
	compiled, err := schema.Build(graph.Schema(bus))
	if err != nil {
		t.Fatal(err)
	}
	executor := exec.New(compiled, nil)
	for _, scenario := range []struct {
		name         string
		subscription string
		mutation     string
		want         string
	}{
		{name: "user", subscription: `subscription { userCreated { name } }`, mutation: `mutation { createUser(input: {name: "Ada"}) { user { name } } }`, want: `{"userCreated":{"name":"Ada"}}`},
		{name: "message", subscription: `subscription { messagePosted(roomId: "general") { body } }`, mutation: `mutation { postMessage(input: {roomId: "general", author: "Ada", body: "Hello"}) { message { body } } }`, want: `{"messagePosted":{"body":"Hello"}}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			stream, err := executor.Subscribe(ctx, core.Request{Query: scenario.subscription})
			if err != nil {
				t.Fatal(err)
			}
			response := executor.Execute(ctx, core.Request{Query: scenario.mutation})
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
				if string(encoded) != scenario.want {
					t.Fatalf("got %s, want %s", encoded, scenario.want)
				}
			case <-ctx.Done():
				t.Fatal("subscription did not receive the mutation event")
			}
		})
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
