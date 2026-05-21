package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/core"
	subscriptiongraph "github.com/patrickkabwe/grx/examples/subscriptions/graph"
	"github.com/patrickkabwe/grx/pkg/client"
	"github.com/patrickkabwe/grx/pkg/pubsub"
)

func TestClientExecuteQueryAgainstServer(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })

	srv, err := grx.NewServer(
		grx.WithSchema(subscriptiongraph.New(subscriptiongraph.WithPubSub(bus))),
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	c := client.New(ts.URL + srv.GraphqlPath)
	gql, httpResp, err := c.Execute(context.Background(), client.FromCore(core.Request{
		Query: `query Q($id: String!) { user(id: $id) { id name } }`,
		Variables: map[string]any{
			"id": "user_42",
		},
	}))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("http status = %d", httpResp.StatusCode)
	}
	if len(gql.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", gql.Errors)
	}
	data, ok := gql.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", gql.Data)
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object, got %T", data["user"])
	}
	if user["id"] != "user_42" || user["name"] != "Ada Lovelace" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestClientRequestHeaderOption(t *testing.T) {
	var saw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.Header.Get("X-Test")
		w.Header().Set("Content-Type", core.MediaTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	t.Cleanup(ts.Close)

	c := client.New(ts.URL)
	_, _, err := c.Execute(context.Background(), client.Request{Query: `{ __typename }`},
		client.WithRequestHeader("X-Test", "alpha"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if saw != "alpha" {
		t.Fatalf("X-Test = %q", saw)
	}
}
