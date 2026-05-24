package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/core"
	subscriptiongraph "github.com/patrickkabwe/grx/examples/subscriptions/graph"
	"github.com/patrickkabwe/grx/pkg/client"
	"github.com/patrickkabwe/grx/pkg/pubsub"
)

func TestClientExecQueryAgainstServer(t *testing.T) {
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
	req := client.Request{
		Query: `query Q($id: String!) { user(id: $id) { id name } }`,
		Variables: map[string]any{
			"id": "user_42",
		},
	}
	gql, err := c.Exec(context.Background(), &req)
	if err != nil {
		t.Fatalf("exec: %v", err)
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

func TestClientExecNilRequest(t *testing.T) {
	c := client.New("http://example.com/graphql")
	_, err := c.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
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
	req := client.Request{Query: `{ __typename }`}
	_, err := c.Exec(context.Background(), &req, client.WithRequestHeader("X-Test", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if saw != "alpha" {
		t.Fatalf("X-Test = %q", saw)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientOptionsAndErrorBranches(t *testing.T) {
	var sawAccept string
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		sawAccept = req.Header.Get("Accept")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":{"ok":true}}`)),
		}, nil
	})}

	c := client.New(" http://example.com/graphql ", client.WithHTTPClient(httpClient), client.WithAccept(""))
	resp, err := c.Exec(context.Background(), &client.Request{Query: `{ ok }`})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected data")
	}
	if sawAccept != client.DefaultAccept {
		t.Fatalf("accept = %q", sawAccept)
	}

	empty := client.New(" ")
	if _, err := empty.PostGraphQL(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected empty URL error")
	}

	badJSONClient := client.New("http://example.com/graphql", client.WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`not-json`))}, nil
	})}))
	if _, err := badJSONClient.Exec(context.Background(), &client.Request{Query: `{ ok }`}); err == nil {
		t.Fatal("expected decode error")
	}

	failingClient := client.New("http://example.com/graphql", client.WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network failed")
	})}))
	if _, err := failingClient.Exec(context.Background(), &client.Request{Query: `{ ok }`}); err == nil {
		t.Fatal("expected transport error")
	}
}
