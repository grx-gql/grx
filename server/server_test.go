package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/examples/basic/graph"
	"github.com/patrickkabwe/grx/pkg/pubsub"
)

func TestServeHTTPServesPlaygroundAtConfiguredPath(t *testing.T) {
	server := Server{playgroundPath: "/playground"}
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
		`const endpoint = "/graphql";`,
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
	server := Server{playgroundPath: "/playground"}
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
		server := Server{playgroundPath: "/playground"}
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
	server, err := New(Config{Schema: graph.NewSchema(bus)})
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
