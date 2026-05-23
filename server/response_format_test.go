package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	subscriptiongraph "github.com/patrickkabwe/grx/examples/subscriptions/graph"
	"github.com/patrickkabwe/grx/exec"
	grxclient "github.com/patrickkabwe/grx/pkg/client"
	grxhttp "github.com/patrickkabwe/grx/pkg/http"
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/schema"
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

type nonNullErrorQuery struct{}

func (nonNullErrorQuery) FailNonNull(context.Context) (string, error) {
	return "", fmt.Errorf("non-null example error")
}

func TestServeHTTPExecutionErrorSerializesTopLevelDataNull(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: nonNullErrorQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	fn := schemaValue.Query.Fields["failNonNull"]
	if fn == nil {
		t.Fatal("expected failNonNull field on query")
	}
	fn.Type = &schema.NonNull{OfType: schemaValue.Types["String"]}

	executor := exec.New(schemaValue, nil)
	tr := grxhttp.New(grxhttp.Config{Path: "/graphql"})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" && r.Method == http.MethodPost {
			tr.Serve(w, r, executor)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)

	payload, err := json.Marshal(grxclient.Request{Query: `query Fail { failNonNull }`})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	httpResp, err := http.Post(ts.URL+"/graphql", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	raw, err := io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body = %s", httpResp.StatusCode, raw)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, raw)
	}
	dataVal, hasData := body["data"]
	if !hasData {
		t.Fatalf("expected data key in execution response, got %#v", body)
	}
	if dataVal != nil {
		t.Fatalf("expected data:null, got %#v", dataVal)
	}
	errs := graphQLErrors(t, body)
	if len(errs) == 0 {
		t.Fatalf("expected field errors, body=%s", raw)
	}
}
