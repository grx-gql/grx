package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	subscriptiongraph "github.com/patrickkabwe/grx/examples/subscriptions/graph"
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/plugin"
)

func TestServeHTTPServesPlaygroundAtConfiguredPath(t *testing.T) {
	server := Server{PlaygroundPath: "/playground"}
	request := httptest.NewRequest(http.MethodGet, "/playground", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := response.Body.String()
	expectedValues := []string{
		"<title>GraphiQL</title>",
		"https://esm.sh/graphiql@5.2.2",
		`const httpEndpoint = "/graphql";`,
		`const wsEndpoint = "/graphql";`,
		`createWSClient({ url: subscriptionUrl.toString() })`,
		`isSubscription(request) ? wsSubscribe(request) : httpFetch(request)`,
	}
	for _, expectedValue := range expectedValues {
		if !strings.Contains(body, expectedValue) {
			t.Fatalf("expected playground HTML to contain %q", expectedValue)
		}
	}
}

func TestServeHTTPDoesNotServePlaygroundAtGraphQLPathWhenConfiguredElsewhere(t *testing.T) {
	server := Server{PlaygroundPath: "/playground"}
	request := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestServeHTTPHandlesFavicon(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodHead}
	for _, method := range methods {
		server := Server{PlaygroundPath: "/playground"}
		request := httptest.NewRequest(method, "/favicon.ico", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		if response.Code != http.StatusNoContent {
			t.Fatalf("expected status %d for %s, got %d", http.StatusNoContent, method, response.Code)
		}
	}
}

func TestServeHTTPReturnsBadRequestForMalformedJSON(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}

	body := graphQLResponseBody(t, response)
	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if !strings.Contains(errorValue["message"].(string), "invalid GraphQL JSON body") {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	if _, exists := body["data"]; exists {
		t.Fatalf("expected malformed JSON error to omit data, got %#v", body["data"])
	}
	assertErrorClassification(t, errorValue, "request")
}

func TestServeHTTPReturnsBadRequestForMissingQuery(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"variables":{"id":"user_42"}}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}

	body := graphQLResponseBody(t, response)
	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != "missing GraphQL query" {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	if _, exists := body["data"]; exists {
		t.Fatalf("expected missing query error to omit data, got %#v", body["data"])
	}
	assertErrorClassification(t, errorValue, "request")
}

func TestServeHTTPReturnsRequestErrorLocationsForInvalidQuery(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"query Broken { user(id: ) { id } }"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
	if _, exists := body["data"]; exists {
		t.Fatalf("expected invalid query error to omit data, got %#v", body["data"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}

	errorValue := graphQLError(t, errors, 0)
	assertErrorClassification(t, errorValue, "request")
	assertErrorLocations(t, errorValue, 1, 25)
}

func TestServeHTTPPreservesSelectionOrderInJSONResponse(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(
		t,
		server,
		`{"query":"query Ordered($id: String!) { user(id: $id) { name id email } __typename }","variables":{"id":"user_42"}}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := response.Body.String()
	assertOrderedSubstring(t, body, `"data":{"user":{`, `"__typename":"Query"`)
	assertOrderedSubstring(t, body, `"name":"Ada Lovelace"`, `"id":"user_42"`)
	assertOrderedSubstring(t, body, `"id":"user_42"`, `"email":"ada@example.com"`)
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	server, err := New(Config{Schema: subscriptiongraph.New(subscriptiongraph.WithPubSub(bus))})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func executeGraphQL(t *testing.T, server *Server, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)
	return response
}

func executeGraphQLHeaders(t *testing.T, server *Server, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)
	return response
}

func graphQLResponseBody(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return body
}

func nestedMap(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()

	nested, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be a map, got %T", key, value[key])
	}
	return nested
}

func assertNoErrors(t *testing.T, body map[string]any) {
	t.Helper()

	if errors, exists := body["errors"]; exists {
		t.Fatalf("expected no errors, got %#v", errors)
	}
}

func graphQLErrors(t *testing.T, body map[string]any) []any {
	t.Helper()

	errors, ok := body["errors"].([]any)
	if !ok {
		t.Fatalf("expected errors list, got %T", body["errors"])
	}
	return errors
}

func graphQLError(t *testing.T, errors []any, index int) map[string]any {
	t.Helper()

	errorValue, ok := errors[index].(map[string]any)
	if !ok {
		t.Fatalf("expected error %d to be a map, got %T", index, errors[index])
	}
	return errorValue
}

func assertExactKeys(t *testing.T, value map[string]any, expectedKeys ...string) {
	t.Helper()

	if len(value) != len(expectedKeys) {
		t.Fatalf("expected keys %#v, got %#v", expectedKeys, value)
	}
	for _, key := range expectedKeys {
		if _, exists := value[key]; !exists {
			t.Fatalf("expected key %q in %#v", key, value)
		}
	}
}

func assertErrorClassification(t *testing.T, value map[string]any, expected string) {
	t.Helper()

	extensions, ok := value["extensions"].(map[string]any)
	if !ok {
		t.Fatalf("expected extensions object, got %T", value["extensions"])
	}
	if extensions["classification"] != expected {
		t.Fatalf("expected error classification %q, got %#v", expected, extensions["classification"])
	}
}

func assertErrorLocations(t *testing.T, value map[string]any, expectedLine float64, expectedColumn float64) {
	t.Helper()

	locations, ok := value["locations"].([]any)
	if !ok || len(locations) != 1 {
		t.Fatalf("expected one error location, got %#v", value["locations"])
	}

	location, ok := locations[0].(map[string]any)
	if !ok {
		t.Fatalf("expected location object, got %T", locations[0])
	}
	if location["line"] != expectedLine || location["column"] != expectedColumn {
		t.Fatalf("expected location (%v,%v), got %#v", expectedLine, expectedColumn, location)
	}
}

func assertOrderedSubstring(t *testing.T, body string, earlier string, later string) {
	t.Helper()

	positions := []int{strings.Index(body, earlier), strings.Index(body, later)}
	if slices.Contains(positions, -1) {
		t.Fatalf("expected response body %q to contain %q before %q", body, earlier, later)
	}
	if positions[0] >= positions[1] {
		t.Fatalf("expected %q before %q in %q", earlier, later, body)
	}
}

func readWebSocketHandshakeStatus(t *testing.T, baseURL string, requestPath string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	req := "GET " + requestPath + " HTTP/1.1\r\n" +
		"Host: " + parsed.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Protocol: " + websocket.Subprotocol + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	raw := string(buf[:n])
	return strings.TrimSpace(strings.Split(raw, "\n")[0])
}

func TestSeparateSubscriptionPathWebSocketRouting(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:           subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		SubscriptionPath: "/graphql/ws",
		Transports:       []core.Transport{websocket.New()},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	status := readWebSocketHandshakeStatus(t, ts.URL, "/graphql")
	if strings.Contains(status, "101") {
		t.Fatalf("unexpected 101 switching protocols on /graphql, got %q", status)
	}

	conn := dialWebSocketAt(t, ts.URL, "/graphql/ws", websocket.Subprotocol)
	_ = conn.Close()
}

func TestSubscriptionPathSeparateWithoutStreamingTransportErrors(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	_, err := New(Config{
		Schema:           subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		SubscriptionPath: "/subs",
		Transports:       nil,
	})
	if err == nil {
		t.Fatal("expected error when SubscriptionPath is split but no WebSocket/SSE transport supplied")
	}
}

func TestGraphQLOptionsReturnsAllow(t *testing.T) {
	server := newTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/graphql", nil)

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if got := response.Header().Get("Allow"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Allow = %q", got)
	}
}

func TestDisableIntrospectionRejectsSchemaQuery(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:               subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		DisableIntrospection: true,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	response := executeGraphQL(t, srv, `{"query":"{ __schema { queryType { name } } }"}`)
	body := graphQLResponseBody(t, response)
	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	msg := graphQLError(t, errors, 0)["message"].(string)
	if !strings.Contains(msg, "introspection is disabled") {
		t.Fatalf("unexpected message %q", msg)
	}
}

type blockPlugin struct {
	plugin.Base
}

func (blockPlugin) ValidationStart(ctx context.Context, req core.Request) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestRequestTimeoutStopsSlowValidation(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:         subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Plugins:        []plugin.Plugin{blockPlugin{}},
		RequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	response := executeGraphQL(t, srv, `{"query":"{ __typename }"}`)
	body := graphQLResponseBody(t, response)
	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", body)
	}
	msg := graphQLError(t, errors, 0)["message"].(string)
	if !strings.Contains(msg, "context deadline exceeded") && !strings.Contains(msg, "canceled") {
		t.Fatalf("unexpected error message %q", msg)
	}
}

type ridPlugin struct {
	plugin.Base
	t *testing.T
}

func (p *ridPlugin) ParsingStart(ctx context.Context, req core.Request) error {
	if core.RequestIDFromContext(ctx) == "" {
		p.t.Fatal("expected request id in context")
	}
	return nil
}

func TestRequestIDMiddlewarePropagatesContext(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	p := &ridPlugin{t: t}
	srv, err := New(Config{
		Schema:     subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Plugins:    []plugin.Plugin{p},
		Middleware: []Middleware{RequestID("X-Request-Id")},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	response := executeGraphQLHeaders(t, srv, `{"query":"{ __typename }"}`, map[string]string{
		"X-Request-Id": "upstream-1",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("X-Request-Id"); got != "upstream-1" {
		t.Fatalf("response X-Request-Id = %q", got)
	}
}
