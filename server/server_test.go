package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	grxclient "github.com/grx-gql/grx/client"
	"github.com/grx-gql/grx/core"
	subscriptiongraph "github.com/grx-gql/grx/examples/subscriptions/graph"
	"github.com/grx-gql/grx/exec"
	grxhttp "github.com/grx-gql/grx/http"
	"github.com/grx-gql/grx/memory-pubsub"
	"github.com/grx-gql/grx/middlewares"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
	"github.com/grx-gql/grx/sse"
	"github.com/grx-gql/grx/websocket"
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
	h := newTestHarness(t)
	status, raw := postGraphQLRaw(t, h, []byte(`{"query":`))

	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	body := decodeGraphQLResponseMap(t, raw)
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
	h := newTestHarness(t)
	status, raw := postGraphQLRaw(t, h, []byte(`{"variables":{"id":"user_42"}}`))

	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}

	body := decodeGraphQLResponseMap(t, raw)
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
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query Broken { user(id: ) { id } }",
	}))
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
	h := newTestHarness(t)
	payload, err := json.Marshal(grxclient.Request{
		Query:     "query Ordered($id: String!) { user(id: $id) { name id email } __typename }",
		Variables: map[string]any{"id": "user_42"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	status, raw := postGraphQLRaw(t, h, payload)
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	body := string(raw)
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

// testHarness runs GraphQL requests through the github.com/grx-gql/grx/client package against a real HTTP server.
// The client is created inside [wrapServerInHarness] via grxclient.New.
type testHarness struct {
	*Server
	HTTP   *httptest.Server
	Client *grxclient.Client
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	return wrapServerInHarness(t, newTestServer(t))
}

func wrapServerInHarness(t *testing.T, srv *Server) *testHarness {
	t.Helper()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	c := grxclient.New(ts.URL + srv.GraphqlPath)
	return &testHarness{Server: srv, HTTP: ts, Client: c}
}

func execGraphQL(t *testing.T, h *testHarness, req *grxclient.Request, opts ...grxclient.RequestOption) grxclient.Response {
	t.Helper()
	resp, err := h.Client.Exec(context.Background(), req, opts...)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	return resp
}

func execGraphQLHTTP(t *testing.T, h *testHarness, req *grxclient.Request, headers map[string]string) (*http.Response, grxclient.Response) {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var opts []grxclient.RequestOption
	for key, value := range headers {
		opts = append(opts, grxclient.WithRequestHeader(key, value))
	}
	httpResp, err := h.Client.PostGraphQL(context.Background(), payload, opts...)
	if err != nil {
		t.Fatalf("post graphql: %v", err)
	}
	body, err := io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	var gql grxclient.Response
	if err := json.Unmarshal(body, &gql); err != nil {
		t.Fatalf("decode GraphQL response (http %d): %v", httpResp.StatusCode, err)
	}
	return httpResp, gql
}

func postGraphQLRaw(t *testing.T, h *testHarness, body []byte, opts ...grxclient.RequestOption) (int, []byte) {
	t.Helper()
	httpResp, err := h.Client.PostGraphQL(context.Background(), body, opts...)
	if err != nil {
		t.Fatalf("post graphql: %v", err)
	}
	raw, err := io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return httpResp.StatusCode, raw
}

func responseToMap(t *testing.T, resp grxclient.Response) map[string]any {
	t.Helper()
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return decodeGraphQLResponseMap(t, raw)
}

func decodeGraphQLResponseMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
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
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "{ __schema { queryType { name } } }",
	}))
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
	plugins.Base
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
		Plugins:        []plugins.Plugin{blockPlugin{}},
		RequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{Query: "{ __typename }"}))
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
	plugins.Base
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
		Plugins:    []plugins.Plugin{p},
		Middleware: []Middleware{Middleware(middlewares.RequestID("X-Request-Id"))},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	httpResp, _ := execGraphQLHTTP(t, h, &grxclient.Request{Query: "{ __typename }"}, map[string]string{
		"X-Request-Id": "upstream-1",
	})
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", httpResp.StatusCode)
	}
	if got := httpResp.Header.Get("X-Request-Id"); got != "upstream-1" {
		t.Fatalf("response X-Request-Id = %q", got)
	}
}

type errorRecorder struct {
	plugins.Base
	errors []error
}

func (r *errorRecorder) Error(ctx context.Context, err error) {
	r.errors = append(r.errors, err)
}

