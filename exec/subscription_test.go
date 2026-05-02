package exec

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type testSubscription struct {
	source <-chan *testUser
}

func (s testSubscription) UserCreated(ctx context.Context) (<-chan *testUser, error) {
	return s.source, nil
}

func newTestSubscriptionExecutor(t *testing.T, source <-chan *testUser) *Executor {
	t.Helper()

	schemaValue, err := schema.Build(schema.Config{
		Query:        testQuery{},
		Subscription: testSubscription{source: source},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return New(schemaValue, nil)
}

func TestExecutorSubscribeStreamsResponses(t *testing.T) {
	source := make(chan *testUser, 2)
	source <- &testUser{ID: "1", Name: "Ada"}
	source <- &testUser{ID: "2", Name: "Grace"}
	close(source)

	executor := newTestSubscriptionExecutor(t, source)

	stream, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription { userCreated { id name } }`,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	expectedNames := []string{"Ada", "Grace"}
	for index, expected := range expectedNames {
		select {
		case res, ok := <-stream:
			if !ok {
				t.Fatalf("expected %d responses, channel closed early", len(expectedNames))
			}
			if len(res.Errors) != 0 {
				t.Fatalf("unexpected errors at index %d: %#v", index, res.Errors)
			}
			payload := responseObject(t, res.Data)
			user := responseObject(t, payload["userCreated"])
			if user["name"] != expected {
				t.Fatalf("expected %s, got %#v", expected, user["name"])
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for response %d", index)
		}
	}

	select {
	case _, open := <-stream:
		if open {
			t.Fatalf("expected stream to close after source closed")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected stream to close after source closed")
	}
}

func TestExecutorSubscribeRejectsNonSubscriptionOperations(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	if _, err := executor.Subscribe(context.Background(), core.Request{Query: `{ user(id: "1") { id } }`}); err == nil {
		t.Fatalf("expected error for query operation")
	}
}

func TestExecutorSubscribeRequiresSingleRootField(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	_, err := executor.Subscribe(context.Background(), core.Request{
		Query: `subscription { userCreated { id } anotherField }`,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exactly one root field") {
		t.Fatalf("expected single-root-field error, got %v", err)
	}
}

func TestExecuteRejectsSubscriptionOperation(t *testing.T) {
	executor := newTestSubscriptionExecutor(t, nil)

	response := executor.Execute(context.Background(), core.Request{
		Query: `subscription { userCreated { id } }`,
	})
	if len(response.Errors) == 0 {
		t.Fatalf("expected error, got none")
	}
	if !strings.Contains(response.Errors[0].Message, "subscription operations") {
		t.Fatalf("expected subscription rejection, got %q", response.Errors[0].Message)
	}
}

func TestExecutorSubscribeStopsOnContextCancel(t *testing.T) {
	source := make(chan *testUser)
	executor := newTestSubscriptionExecutor(t, source)

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := executor.Subscribe(ctx, core.Request{
		Query: `subscription { userCreated { id name } }`,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	cancel()

	select {
	case _, open := <-stream:
		if open {
			t.Fatalf("expected stream to close on cancel")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected stream to close on cancel")
	}
}
