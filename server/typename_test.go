package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestServeHTTPExecutesNamedTypenameOnlyQuery(t *testing.T) {
	server := newTestServer(t)
	body, err := json.Marshal(map[string]any{
		"query": "query MyQuery {\n  __typename\n}",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	response := executeGraphQL(t, server, string(body))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	responseBody := graphQLResponseBody(t, response)
	assertNoErrors(t, responseBody)

	data := nestedMap(t, responseBody, "data")
	assertExactKeys(t, data, "__typename")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}
}