func TestSecurityMasksResolverErrorsAndPreservesRawError(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	recorder := &errorRecorder{}
	srv, err := New(Config{
		Schema:             subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Plugins:            []plugins.Plugin{recorder},
		MaskInternalErrors: true,
		ClientErrorMessage: "hidden",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query ExampleError { errorExample }",
	}))
	errors := graphQLErrors(t, body)
	msg := graphQLError(t, errors, 0)["message"].(string)
	if msg != "hidden" {
		t.Fatalf("expected masked error, got %q", msg)
	}
	if len(recorder.errors) != 1 || !strings.Contains(recorder.errors[0].Error(), "example error from basic server") {
		t.Fatalf("expected raw error to be preserved, got %#v", recorder.errors)
	}
}

func TestSecurityOperationAuthorizerRejectsOperation(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema: subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		OperationAuthorizer: func(ctx context.Context, operation exec.OperationContext) error {
			if operation.Kind == core.OperationMutation {
				return errors.New("mutation denied")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: `mutation CreatePost { createPost(input: {title: "x", body: "y", authorId: "user_1"}) { post { id } } }`,
	}))
	errors := graphQLErrors(t, body)
	msg := graphQLError(t, errors, 0)["message"].(string)
	if msg != "mutation denied" {
		t.Fatalf("unexpected error message %q", msg)
	}
}

func TestSecurityFieldAuthorizerRejectsField(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema: subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		FieldAuthorizer: func(ctx context.Context, field exec.FieldAuthorizationContext) error {
			if field.ParentType == "User" && field.FieldName == "email" {
				return errors.New("email denied")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query:     "query GetUser($id: String!) { user(id: $id) { id email } }",
		Variables: map[string]any{"id": "user_42"},
	}))
	errors := graphQLErrors(t, body)
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != "email denied" {
		t.Fatalf("unexpected error: %#v", errorValue)
	}
	path, ok := errorValue["path"].([]any)
	if !ok || len(path) != 2 || path[0] != "user" || path[1] != "email" {
		t.Fatalf("unexpected path: %#v", errorValue["path"])
	}
}

func TestSecurityRateLimiterRejectsOperation(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema: subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		RateLimiter: func(ctx context.Context, operation exec.OperationContext) error {
			return errors.New("rate limited")
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{Query: "{ __typename }"}))
	errors := graphQLErrors(t, body)
	if graphQLError(t, errors, 0)["message"] != "rate limited" {
		t.Fatalf("unexpected errors: %#v", errors)
	}
}

func TestSecurityTrustedDocumentsRejectsUnknownQuery(t *testing.T) {
	query := "{ __typename }"
	sum := sha256.Sum256([]byte(query))
	hash := hex.EncodeToString(sum[:])
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:           subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		TrustedDocuments: map[string]string{hash: query},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	allowed := responseToMap(t, execGraphQL(t, h, &grxclient.Request{Query: "{ __typename }"}))
	assertNoErrors(t, allowed)

	rejected := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: `query Untrusted { user(id: "user_42") { id } }`,
	}))
	body := rejected
	errors := graphQLErrors(t, body)
	if graphQLError(t, errors, 0)["message"] != "operation is not trusted" {
		t.Fatalf("unexpected errors: %#v", errors)
	}
}

func TestSecurityRejectsUnknownVariables(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:                 subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		RejectUnknownVariables: true,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query:     "query GetUser($id: String!) { user(id: $id) { id } }",
		Variables: map[string]any{"id": "user_42", "extra": "x"},
	}))
	errors := graphQLErrors(t, body)
	if graphQLError(t, errors, 0)["message"] != `unknown variable "extra"` {
		t.Fatalf("unexpected errors: %#v", errors)
	}
}

type sdlTestQuery struct{}

func (sdlTestQuery) Hello(ctx context.Context) (string, error) {
	return "hi", nil
}

