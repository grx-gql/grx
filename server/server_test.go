package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/patrickkabwe/grx/exec"
	grxclient "github.com/patrickkabwe/grx/pkg/client"
	"github.com/patrickkabwe/grx/pkg/pubsub"
	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/schema"
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

// testHarness runs GraphQL requests through pkg/client against a real HTTP server.
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
	plugin.Base
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
		Plugins:            []plugin.Plugin{recorder},
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
