package server

import (
	"strings"
	"testing"

	subscriptiongraph "github.com/patrickkabwe/grx/examples/subscriptions/graph"
	grxclient "github.com/patrickkabwe/grx/pkg/client"
	"github.com/patrickkabwe/grx/pkg/pubsub"
)

func TestServeHTTPReturnsRequestIDInResponseExtensions(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:     subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Middleware: []Middleware{RequestID("X-Request-Id")},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)

	httpResp, resp := execGraphQLHTTP(t, h, &grxclient.Request{Query: "{ __typename }"}, map[string]string{
		"X-Request-Id": "upstream-1",
	})
	if httpResp.StatusCode != 200 {
		t.Fatalf("status = %d", httpResp.StatusCode)
	}
	if resp.Extensions == nil || resp.Extensions["requestId"] != "upstream-1" {
		t.Fatalf("expected requestId in response extensions, got %#v", resp.Extensions)
	}
}

func TestServeHTTPRequestErrorIncludesRequestIDInExtensions(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:     subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Middleware: []Middleware{RequestID("X-Request-Id")},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)

	_, resp := execGraphQLHTTP(t, h, &grxclient.Request{
		Query: "query Broken { user(id: ) { id } }",
	}, map[string]string{"X-Request-Id": "upstream-1"})
	if len(resp.Errors) != 1 {
		t.Fatalf("expected one error, got %#v", resp.Errors)
	}
	if resp.Data != nil {
		t.Fatalf("expected request error to omit data, got %#v", resp.Data)
	}
	if resp.Extensions == nil || resp.Extensions["requestId"] != "upstream-1" {
		t.Fatalf("expected requestId in response extensions, got %#v", resp.Extensions)
	}
}

func TestServeHTTPRequestErrorResponseOmitsDataKeyOnWire(t *testing.T) {
	h := newTestHarness(t)
	_, raw := postGraphQLRaw(t, h, []byte(`{"query":`))
	body := string(raw)
	if strings.Contains(body, `"data"`) {
		t.Fatalf("expected request error JSON to omit data key, got %s", body)
	}
	if !strings.Contains(body, `"classification":"request"`) && !strings.Contains(body, `"classification": "request"`) {
		t.Fatalf("expected request error classification on wire, got %s", body)
	}
}