func TestServeHTTPSchemaSDL(t *testing.T) {
	srv, err := New(Config{
		Schema:        schema.Config{Query: sdlTestQuery{}},
		SchemaSDLPath: "/schema.graphql",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/schema.graphql", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "type Query") || !strings.Contains(body, "hello") {
		t.Fatalf("unexpected SDL body:\n%s", body)
	}
	if !strings.Contains(body, "schema {") {
		t.Fatalf("expected schema block in SDL")
	}
}

type trivialQuery struct{}

func (trivialQuery) Hi() string { return "hi" }

func TestPathRestrictedMatchRequiresCanonicalPath(t *testing.T) {
	inner := &noopTransport{}
	pr := pathRestricted{path: "/graphql", Transport: inner}

	off := httptest.NewRequest(http.MethodGet, "/other", nil)
	if pr.Match(off) {
		t.Fatal("unexpected match for wrong path")
	}

	on := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	if !pr.Match(on) {
		t.Fatal("expected match on configured path")
	}
}

func TestNewErrorsWhenSeparateSubsHasNoStreamingTransports(t *testing.T) {
	_, err := New(Config{
		Schema:           schema.Config{Query: trivialQuery{}},
		GraphQLPath:      "/gq",
		SubscriptionPath: "/subs",
		Transports: []core.Transport{
			customHTTPTransport{},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "SubscriptionPath differs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDefaultPathsAndSchemaSDL(t *testing.T) {
	srv, err := New(Config{
		Schema:         schema.Config{Query: trivialQuery{}},
		GraphQLPath:    "query",
		PlaygroundPath: "",
		SchemaSDLPath:  "schema.graphql",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if srv.GraphqlPath != "/query" || srv.PlaygroundPath != "" || srv.schemaSDLPath != "/schema.graphql" {
		t.Fatalf("paths = graphql:%q playground:%q sdl:%q", srv.GraphqlPath, srv.PlaygroundPath, srv.schemaSDLPath)
	}

	req := httptest.NewRequest(http.MethodGet, "/schema.graphql", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "schema") {
		t.Fatalf("SDL response status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNewCoversOptionalConfigBranches(t *testing.T) {
	_, err := New(Config{
		Schema:                 schema.Config{Query: trivialQuery{}},
		SubscriptionPath:       "/events",
		Transports:             []core.Transport{sse.New()},
		DisableIntrospection:   true,
		MaskInternalErrors:     true,
		ClientErrorMessage:     "masked",
		RejectUnknownVariables: true,
		MaxSelectionDepth:      10,
		MaxSelectionCount:      10,
		MaxAliasCount:          10,
		MaxRootFieldCount:      10,
		DocumentCacheSize:      2,
		MaxHTTPRequestBytes:    1024,
		EnableResponseGzip:     true,
		PersistedQueries:       map[string]string{"hash": "{ hi }"},
		RequirePersistedQuery:  true,
		StrictPersistedQueries: true,
		MaxVariableBytes:       128,
		OperationAuthorizer: func(context.Context, exec.OperationContext) error {
			return nil
		},
		FieldAuthorizer: func(context.Context, exec.FieldAuthorizationContext) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new server with options: %v", err)
	}
}

func TestServerMaxSelectionDepthRejectsDeepQuery(t *testing.T) {
	srv, err := New(Config{
		Schema:            schema.Config{Query: depthLimitQuery{}},
		MaxSelectionDepth: 1,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)

	status, raw := postGraphQLRaw(t, h, []byte(`{"query":"{ node { child { value } } }"}`))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	body := decodeGraphQLResponseMap(t, raw)
	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("errors = %#v", errors)
	}
	message := graphQLError(t, errors, 0)["message"].(string)
	if !strings.Contains(message, "selection depth exceeds limit") {
		t.Fatalf("message = %q", message)
	}
}

func TestServeHTTPInternalBranches(t *testing.T) {
	srv, err := New(Config{Schema: schema.Config{Query: trivialQuery{}}})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodHead, faviconPath, http.StatusNoContent},
		{http.MethodOptions, "/graphql", http.StatusNoContent},
		{http.MethodGet, "/missing", http.StatusNotFound},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

type noopTransport struct{}

func (noopTransport) Match(*http.Request) bool { return true }

func (noopTransport) Serve(http.ResponseWriter, *http.Request, core.Executor) {}

// customHTTPTransport is not *websocket.WebSocketTransport or *sse.Transport.

type customHTTPTransport struct{}

func (customHTTPTransport) Match(*http.Request) bool { return true }

func (customHTTPTransport) Serve(http.ResponseWriter, *http.Request, core.Executor) {}

type failingBodyWriter struct {
	hdr  http.Header
	code int
}

func (w *failingBodyWriter) Header() http.Header {
	if w.hdr == nil {
		w.hdr = make(http.Header)
	}
	return w.hdr
}

func (w *failingBodyWriter) WriteHeader(code int) { w.code = code }

func (w *failingBodyWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestServePlaygroundReturns500WhenWriteFails(t *testing.T) {
	w := &failingBodyWriter{}
	servePlayground(w, "/graphql", "/graphql")
	if w.code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.code)
	}
}

func TestNormalizePathBranches(t *testing.T) {
	cases := map[string]string{
		"":          "/default",
		"graphql":   "/graphql",
		"/graphql/": "/graphql/",
		"/":         "/",
	}
	for input, want := range cases {
		if got := normalizePath(input, "/default"); got != want {
			t.Fatalf("normalizePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWithRequestDeadlineBranches(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/", nil)
	srv := &Server{}
	without, cancel := srv.withRequestDeadline(base)
	defer cancel()
	if without.Context() != base.Context() {
		t.Fatal("zero timeout should keep original request")
	}

	srv.requestTimeout = time.Minute
	with, cancel := srv.withRequestDeadline(base)
	defer cancel()
	if _, ok := with.Context().Deadline(); !ok {
		t.Fatal("expected deadline")
	}
}

func TestServeHTTPExecutesQuery(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query GetUser($id: String!) { __typename user(id: $id) { __typename id name email } }",
		Variables: map[string]any{
			"id": "user_42",
		},
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}

	user := nestedMap(t, data, "user")
	assertExactKeys(t, user, "__typename", "id", "name", "email")
	if user["__typename"] != "User" {
		t.Fatalf("expected user typename User, got %#v", user["__typename"])
	}
	if user["id"] != "user_42" {
		t.Fatalf("expected id user_42, got %#v", user["id"])
	}
	if user["name"] != "Ada Lovelace" {
		t.Fatalf("expected name Ada Lovelace, got %#v", user["name"])
	}
	if user["email"] != "ada@example.com" {
		t.Fatalf("expected email ada@example.com, got %#v", user["email"])
	}
}

func TestServeHTTPReturnsQueryValidationErrorForUnknownField(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query GetUser($id: String!) { __typename missing user(id: $id) { id name } }",
		Variables: map[string]any{
			"id": "user_42",
		},
	}))
	if _, exists := body["data"]; exists {
		t.Fatalf("expected validation error to omit data, got %#v", body["data"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != `Cannot query field "missing" on type "Query".` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	code, _ := errorValue["extensions"].(map[string]any)["code"].(string)
	if code != "GRAPHQL_VALIDATION_FAILED" {
		t.Fatalf("expected validation error code, got %#v", errorValue["extensions"])
	}
	assertErrorLocations(t, errorValue, 1, 42)
}

func TestServeHTTPReturnsExampleFieldErrorFromBasicSchema(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query ExampleError { errorExample }",
	}))
	dataRaw, ok := body["data"]
	if !ok {
		t.Fatalf("expected data key with null value, got %#v", body)
	}
	if dataRaw != nil {
		t.Fatalf("expected top-level data null, got %#v", dataRaw)
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}

	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != "example error from basic server" {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	assertErrorClassification(t, errorValue, "field")
	assertErrorLocations(t, errorValue, 1, 22)
	path, ok := errorValue["path"].([]any)
	if !ok || len(path) != 1 || path[0] != "errorExample" {
		t.Fatalf("expected errorExample path, got %#v", errorValue["path"])
	}
}

func TestServeHTTPExecutesMutation(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "mutation CreateUser($input: UserCreateInput!) { __typename createUser(input: $input) { __typename user { __typename id name email } } }",
		Variables: map[string]any{
			"input": map[string]any{
				"name":  "Grace Hopper",
				"email": "grace@example.com",
			},
		},
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Mutation" {
		t.Fatalf("expected root typename Mutation, got %#v", data["__typename"])
	}

	createUser := nestedMap(t, data, "createUser")
	assertExactKeys(t, createUser, "__typename", "user")
	if createUser["__typename"] != "UserCreatePayload" {
		t.Fatalf("expected payload typename UserCreatePayload, got %#v", createUser["__typename"])
	}

	user := nestedMap(t, createUser, "user")
	assertExactKeys(t, user, "__typename", "id", "name", "email")
	if user["__typename"] != "User" {
		t.Fatalf("expected user typename User, got %#v", user["__typename"])
	}
	if user["id"] != "user_1" {
		t.Fatalf("expected id user_1, got %#v", user["id"])
	}
	if user["name"] != "Grace Hopper" {
		t.Fatalf("expected name Grace Hopper, got %#v", user["name"])
	}
	if user["email"] != "grace@example.com" {
		t.Fatalf("expected email grace@example.com, got %#v", user["email"])
	}
}

func TestServeHTTPExecutesMutationWithInlineInputObject(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: `mutation MyMutation { __typename createUser(input: {email: "test@gmail.com", name: "test"}) { user { email } } }`,
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Mutation" {
		t.Fatalf("expected root typename Mutation, got %#v", data["__typename"])
	}
	createUser := nestedMap(t, data, "createUser")
	user := nestedMap(t, createUser, "user")
	if user["email"] != "test@gmail.com" {
		t.Fatalf("expected email test@gmail.com, got %#v", user["email"])
	}
}

func TestServeHTTPReturnsMutationValidationErrorForUnknownField(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "mutation CreateUser($input: UserCreateInput!) { __typename missing createUser(input: $input) { user { id name } } }",
		Variables: map[string]any{
			"input": map[string]any{
				"name": "Grace Hopper",
			},
		},
	}))
	if _, exists := body["data"]; exists {
		t.Fatalf("expected validation error to omit data, got %#v", body["data"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != `Cannot query field "missing" on type "Mutation".` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	assertErrorLocations(t, errorValue, 1, 60)
}

func TestServeHTTPReturnsRequestIDInResponseExtensions(t *testing.T) {
	bus := pubsub.NewMemory()
	t.Cleanup(func() { _ = bus.Close() })
	srv, err := New(Config{
		Schema:     subscriptiongraph.New(subscriptiongraph.WithPubSub(bus)),
		Middleware: []Middleware{Middleware(middlewares.RequestID("X-Request-Id"))},
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
		Middleware: []Middleware{Middleware(middlewares.RequestID("X-Request-Id"))},
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

type streamingExecutor struct {
	t       *testing.T
	source  chan core.Response
	subErr  error
	gotReq  core.Request
	subOnce sync.Once
}

func newStreamingExecutor(t *testing.T) *streamingExecutor {
	return &streamingExecutor{t: t, source: make(chan core.Response, 8)}
}

func (e *streamingExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	return core.Response{Data: map[string]any{"executed": true}}
}

func (e *streamingExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	switch {
	case strings.HasPrefix(strings.TrimSpace(req.Query), "subscription"):
		return core.OperationSubscription, nil
	case strings.HasPrefix(strings.TrimSpace(req.Query), "mutation"):
		return core.OperationMutation, nil
	default:
		return core.OperationQuery, nil
	}
}

func (e *streamingExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	e.subOnce.Do(func() { e.gotReq = req })
	if e.subErr != nil {
		return nil, e.subErr
	}
	out := make(chan core.Response)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case res, open := <-e.source:
				if !open {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}
	}()
	return out, nil
}

func newSubscriptionServer(t *testing.T, transports ...core.Transport) (*Server, *streamingExecutor) {
	t.Helper()
	executor := newStreamingExecutor(t)
	return &Server{
		executor:         executor,
		PlaygroundPath:   "/playground",
		GraphqlPath:      "/graphql",
		SubscriptionPath: "/graphql",
		separateSubs:     false,
		mainChain:        transports,
		subChain:         nil,
	}, executor
}

func TestSSEStreamsSubscriptionPayloads(t *testing.T) {
	srv, executor := newSubscriptionServer(t, sse.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	body := strings.NewReader(`{"query":"subscription { userCreated { id } }"}`)
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/graphql", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch SSE request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", got)
	}

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "2"}}}
	close(executor.source)

	events := readSSEEvents(t, response.Body, 3)

	if events[0].Event != "next" {
		t.Fatalf("expected first event next, got %q", events[0].Event)
	}
	first := decodeJSON(t, events[0].Data)
	if id := nestedValue(t, first, "data", "userCreated", "id"); id != "1" {
		t.Fatalf("expected first id 1, got %#v", id)
	}

	second := decodeJSON(t, events[1].Data)
	if id := nestedValue(t, second, "data", "userCreated", "id"); id != "2" {
		t.Fatalf("expected second id 2, got %#v", id)
	}

	if events[2].Event != "complete" {
		t.Fatalf("expected complete event, got %q", events[2].Event)
	}
}

func TestSSESupportsGetWithQueryParameters(t *testing.T) {
	srv, executor := newSubscriptionServer(t, sse.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	values := url.Values{
		"query":     {`subscription { userCreated { id } }`},
		"variables": {`{"limit":3}`},
	}
	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch SSE GET: %v", err)
	}
	defer response.Body.Close()

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	close(executor.source)

	events := readSSEEvents(t, response.Body, 2)

	if events[0].Event != "next" || events[1].Event != "complete" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if executor.gotReq.Query != `subscription { userCreated { id } }` {
		t.Fatalf("unexpected query in executor: %q", executor.gotReq.Query)
	}
	if executor.gotReq.Variables["limit"] != float64(3) {
		t.Fatalf("expected variables limit 3, got %#v", executor.gotReq.Variables)
	}
}

func TestSSEDisabledByDefault(t *testing.T) {
	srv, _ := newSubscriptionServer(t)
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when SSE disabled, got %d", response.StatusCode)
	}
}

func TestWebSocketStreamsSubscriptionPayloads(t *testing.T) {
	srv, executor := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	if got := readServerJSON(t, conn); got["type"] != "connection_ack" {
		t.Fatalf("expected connection_ack, got %#v", got)
	}

	writeClientText(t, conn, `{"id":"sub-1","type":"subscribe","payload":{"query":"subscription { userCreated { id } }"}}`)

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "2"}}}

	first := readServerJSON(t, conn)
	if first["type"] != "next" || first["id"] != "sub-1" {
		t.Fatalf("expected next message, got %#v", first)
	}
	if id := nestedValue(t, first, "payload", "data", "userCreated", "id"); id != "1" {
		t.Fatalf("expected first id 1, got %#v", id)
	}

	second := readServerJSON(t, conn)
	if id := nestedValue(t, second, "payload", "data", "userCreated", "id"); id != "2" {
		t.Fatalf("expected second id 2, got %#v", id)
	}

	close(executor.source)

	complete := readServerJSON(t, conn)
	if complete["type"] != "complete" || complete["id"] != "sub-1" {
		t.Fatalf("expected complete, got %#v", complete)
	}
}

func TestWebSocketRejectsSubscribeBeforeInit(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"id":"1","type":"subscribe","payload":{"query":"subscription { userCreated { id } }"}}`)

	opcode, _, _, err := readServerFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close frame, got err=%v", err)
	}
	if opcode != websocket.OpcodeClose {
		t.Fatalf("expected close opcode, got %d", opcode)
	}
}

func TestWebSocketRespondsToPing(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	_ = readServerJSON(t, conn)

	writeClientText(t, conn, `{"type":"ping"}`)
	if got := readServerJSON(t, conn); got["type"] != "pong" {
		t.Fatalf("expected pong, got %#v", got)
	}
}

func TestWebSocketDisabledByDefault(t *testing.T) {
	srv, _ := newSubscriptionServer(t)
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/graphql", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	request.Header.Set("Sec-WebSocket-Protocol", websocket.Subprotocol)

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("dispatch request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when WS disabled, got %d", response.StatusCode)
	}
}

func TestWebSocketConnectionInitTimeoutCloses4408(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New(websocket.Config{
		ConnectionInitTimeout: 100 * time.Millisecond,
	}))
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	opcode, payload, _, err := readServerFrame(t, conn)
	if err != nil {
		t.Fatalf("expected close frame, got err=%v", err)
	}
	if opcode != websocket.OpcodeClose {
		t.Fatalf("expected close opcode, got 0x%X", opcode)
	}
	if len(payload) < 2 {
		t.Fatalf("close frame missing status code")
	}
	code := binary.BigEndian.Uint16(payload[:2])
	if code != 4408 {
		t.Fatalf("expected close code 4408, got %d", code)
	}
}

func TestWebSocketOnConnectAuthorizesAndRejects(t *testing.T) {
	authorize := func(ctx context.Context, payload json.RawMessage) (context.Context, json.RawMessage, error) {
		var creds struct {
			Token string `json:"token"`
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &creds)
		}
		if creds.Token != "letmein" {
			return nil, nil, errorString("invalid token")
		}
		return ctx, json.RawMessage(`{"user":"alice"}`), nil
	}

	srv, _ := newSubscriptionServer(t, websocket.New(websocket.Config{
		OnConnect: authorize,
	}))
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	t.Run("rejects bad token", func(t *testing.T) {
		conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
		defer conn.Close()

		writeClientText(t, conn, `{"type":"connection_init","payload":{"token":"nope"}}`)

		opcode, payload, _, err := readServerFrame(t, conn)
		if err != nil {
			t.Fatalf("expected close, got err=%v", err)
		}
		if opcode != websocket.OpcodeClose {
			t.Fatalf("expected close opcode, got 0x%X", opcode)
		}
		if code := binary.BigEndian.Uint16(payload[:2]); code != 4403 {
			t.Fatalf("expected close code 4403, got %d", code)
		}
	})

	t.Run("accepts good token and emits ack payload", func(t *testing.T) {
		conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
		defer conn.Close()

		writeClientText(t, conn, `{"type":"connection_init","payload":{"token":"letmein"}}`)

		ack := readServerJSON(t, conn)
		if ack["type"] != "connection_ack" {
			t.Fatalf("expected connection_ack, got %#v", ack)
		}
		ackPayload, ok := ack["payload"].(map[string]any)
		if !ok {
			t.Fatalf("expected ack payload, got %#v", ack)
		}
		if ackPayload["user"] != "alice" {
			t.Fatalf("expected alice, got %#v", ackPayload)
		}
	})
}

func TestWebSocketRejectsSubscribeWithEmptyQuery(t *testing.T) {
	srv, _ := newSubscriptionServer(t, websocket.New())
	httpServer := httptest.NewServer(srv)
	defer httpServer.Close()

	conn := dialWebSocket(t, httpServer.URL, websocket.Subprotocol)
	defer conn.Close()

	writeClientText(t, conn, `{"type":"connection_init"}`)
	_ = readServerJSON(t, conn)

	writeClientText(t, conn, `{"id":"sub-1","type":"subscribe","payload":{"query":""}}`)

	first := readServerJSON(t, conn)
	if first["type"] != "error" {
		t.Fatalf("expected error message, got %#v", first)
	}
	second := readServerJSON(t, conn)
	if second["type"] != "complete" {
		t.Fatalf("expected complete, got %#v", second)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestNewServerBuildsSubscriptionRoot(t *testing.T) {
	subscription := schemaSubscriptionRoot{}
	srv, err := New(Config{
		Schema: schema.Config{
			Query:        schemaQueryRoot{},
			Subscription: subscription,
		},
		Plugins:    []plugins.Plugin{},
		Transports: []core.Transport{websocket.New(), sse.New()},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	// The server appends a default HTTP+JSON transport to the chain, so
	// the user-supplied two transports become three after construction.
	if len(srv.mainChain) != 3 {
		t.Fatalf("expected 3 transports (websocket, sse, default http), got %d", len(srv.mainChain))
	}
}

func TestNewAppliesRequestLimitsToSSETransport(t *testing.T) {
	srv, err := New(Config{
		Schema: schema.Config{
			Query:        schemaQueryRoot{},
			Subscription: schemaSubscriptionRoot{},
		},
		Transports:          []core.Transport{sse.New()},
		MaxHTTPRequestBytes: 12,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	h := wrapServerInHarness(t, srv)

	req, err := http.NewRequest(http.MethodPost, h.HTTP.URL+srv.GraphqlPath, strings.NewReader(`{"query":"subscription { hello }"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post sse: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

type schemaQueryRoot struct{}

func (schemaQueryRoot) Hello() string { return "hi" }

type depthLimitQuery struct{}

func (depthLimitQuery) Node() depthLimitNode {
	return depthLimitNode{Child: &depthLimitNode{Value: "ok"}, Value: "ok"}
}

type depthLimitNode struct {
	Child *depthLimitNode
	Value string
}

type schemaSubscriptionRoot struct{}

func (schemaSubscriptionRoot) Hello(ctx context.Context) (<-chan string, error) {
	out := make(chan string)
	close(out)
	return out, nil
}

// --- helpers -----------------------------------------------------------

type sseEvent struct {
	Event string
	Data  string
}

func readSSEEvents(t *testing.T, body io.Reader, count int) []sseEvent {
	t.Helper()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	events := make([]sseEvent, 0, count)
	current := sseEvent{}
	deadline := time.Now().Add(5 * time.Second)
	for len(events) < count {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for SSE events; got %#v", events)
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan SSE: %v", err)
			}
			break
		}
		line := scanner.Text()
		switch {
		case line == "":
			if current.Event != "" || current.Data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		case strings.HasPrefix(line, "event: "):
			current.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			value := strings.TrimPrefix(line, "data: ")
			if current.Data == "" {
				current.Data = value
			} else {
				current.Data += "\n" + value
			}
		}
	}
	return events
}

func decodeJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode json %q: %v", raw, err)
	}
	return value
}

func nestedValue(t *testing.T, value map[string]any, keys ...string) any {
	t.Helper()
	current := any(value)
	for _, key := range keys {
		mapValue, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected map at key path %v, got %T", keys, current)
		}
		current = mapValue[key]
	}
	return current
}

func dialWebSocket(t *testing.T, baseURL string, subprotocol string) net.Conn {
	return dialWebSocketAt(t, baseURL, "/graphql", subprotocol)
}

func dialWebSocketAt(t *testing.T, baseURL string, requestPath string, subprotocol string) net.Conn {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("random key: %v", err)
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	request := strings.Join([]string{
		"GET " + requestPath + " HTTP/1.1",
		"Host: " + parsed.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Version: 13",
		"Sec-WebSocket-Key: " + key,
		"Sec-WebSocket-Protocol: " + subprotocol,
		"", "",
	}, "\r\n")

	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("send handshake: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		t.Fatalf("expected 101 switching protocols, got %q", statusLine)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	wrapped := &readerConn{Conn: conn, reader: reader}
	return wrapped
}

type readerConn struct {
	net.Conn
	reader *bufio.Reader
}

func (r *readerConn) Read(b []byte) (int, error) {
	return r.reader.Read(b)
}

func writeClientText(t *testing.T, conn net.Conn, payload string) {
	t.Helper()

	body := []byte(payload)
	header := []byte{0x81} // FIN + text
	length := len(body)
	maskBit := byte(0x80)
	switch {
	case length < 126:
		header = append(header, maskBit|byte(length))
	case length <= 0xFFFF:
		header = append(header, maskBit|126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(length))
	default:
		header = append(header, maskBit|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(length))
	}
	mask := []byte{0xA1, 0xB2, 0xC3, 0xD4}
	frame := append([]byte{}, header...)
	frame = append(frame, mask...)
	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = body[i] ^ mask[i%4]
	}
	frame = append(frame, masked...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func readServerFrame(t *testing.T, conn net.Conn) (opcode byte, payload []byte, fin bool, err error) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return 0, nil, false, err
	}
	fin = buffer[0]&0x80 != 0
	opcode = buffer[0] & 0x0F
	masked := buffer[1]&0x80 != 0
	length := uint64(buffer[1] & 0x7F)
	switch length {
	case 126:
		extra := make([]byte, 2)
		if _, err := io.ReadFull(conn, extra); err != nil {
			return 0, nil, false, err
		}
		length = uint64(binary.BigEndian.Uint16(extra))
	case 127:
		extra := make([]byte, 8)
		if _, err := io.ReadFull(conn, extra); err != nil {
			return 0, nil, false, err
		}
		length = binary.BigEndian.Uint64(extra)
	}
	if masked {
		mask := make([]byte, 4)
		if _, err := io.ReadFull(conn, mask); err != nil {
			return 0, nil, false, err
		}
		payload = make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false, err
		}
		for index := range payload {
			payload[index] ^= mask[index%4]
		}
		return opcode, payload, fin, nil
	}
	payload = make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, false, err
		}
	}
	return opcode, payload, fin, nil
}

func readServerJSON(t *testing.T, conn net.Conn) map[string]any {
	t.Helper()
	for {
		opcode, payload, _, err := readServerFrame(t, conn)
		if err != nil {
			t.Fatalf("read server frame: %v", err)
		}
		if opcode == websocket.OpcodeText {
			var message map[string]any
			if err := json.Unmarshal(payload, &message); err != nil {
				t.Fatalf("decode json %q: %v", string(payload), err)
			}
			return message
		}
		if opcode == websocket.OpcodeClose {
			t.Fatalf("server closed connection: %s", string(payload))
		}
	}
}

func TestServeHTTPExecutesNamedTypenameOnlyQuery(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query MyQuery {\n  __typename\n}",
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	assertExactKeys(t, data, "__typename")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}
}
